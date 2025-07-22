package vote

import (
	"time"

	"gorm.io/gorm"
)

// VoteResult 定义了投票结果的枚举类型
type VoteResult string

const (
	ResultAWins VoteResult = "A_WINS"
	ResultBWins VoteResult = "B_WINS"
	ResultDraw  VoteResult = "DRAW"
	ResultSkip  VoteResult = "SKIP"
)

// Vote 定义了单次投票记录的数据结构 (事件日志)
type Vote struct {
	// gorm.Model 提供了自增的ID主键, CreatedAt, UpdatedAt, DeletedAt
	gorm.Model

	SpellA_ID string     `json:"spell_a_id"`
	SpellB_ID string     `json:"spell_b_id"`
	Result    VoteResult `json:"result"`

	// --- 上下文信息 ---
	UserIdentifier string    `json:"user_identifier"`      // 用户的UUID
	UserIP         string    `json:"user_ip" gorm:"index"` // 用户的IP地址
	Multiplier     float64   `json:"multiplier"`           // 本次投票的权重/惩罚倍率
	VoteTime       time.Time `json:"vote_time"`            // 精确的投票时间
}
