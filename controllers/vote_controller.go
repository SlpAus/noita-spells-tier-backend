package controllers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
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
	VoteLimit  = 80              // 每个 IP 在时间窗口内的最大投票次数
	VoteWindow = 1 * time.Minute // 时间窗口大小
)

type VotingResult struct {
	Winner    uint `json:"winner"`
	Loser     uint `json:"loser"`
	FilterNum uint `json:"filternum"`
}

// 投票规则：score采用elo机制，胜率计算就是胜场/总场数

var K float64 = 4
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

	// 从 Redis 中获取上一次 getItem 的缓存
	var lastGetItem models.LastGetItem
	key := fmt.Sprintf("last_get_item:%s", userID)
	data, err := database.Rdb.Get(database.Ctx, key).Result()
	if err != nil {
		fmt.Println("从Redis获取lastGetItem失败:", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到上一次的道具"})
		return
	}
	if err := json.Unmarshal([]byte(data), &lastGetItem); err != nil {
		fmt.Println("解析Redis数据失败:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析上一次道具数据失败"})
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

	// 删除上一次的 getItem 缓存
	if err := database.Rdb.Del(database.Ctx, key).Err(); err != nil {
		fmt.Println("无法删除上一次的道具缓存:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法删除上一次的道具缓存"})
		return
	}

	Type := 0
	if voteResult == Nobody {
		Type = 1
	}

	// 储存到数据库

	var vote models.Vote
	vote.Winner = winnerItem.ItemID
	vote.Loser = loserItem.ItemID
	vote.Weight = weight
	vote.IP = ip
	vote.Type = Type

	database.DB.Save(&vote)

	// 更新items
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

	fmt.Println("投票结果:", winnerItem.Name, "得分:", winnerItem.Score, "胜率:", winnerItem.WinRate, "胜场:", winnerItem.WinCount, "总场次:", winnerItem.Total)
	fmt.Println("投票结果:", loserItem.Name, "得分:", loserItem.Score, "胜率:", loserItem.WinRate, "胜场:", loserItem.WinCount, "总场次:", loserItem.Total)

	database.DB.Save(&winnerItem)
	database.DB.Save(&loserItem)
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

const (
	// 定向投票每个IP每小时至多三次
	PVoteLimit  = 3
	PVoteWindow = 1 * time.Hour
)

// 辅助函数：计算道具排名，排名数字 = (比其得分更高的道具数量 + 1)
// 假设 Score 越高排名越靠前（1 表示第一名）
func getRank(item models.Item) (int, error) {
	var count int64
	if err := database.DB.Model(&models.Item{}).Where("score > ?", item.Score).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

type PvoteResult struct {
	Winner      uint   `json:"winner"`      //定向投票的胜者
	Loser       uint   `json:"loser"`       //定向投票的失败者
	Description string `json:"description"` // 定向投票的理由
}

func Pvote(c *gin.Context) {
	var PvoteRes PvoteResult
	if err := c.ShouldBindJSON(&PvoteRes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	if PvoteRes.Loser == 628 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申诉驳回：请勿乱投票，谢谢！"})
		return
	}

	ip := c.ClientIP()

	// 检查投票频率
	pvoteCountKey := fmt.Sprintf("pvote_count:%s", ip)
	pvoteCount, err := database.Rdb.Get(database.Ctx, pvoteCountKey).Int64()
	if err != nil && err != redis.Nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法检查定向投票频率"})
		return
	}

	fmt.Println("ip:", ip, "定向投票次数:", pvoteCount)

	// 同一个IP不能重复winner和loser完全相同的投票
	var pvote models.Pvote
	if err := database.DB.Where("winner = ? AND loser = ? AND ip = ?", PvoteRes.Winner, PvoteRes.Loser, ip).First(&pvote).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "请勿重复投票"})
		return
	}

	// 获取道具数据

	var winnerItem models.Item
	if err := database.DB.First(&winnerItem, "item_id = ?", PvoteRes.Winner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "胜利者道具未找到"})
		return
	}

	var loserItem models.Item
	if err := database.DB.First(&loserItem, "item_id = ?", PvoteRes.Loser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "失败者道具未找到"})
		return
	}

	// 计算winner和loser的排名
	winnerRank, err := getRank(winnerItem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取胜利者排名"})
		return
	}

	loserRank, err := getRank(loserItem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取失败者排名"})
		return
	}

	// 只有较低排名（数值更大的一方）才能获胜：要求 pvoteResult.Winner 对应的道具排名必须大于 pvoteResult.Loser
	if winnerRank <= loserRank {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申诉驳回：获胜道具必须是较弱的一方！"})
		return
	}
	// 对于排名前50的失败者，道具排名差不能超过20
	if loserRank <= 50 && (winnerRank-loserRank) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申诉驳回：排名差超过20!"})
		return
	}
	// 对于其他失败者，道具排名差不能超过300
	if loserRank > 50 && (winnerRank-loserRank) > 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "申诉驳回：排名差超过300！"})
		return
	}

	if pvoteCount >= PVoteLimit {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "申诉驳回：每个IP每小时只能申诉三次"})
		return
	}
	if err := database.Rdb.Incr(database.Ctx, pvoteCountKey).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法增加定向投票计数"})
		return
	}
	if err := database.Rdb.Expire(database.Ctx, pvoteCountKey, PVoteWindow).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法设置定向投票计数过期时间"})
		return
	}

	// 记录定向投票到数据库
	var pvoteRecord models.Pvote
	pvoteRecord.Winner = PvoteRes.Winner
	pvoteRecord.Loser = PvoteRes.Loser
	pvoteRecord.Description = PvoteRes.Description
	pvoteRecord.IP = ip

	database.DB.Save(&pvoteRecord)

	// 更新道具数据
	weight := 1.0
	var vote models.Vote
	vote.Winner = winnerItem.ItemID
	vote.Loser = loserItem.ItemID
	vote.Weight = weight
	vote.IP = ip
	vote.Type = 0
	if err := database.DB.Save(&vote).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存定向投票失败"})
		return
	}

	// 根据投票结果更新道具数据
	winnerItem.Score, loserItem.Score = elo(winnerItem.Score, loserItem.Score, weight)
	winnerItem.Total += weight
	loserItem.Total += weight
	winnerItem.WinCount += weight
	winnerItem.WinRate = winnerItem.WinCount / winnerItem.Total
	loserItem.WinRate = loserItem.WinCount / loserItem.Total

	if err := database.DB.Save(&winnerItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新胜者道具失败"})
		return
	}
	if err := database.DB.Save(&loserItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败者道具失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "结果已保存"})

}

func PvoteNum(c *gin.Context) {
	ip := c.ClientIP()
	pvoteCountKey := fmt.Sprintf("pvote_count:%s", ip)
	pvoteCount, err := database.Rdb.Get(database.Ctx, pvoteCountKey).Int64()
	if err != nil && err != redis.Nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法检查定向投票频率"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pvoteNum": pvoteCount})
}

func GetPvoteRecord(c *gin.Context) {
	ip := c.ClientIP()
	var pvoteRecords []models.Pvote
	if err := database.DB.Where("ip = ?", ip).Find(&pvoteRecords).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取定向投票记录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pvoteRecords": pvoteRecords})
}
