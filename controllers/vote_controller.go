package controllers

import (
	"fmt"
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

func SendVoting(c *gin.Context) {
	var votingResult VotingResult
	if err := c.ShouldBindJSON(&votingResult); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 更新胜利者的参数
	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, votingResult.Winner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}
	winnerItem.Score++
	winnerItem.Total++
	winnerItem.WinRate = winnerItem.Score / winnerItem.Total

	// debug
	fmt.Println(winnerItem)

	database.DB.Save(&winnerItem)

	// 更新失败者的参数
	var loserItem models.Item
	if err := database.DB.First(&loserItem, votingResult.Loser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}
	loserItem.Total++
	loserItem.Score--
	loserItem.WinRate = loserItem.Score / loserItem.Total

	// debug
	fmt.Println(loserItem)

	database.DB.Save(&loserItem)

	c.JSON(http.StatusOK, gin.H{"message": "投票结果已保存"})
}
