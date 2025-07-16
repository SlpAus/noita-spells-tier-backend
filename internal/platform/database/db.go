package database

import (
	"fmt"
	"log"
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
}
