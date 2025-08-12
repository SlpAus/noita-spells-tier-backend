package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"

	"gorm.io/gorm"
)

// RawSpell 对应 spells_raw.json 中的原始数据结构
type RawSpell struct {
	Sprite      string `json:"sprite"`
	Name        string `json:"name"`
	Type        int    `json:"type"`
	ID          string `json:"id"`
	Description string `json:"description"`
}

// loadTranslations 从 common.csv 加载翻译数据
func loadTranslations(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开翻译文件 %s: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("无法读取CSV数据: %w", err)
	}

	translations := make(map[string]string)
	for _, record := range records {
		if len(record) >= 10 {
			key := record[0]
			value := record[9] // 第10列是中文翻译
			if value == "" {
				value = record[1] // 备选第2列的英文文本
			}
			translations[key] = value
		}
	}
	fmt.Println("成功加载", len(translations), "条翻译。")
	return translations, nil
}

// preprocessSpells 是核心处理函数
func preprocessSpells() ([]spell.Spell, error) {
	translations, err := loadTranslations("./assets/data/translations/common.csv")
	if err != nil {
		return nil, err
	}

	rawFile, err := os.ReadFile("./assets/spells_raw.json")
	if err != nil {
		return nil, fmt.Errorf("无法读取 spells_raw.json: %w", err)
	}

	var rawSpells []RawSpell
	if err := json.Unmarshal(rawFile, &rawSpells); err != nil {
		return nil, fmt.Errorf("解析 spells_raw.json 失败: %w", err)
	}
	fmt.Println("成功读取", len(rawSpells), "条原始法术数据。")

	var dbSpells []spell.Spell
	for _, rawSpell := range rawSpells {
		nameKey := strings.TrimPrefix(rawSpell.Name, "$")
		descKey := strings.TrimPrefix(rawSpell.Description, "$")

		nameCN, ok := translations[nameKey]
		if !ok {
			nameCN = nameKey
		}
		descCN, ok := translations[descKey]
		if !ok {
			descCN = descKey
		}

		spriteFileName := filepath.Base(rawSpell.Sprite)

		dbSpell := spell.Spell{
			SpellID:     rawSpell.ID,
			Name:        nameCN,
			Description: descCN,
			Sprite:      spriteFileName,
			Type:        rawSpell.Type,
			Score:       1500,
			Total:       0,
			Win:         0,
			RankScore:   0.5,
		}
		dbSpells = append(dbSpells, dbSpell)
	}

	return dbSpells, nil
}

func dropUserTablesExcept(db *gorm.DB, tablesToKeep []string) error {
	// 1. 获取数据库中所有表的名称
	var tableNames []string
	// 避开SQLite的元数据
	err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';").Scan(&tableNames).Error
	if err != nil {
		return fmt.Errorf("无法获取表列表: %w", err)
	}

	// 2. 创建一个map以便快速查找需要保留的表
	keepMap := make(map[string]bool)
	for _, name := range tablesToKeep {
		keepMap[name] = true
	}

	// 3. 遍历所有表，如果不在保留列表中，则使用Migrator删除它
	for _, tableName := range tableNames {
		if !keepMap[tableName] {
			fmt.Printf("正在删除表: %s\n", tableName)
			if err := db.Migrator().DropTable(tableName); err != nil {
				return fmt.Errorf("删除表 %s 失败: %w", tableName, err)
			}
		}
	}

	return nil
}

func parseTableName(db *gorm.DB, model interface{}) (string, error) {
	// 1. 输入校验
	if model == nil {
		return "", fmt.Errorf("模型不能为nil")
	}
	if db == nil {
		return "", fmt.Errorf("数据库句柄不能为nil")
	}

	// 2. 创建一个临时的gorm.Statement
	// Statement是GORM用于构建SQL的内部对象，包含了schema解析逻辑。
	stmt := &gorm.Statement{DB: db}

	// 3. 调用stmt.Parse()来解析模型
	// 这个方法会填充stmt.Schema字段
	if err := stmt.Parse(model); err != nil {
		return "", fmt.Errorf("解析模型失败: %w", err)
	}

	// 4. 从解析后的Schema中安全地获取表名
	return stmt.Schema.Table, nil
}

// buildDatabase 使用处理好的法术数据填充数据库
func buildDatabase() {
	fmt.Println("开始构建数据库...")
	dbSpells, err := preprocessSpells()
	if err != nil {
		log.Fatalf("预处理数据失败: %v", err)
	}

	database.InitDB()
	if err := dropUserTablesExcept(database.DB, []string{}); err != nil {
		log.Fatalf("删除旧表失败: %v", err)
	}
	fmt.Println("所有旧表已删除。")
	database.DB.AutoMigrate(&spell.Spell{})

	result := database.DB.Create(&dbSpells)
	if result.Error != nil {
		log.Fatalf("向数据库插入数据失败: %v", result.Error)
	}

	fmt.Printf("数据库构建完成！成功插入 %d 条法术数据。\n", result.RowsAffected)
}

// cleanDatabase 重置所有法术的分数和战绩
func cleanDatabase() {
	fmt.Println("开始重置数据库...")
	database.InitDB()

	tableName, err := parseTableName(database.DB, &spell.Spell{})
	if err != nil {
		log.Fatalf("无法解析保留的数据表名: %v", err)
	}
	if err := dropUserTablesExcept(database.DB, []string{tableName}); err != nil {
		log.Fatalf("删除旧表失败: %v", err)
	}
	fmt.Printf("除 %s 外的的旧表已删除。\n", tableName)

	database.DB.AutoMigrate(&spell.Spell{})
	// GORM 默认开启了安全模式，不允许在没有 WHERE 条件的情况下进行全局更新。
	// 我们需要通过 .Session(&gorm.Session{AllowGlobalUpdate: true}) 显式地允许全局更新来重置所有记录。
	result := database.DB.Model(&spell.Spell{}).
		Session(&gorm.Session{AllowGlobalUpdate: true}).
		Updates(map[string]interface{}{
			"score":      1500,
			"total":      0,
			"win":        0,
			"rank_score": 0.5,
		})

	if result.Error != nil {
		log.Fatalf("清理数据库失败: %v", result.Error)
	}
	fmt.Printf("数据库清理完成！重置了 %d 条记录的分数。\n", result.RowsAffected)
}

func main() {
	task := flag.String("task", "build", "要执行的任务: 'build' (构建并填充数据库) 或 'clean' (重置分数)")
	flag.Parse()

	switch *task {
	case "build":
		buildDatabase()
	case "clean":
		cleanDatabase()
	default:
		fmt.Println("未知的任务:", *task)
		fmt.Println("可用任务: 'build', 'clean'")
		os.Exit(1)
	}
}
