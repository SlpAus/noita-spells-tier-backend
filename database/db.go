package database

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("ranking.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	// 自动迁移（创建表）
	err = DB.AutoMigrate(&models.Item{}, &models.Pool{}, &models.Vote{}, &models.LastGetItem{}, &models.Pvote{})
	if err != nil {
		log.Fatal("数据表迁移失败:", err)
	}

	// 定时备份
	go func() {
		for {
			backupDB()
			time.Sleep(1 * time.Hour)
		}
	}()

	fmt.Println("SQLite 数据库初始化完成！")
}

func backupDB() {
	timestamp := time.Now().Format("20060102_150405")
	backupDir := "backups"
	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		fmt.Printf("无法创建备份目录：%v\n", err)
		return
	}
	backupFile := fmt.Sprintf("%s/backup_ranking_%s.db", backupDir, timestamp)
	// 通过标准输入传递 .backup 命令进行安全备份
	cmd := exec.Command("sqlite3", "ranking.db")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(".backup %s\n", backupFile))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("备份失败：%v\n输出：%s\n", err, output)
		return
	}
	fmt.Printf("数据库备份成功，文件保存为：%s\n", backupFile)

	// 清理旧的备份，只保留最新3个
	files, err := os.ReadDir(backupDir)
	if err != nil {
		fmt.Printf("读取备份目录失败：%v\n", err)
		return
	}
	// 收集符合文件名规则的备份文件
	var backupFiles []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "backup_ranking_") && strings.HasSuffix(file.Name(), ".db") {
			backupFiles = append(backupFiles, file)
		}
	}
	// 如果备份文件超过3个，则删除最旧的那几个
	if len(backupFiles) > 3 {
		// 排序：根据文件名排序，因为文件名中的时间戳格式保证了字典排序与时间顺序一致
		sort.Slice(backupFiles, func(i, j int) bool {
			return backupFiles[i].Name() < backupFiles[j].Name()
		})
		// 删除多余的备份
		for i := 0; i < len(backupFiles)-3; i++ {
			path := fmt.Sprintf("%s/%s", backupDir, backupFiles[i].Name())
			if err := os.Remove(path); err != nil {
				fmt.Printf("删除旧备份失败: %s, 错误: %v\n", path, err)
			} else {
				fmt.Printf("已删除旧备份: %s\n", path)
			}
		}
	}
}
