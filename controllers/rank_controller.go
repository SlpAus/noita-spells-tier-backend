package controllers

import (
	"net/http"
	"strconv"

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

	query, err := applyFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query = query.Select("DISTINCT  items.name, items.score, items.win_rate, items.total")
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

// 从全部的投票中获取某个道具和其他道具的对位数据
// get /api/rank/getItemRank?itemID=XXX&itemPools=A,B,C...&startQuality=1&endQuality=2&canBeLost=true

func GetItemRank(c *gin.Context) {
	itemIDstr := c.Query("itemID")
	if itemIDstr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemID should not be empty"})
		return
	}

	itemID, err := strconv.Atoi(itemIDstr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemID should be a number"})
		return
	}

	query, err := applyFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	query = query.Distinct("items.id, items.item_id, items.name")
	var filteredItems []models.Item
	query.Find(&filteredItems)

	var votes []models.Vote
	database.DB.Where("winner = ? OR loser = ?", itemID, itemID).Find(&votes)

	type ItemRank struct {
		ItemID   uint    `json:"id"`
		Name     string  `json:"name"`
		Total    float64 `json:"total"`
		WinCount float64 `json:"wincount"`
		WinRate  float64 `json:"winrate"`
	}

	itemRanks := make(map[uint]*ItemRank)

	for _, vote := range votes {
		var otherItemID uint
		var isWinner bool
		if vote.Winner == uint(itemID) {
			otherItemID = vote.Loser
			isWinner = true
		} else {
			otherItemID = vote.Winner
			isWinner = false
		}

		for _, filteredItem := range filteredItems {
			if filteredItem.ItemID == otherItemID {
				if _, exists := itemRanks[otherItemID]; !exists {
					itemRanks[otherItemID] = &ItemRank{
						ItemID: otherItemID,
						Name:   filteredItem.Name,
					}
				}
				itemRanks[otherItemID].Total += vote.Weight
				if isWinner {
					itemRanks[otherItemID].WinCount += vote.Weight
				}
				itemRanks[otherItemID].WinRate = itemRanks[otherItemID].WinCount / itemRanks[otherItemID].Total
				break
			}
		}
	}

	var itemRankResponses []ItemRank
	for _, itemRank := range itemRanks {
		itemRankResponses = append(itemRankResponses, *itemRank)
	}

	c.JSON(http.StatusOK, itemRankResponses)
}
