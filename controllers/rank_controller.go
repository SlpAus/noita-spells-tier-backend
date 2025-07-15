package controllers

import (
	"net/http"
	"strconv"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/models"
	"github.com/gin-gonic/gin"
)

type Ranking struct {
	Rank       uint    `json:"rank"`
	Name       string  `json:"name"`
	Score      float64 `json:"score"`
	Winpercent float64 `json:"winpercent"`
	Totals     float64 `json:"totals"`
}

type ItemRank struct {
	ItemID   uint    `json:"id"`
	Name     string  `json:"name"`
	Total    float64 `json:"total"`
	WinCount float64 `json:"wincount"`
	WinRate  float64 `json:"winrate"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "道具数量不足"})
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
				if isWinner && vote.Type != 1 {
					itemRanks[otherItemID].WinCount += vote.Weight
				}
				if itemRanks[otherItemID].Total == 0 {
					itemRanks[otherItemID].WinRate = 0
				} else {
					itemRanks[otherItemID].WinRate = itemRanks[otherItemID].WinCount / itemRanks[otherItemID].Total
				}
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

func GetMyRank(c *gin.Context) {
	// 获得自己IP的所有投票，生成个性化报告,例如投票是否符合总榜排名（计算符合总榜排名率，找出和总榜差距最大的vote），投票次数最多的道具等等

	// 获得自己的所有投票
	ip := c.ClientIP()
	var votes []models.Vote
	database.DB.Where("ip = ?", ip).Find(&votes)

	// 分析投票次数最多的道具
	itemVotes := make(map[uint]float64)
	for _, vote := range votes {
		itemVotes[vote.Winner] += vote.Weight
		itemVotes[vote.Loser] += vote.Weight
	}

	var maxItemID uint
	var maxItemVotes float64
	for itemID, votes := range itemVotes {
		if votes > maxItemVotes {
			maxItemVotes = votes
			maxItemID = itemID
		}
	}

	// 获取总榜排名
	var rankings []models.Item
	database.DB.Order("score desc, win_rate desc").Find(&rankings)

	// 生成总榜排名映射
	rankingMap := make(map[uint]int)
	for i, item := range rankings {
		rankingMap[item.ItemID] = i + 1
	}

	// 计算符合总榜排名率和找出和总榜差距最大的vote
	var totalVotes int
	var matchingVotes int
	var maxDifference int
	var maxDifferenceVote models.Vote

	for _, vote := range votes {
		winnerRank := rankingMap[vote.Winner]
		loserRank := rankingMap[vote.Loser]
		if winnerRank < loserRank {
			matchingVotes++
		}
		totalVotes++

		difference := winnerRank - loserRank
		if difference > maxDifference {
			maxDifference = difference
			maxDifferenceVote = vote
		}
	}

	matchingRate := float64(matchingVotes) / float64(totalVotes)

	// 返回个性化报告

	c.JSON(http.StatusOK, gin.H{
		"total_votes":      totalVotes,
		"most_voted_item":  maxItemID,
		"most_voted_times": maxItemVotes,
		"matching_rate":    matchingRate,
		"max_difference":   maxDifference,
		"max_diff_vote":    maxDifferenceVote,
	})
}
