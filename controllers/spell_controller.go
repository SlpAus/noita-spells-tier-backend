package controllers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RankingSpellResponse 定义了排行榜API返回的法术数据结构
type RankingSpellResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"imageUrl"`
	Type     int     `json:"type"` // 新增字段
	Score    float64 `json:"score"`
	Total    int     `json:"total"`
	Win      int     `json:"win"`
}

// SpellImageResponse 定义了根据ID获取单个法术信息时返回的精简数据结构
type SpellImageResponse struct {
	ID       string `json:"id"`
	ImageURL string `json:"imageUrl"`
	Type     int    `json:"type"`
}

// formatSpellForRanking 将数据库模型转换为排行榜API的响应模型
func formatSpellForRanking(spell models.Spell, c *gin.Context) RankingSpellResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, spell.Sprite)

	return RankingSpellResponse{
		ID:       spell.SpellID,
		Name:     spell.Name,
		ImageURL: imageURL,
		Type:     spell.Type, // 新增字段
		Score:    spell.Score,
		Total:    spell.Total,
		Win:      spell.Win,
	}
}

// formatSpellForImage 将数据库模型转换为单个法术图片API的响应模型
func formatSpellForImage(spell models.Spell, c *gin.Context) SpellImageResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, spell.Sprite)

	return SpellImageResponse{
		ID:       spell.SpellID,
		ImageURL: imageURL,
		Type:     spell.Type,
	}
}

// GetRanking 获取法术排行榜
func GetRanking(c *gin.Context) {
	var spells []models.Spell

	result := database.DB.Order("score desc").Find(&spells)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取排行榜数据"})
		return
	}

	var rankingResponses []RankingSpellResponse
	for _, spell := range spells {
		rankingResponses = append(rankingResponses, formatSpellForRanking(spell, c))
	}

	c.JSON(http.StatusOK, rankingResponses)
}

// GetSpellPair 获取一对用于对战的法术
func GetSpellPair(c *gin.Context) {
	// TODO: 在这里实现从数据库随机获取两个法术的逻辑
	c.JSON(http.StatusOK, gin.H{"message": "GetSpellPair endpoint is working!"})
}

// GetSpellByID 根据ID获取单个法术的信息
func GetSpellByID(c *gin.Context) {
	// 从URL路径中获取法术ID
	spellID := c.Param("id")

	var spell models.Spell
	// 根据 spell_id 在数据库中查找法术
	// .First() 方法会在找到第一条记录后停止，性能较好
	result := database.DB.Where("spell_id = ?", spellID).First(&spell)

	// 检查查询结果是否有错误
	if result.Error != nil {
		// 如果错误是 gorm.ErrRecordNotFound，说明数据库里没有这条记录
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("找不到ID为 %s 的法术", spellID)})
		} else {
			// 如果是其他数据库错误，返回500服务器内部错误
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		}
		return
	}

	// 如果找到了法术，格式化数据并返回200成功响应
	c.JSON(http.StatusOK, formatSpellForImage(spell, c))
}
