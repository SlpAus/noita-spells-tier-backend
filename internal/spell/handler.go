package spell

import (
	"fmt"
	"net/http"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/gin-gonic/gin"
)

// --- API响应模型 ---
type RankingSpellResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ImageURL  string  `json:"imageUrl"`
	Type      int     `json:"type"`
	Score     float64 `json:"score"`
	Total     float64 `json:"total"`
	Win       float64 `json:"win"`
	RankScore float64 `json:"rankScore"`
}
type SpellImageResponse struct {
	ID       string `json:"id"`
	ImageURL string `json:"imageUrl"`
	Type     int    `json:"type"`
}
type GetPairAPIResponse struct {
	SpellA    SpellPairResponse `json:"spellA"`
	SpellB    SpellPairResponse `json:"spellB"`
	PairID    string            `json:"pairId"`
	Signature string            `json:"signature"`
}
type SpellPairResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Type        int    `json:"type"`
	Rank        int64  `json:"rank"` // 重新添加Rank字段
}

// --- 数据格式化辅助函数 (现在使用 services DTOs) ---
func formatForRanking(dto RankedSpellDTO, c *gin.Context) RankingSpellResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, dto.Info.Sprite)
	return RankingSpellResponse{
		ID:        dto.ID,
		Name:      dto.Info.Name,
		ImageURL:  imageURL,
		Type:      dto.Info.Type,
		Score:     dto.Stats.Score,
		Total:     dto.Stats.Total,
		Win:       dto.Stats.Win,
		RankScore: dto.Stats.RankScore, // 增加新字段
	}
}
func formatForImage(dto SpellImageDTO, c *gin.Context) SpellImageResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, dto.Info.Sprite)
	return SpellImageResponse{
		ID:       dto.ID,
		ImageURL: imageURL,
		Type:     dto.Info.Type,
	}
}
func formatForPair(dto PairSpellDTO, c *gin.Context) SpellPairResponse {
	imageURL := fmt.Sprintf("http://%s/images/spells/%s", c.Request.Host, dto.Info.Sprite)
	return SpellPairResponse{
		Name:        dto.Info.Name,
		Description: dto.Info.Description,
		ImageURL:    imageURL,
		Type:        dto.Info.Type,
		Rank:        dto.CurrentRank, // 映射Rank字段
	}
}

// --- 控制器函数 ---

// GetRanking 获取法术排行榜
func GetRanking(c *gin.Context) {
	rankedSpells, err := GetRankedSpells()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取排行榜数据失败"})
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
	spellDTO, err := GetSpellImageInfoByID(spellID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
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
	if !database.IsRedisHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务暂时不可用，请稍后重试"})
		return
	}

	// 1. 解析可选的查询参数
	excludeA := c.Query("excludeA")
	excludeB := c.Query("excludeB")

	// 2. 验证参数：要么都没有，要么都有
	if (excludeA != "" && excludeB == "") || (excludeA == "" && excludeB != "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "必须同时提供 excludeA 和 excludeB，或都不提供"})
		return
	}

	// 3. 调用服务层获取法术对和签名
	responseDTO, err := GetNewSpellPair(excludeA, excludeB)
	if err != nil {
		if err.Error() == "服务暂时不可用，请稍后重试" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		} else if err.Error() == "数据库中法术数量不足" || err.Error() == "排除后剩余法术数量不足" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取法术对时发生内部错误"})
		}
		return
	}

	// 4. 将服务层返回的DTO格式化为最终的API响应
	apiResponse := GetPairAPIResponse{
		SpellA:    formatForPair(responseDTO.SpellA, c),
		SpellB:    formatForPair(responseDTO.SpellB, c),
		PairID:    responseDTO.Payload.PairID,
		Signature: responseDTO.Signature,
	}
	apiResponse.SpellA.ID = responseDTO.Payload.SpellAID
	apiResponse.SpellB.ID = responseDTO.Payload.SpellBID

	c.JSON(http.StatusOK, apiResponse)
}
