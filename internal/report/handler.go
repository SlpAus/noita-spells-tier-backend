package report

import (
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
)

// UserReport 是最终生成并返回给用户的个性化报告。
// 字段的 omitempty 标签表示如果该字段为零值（如0, "", nil），则在JSON序列化时忽略它。
// 这对于动态决定是否包含某些指标非常有用。
type UserReport struct {
	UserID      string    `json:"userId"`
	GeneratedAt time.Time `json:"generatedAt"`

	// --- 基础统计 ---
	TotalVotes            int          `json:"totalVotes"`
	VoteRankPercent       float64      `json:"voteRankpercent"` // 在所有投票者中的位置比例
	Choices               ChoiceCounts `json:"choices"`
	DecisionRate          *float64     `json:"decisionRate,omitempty"`          // 决断率
	CommunityDecisionRate *float64     `json:"communityDecisionRate,omitempty"` // 社区平均决断率

	// --- 投票倾向 ---
	CommunityConsistencyIndex *float64 `json:"communityConsistencyIndex,omitempty"` // 社区一致性指数
	UpsetTendency             *float64 `json:"upsetTendency,omitempty"`             // 以弱胜强倾向

	// --- 趣味高光时刻 ---
	MostChosen     *MostChosenSpell     `json:"mostChosen,omitempty"`     // 最多选择
	HighestWinRate *HighestWinRateSpell `json:"highestWinRate,omitempty"` // 最高个人胜率
	ChosenOne      *ContrarianSpell     `json:"chosenOne,omitempty"`      // 天选之子
	Nemesis        *ContrarianSpell     `json:"nemesis,omitempty"`        // 一生之敌
	MostSubversive *HighlightVote       `json:"mostSubversive,omitempty"` // 最颠覆的对决

	// --- 里程碑与记录 ---
	FirstVote            *MilestoneVote   `json:"firstVote,omitempty"`
	Milestones           []MilestoneVote  `json:"milestones,omitempty"`           // 25, 100, 500, 1000票等
	BusiestDay           *ActivityRecord  `json:"busiestDay,omitempty"`           // 最肝的一天/24小时
	FirstEncounterTop    *EncounterRecord `json:"firstEncounterTop,omitempty"`    // 首次遭遇顶级法术
	FirstEncounterBottom *EncounterRecord `json:"firstEncounterBottom,omitempty"` // 首次遭遇垫底法术
}

// ChoiceCounts 记录了用户做出不同选择的次数。
type ChoiceCounts struct {
	Wins int `json:"wins"` // 选择A或B
	Draw int `json:"draw"` // 双输
	Skip int `json:"skip"` // 跳过
}

// MostChosenSpell 记录了用户选择最多的法术。
type MostChosenSpell struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VoteCount int    `json:"voteCount"`
}

// HighestWinRateSpell 记录了用户最偏好的法术。
type HighestWinRateSpell struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	WinRate float64 `json:"winRate"`
}

// ContrarianSpell 记录了用户观点最独特的法术。
type ContrarianSpell struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	WinRate            float64 `json:"winRate"`
	CommunityScoreRate float64 `json:"communityScoreRate"`
}

// SpellNameRank 是带名字和排名的法术。
type SpellNameRank struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Rank int64  `json:"rank,omitempty"`
}

// HighlightVote 用于记录一次特殊的投票。
type HighlightVote struct {
	SpellA SpellNameRank   `json:"spellA"`
	SpellB SpellNameRank   `json:"spellB"`
	Result vote.VoteResult `json:"result"`
}

// MilestoneVote 记录用户的里程碑时刻。
type MilestoneVote struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"spellA"` // 不使用Rank
	SpellB     SpellNameRank   `json:"spellB"`
	Result     vote.VoteResult `json:"result"`
}

// ActivityRecord 记录用户的活跃数据。
type ActivityRecord struct {
	FromDate  time.Time `json:"fromDate"` // 对齐到天
	ToDate    time.Time `json:"toDate"`
	VoteCount int       `json:"voteCount"`
}

// EncounterRecord 记录首次遇到特殊法术的事件。
type EncounterRecord struct {
	SpellA   SpellNameRank   `json:"spellA"`
	SpellB   SpellNameRank   `json:"spellB"`
	SpecialA bool            `json:"specialA"`
	SpecialB bool            `json:"specialB"`
	Result   vote.VoteResult `json:"result"`
	Date     time.Time       `json:"date"` // 对齐到天
}
