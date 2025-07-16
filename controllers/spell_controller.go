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

// --- API响应模型 ---

// SpellPairResponse 定义了获取法术对战组合时，返回的单个法术的数据结构
type SpellPairResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Type        int    `json:"type"`
	Rank        int64  `json:"rank"` // 新增字段：即时排名
}

// RankingSpellResponse 定义了排行榜API返回的法术数据结构
type RankingSpellResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"imageUrl"`
	Type     int     `json:"type"`
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

// --- 数据格式化辅助函数 ---

// formatSpellForPair 将数据库模型和计算出的排名，转换为法术对API的响应模型
func formatSpellForPair(spell models.Spell, rank int64, c *gin.Context) SpellPairResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, spell.Sprite)
	return SpellPairResponse{
		ID:          spell.SpellID,
		Name:        spell.Name,
		Description: spell.Description,
		ImageURL:    imageURL,
		Type:        spell.Type,
		Rank:        rank,
	}
}

// formatSpellForRanking 将数据库模型转换为排行榜API的响应模型
func formatSpellForRanking(spell models.Spell, c *gin.Context) RankingSpellResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, spell.Sprite)
	return RankingSpellResponse{
		ID:       spell.SpellID,
		Name:     spell.Name,
		ImageURL: imageURL,
		Type:     spell.Type,
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

// --- 控制器函数 ---

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
	var spells []models.Spell
	// 1. 随机获取两个法术
	result := database.DB.Order("RANDOM()").Limit(2).Find(&spells)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取法术对时发生数据库错误"})
		return
	}
	if result.RowsAffected < 2 {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据库中法术数量不足，无法组成对战"})
		return
	}

	// 2. 为获取到的两个法术分别计算排名
	var spellResponses []SpellPairResponse
	for _, spell := range spells {
		var rank int64
		// 排名 = 1 + 有多少个法术的分数比我高
		database.DB.Model(&models.Spell{}).Where("score > ?", spell.Score).Count(&rank)
		rank++ // 排名从1开始

		spellResponses = append(spellResponses, formatSpellForPair(spell, rank, c))
	}

	// 3. 成功返回两条法术的数据，包含即时排名
	c.JSON(http.StatusOK, spellResponses)
}

// GetSpellByID 根据ID获取单个法术的信息
func GetSpellByID(c *gin.Context) {
	spellID := c.Param("id")
	var spell models.Spell
	result := database.DB.Where("spell_id = ?", spellID).First(&spell)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("找不到ID为 %s 的法术", spellID)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		}
		return
	}

	c.JSON(http.StatusOK, formatSpellForImage(spell, c))
}
