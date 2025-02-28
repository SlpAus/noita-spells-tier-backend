package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
)

// get /api/item/getItems?num=X&startQuality=X&endQuality=X&canBeLost=X
// &itemPools=X,Y,Z...
func GetItems(c *gin.Context) {
	// 从请求中获取参数
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
			Select("DISTINCT items.id, items.item_id, items.name, items.url, items.quality, items.descrption")
	}
	var items []models.Item
	// 计算经过过滤后还剩多少道具
	var FilterNum int
	query.Order("RANDOM()").Find(&items)
	FilterNum = len(items)
	// 随机取出两个
	if len(items) >= num {
		items = items[:num]
	} else {
		fmt.Println("道具数量不足")
		c.JSON(http.StatusBadRequest, gin.H{"error": "道具数量不足,剩余道具数量:" + strconv.Itoa(FilterNum)})
		return
	}
	fmt.Println(items)

	var itemResponses []models.ItemResponse
	for _, item := range items {
		itemResponses = append(itemResponses, models.ItemResponse{
			ItemID:     item.ItemID,
			Name:       item.Name,
			Url:        item.Url,
			Quality:    item.Quality,
			Descrption: item.Descrption,
			FilterNum:  uint(FilterNum),
		})
	}
	c.JSON(http.StatusOK, itemResponses)
}
