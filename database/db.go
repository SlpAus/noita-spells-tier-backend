package database

import (
	"fmt"
	"log"

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
	err = DB.AutoMigrate(&models.Item{}, &models.Pool{}, &models.Vote{})
	if err != nil {
		log.Fatal("数据表迁移失败:", err)
	}

	fmt.Println("SQLite 数据库初始化完成！")
}
