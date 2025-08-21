package report

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
)

const (
	// 1.MinVotesForDecisionRate 是包含决断率字段所需的最小总投票数。
	MinVotesForDecisionRate = 5

	// 2.MinWinsForTendency 是包含投票倾向相关字段所需的最小胜负投票数。
	MinWinsForTendency = 5

	// 3.MinWinsForMostChosen 是法术被计入“最常选择”所需的最小胜利次数。
	MinWinsForMostChosen = 2

	// 6.MaxMilestones 是报告中最多显示的里程碑数量。
	MaxMilestones = 4

	// 7.MinVotesForBusiestDay 是计入“最肝的一天”所需的最小投票数。
	MinVotesForBusiestDay = 5
)

const (
	TotalVotesToSpellsRatioForWinRateForSpellMode = 2.0
	MinTotalGamesForWinRateForSpellMode           = 2
	TopTierRatioForSpellMode                      = 0.025
	BottomTierRatioForSpellMode                   = 0.025
)

const (
	TotalVotesToSpellsRatioForWinRateForPerkMode = 3.0
	MinTotalGamesForWinRateForPerkMode           = 3
	TopTierRatioForPerkMode                      = 0.05
	BottomTierRatioForPerkMode                   = 0.05
)

var (
	// 4.TotalVotesToSpellsRatioForWinRate 是计算“最高胜率”法术的准入门槛，
	// 用户的总投票数需要达到 (总法术数 * 这个倍数)。
	TotalVotesToSpellsRatioForWinRate float64

	// 5.MinTotalGamesForWinRate 是法术胜率被计算所需的最小有效场次。
	MinTotalGamesForWinRate int

	// 8.TopTierRatio 定义了被视为“顶级”法术的排名比例。
	TopTierRatio float64

	// 9.BottomTierRatio 定义了被视为“垫底”法术的排名比例。
	BottomTierRatio float64
)

func loadAlgorithmConsts(mode config.AppMode) {
	switch mode {
	case config.AppModeSpell:
		TotalVotesToSpellsRatioForWinRate = TotalVotesToSpellsRatioForWinRateForSpellMode
		MinTotalGamesForWinRate = MinTotalGamesForWinRateForSpellMode
		TopTierRatio = TopTierRatioForSpellMode
		BottomTierRatio = BottomTierRatioForSpellMode
	case config.AppModePerk:
		TotalVotesToSpellsRatioForWinRate = TotalVotesToSpellsRatioForWinRateForPerkMode
		MinTotalGamesForWinRate = MinTotalGamesForWinRateForPerkMode
		TopTierRatio = TopTierRatioForPerkMode
		BottomTierRatio = BottomTierRatioForPerkMode
	}
}

// milestoneNumbers 定义了我们关心的特定投票数里程碑。
var milestoneNumbers = []int{25, 50, 100, 250, 500, 1000}

// calculateDecisionRate 根据用户统计数据计算决断率。
// 决断率 = (有效选择数) / (总投票数)
// 其中，平局(Draw)算作0.5个有效选择。
func calculateDecisionRate(stats user.UserStats) float64 {
	totalVotes := stats.Wins + stats.Draw + stats.Skip

	if totalVotes == 0 {
		return 0.0
	}

	decisionRate := (float64(stats.Wins) + float64(stats.Draw)*0.5) / float64(totalVotes)
	return decisionRate
}

// calculateCommunityConsistencyIndex 计算社区一致性指数。
// 指数 = (胜者排名高于败者的次数) / (总胜负次数)
func calculateCommunityConsistencyIndex(userVotes []userVoteRecord, spellRank map[string]int) (float64, error) {
	if spellRank == nil {
		return 0.0, fmt.Errorf("spellRank 不能为nil")
	}

	var winLossVotes int
	var consistentWins int

	for _, userVote := range userVotes {
		if userVote.Result == vote.ResultAWins || userVote.Result == vote.ResultBWins {
			winLossVotes++
			winnerRank, ok := spellRank[userVote.SpellA_ID]
			if !ok {
				return 0.0, fmt.Errorf("法术 %s 不存在", userVote.SpellA_ID)
			}
			loserRank, ok := spellRank[userVote.SpellB_ID]
			if !ok {
				return 0.0, fmt.Errorf("法术 %s 不存在", userVote.SpellB_ID)
			}

			if userVote.Result == vote.ResultBWins {
				winnerRank, loserRank = loserRank, winnerRank
			}

			if winnerRank < loserRank {
				consistentWins++
			}
		}
	}

	if winLossVotes == 0 {
		return 0.0, nil
	}
	return float64(consistentWins) / float64(winLossVotes), nil
}

// calculateUpsetTendency 计算以弱胜强倾向指数。
// 指数归一化到 [0, 1] 区间，0.5代表无倾向。
func calculateUpsetTendency(userVotes []userVoteRecord, spellRankScore map[string]float64) (float64, error) {
	if spellRankScore == nil {
		return 0.0, fmt.Errorf("spellRankScore 不能为nil")
	}

	var winLossVotes int
	var upsetScoreSum float64

	for _, userVote := range userVotes {
		if userVote.Result == vote.ResultAWins || userVote.Result == vote.ResultBWins {
			winLossVotes++
			winnerScore, ok := spellRankScore[userVote.SpellA_ID]
			if !ok {
				return 0.0, fmt.Errorf("法术 %s 不存在", userVote.SpellA_ID)
			}
			loserScore, ok := spellRankScore[userVote.SpellB_ID]
			if !ok {
				return 0.0, fmt.Errorf("法术 %s 不存在", userVote.SpellB_ID)
			}

			if userVote.Result == vote.ResultBWins {
				winnerScore, loserScore = loserScore, winnerScore
			}

			upsetScoreSum += (loserScore - winnerScore)
		}
	}

	if winLossVotes == 0 {
		return 0.5, nil // 无胜负记录，返回中值
	}

	// 归一化处理
	normalized := (upsetScoreSum/float64(winLossVotes))*0.5 + 0.5
	return normalized, nil
}

// calculateMostChosenSpell 计算用户最常选择的法术。
func calculateMostChosenSpell(userVotes []userVoteRecord) (*MostChosenSpell, error) {
	spellWinCounts := make(map[string]int)
	var mostChosenSpellID string
	maxWins := 0

	for _, userVote := range userVotes {
		var winnerID string
		if userVote.Result == vote.ResultAWins {
			winnerID = userVote.SpellA_ID
		} else if userVote.Result == vote.ResultBWins {
			winnerID = userVote.SpellB_ID
		}

		if winnerID != "" {
			spellWinCounts[winnerID]++
			if spellWinCounts[winnerID] > maxWins {
				maxWins = spellWinCounts[winnerID]
				mostChosenSpellID = winnerID
			}
		}
	}

	if maxWins < MinWinsForMostChosen {
		return nil, nil
	}

	mostChosenSpellName, err := getSpellNameByID(mostChosenSpellID)
	if err != nil {
		return nil, err
	}

	return &MostChosenSpell{
		ID:        mostChosenSpellID,
		Name:      mostChosenSpellName,
		VoteCount: maxWins,
	}, nil
}

// 从 []userVoteRecord 计算其包含的法术的胜率，排除有效场次小于 MinTotalGamesForWinRate 的法术。
func calcualteWinRateForSpells(userVotes []userVoteRecord) map[string]float64 {
	totalSpells := spell.GetSpellCount()
	spellWinRates := make(map[string]float64, totalSpells)
	type gameStats struct {
		wins  int
		loses int
	}
	spellGameCounts := make(map[string]gameStats, totalSpells)

	for _, userVote := range userVotes {
		statsA := spellGameCounts[userVote.SpellA_ID]
		statsB := spellGameCounts[userVote.SpellB_ID]

		switch userVote.Result {
		case vote.ResultAWins:
			statsA.wins++
			statsB.loses++
		case vote.ResultBWins:
			statsA.loses++
			statsB.wins++
		case vote.ResultDraw:
			statsA.loses++
			statsB.loses++
		}
		spellGameCounts[userVote.SpellA_ID] = statsA
		spellGameCounts[userVote.SpellB_ID] = statsB
	}

	for spellID, stats := range spellGameCounts {
		totalGames := stats.wins + stats.loses
		if totalGames >= MinTotalGamesForWinRate {
			spellWinRates[spellID] = float64(stats.wins) / float64(totalGames)
		}
	}

	return spellWinRates
}

// calculateHighestWinRateSpell 计算用户胜率最高的法术。
func calculateHighestWinRateSpell(spellWinRates map[string]float64) (*HighestWinRateSpell, error) {
	if spellWinRates == nil {
		return nil, fmt.Errorf("spellWinRates 不能为nil")
	}

	var bestWinRate float64
	var candidates []string

	for spellID, winRate := range spellWinRates {
		if winRate > bestWinRate {
			bestWinRate = winRate
			candidates = []string{spellID}
		} else if winRate == bestWinRate {
			candidates = append(candidates, spellID)
		}
	}

	if bestWinRate == 0.0 || len(candidates) == 0 {
		return nil, nil
	}

	// 从候选者中随机选择一个
	chosenID := candidates[rand.IntN(len(candidates))]

	chosenName, err := getSpellNameByID(chosenID)
	if err != nil {
		return nil, err
	}

	return &HighestWinRateSpell{
		ID:      chosenID,
		Name:    chosenName,
		WinRate: bestWinRate,
	}, nil
}

// calculateChosenOne 计算用户的“天选之子”法术。
// 即个人胜率远高于社区平均水平的法术。
func calculateChosenOne(spellWinRates map[string]float64, spellRankScore map[string]float64) (*ContrarianSpell, error) {
	if spellWinRates == nil || spellRankScore == nil {
		return nil, fmt.Errorf("传入的map不能为nil")
	}

	var maxDiff float64 = 0
	var chosenOneID string

	for spellID, winRate := range spellWinRates {
		rankScore, ok := spellRankScore[spellID]
		if !ok {
			return nil, fmt.Errorf("法术 %s 在spellRankScore中不存在", spellID)
		}
		diff := winRate - rankScore
		if diff > maxDiff {
			maxDiff = diff
			chosenOneID = spellID
		}
	}

	if chosenOneID == "" {
		return nil, nil
	}

	chosenOneName, err := getSpellNameByID(chosenOneID)
	if err != nil {
		return nil, err
	}

	return &ContrarianSpell{
		ID:                 chosenOneID,
		Name:               chosenOneName,
		WinRate:            spellWinRates[chosenOneID],
		CommunityScoreRate: spellRankScore[chosenOneID],
	}, nil
}

// calculateNemesis 计算用户的“一生之敌”法术。
// 即个人胜率远低于社区平均水平的法术。
func calculateNemesis(spellWinRates map[string]float64, spellRankScore map[string]float64) (*ContrarianSpell, error) {
	if spellWinRates == nil || spellRankScore == nil {
		return nil, fmt.Errorf("传入的map不能为nil")
	}

	var minDiff float64 = 0
	var nemesisID string

	for spellID, winRate := range spellWinRates {
		rankScore, ok := spellRankScore[spellID]
		if !ok {
			return nil, fmt.Errorf("法术 %s 在spellRankScore中不存在", spellID)
		}
		diff := winRate - rankScore
		if diff < minDiff {
			minDiff = diff
			nemesisID = spellID
		}
	}

	if nemesisID == "" {
		return nil, nil
	}

	nemesisName, err := getSpellNameByID(nemesisID)
	if err != nil {
		return nil, err
	}

	return &ContrarianSpell{
		ID:                 nemesisID,
		Name:               nemesisName,
		WinRate:            spellWinRates[nemesisID],
		CommunityScoreRate: spellRankScore[nemesisID],
	}, nil
}

// calculateMostSubversiveVote 计算用户最具颠覆性的一票。
// 即选择的胜者，其社区排名远低于败者。
func calculateMostSubversiveVote(userVotes []userVoteRecord, spellRank map[string]int) (*SpellHighlightVote, error) {
	if spellRank == nil {
		return nil, fmt.Errorf("spellRank 不能为nil")
	}

	maxRankDiff := 0
	var subversiveVote userVoteRecord
	subversiveVoteIndex := -1

	for i, userVote := range userVotes {
		if userVote.Result == vote.ResultAWins || userVote.Result == vote.ResultBWins {
			winnerID := userVote.SpellA_ID
			loserID := userVote.SpellB_ID
			if userVote.Result == vote.ResultBWins {
				winnerID, loserID = loserID, winnerID
			}

			winnerRank, ok := spellRank[winnerID]
			if !ok {
				return nil, fmt.Errorf("法术 %s 不存在", winnerID)
			}
			loserRank, ok := spellRank[loserID]
			if !ok {
				return nil, fmt.Errorf("法术 %s 不存在", loserID)
			}

			rankDiff := winnerRank - loserRank
			if rankDiff > maxRankDiff {
				maxRankDiff = rankDiff
				subversiveVote = userVote
				subversiveVoteIndex = i
			}
		}
	}

	if subversiveVoteIndex < 0 {
		return nil, nil
	}

	// 填充结果
	spellAName, err := getSpellNameByID(subversiveVote.SpellA_ID)
	if err != nil {
		return nil, err
	}
	spellBName, err := getSpellNameByID(subversiveVote.SpellB_ID)
	if err != nil {
		return nil, err
	}

	return &SpellHighlightVote{
		VoteNumber: subversiveVoteIndex + 1,
		SpellA: SpellNameRank{
			ID:   subversiveVote.SpellA_ID,
			Name: spellAName,
			Rank: int64(spellRank[subversiveVote.SpellA_ID]),
		},
		SpellB: SpellNameRank{
			ID:   subversiveVote.SpellB_ID,
			Name: spellBName,
			Rank: int64(spellRank[subversiveVote.SpellB_ID]),
		},
		Result: subversiveVote.Result,
	}, nil
}

// truncateToDay 将时间截断到当天的零点。
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// calculateFirstVote 获取用户的第一次投票作为里程碑。
func calculateFirstVote(userVotes []userVoteRecord) (*SpellMilestoneVote, error) {
	if len(userVotes) == 0 {
		return nil, nil
	}
	firstVote := userVotes[0]

	spellAName, err := getSpellNameByID(firstVote.SpellA_ID)
	if err != nil {
		return nil, err
	}
	spellBName, err := getSpellNameByID(firstVote.SpellB_ID)
	if err != nil {
		return nil, err
	}

	return &SpellMilestoneVote{
		VoteNumber: 1,
		SpellA:     SpellNameRank{ID: firstVote.SpellA_ID, Name: spellAName},
		SpellB:     SpellNameRank{ID: firstVote.SpellB_ID, Name: spellBName},
		Result:     firstVote.Result,
		Date:       truncateToDay(firstVote.VoteTime),
	}, nil
}

// calculateMilestones 根据定义的里程碑数字，从用户投票历史中提取里程碑事件。
func calculateMilestones(userVotes []userVoteRecord) ([]SpellMilestoneVote, error) {
	var milestones []SpellMilestoneVote
	totalUserVotes := len(userVotes)

	// 筛选出用户已达成的里程碑
	achievedMilestones := make([]int, 0)
	for _, m := range milestoneNumbers {
		if totalUserVotes >= m {
			achievedMilestones = append(achievedMilestones, m)
		}
	}

	// 如果达成的里程碑超过最大显示数量，则只取最后的几个
	if len(achievedMilestones) > MaxMilestones {
		achievedMilestones = achievedMilestones[len(achievedMilestones)-MaxMilestones:]
	}

	for _, m := range achievedMilestones {
		voteRecord := userVotes[m-1]
		spellAName, err := getSpellNameByID(voteRecord.SpellA_ID)
		if err != nil {
			return nil, err
		}
		spellBName, err := getSpellNameByID(voteRecord.SpellB_ID)
		if err != nil {
			return nil, err
		}

		milestones = append(milestones, SpellMilestoneVote{
			VoteNumber: m,
			SpellA:     SpellNameRank{ID: voteRecord.SpellA_ID, Name: spellAName},
			SpellB:     SpellNameRank{ID: voteRecord.SpellB_ID, Name: spellBName},
			Result:     voteRecord.Result,
			Date:       truncateToDay(voteRecord.VoteTime),
		})
	}

	return milestones, nil
}

// calculateBusiestDay 使用滑动窗口算法找出用户在24小时内投票最密集的一天。
func calculateBusiestDay(userVotes []userVoteRecord) *ActivityRecord {
	if len(userVotes) < MinVotesForBusiestDay {
		return nil
	}

	maxVotes := 0
	var busiestStart, busiestEnd time.Time
	left := 0

	for right, vote := range userVotes {
		// 移动左指针，直到窗口小于等于24小时
		for vote.VoteTime.Sub(userVotes[left].VoteTime) > 24*time.Hour {
			left++
		}

		// 当前窗口的投票数
		currentVotes := right - left + 1
		if currentVotes > maxVotes {
			maxVotes = currentVotes
			busiestStart = userVotes[left].VoteTime
			busiestEnd = vote.VoteTime
		}
	}

	if maxVotes < MinVotesForBusiestDay {
		return nil
	}

	return &ActivityRecord{
		FromDate:  truncateToDay(busiestStart),
		ToDate:    truncateToDay(busiestEnd),
		VoteCount: maxVotes,
	}
}

// calculateFirstEncounterTop 查找用户首次遇到顶级法术的投票。
func calculateFirstEncounterTop(userVotes []userVoteRecord, spellRank map[string]int, rankToSpell []string) (*SpellEncounterRecord, error) {
	if spellRank == nil || rankToSpell == nil {
		return nil, fmt.Errorf("传入的map或slice不能为nil")
	}

	count := int(float64(len(rankToSpell)) * TopTierRatio)
	count = min(max(count, 1), len(rankToSpell))

	topSpells := make(map[string]struct{}, count)
	for _, spellID := range rankToSpell[:count] {
		topSpells[spellID] = struct{}{}
	}

	for i, userVote := range userVotes {
		_, isAIn := topSpells[userVote.SpellA_ID]
		_, isBIn := topSpells[userVote.SpellB_ID]

		if isAIn || isBIn {
			spellAName, err := getSpellNameByID(userVote.SpellA_ID)
			if err != nil {
				return nil, err
			}
			spellBName, err := getSpellNameByID(userVote.SpellB_ID)
			if err != nil {
				return nil, err
			}
			return &SpellEncounterRecord{
				VoteNumber: i + 1,
				SpellA:     SpellNameRank{ID: userVote.SpellA_ID, Name: spellAName, Rank: int64(spellRank[userVote.SpellA_ID])},
				SpellB:     SpellNameRank{ID: userVote.SpellB_ID, Name: spellBName, Rank: int64(spellRank[userVote.SpellB_ID])},
				SpecialA:   isAIn,
				SpecialB:   isBIn,
				Result:     userVote.Result,
				Date:       truncateToDay(userVote.VoteTime),
			}, nil
		}
	}

	return nil, nil
}

// calculateFirstEncounterBottom 查找用户首次遇到垫底法术的投票。
func calculateFirstEncounterBottom(userVotes []userVoteRecord, spellRank map[string]int, rankToSpell []string) (*SpellEncounterRecord, error) {
	if spellRank == nil || rankToSpell == nil {
		return nil, fmt.Errorf("传入的map或slice不能为nil")
	}

	count := int(float64(len(rankToSpell)) * BottomTierRatio)
	count = min(max(count, 1), len(rankToSpell))

	bottomSpells := make(map[string]struct{}, count)
	for _, spellID := range rankToSpell[len(rankToSpell)-count:] {
		bottomSpells[spellID] = struct{}{}
	}

	for i, userVote := range userVotes {
		_, isAIn := bottomSpells[userVote.SpellA_ID]
		_, isBIn := bottomSpells[userVote.SpellB_ID]

		if isAIn || isBIn {
			spellAName, err := getSpellNameByID(userVote.SpellA_ID)
			if err != nil {
				return nil, err
			}
			spellBName, err := getSpellNameByID(userVote.SpellB_ID)
			if err != nil {
				return nil, err
			}
			return &SpellEncounterRecord{
				VoteNumber: i + 1,
				SpellA:     SpellNameRank{ID: userVote.SpellA_ID, Name: spellAName, Rank: int64(spellRank[userVote.SpellA_ID])},
				SpellB:     SpellNameRank{ID: userVote.SpellB_ID, Name: spellBName, Rank: int64(spellRank[userVote.SpellB_ID])},
				SpecialA:   isAIn,
				SpecialB:   isBIn,
				Result:     userVote.Result,
				Date:       truncateToDay(userVote.VoteTime),
			}, nil
		}
	}

	return nil, nil
}
