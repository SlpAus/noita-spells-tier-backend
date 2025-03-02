package controllers

import (
	"fmt"
	"math"
	"net/http"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

type VoteResult struct {
	Result int `json:"result"`
}

const (
	Left int = iota + 1
	Right
	Nobody
	SKIP
)

type VotingResult struct {
	Winner    uint `json:"winner"`
	Loser     uint `json:"loser"`
	FilterNum uint `json:"filternum"`
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
	var voteResult_JSON VoteResult
	if err := c.ShouldBindJSON(&voteResult_JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	var voteResult int = voteResult_JSON.Result

	if voteResult == SKIP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "跳过投票"})
		return
	}

	// 根据cookie获取上一次getItem的结果

	userID, err := c.Cookie("user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var lastGetItem models.LastGetItem
	if err := database.DB.First(&lastGetItem, "user_id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到上一次的道具"})
		return
	}

	var Winner uint
	var Loser uint
	var FilterNum uint

	if voteResult == Left {
		Winner = lastGetItem.Left
		Loser = lastGetItem.Right
	} else {
		Winner = lastGetItem.Right
		Loser = lastGetItem.Left
	}
	FilterNum = lastGetItem.FilterNum

	// 根据上一次储存在redis中的返回数据，判断投票结果

	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, "item_id = ?", Winner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}

	var loserItem models.Item
	if err := database.DB.First(&loserItem, "item_id = ?", Loser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}
	weight := float64(FilterNum) / float64(totalItem)

	if (voteResult == Left) || (voteResult == Right) {
		// 更新得分和胜率
		winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, weight)
		winnerItem.Total += weight
		loserItem.Total += weight
		winnerItem.WinCount += weight
		winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
		loserItem.WinRate = loserItem.WinCount / loserItem.Total

		fmt.Println("投票结果:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate, "胜场:", winnerItem.WinCount, "总场次:", winnerItem.Total)
		fmt.Println("投票结果:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate, "胜场:", loserItem.WinCount, "总场次:", loserItem.Total)

	} else {
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
