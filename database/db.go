package database

import (
	"fmt"
	"log"
	"github.com/SlpAus/noita-spells-tier-backend/models" // 确保这里的模块名和你的go.mod一致
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB() {
	var err error

	// GORM日志配置
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: 0,
			LogLevel:      logger.Silent, // 在生产环境中可以设为Silent
			Colorful:      true,
		},
	)

	// 连接到SQLite数据库
	DB, err = gorm.Open(sqlite.Open("ranking.db"), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		fmt.Println("连接数据库失败", err)
		panic(err)
	}

	fmt.Println("数据库连接成功！")

	// 自动迁移：GORM会自动创建或更新表结构以匹配我们的模型
	// 这里只包含我们新项目中需要的模型
	err = DB.AutoMigrate(&models.Spell{}, &models.Vote{})
	if err != nil {
		fmt.Println("数据库迁移失败", err)
		panic(err)
	}

	fmt.Println("数据库迁移成功！")
}
