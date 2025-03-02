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

	// 查询每个 IP 的投票数，并统计一共有多少个 IP 参与了投票，同时按照投票数从高到低排序
	var results []IPVoteCount
	database.DB.Model(&models.Vote{}).
		Select("ip, COUNT(*) as count").
		Group("ip").
		Order("count DESC").
		Find(&results)

	// 打印每个 IP 的投票数
	for _, result := range results {
		fmt.Printf("IP: %s, 投票数: %d\n", result.IP, result.Count)
	}

	// 统计一共有多少个 IP 参与了投票
	var ipCount int64
	database.DB.Model(&models.Vote{}).
		Select("ip").
		Group("ip").
		Count(&ipCount)
	fmt.Printf("一共有 %d 个 IP 参与了投票\n", ipCount)

	// 查询特定 IP 的所有投票记录
	var votes []models.Vote
	ip := "111.25.241.9"
	database.DB.Where("ip = ?", ip).Find(&votes)

}
