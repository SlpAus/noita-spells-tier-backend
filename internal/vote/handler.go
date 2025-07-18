package vote

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubmitVoteRequestBody 定义了前端提交投票时，请求体的JSON结构 (扁平化)
type SubmitVoteRequestBody struct {
	SpellAID  string     `json:"spellA" binding:"required"`
	SpellBID  string     `json:"spellB" binding:"required"`
	Result    VoteResult `json:"result" binding:"required"`
	PairID    string     `json:"pairId" binding:"required"`
	Signature string     `json:"signature" binding:"required"`
}

// SubmitVote 处理前端提交的投票结果
func SubmitVote(c *gin.Context) {
	var body SubmitVoteRequestBody
	// 1. 绑定并验证请求体
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	// 2. 构造TokenPayload以用于验证
	payloadToValidate := token.TokenPayload{
		PairID:   body.PairID,
		SpellAID: body.SpellAID,
		SpellBID: body.SpellBID,
	}

	// 3. 验证签名
	if !token.ValidateVoteSignature(payloadToValidate, body.Signature) {
		c.JSON(http.StatusOK, gin.H{"message": "投票已记录"}) // 静默失败
		return
	}

	// 4. 验证PairID (占位符)
	if !ValidatePairID(body.PairID) {
		c.JSON(http.StatusOK, gin.H{"message": "投票已记录"}) // 静默失败
		return
	}

	// 5. 从上下文中获取userID
	userID := c.GetString(user.UserIDKey)

	// 6. 验证并激活用户
	parsedUUID, err := uuid.Parse(userID)
	if err != nil || parsedUUID.Version() != 7 {
		userID = "" // 如果格式或版本不合法，视为匿名用户
	} else {
		// 检查时间戳是否在未来
		uuidTimestamp := parsedUUID.Time()
		sec, nsec := uuidTimestamp.UnixTime()
		uuidTime := time.Unix(sec, nsec)
		if !uuidTime.Before(time.Now()) {
			userID = "" // 如果时间戳不合法，视为匿名用户
		}
	}

	if userID != "" {
		if err := user.ActivateUser(userID); err != nil {
			fmt.Printf("激活用户 %s 失败: %v\n", userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "处理用户会话失败"})
			return
		}
	}

	// 7. 调用核心服务处理投票
	if err := ProcessVote(body.SpellAID, body.SpellBID, body.Result, userID); err != nil {
		fmt.Printf("处理投票时发生错误: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理投票时发生内部错误"})
		return
	}

	// 8. 成功返回
	c.JSON(http.StatusOK, gin.H{"message": "投票成功"})
}
