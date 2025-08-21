package report

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	"github.com/gin-gonic/gin"
)

// --- 模式 ---
var appMode config.AppMode

func initHandlerMode(mode config.AppMode) {
	appMode = mode
}

// UserReport 是最终生成并返回给用户的个性化报告。
// 字段的 omitempty 标签表示如果该字段为零值（如0, "", nil），则在JSON序列化时忽略它。
// 这对于动态决定是否包含某些指标非常有用。
type SpellUserReport struct {
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
	MostSubversive *SpellHighlightVote  `json:"mostSubversive,omitempty"` // 最颠覆的对决

	// --- 里程碑与记录 ---
	FirstVote            *SpellMilestoneVote   `json:"firstVote,omitempty"`
	Milestones           []SpellMilestoneVote  `json:"milestones,omitempty"`           // 25, 100, 500, 1000票等
	BusiestDay           *ActivityRecord       `json:"busiestDay,omitempty"`           // 最肝的一天/24小时
	FirstEncounterTop    *SpellEncounterRecord `json:"firstEncounterTop,omitempty"`    // 首次遭遇顶级法术
	FirstEncounterBottom *SpellEncounterRecord `json:"firstEncounterBottom,omitempty"` // 首次遭遇垫底法术
}

type PerkUserReport struct {
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
	MostSubversive *PerkHighlightVote   `json:"mostSubversive,omitempty"` // 最颠覆的对决

	// --- 里程碑与记录 ---
	FirstVote            *PerkMilestoneVote   `json:"firstVote,omitempty"`
	Milestones           []PerkMilestoneVote  `json:"milestones,omitempty"`           // 25, 100, 500, 1000票等
	BusiestDay           *ActivityRecord      `json:"busiestDay,omitempty"`           // 最肝的一天/24小时
	FirstEncounterTop    *PerkEncounterRecord `json:"firstEncounterTop,omitempty"`    // 首次遭遇顶级天赋
	FirstEncounterBottom *PerkEncounterRecord `json:"firstEncounterBottom,omitempty"` // 首次遭遇垫底天赋
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
type SpellHighlightVote struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"spellA"`
	SpellB     SpellNameRank   `json:"spellB"`
	Result     vote.VoteResult `json:"result"`
}
type PerkHighlightVote struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"perkA"` // 改名
	SpellB     SpellNameRank   `json:"perkB"` // 改名
	Result     vote.VoteResult `json:"result"`
}

// MilestoneVote 记录用户的里程碑时刻。
type SpellMilestoneVote struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"spellA"` // 不使用Rank
	SpellB     SpellNameRank   `json:"spellB"`
	Result     vote.VoteResult `json:"result"`
	Date       time.Time       `json:"date"` // 对齐到天
}
type PerkMilestoneVote struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"perkA"` // 不使用Rank
	SpellB     SpellNameRank   `json:"perkB"`
	Result     vote.VoteResult `json:"result"`
	Date       time.Time       `json:"date"` // 对齐到天
}

// ActivityRecord 记录用户的活跃数据。
type ActivityRecord struct {
	FromDate  time.Time `json:"fromDate"` // 对齐到天
	ToDate    time.Time `json:"toDate"`
	VoteCount int       `json:"voteCount"`
}

// EncounterRecord 记录首次遇到特殊法术的事件。
type SpellEncounterRecord struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"spellA"`
	SpellB     SpellNameRank   `json:"spellB"`
	SpecialA   bool            `json:"specialA"`
	SpecialB   bool            `json:"specialB"`
	Result     vote.VoteResult `json:"result"`
	Date       time.Time       `json:"date"` // 对齐到天
}
type PerkEncounterRecord struct {
	VoteNumber int             `json:"voteNumber"`
	SpellA     SpellNameRank   `json:"perkA"`
	SpellB     SpellNameRank   `json:"perkB"`
	SpecialA   bool            `json:"specialA"`
	SpecialB   bool            `json:"specialB"`
	Result     vote.VoteResult `json:"result"`
	Date       time.Time       `json:"date"` // 对齐到天
}

func GetReport(c *gin.Context) {
	userID := c.GetString(user.UserIDKey)
	if !user.IsValidUUID(userID) {
		userID = ""
	}

	report, err := GenerateUserReport(userID)
	if err != nil {
		fmt.Printf("%v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成报告时时发生内部错误"})
		return
	}

	switch appMode {
	case config.AppModeSpell:
		c.JSON(http.StatusOK, report)
	case config.AppModePerk:
		c.JSON(http.StatusOK, SpellUserReportToPerkUserReport(report))
	}
}

func SpellUserReportToPerkUserReport(origin *SpellUserReport) *PerkUserReport {
	if origin == nil {
		return nil
	}

	target := PerkUserReport{}
	target.UserID = origin.UserID
	target.GeneratedAt = origin.GeneratedAt

	target.TotalVotes = origin.TotalVotes
	target.VoteRankPercent = origin.VoteRankPercent
	target.Choices = origin.Choices
	target.DecisionRate = origin.DecisionRate
	target.CommunityDecisionRate = origin.CommunityDecisionRate

	target.CommunityConsistencyIndex = origin.CommunityConsistencyIndex
	target.UpsetTendency = origin.UpsetTendency

	target.MostChosen = origin.MostChosen
	target.HighestWinRate = origin.HighestWinRate
	target.ChosenOne = origin.ChosenOne
	target.Nemesis = origin.Nemesis
	if origin.MostSubversive != nil {
		target.MostSubversive = &PerkHighlightVote{
			VoteNumber: origin.MostSubversive.VoteNumber,
			SpellA:     origin.MostSubversive.SpellA,
			SpellB:     origin.MostSubversive.SpellB,
			Result:     origin.MostSubversive.Result,
		}
	}

	if origin.FirstVote != nil {
		target.FirstVote = &PerkMilestoneVote{
			VoteNumber: origin.FirstVote.VoteNumber,
			SpellA:     origin.FirstVote.SpellA,
			SpellB:     origin.FirstVote.SpellB,
			Result:     origin.FirstVote.Result,
			Date:       origin.FirstVote.Date,
		}
	}
	if origin.Milestones != nil {
		target.Milestones = make([]PerkMilestoneVote, 0, len(origin.Milestones))
		for _, value := range origin.Milestones {
			target.Milestones = append(target.Milestones, PerkMilestoneVote{
				VoteNumber: value.VoteNumber,
				SpellA:     value.SpellA,
				SpellB:     value.SpellB,
				Result:     value.Result,
				Date:       value.Date,
			})
		}
	}
	target.BusiestDay = origin.BusiestDay
	if origin.FirstEncounterTop != nil {
		target.FirstEncounterTop = &PerkEncounterRecord{
			VoteNumber: origin.FirstEncounterTop.VoteNumber,
			SpellA:     origin.FirstEncounterTop.SpellA,
			SpellB:     origin.FirstEncounterTop.SpellB,
			SpecialA:   origin.FirstEncounterTop.SpecialA,
			SpecialB:   origin.FirstEncounterTop.SpecialB,
			Result:     origin.FirstEncounterTop.Result,
			Date:       origin.FirstEncounterTop.Date,
		}
	}
	if origin.FirstEncounterBottom != nil {
		target.FirstEncounterBottom = &PerkEncounterRecord{
			VoteNumber: origin.FirstEncounterBottom.VoteNumber,
			SpellA:     origin.FirstEncounterBottom.SpellA,
			SpellB:     origin.FirstEncounterBottom.SpellB,
			SpecialA:   origin.FirstEncounterBottom.SpecialA,
			SpecialB:   origin.FirstEncounterBottom.SpecialB,
			Result:     origin.FirstEncounterBottom.Result,
			Date:       origin.FirstEncounterBottom.Date,
		}
	}
	return &target
}
