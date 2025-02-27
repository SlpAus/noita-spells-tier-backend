package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

func GetItems(c *gin.Context) {
	// 从请求中获取参数
	fmt.Println(c.Query("num"))
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "num should be a number"})
		return
	}

	startQualityStr := c.Query("startQuality")
	startQuality, err := strconv.Atoi(startQualityStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startQuality should be a number"})
		return
	}

	endQualityStr := c.Query("endQuality")
	endQuality, err := strconv.Atoi(endQualityStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endQuality should be a number"})
		return
	}

	canBeLostStr := c.Query("canBeLost")
	canBeLost, err := strconv.ParseBool(canBeLostStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "canBeLost should be a boolean"})
		return
	}

	query := database.DB.Where("quality BETWEEN ? AND ?", startQuality, endQuality)
	if canBeLost {
		query = query.Where("lost = ?", true)
	}

	var items []models.Item
	query.Order("RANDOM()").Limit(int(num)).Find(&items)

	var itemResponses []models.ItemResponse
	for _, item := range items {
		itemResponses = append(itemResponses, models.ItemResponse{
			ItemID:     item.ItemID,
			Name:       item.Name,
			Url:        item.Url,
			Quality:    item.Quality,
			Descrption: item.Descrption,
		})
	}

	c.JSON(http.StatusOK, itemResponses)
}
