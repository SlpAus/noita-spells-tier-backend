package controllers

import (
	"net/http"
	"strconv"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

func GetItems(c *gin.Context) {
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var items []models.Item
	database.DB.Order("RANDOM()").Limit(num).Find(&items)

	var itemResponses []models.ItemResponse
	for _, item := range items {
		itemResponses = append(itemResponses, models.ItemResponse{
			ItemID:  item.ItemID,
			Name:    item.Name,
			Url:     item.Url,
			Quality: item.Quality,
		})
	}

	c.JSON(http.StatusOK, itemResponses)
}
