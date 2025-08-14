package vote

import (
	"encoding/json"
	"fmt"
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

// UnmarshalJSON 为 VoteResult 类型实现了 json.Unmarshaler 接口。
// 这使得我们可以自定义JSON解析和验证逻辑。
func (r *VoteResult) UnmarshalJSON(data []byte) error {
	// 1. 正常解析
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// 2. 检查解析出的字符串是否符合枚举
	switch VoteResult(s) {
	case ResultAWins, ResultBWins, ResultDraw, ResultSkip:
		// 3. 如果是有效值，则将其赋值给我们的接收者
		*r = VoteResult(s)
		return nil
	default:
		// 4. 如果不是有效值，则返回一个错误，这将导致外层的JSON解析失败
		return fmt.Errorf("无效的 VoteResult 值: '%s'。", s)
	}
}

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
