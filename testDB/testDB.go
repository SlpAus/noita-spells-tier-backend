package main

import (
	"fmt"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
)

// IPVoteCount 结构体用于存储每个 IP 的投票数
type IPVoteCount struct {
	IP    string
	Count int64
}

// 测试vote数据有没有被写入
func main() {
	database.InitDB()

	//查看有多少vote数据
	var votes []models.Vote
	database.DB.Find(&votes)
	fmt.Println("votes count: ", len(votes))
}
