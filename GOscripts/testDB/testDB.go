package main

import (
	"fmt"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
)

// IPVoteCount 结构体用于存储每个 IP 的投票数（这里用于其他查询）
type IPVoteCount struct {
	IP    string
	Count int64
}

// detect_loser_items 根据传入的道具ID查询输掉的投票，并统计各IP出现次数
func detect_loser_items(loserID uint) {
	var ipCounts []IPVoteCount
	err := database.DB.Table("votes").
		Select("ip, count(*) as count").
		Where("loser = ?", loserID).
		Group("ip").
		Order("count asc").
		Scan(&ipCounts).Error
	if err != nil {
		fmt.Println("查询失败：", err)
		return
	}

	fmt.Printf("道具ID %d 输掉的投票中，各IP出现次数统计：\n", loserID)
	for _, v := range ipCounts {
		fmt.Printf("IP: %s, 出现次数: %d\n", v.IP, v.Count)
	}
}

func detect_loseorwin_items(loserID uint) {
	var ipCounts []IPVoteCount
	err := database.DB.Table("votes").
		Select("ip, count(*) as count").
		Where("loser = ? OR winner = ?", loserID, loserID).
		Group("ip").
		Order("count asc").
		Scan(&ipCounts).Error
	if err != nil {
		fmt.Println("查询失败：", err)
		return
	}

	fmt.Printf("道具ID %d 输掉的投票中，各IP出现次数统计：\n", loserID)
	for _, v := range ipCounts {
		fmt.Printf("IP: %s, 出现次数: %d\n", v.IP, v.Count)
	}
}

// detect_ip_vote 根据传入的 IP 查询其所有投票，并分析总票数、各胜利道具、失败道具及总权重等信息
func detect_ip_vote(ip string) {
	// 查询所有该 IP 的投票记录
	var votes []models.Vote
	err := database.DB.Where("ip = ?", ip).Find(&votes).Error
	if err != nil {
		fmt.Println("查询投票记录失败：", err)
		return
	}
	if len(votes) == 0 {
		fmt.Printf("IP %s 没有投票记录\n", ip)
		return
	}

	// 分析投票数据
	totalVotes := len(votes)
	var totalWeight float64
	winnerCounts := make(map[uint]int)
	loserCounts := make(map[uint]int)
	for _, vote := range votes {
		totalWeight += vote.Weight
		winnerCounts[vote.Winner]++
		loserCounts[vote.Loser]++
	}

	// 打印统计结果
	fmt.Printf("IP: %s 总投票数: %d，总权重: %f\n", ip, totalVotes, totalWeight)
	fmt.Println("各胜利道具出现统计：")
	for itemID, count := range winnerCounts {
		fmt.Printf("胜利道具ID: %d, 投票次数: %d\n", itemID, count)
	}
	fmt.Println("各失败道具出现统计：")
	for itemID, count := range loserCounts {
		fmt.Printf("失败道具ID: %d, 投票次数: %d\n", itemID, count)
	}
}

func Total_vote() {
	// 打印总票数
	var total_votes int64
	err := database.DB.Table("votes").Count(&total_votes).Error
	if err != nil {
		fmt.Println("查询失败：", err)
		return
	}
	fmt.Printf("总票数：%d\n", total_votes)
}

func main() {
	database.InitDB()
	detect_ip_vote("111.25.239.199")
}
