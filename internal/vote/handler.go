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

// SubmitVoteRequestBody 定义了前端提交投票时，请求体的JSON结构
type SubmitVoteRequestBody struct {
	SpellAID  string     `json:"spellA" binding:"required"`
	SpellBID  string     `json:"spellB" binding:"required"`
	Result    VoteResult `json:"result" binding:"required"`
	PairID    string     `json:"pairId" binding:"required"`
	Signature string     `json:"signature" binding:"required"`
}

// SubmitVote 处理前端提交的投票结果
func SubmitVote(c *gin.Context) {
	// 1. 服务降级检查
	if !database.IsRedisHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "服务暂时不可用，请稍后重试"})
		return
	}

	var body SubmitVoteRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	// 2. 签名验证
	payloadToValidate := token.TokenPayload{PairID: body.PairID, SpellAID: body.SpellAID, SpellBID: body.SpellBID}
	if !token.ValidateVoteSignature(payloadToValidate, body.Signature) {
		c.JSON(http.StatusOK, gin.H{"message": "投票已记录"}) // 静默失败
		return
	}

	// 3. 防重放攻击检查
	isReplay, err := CheckAndUsePairID(body.PairID)
	if err != nil {
		fmt.Printf("检查PairID %s 时发生错误: %v\n", body.PairID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证投票时发生内部错误"})
		return
	}
	if isReplay {
		c.JSON(http.StatusOK, gin.H{"message": "投票已记录"}) // 静默失败
		return
	}

	// 4. IP频率限制 (带补偿操作)
	ip := c.ClientIP()
	voteTime := time.Now()
	count, compensator, err := IncrementIPVoteCount(ip, voteTime)
	if err != nil {
		fmt.Printf("IP计数器失败 for IP %s: %v\n", ip, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理投票时发生内部错误"})
		return
	}
	defer compensator.RollbackUnlessCommitted() // 默认在函数结束时执行回滚

	// 5. 计算投票权重
	multiplier := calculateMultiplierForCount(count)

	// 6. 验证用户
	userID := c.GetString(user.UserIDKey)
	if !user.IsValidUUID(userID) {
		userID = ""
	}

	// 7. 构造最终的投票记录
	newVote := Vote{
		SpellA_ID:      body.SpellAID,
		SpellB_ID:      body.SpellBID,
		Result:         body.Result,
		UserIdentifier: userID,
		UserIP:         ip,
		Multiplier:     multiplier,
		VoteTime:       voteTime,
	}

	// 8. 持久化投票事件到SQLite (带重试)
	const maxRetry = 3
	const delay = 50 * time.Millisecond

	var createErr error
	for i := 0; i < maxRetry; i++ {
		createErr = database.DB.Create(&newVote).Error
		if createErr == nil {
			break
		}
		if !database.IsRetryableError(createErr) {
			break
		}
		time.Sleep(delay)
	}
	if createErr != nil {
		fmt.Printf("严重错误: 无法将vote写入SQLite: %v\n", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法记录投票"})
		// IP计数器的补偿操作将在这里被defer自动调用
		return
	}

	// 9. 确认IP计数器的更改
	compensator.Commit()

	// 10. 提交到后台处理器
	submitVoteToQueue(newVote)

	// 11. 成功返回
	c.JSON(http.StatusOK, gin.H{"message": "投票成功"})
}
