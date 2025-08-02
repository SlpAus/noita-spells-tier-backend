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
	gorm.Model

	SpellA_ID string
	SpellB_ID string
	Result    VoteResult

	UserIdentifier string `gorm:"index"`
	UserIP         string
	Multiplier     float64
	VoteTime       time.Time `gorm:"index"`
}
