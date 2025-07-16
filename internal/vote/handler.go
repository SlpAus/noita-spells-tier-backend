package vote

import (
	"errors"
	"math"
	"net/http"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause" // *** 新增的导入语句 ***
)

// VoteRequestBody 定义了前端提交投票时，请求体的JSON结构
type VoteRequestBody struct {
	SpellA_ID string             `json:"spell_a_id" binding:"required"`
	SpellB_ID string             `json:"spell_b_id" binding:"required"`
	Result    VoteResult  `json:"result" binding:"required"`
	// UserIdentifier 暂时不用，为后续阶段做准备
	// UserIdentifier string          `json:"user_identifier"`
}

// eloKFactor 是ELO算法中的K值，它决定了每次对战后分数变化的大小。
// 值越高，分数变化越剧烈。32是一个常用的标准值。
const eloKFactor = 32

// calculateElo 计算对战后的新ELO分数
// 它接受胜者和败者的当前分数，返回他们的新分数
func calculateElo(winnerScore, loserScore float64) (newWinnerScore, newLoserScore float64) {
	// ELO算法公式：
	// 1. 计算双方的期望胜率
	expectedWinner := 1.0 / (1.0 + math.Pow(10, (loserScore-winnerScore)/400.0))
	expectedLoser := 1.0 / (1.0 + math.Pow(10, (winnerScore-loserScore)/400.0))

	// 2. 根据实际结果(胜=1, 负=0)和期望胜率，更新分数
	newWinnerScore = winnerScore + eloKFactor*(1-expectedWinner)
	newLoserScore = loserScore + eloKFactor*(0-expectedLoser)

	return
}

// SubmitVote 处理前端提交的投票结果
func SubmitVote(c *gin.Context) {
	var body VoteRequestBody

	// 1. 绑定并验证请求体
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	// 2. 使用数据库事务来保证数据一致性
	// GORM的Transaction会确保内部的所有数据库操作要么全部成功，要么全部失败回滚。
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var spellA, spellB spell.Spell

		// 在事务中查询两个法术，使用Clauses(clause.Locking{Strength: "UPDATE"})来锁定行，防止并发问题
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("spell_id = ?", body.SpellA_ID).First(&spellA).Error; err != nil {
			return errors.New("找不到法术: " + body.SpellA_ID)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("spell_id = ?", body.SpellB_ID).First(&spellB).Error; err != nil {
			return errors.New("找不到法术: " + body.SpellB_ID)
		}

		// 3. 根据投票结果，更新分数和场次
		switch body.Result {
		case ResultAWins:
			spellA.Score, spellB.Score = calculateElo(spellA.Score, spellB.Score)
			spellA.Win++
			spellA.Total++
			spellB.Total++
		case ResultBWins:
			spellB.Score, spellA.Score = calculateElo(spellB.Score, spellA.Score)
			spellB.Win++
			spellB.Total++
			spellA.Total++
		case ResultDraw:
			spellA.Total++
			spellB.Total++
		case ResultSkip:
			// 跳过，不更新任何分数和场次
		default:
			return errors.New("无效的投票结果")
		}

		// 4. 保存对两个法术的更新
		if err := tx.Save(&spellA).Error; err != nil {
			return err
		}
		if err := tx.Save(&spellB).Error; err != nil {
			return err
		}

		// 5. 创建并保存一条新的投票记录
		newVote := Vote{
			SpellA_ID: body.SpellA_ID,
			SpellB_ID: body.SpellB_ID,
			Result:    body.Result,
			// UserIdentifier: body.UserIdentifier, // 暂时不用
		}
		if err := tx.Create(&newVote).Error; err != nil {
			return err
		}

		// 如果所有操作都成功，事务会自动提交
		return nil
	})

	// 6. 处理事务的结果
	if err != nil {
		// 如果事务失败，返回500错误和具体的失败原因
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理投票失败: " + err.Error()})
		return
	}

	// 7. 成功返回
	c.JSON(http.StatusOK, gin.H{"message": "投票成功"})
}
