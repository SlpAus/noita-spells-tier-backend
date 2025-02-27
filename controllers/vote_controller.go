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
	Type   string `json:"type"`
	Winner uint   `json:"winner"`
	Loser  uint   `json:"loser"`
}

// 投票规则：score采用elo机制，胜率计算就是胜场/总场数

var K float64 = 32

func elo(winnerScore, loserScore float64) (float64, float64) {
	var winnerExpect float64 = 1 / (1 + math.Pow(10, (loserScore-winnerScore)/400))
	var loserExpect float64 = 1 / (1 + math.Pow(10, (winnerScore-loserScore)/400))

	winnerScore += K * (1 - winnerExpect)
	loserScore += K * (0 - loserExpect)

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

	// 更新得分和胜率
	winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score)
	winnerItem.WinRate = (winnerItem.WinRate*winnerItem.Total + 1) / (winnerItem.Total + 1)
	loserItem.WinRate = (loserItem.WinRate * loserItem.Total) / (loserItem.Total + 1)
	winnerItem.Total++
	loserItem.Total++

	database.DB.Save(&winnerItem)
	database.DB.Save(&loserItem)

	fmt.Println("投票结果:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate)
	fmt.Println("投票结果:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate)

	c.JSON(http.StatusOK, gin.H{"message": "投票结果已保存"})
}
