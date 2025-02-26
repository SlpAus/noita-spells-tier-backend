package controllers

import (
	"net/http"
	"strconv"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

type Ranking struct {
	Rank       uint    `json:"rank"`
	Name       string  `json:"name"`
	Score      float64 `json:"score"`
	Winpercent float64 `json:"winpercent"`
}

// get /api/getRanking?type=XXX

func GetRanking(c *gin.Context) {
	var rankings []models.Item

	startQualityStr := c.Query("startQuality")
	endQualityStr := c.Query("endQuality")
	canBeLostStr := c.Query("canBeLost")

	startQuality, err := strconv.Atoi(startQualityStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startQuality should be a number"})
		return
	}
	endQuality, err := strconv.Atoi(endQualityStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endQuality should be a number"})
		return
	}
	canBeLost, err := strconv.ParseBool(canBeLostStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "canBeLost should be a boolean"})
		return
	}

	query := database.DB.Where("quality BETWEEN ? AND ?", startQuality, endQuality)
	if canBeLost {
		query = query.Where("lost = ?", true)
	}
	// 以win_rate,score降序排列
	query.Order("win_rate desc,score desc").Find(&rankings)

	var rankingResponses []Ranking
	for i, item := range rankings {
		rankingResponses = append(rankingResponses, Ranking{
			Rank:       uint(i + 1),
			Name:       item.Name,
			Score:      item.Score,
			Winpercent: item.WinRate,
		})
	}

	c.JSON(http.StatusOK, rankingResponses)

}
