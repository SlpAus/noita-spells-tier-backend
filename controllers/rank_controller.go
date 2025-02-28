package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

type Ranking struct {
	Rank       uint    `json:"rank"`
	Name       string  `json:"name"`
	Score      float64 `json:"score"`
	Winpercent float64 `json:"winpercent"`
	Totals     float64 `json:"totals"`
}

// get /api/getRanking?type=XXX&startQuality=X&endQuality=X&canBeLost=X
// &itemPools=X,Y,Z...

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
	itemPoolsStr := c.Query("itemPools")
	var itemPools []string
	if itemPoolsStr != "" {
		itemPools = strings.Split(itemPoolsStr, ",")
	}

	query := database.DB.Where("quality BETWEEN ? AND ?", startQuality, endQuality)
	if canBeLost {
		query = query.Where("lost = ?", true)
	}

	if len(itemPools) > 0 {
		query = query.Joins("JOIN item_pools ON items.id = item_pools.item_id").
			Joins("JOIN pools ON pools.id = item_pools.pool_id").
			Where("pools.name IN ?", itemPools).
			Select("DISTINCT  items.name, items.score, items.win_rate, items.total")
	}

	// 以score , win_rate降序排列
	query.Order("score desc,win_rate desc").Find(&rankings)

	var rankingResponses []Ranking
	for i, item := range rankings {
		rankingResponses = append(rankingResponses, Ranking{
			Rank:       uint(i + 1),
			Name:       item.Name,
			Score:      item.Score,
			Winpercent: item.WinRate,
			Totals:     item.Total,
		})
	}
	if len(rankingResponses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到道具"})
		return
	}

	c.JSON(http.StatusOK, rankingResponses)

}
