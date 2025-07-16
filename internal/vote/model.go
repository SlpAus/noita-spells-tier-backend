package vote

import (
	"gorm.io/gorm"
)

// VoteResult 定义了投票结果的枚举类型
type VoteResult string

const (
	// ResultAWins 表示法术A获胜
	ResultAWins VoteResult = "A_WINS"
	// ResultBWins 表示法术B获胜
	ResultBWins VoteResult = "B_WINS"
	// ResultDraw 表示平局或双输
	ResultDraw VoteResult = "DRAW"
	// ResultSkip 表示用户跳过了此轮投票
	ResultSkip VoteResult = "SKIP"
)

// Vote 定义了单次投票记录的数据结构
// 它清晰地记录了参与对决的双方和最终结果
type Vote struct {
	gorm.Model

	// SpellA_ID 是参与对决的第一个法术的ID
	SpellA_ID string `json:"spell_a_id"`

	// SpellB_ID 是参与对决的第二个法术的ID
	SpellB_ID string `json:"spell_b_id"`

	// Result 记录本次投票的结果
	Result VoteResult `json:"result"`

	// UserIdentifier 是用于识别用户的唯一标识
	UserIdentifier string `json:"user_identifier"`
}
