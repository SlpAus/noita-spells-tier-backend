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

	// 打印vote表的大小
	var voteCount int64
	database.DB.Model(&models.Vote{}).Count(&voteCount)
	fmt.Println("vote表大小:", voteCount)

	// 从vote表中删除所有ip为120.244.146.65和111.25.241.9的记录
	database.DB.Where("ip = ?", "120.244.146.65").Delete(&models.Vote{})
	database.DB.Where("ip = ?", "111.25.241.9").Delete(&models.Vote{})

	// 重新打印vote表的大小
	database.DB.Model(&models.Vote{}).Count(&voteCount)
	fmt.Println("vote表大小:", voteCount)

	fmt.Println("删除完成")
}
