package database

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Qiuarctica/isaac-ranking-backend/models"
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
	err = DB.AutoMigrate(&models.Item{}, &models.Pool{}, &models.Vote{}, &models.LastGetItem{})
	if err != nil {
		log.Fatal("数据表迁移失败:", err)
	}

	// 定时备份
	go func() {
		for {
			backupDB()
			time.Sleep(4 * time.Hour)
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
}
