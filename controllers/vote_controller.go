package controllers

import (
	"fmt"
	"math"
	"net/http"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

type VotingResult struct {
	Type      string `json:"type"`
	Winner    uint   `json:"winner"`
	Loser     uint   `json:"loser"`
	FilterNUm uint   `json:"filternum"`
}

// 投票规则：score采用elo机制，胜率计算就是胜场/总场数

var K float64 = 8
var BasicScore float64 = 1
var totalItem uint = 705

func elo(winnerScore, loserScore, weight float64) (float64, float64) {
	var winnerExpect float64 = 1 / (1 + math.Pow(10, (loserScore-winnerScore)/400))
	var loserExpect float64 = 1 / (1 + math.Pow(10, (winnerScore-loserScore)/400))

	winnerScore += weight * (BasicScore + K*(1-winnerExpect))
	loserScore += weight * (-1*BasicScore + K*(0-loserExpect))

	return winnerScore, loserScore
}

func SendVoting(c *gin.Context) {
	var votingResult VotingResult
	if err := c.ShouldBindJSON(&votingResult); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}
	fmt.Println(votingResult)

	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, "item_id = ?", votingResult.Winner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}

	var loserItem models.Item
	if err := database.DB.First(&loserItem, "item_id = ?", votingResult.Loser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}
	weight := float64(votingResult.FilterNUm) / float64(totalItem)

	if votingResult.Type == "item" {
		// 更新得分和胜率
		winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, weight)
		winnerItem.Total += weight
		loserItem.Total += weight
		winnerItem.WinCount += weight
		winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
		loserItem.WinRate = loserItem.WinCount / loserItem.Total

		fmt.Println("投票结果:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate, "胜场:", winnerItem.WinCount, "总场次:", winnerItem.Total)
		fmt.Println("投票结果:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate, "胜场:", loserItem.WinCount, "总场次:", loserItem.Total)

	} else if votingResult.Type == "nobody" {
		// 不更改score,所有人总场次+1
		winnerItem.Total += weight
		loserItem.Total += weight
		winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
		loserItem.WinRate = loserItem.WinCount / loserItem.Total
		fmt.Println("投票结果: 无人胜出")
	}

	// 记录投票到数据库

	var vote models.Vote
	vote.Winner = winnerItem.ItemID
	vote.Loser = loserItem.ItemID
	vote.Weight = weight
	vote.IP = c.ClientIP()

	database.DB.Save(&vote)
	database.DB.Save(&winnerItem)
	database.DB.Save(&loserItem)

	c.JSON(http.StatusOK, gin.H{"message": "投票结果已保存"})
}
