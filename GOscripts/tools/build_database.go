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

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/models"

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

// CleanSpell 定义了用于生成干净的 spells.json 的数据结构
type CleanSpell struct {
	SpellID     string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sprite      string `json:"sprite"`
	Type        int    `json:"type"`
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
			translations[key] = value
		}
	}
	fmt.Println("成功加载", len(translations), "条翻译。")
	return translations, nil
}

// preprocessSpells 是核心处理函数
func preprocessSpells() ([]CleanSpell, []models.Spell, error) {
	translations, err := loadTranslations("./assets/data/translations/common.csv")
	if err != nil {
		return nil, nil, err
	}

	rawFile, err := os.ReadFile("./assets/spells_raw.json")
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取 spells_raw.json: %w", err)
	}

	var rawSpells []RawSpell
	if err := json.Unmarshal(rawFile, &rawSpells); err != nil {
		return nil, nil, fmt.Errorf("解析 spells_raw.json 失败: %w", err)
	}
	fmt.Println("成功读取", len(rawSpells), "条原始法术数据。")

	var cleanSpells []CleanSpell
	var dbSpells []models.Spell
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

		cleanSpell := CleanSpell{
			SpellID:     rawSpell.ID,
			Name:        nameCN,
			Description: descCN,
			Sprite:      spriteFileName,
			Type:        rawSpell.Type,
		}
		cleanSpells = append(cleanSpells, cleanSpell)

		dbSpell := models.Spell{
			SpellID:     rawSpell.ID,
			Name:        nameCN,
			Description: descCN,
			Sprite:      spriteFileName,
			Type:        rawSpell.Type,
			Score:       1500,
			Total:       0,
			Win:         0,
		}
		dbSpells = append(dbSpells, dbSpell)
	}

	finalJSON, err := json.MarshalIndent(cleanSpells, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("无法序列化最终的spells.json: %w", err)
	}
	err = os.WriteFile("./assets/spells.json", finalJSON, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("无法写入 spells.json: %w", err)
	}
	fmt.Println("成功生成干净的 spells.json 文件。")

	return cleanSpells, dbSpells, nil
}

// buildDatabase 使用处理好的法术数据填充数据库
func buildDatabase() {
	fmt.Println("开始构建数据库...")
	_, dbSpells, err := preprocessSpells()
	if err != nil {
		log.Fatalf("预处理数据失败: %v", err)
	}

	database.InitDB()
	database.DB.AutoMigrate(&models.Spell{})
	database.DB.Exec("DELETE FROM spells")

	result := database.DB.Create(&dbSpells)
	if result.Error != nil {
		log.Fatalf("向数据库插入数据失败: %v", result.Error)
	}

	fmt.Printf("数据库构建完成！成功插入 %d 条法术数据。\n", result.RowsAffected)
}

// cleanDatabase 重置所有法术的分数和战绩
func cleanDatabase() {
	fmt.Println("开始清理数据库分数...")
	database.InitDB()

	// *** 修改部分开始 ***
	// GORM 默认开启了安全模式，不允许在没有 WHERE 条件的情况下进行全局更新。
	// 我们需要通过 .Session(&gorm.Session{AllowGlobalUpdate: true}) 显式地允许全局更新来重置所有记录。
	result := database.DB.Model(&models.Spell{}).
		Session(&gorm.Session{AllowGlobalUpdate: true}).
		Updates(map[string]interface{}{
			"score": 1500,
			"total": 0,
			"win":   0,
		})
	// *** 修改部分结束 ***

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
