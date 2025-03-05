package main

import (
	"fmt"
	"math"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
)

const (
	K          = 4
	BasicScore = 1
)

func elo(winnerScore, loserScore, weight float64) (float64, float64) {
	winnerExpect := 1 / (1 + math.Pow(10, (loserScore-winnerScore)/400))
	loserExpect := 1 / (1 + math.Pow(10, (winnerScore-loserScore)/400))

	winnerScore += weight * (BasicScore + K*(1-winnerExpect))
	loserScore += weight * (-1*BasicScore + K*(0-loserExpect))

	return winnerScore, loserScore
}

func main() {
	// 初始化数据库
	database.InitDB()

	// 查询所有 vote 数据
	var votes []models.Vote
	database.DB.Find(&votes)

	// 创建一个 map 来存储每个道具的统计信息
	itemStats := make(map[uint]*models.Item)

	// 查询所有道具信息
	var items []models.Item
	database.DB.Find(&items)

	// 初始化 itemStats 但是清除所有得分，胜率，总场次等投票数据
	for _, item := range items {
		itemStats[item.ItemID] = &item
		itemStats[item.ItemID].Score = 0
		itemStats[item.ItemID].WinRate = 0
		itemStats[item.ItemID].Total = 0
		itemStats[item.ItemID].WinCount = 0

	}

	// 遍历 vote 数据，更新每个道具的得分、胜率和总场次
	for _, vote := range votes {
		if _, exists := itemStats[vote.Winner]; !exists {
			itemStats[vote.Winner] = &models.Item{ItemID: vote.Winner}
		}
		if _, exists := itemStats[vote.Loser]; !exists {
			itemStats[vote.Loser] = &models.Item{ItemID: vote.Loser}
		}

		winnerItem := itemStats[vote.Winner]
		loserItem := itemStats[vote.Loser]

		winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, vote.Weight)
		winnerItem.Total += vote.Weight
		loserItem.Total += vote.Weight
		winnerItem.WinCount += vote.Weight
		winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
		loserItem.WinRate = loserItem.WinCount / loserItem.Total
	}

	// 以胜率降序排列道具
	var updatedItems []models.Item
	for _, item := range itemStats {
		updatedItems = append(updatedItems, *item)
	}
	for i := 0; i < len(updatedItems); i++ {
		for j := i + 1; j < len(updatedItems); j++ {
			if updatedItems[i].WinRate < updatedItems[j].WinRate {
				updatedItems[i], updatedItems[j] = updatedItems[j], updatedItems[i]
			}
		}
	}

	// 打印
	for _, item := range updatedItems {
		fmt.Printf("道具ID: %d, 名称：%s, 得分: %f, 胜率: %f, 总场次: %f\n", item.ItemID, item.Name, item.Score, item.WinRate, item.Total)
	}

	// 更新数据库
	for _, item := range updatedItems {
		database.DB.Model(&models.Item{}).Where("item_id = ?", item.ItemID).Updates(item)
	}

	fmt.Println("所有道具的得分、胜率和总场次已重新计算并更新到数据库中")
}
