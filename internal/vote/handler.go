package vote

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/gin-gonic/gin"
)

// SubmitVoteRequestBody ... (结构体定义不变)
type SubmitVoteRequestBody struct {
	SpellAID  string     `json:"spellA" binding:"required"`
	SpellBID  string     `json:"spellB" binding:"required"`
	Result    VoteResult `json:"result" binding:"required"`
	PairID    string     `json:"pairId" binding:"required"`
	Signature string     `json:"signature" binding:"required"`
}

// SubmitVote 处理前端提交的投票结果
func SubmitVote(c *gin.Context) {
	// *** 新增：服务降级逻辑 ***
	if !database.IsRedisHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务暂时不可用，请稍后重试"})
		return
	}

	var body SubmitVoteRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	payloadToValidate := token.TokenPayload{PairID: body.PairID, SpellAID: body.SpellAID, SpellBID: body.SpellBID}
	if !token.ValidateVoteSignature(payloadToValidate, body.Signature) {
		c.JSON(http.StatusOK, gin.H{"message": "投票已记录"})
		return
	}

	userID := c.GetString(user.UserIDKey)
	if !user.QueueUserActivationIfValid(userID) {
		userID = ""
	}

	newVote := Vote{
		SpellA_ID:      body.SpellAID,
		SpellB_ID:      body.SpellBID,
		Result:         body.Result,
		UserIdentifier: userID,
		UserIP:         c.ClientIP(),
		Multiplier:     1.0,
		VoteTime:       time.Now(),
	}

	var err error
	for i := 0; i < 3; i++ {
		err = database.DB.Create(&newVote).Error
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		fmt.Printf("严重错误: 无法将vote写入SQLite: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法记录投票"})
		return
	}

	submitVoteToQueue(newVote)

	c.JSON(http.StatusOK, gin.H{"message": "投票成功"})
}
