package controllers

import (
	"net/http"

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
	Type := c.Query("type")
	if Type == "item" {
		database.DB.Order("win_rate desc").Find(&rankings)
	}

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
