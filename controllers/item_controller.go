package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// get /api/item/getItems?num=X&startQuality=X&endQuality=X&canBeLost=X
// &itemPools=X,Y,Z...

// filters接口:
// startQuality: 道具最低品质 0-4的数字
// endQuality: 道具最高品质 0-4的数字
// canBeLost: 是否被里lost筛选 true: 显示里lost可获得的道具 false: 显示所有道具
// itemPools: 道具池筛选 以逗号分割的字符串
// isActive: 是否是主动道具 0:显示全部 1:只显示主动 2:只显示被动

func applyFilters(c *gin.Context) (*gorm.DB, error) {
	startQualityStr := c.Query("startQuality")
	startQuality, err := strconv.Atoi(startQualityStr)
	if err != nil {
		return nil, fmt.Errorf("startQuality should be a number")
	}

	endQualityStr := c.Query("endQuality")
	endQuality, err := strconv.Atoi(endQualityStr)
	if err != nil {
		return nil, fmt.Errorf("endQuality should be a number")
	}

	canBeLostStr := c.Query("canBeLost")
	canBeLost, err := strconv.ParseBool(canBeLostStr)
	if err != nil {
		return nil, fmt.Errorf("canBeLost should be a boolean")
	}

	itemPoolsStr := c.Query("itemPools")
	var itemPools []string
	if itemPoolsStr != "" {
		itemPools = strings.Split(itemPoolsStr, ",")
	}

	isActivteStr := c.Query("isActive")
	isActive, err := strconv.Atoi(isActivteStr)
	if err != nil {
		return nil, fmt.Errorf("isActive should be a number")
	}

	// 道具品质
	query := database.DB.Where("quality BETWEEN ? AND ?", startQuality, endQuality)
	// 里lost
	if canBeLost {
		query = query.Where("lost = ?", true)
	}

	// 主动道具 其中主动道具是道具描述中包含"使用后，"的道具
	if isActive == 1 {
		query = query.Where("descrption LIKE ?", "%使用后%")
	} else if isActive == 2 {
		query = query.Where("descrption NOT LIKE ?", "%使用后%")
	}

	// 道具池
	if len(itemPools) > 0 {
		query = query.Joins("JOIN item_pools ON items.id = item_pools.item_id").
			Joins("JOIN pools ON pools.id = item_pools.pool_id").
			Where("pools.name IN ?", itemPools)
	}
	return query, nil
}

func GetItems(c *gin.Context) {
	// 从请求中获取参数
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "num should be a number"})
		return
	}

	if num == 0 {
		// 返回所有道具
		var items []models.Item
		database.DB.Find(&items)
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
		return
	}
	query, err := applyFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	query = query.Select("DISTINCT items.id, items.item_id, items.name, items.url, items.quality, items.descrption")

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
	userID, err := c.Cookie("user_id")
	if err != nil {
		// 如果不存在，生成一个新的 UUID 并设置 Cookie
		userID = uuid.New().String()
		c.SetCookie("user_id", userID, 0, "/", "", false, true)

	}

	if num == 2 {

		// 组装需缓存的数据
		lastGetItem := models.LastGetItem{
			UserID:    userID,
			Left:      itemResponses[0].ItemID,
			Right:     itemResponses[1].ItemID,
			FilterNum: uint(FilterNum),
		}

		// 将对象序列化为 JSON 字符串
		data, err := json.Marshal(lastGetItem)
		if err != nil {
			fmt.Println("序列化 lastGetItem 失败：", err)
		} else {
			// 设置 Redis key，过期时间可根据需求调整(此处设置为 24 小时)
			key := fmt.Sprintf("last_get_item:%s", userID)
			err = database.Rdb.Set(database.Ctx, key, data, 24*time.Hour).Err()
			if err != nil {
				fmt.Println("保存到 Redis 失败：", err)
			}
		}

	}

	c.JSON(http.StatusOK, itemResponses)
}
