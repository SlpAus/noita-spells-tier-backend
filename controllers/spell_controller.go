package controllers

import (
	"fmt"
	"net/http"

	"github.com/SlpAus/noita-spells-tier-backend/services"
	"github.com/gin-gonic/gin"
)

// --- API响应模型 (不变) ---
type SpellPairResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Type        int    `json:"type"`
	Rank        int64  `json:"rank"`
}
type RankingSpellResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"imageUrl"`
	Type     int     `json:"type"`
	Score    float64 `json:"score"`
	Total    int     `json:"total"`
	Win      int     `json:"win"`
}
type SpellImageResponse struct {
	ID       string `json:"id"`
	ImageURL string `json:"imageUrl"`
	Type     int    `json:"type"`
}

// --- 数据格式化辅助函数 (现在使用 services DTOs) ---
func formatForRanking(dto services.RankedSpellDTO, c *gin.Context) RankingSpellResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, dto.Info.Sprite)
	return RankingSpellResponse{
		ID:       dto.ID,
		Name:     dto.Info.Name,
		ImageURL: imageURL,
		Type:     dto.Info.Type,
		Score:    dto.Stats.Score,
		Total:    dto.Stats.Total,
		Win:      dto.Stats.Win,
	}
}
func formatForImage(dto services.SpellImageDTO, c *gin.Context) SpellImageResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, dto.Info.Sprite)
	return SpellImageResponse{
		ID:       dto.ID,
		ImageURL: imageURL,
		Type:     dto.Info.Type,
	}
}

// --- 控制器函数 ---

// GetRanking 获取法术排行榜
func GetRanking(c *gin.Context) {
	rankedSpells, err := services.GetRankedSpells()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取排行榜数据失败: " + err.Error()})
		return
	}

	var responses []RankingSpellResponse
	for _, spellDTO := range rankedSpells {
		responses = append(responses, formatForRanking(spellDTO, c))
	}
	c.JSON(http.StatusOK, responses)
}

// GetSpellByID 根据ID获取单个法术的信息
func GetSpellByID(c *gin.Context) {
	spellID := c.Param("id")
	spellDTO, err := services.GetSpellImageInfoByID(spellID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败: " + err.Error()})
		return
	}
	if spellDTO == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("找不到ID为 %s 的法术", spellID)})
		return
	}
	c.JSON(http.StatusOK, formatForImage(*spellDTO, c))
}

// GetSpellPair 获取一对用于对战的法术
func GetSpellPair(c *gin.Context) {
	// TODO: 迁移此函数的逻辑到 services 层，并实现高级匹配算法
	c.JSON(http.StatusOK, gin.H{"message": "GetSpellPair endpoint is working!"})
}
