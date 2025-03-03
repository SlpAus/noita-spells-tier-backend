package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type VoteResult struct {
	Result int `json:"result"`
}

const (
	Left int = iota + 1
	Right
	Nobody
	SKIP
)

const (
	VoteLimit  = 60              // 每个 IP 在时间窗口内的最大投票次数
	VoteWindow = 1 * time.Minute // 时间窗口大小
)

type VotingResult struct {
	Winner    uint `json:"winner"`
	Loser     uint `json:"loser"`
	FilterNum uint `json:"filternum"`
}

// 投票规则：score采用elo机制，胜率计算就是胜场/总场数

var K float64 = 8
var BasicScore float64 = 1
var totalItem uint = 705

func elo(winnerScore, loserScore, weight float64) (float64, float64) {
	var winnerExpect float64 = 1 / (1 + math.Pow(10, (loserScore-winnerScore)/400))
	var loserExpect float64 = 1 / (1 + math.Pow(10, (winnerScore-loserScore)/400))

	winnerScore += weight * (BasicScore + K*(1-winnerExpect))
	loserScore += weight * (-1*BasicScore + K*(0-loserExpect))

	return winnerScore, loserScore
}

func SendVoting(c *gin.Context) {
	var voteResult_JSON VoteResult
	if err := c.ShouldBindJSON(&voteResult_JSON); err != nil {
		fmt.Println("无效的请求数据:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	var voteResult int = voteResult_JSON.Result

	if voteResult == SKIP {
		fmt.Println("跳过投票")
		c.JSON(http.StatusBadRequest, gin.H{"error": "跳过投票"})
		return
	}

	// 根据cookie获取上一次getItem的结果

	userID, err := c.Cookie("user_id")
	if err != nil {
		fmt.Println("无效的用户ID:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var lastGetItem models.LastGetItem
	if err := database.DB.First(&lastGetItem, "user_id = ?", userID).Error; err != nil {
		fmt.Println("未找到上一次的道具:", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到上一次的道具"})
		return
	}

	var Winner uint
	var Loser uint
	var FilterNum uint

	if voteResult == Left {
		Winner = lastGetItem.Left
		Loser = lastGetItem.Right
	} else {
		Winner = lastGetItem.Right
		Loser = lastGetItem.Left
	}
	FilterNum = lastGetItem.FilterNum

	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, "item_id = ?", Winner).Error; err != nil {
		fmt.Println("胜利者道具未找到:", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}

	var loserItem models.Item
	if err := database.DB.First(&loserItem, "item_id = ?", Loser).Error; err != nil {
		fmt.Println("失败者道具未找到:", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}
	weight := float64(FilterNum) / float64(totalItem)

	// 检查投票频率
	ip := c.ClientIP()
	voteCountKey := fmt.Sprintf("vote_count:%s", ip)
	voteCount, err := database.Rdb.Get(database.Ctx, voteCountKey).Int64()
	if err != nil && err != redis.Nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法检查投票频率"})
		return
	}

	fmt.Println("ip:", ip, "投票次数:", voteCount)

	if voteCount >= VoteLimit {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "投票频率过高，请稍后再试"})
		return
	}

	// 增加投票计数
	if err := database.Rdb.Incr(database.Ctx, voteCountKey).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法增加投票计数"})
		return
	}

	// 设置投票计数的过期时间
	if err := database.Rdb.Expire(database.Ctx, voteCountKey, VoteWindow).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法设置投票计数过期时间"})
		return
	}

	Type := 0
	if voteResult == Nobody {
		Type = 1
	}

	// 将投票数据储存到 Redis

	voteData := map[string]interface{}{
		"type":      Type,
		"winner":    Winner,
		"loser":     Loser,
		"weight":    weight,
		"ip":        ip,
		"timestamp": time.Now().Unix(),
	}
	voteKey := fmt.Sprintf("vote:%s:%d", ip, time.Now().UnixNano())
	if err := database.Rdb.HMSet(database.Ctx, voteKey, voteData).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法存储投票数据"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "投票结果已保存"})
}

// 私用接口，直接发送投票结果，可以定向投票

// POST /api/v1/vote/send

func SendVotingResult(c *gin.Context) {
	var votingResult_JSON VotingResult
	if err := c.ShouldBindJSON(&votingResult_JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	var Winner uint = votingResult_JSON.Winner
	var Loser uint = votingResult_JSON.Loser
	var FilterNum uint = votingResult_JSON.FilterNum

	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, "item_id = ?", Winner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}

	var loserItem models.Item
	if err := database.DB.First(&loserItem, "item_id = ?", Loser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}
	weight := float64(FilterNum) / float64(totalItem)

	// 更新得分和胜率
	winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, weight)
	winnerItem.Total += weight
	loserItem.Total += weight
	winnerItem.WinCount += weight
	winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
	loserItem.WinRate = loserItem.WinCount / loserItem.Total

	fmt.Println("投票结果:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate, "胜场:", winnerItem.WinCount, "总场次:", winnerItem.Total)
	fmt.Println("投票结果:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate, "胜场:", loserItem.WinCount, "总场次:", loserItem.Total)

	// 记录投票到数据库

	var vote models.Vote
	vote.Winner = winnerItem.ItemID
	vote.Loser = loserItem.ItemID
	vote.Weight = weight
	vote.IP = c.ClientIP()

	database.DB.Save(&vote)
	database.DB.Save(&winnerItem)
	database.DB.Save(&loserItem)

	c.JSON(http.StatusOK, gin.H{"message": "投票结果已保存"})
}

func ProcessVotes() {
	// 获得所有投票数据

	keys, err := database.Rdb.Keys(database.Ctx, "vote:*").Result()
	if err != nil {
		fmt.Println("无法获取投票数据:", err)
		return
	}

	for _, key := range keys {
		voteData, err := database.Rdb.HGetAll(database.Ctx, key).Result()
		if err != nil {
			fmt.Println("无法获取投票数据:", err)
			continue
		}

		Winner := voteData["winner"]
		Loser := voteData["loser"]
		Weight := voteData["weight"]

		var winnerItem models.Item
		if err := database.DB.First(&winnerItem, "item_id = ?", Winner).Error; err != nil {
			fmt.Println("胜利者道具未找到:", err)
			continue
		}

		var loserItem models.Item
		if err := database.DB.First(&loserItem, "item_id = ?", Loser).Error; err != nil {
			fmt.Println("失败者道具未找到:", err)
			continue
		}

		weight, err := strconv.ParseFloat(Weight, 64)
		if err != nil {
			fmt.Println("无法解析权重:", err)
			continue
		}

		Type, err := strconv.Atoi(voteData["type"])
		if err != nil {
			fmt.Println("无法解析类型:", err)
			continue
		}

		if Type == 0 {
			// 更新得分和胜率
			winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, weight)
			winnerItem.Total += weight
			loserItem.Total += weight
			winnerItem.WinCount += weight
			winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
			loserItem.WinRate = loserItem.WinCount / loserItem.Total
		} else {
			// 无人胜利
			winnerItem.Total += weight
			loserItem.Total += weight
			winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
			loserItem.WinRate = loserItem.WinCount / loserItem.Total
		}

		// 记录投票到数据库

		var vote models.Vote
		vote.Winner = winnerItem.ItemID
		vote.Loser = loserItem.ItemID
		vote.Weight = weight
		vote.IP = voteData["ip"]
		vote.Type = Type

		fmt.Println("Winner:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate, "胜场:", winnerItem.WinCount, "总场次:", winnerItem.Total)
		fmt.Println("Loser:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate, "胜场:", loserItem.WinCount, "总场次:", loserItem.Total)

		database.DB.Save(&vote)
		database.DB.Save(&winnerItem)
		database.DB.Save(&loserItem)
		// 删除投票数据
		database.Rdb.Del(database.Ctx, key)
	}
	fmt.Println("所有投票数据已处理")

}
