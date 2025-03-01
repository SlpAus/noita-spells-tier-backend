package main

import (
	"fmt"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
)

// 测试vote数据有没有被写入
func main() {

	database.InitDB()

	// 打印vote表

	var votes []models.Vote
	var count int64
	database.DB.Find(&votes).Count(&count)
	fmt.Println(count)

}
