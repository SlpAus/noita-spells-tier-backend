package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	"github.com/redis/go-redis/v9"
)

const (
	CacheTTL = 1 * time.Minute
)

// userVoteRecord 是一个内部结构体，用于从vote表中仅查询生成报告所需的最小字段。
type userVoteRecord struct {
	SpellA_ID string
	SpellB_ID string
	Result    vote.VoteResult
	VoteTime  time.Time
}

// GenerateUserReport 是生成用户报告的统一入口。
// 它会检查Redis的健康状况，并相应地选择从Redis实时数据或从内存快照生成报告。
func GenerateUserReport(userID string) (*UserReport, error) {
	// 用户无效，快速返回
	if userID == "" {
		return &UserReport{
			GeneratedAt:     time.Now(),
			VoteRankPercent: 1.0,
		}, nil
	}

	if database.IsRedisHealthy() {
		return generateReportFromRedis(userID)
	}
	return generateReportFromMirrorRepo(userID)
}

// 填充 report 中 Name 字段用的辅助方法。
func getSpellNameByID(id string) (string, error) {
	index, ok := spell.GetSpellIndexByID(id)
	if !ok {
		return "", fmt.Errorf("法术ID %s 无效", id)
	}
	info, _ := spell.GetSpellInfoByIndex(index)
	return info.Name, nil
}

// generateReportFromRedis 包含从Redis生成报告的完整逻辑，包括缓存。
func generateReportFromRedis(userID string) (report *UserReport, err error) {
	// 1. 尝试从缓存获取
	cachedReport, err := GetReportCache(userID)
	if err == nil && cachedReport != nil {
		return cachedReport, nil
	}

	// 2. 缓存未命中，生成新报告
	report = &UserReport{
		UserID:      userID,
		GeneratedAt: time.Now(),
	}

	// 在 generateReportFromRedis 退出时，将新报告（如有）存入缓存
	defer func() {
		if report == nil {
			return
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("严重错误: 缓存报告的goroutine发生panic: %v\n", r)
				}
			}()
			_ = SetReportCache(report, CacheTTL)
		}()
	}()

	// a. 从Redis获取数据
	pipe := database.RDB.TxPipeline()
	lastVoteIDCmd := pipe.Get(database.Ctx, metadata.RedisLastProcessedVoteIDKey)
	userStatsCmd := pipe.HGet(database.Ctx, user.StatsKey, userID)
	userRankCmd := pipe.ZRank(database.Ctx, user.RankingKey, userID)
	totalStatsCmd := pipe.HGet(database.Ctx, user.StatsKey, user.TotalStatsKey)
	totalVotersCmd := pipe.ZCard(database.Ctx, user.RankingKey)
	spellRankingCmd := pipe.ZRevRangeWithScores(database.Ctx, spell.RankingKey, 0, -1)
	_, err = pipe.Exec(database.Ctx)
	if err != nil {
		return nil, fmt.Errorf("从Redis获取用户统计数据时出错: %w", err)
	}

	// b.1. 解析通用字段
	lastVoteID, err := lastVoteIDCmd.Uint64()
	if err != nil {
		return nil, fmt.Errorf("获取 lastVoteID 的结果时出错: %w", err)
	}

	userStatsJSON, err := userStatsCmd.Result()
	if err != nil {
		if err == redis.Nil {
			// 如果快照中没有该用户，则返回一个空的报告
			report.VoteRankPercent = 1.0
			return report, nil
		}
		return nil, fmt.Errorf("获取 userStatsJSON 时出错: %w", err)
	}
	var userStats user.UserStats
	if err := json.Unmarshal([]byte(userStatsJSON), &userStats); err != nil {
		return nil, fmt.Errorf("解析 userStatsJSON 时出错: %w", err)
	}

	userRank, err := userRankCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("获取 userRank 的结果时出错: %w", err)
	}

	totalVoters, err := totalVotersCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("获取 totalVoters 的结果时出错: %w", err)
	}

	// 获取法术排名和分数
	spellRanking, err := spellRankingCmd.Result()
	if err != nil {
		return nil, fmt.Errorf("获取 spellRanking 的结果时出错: %w", err)
	}
	spellRank := make(map[string]int, len(spellRanking))
	spellRankScore := make(map[string]float64, len(spellRanking))
	rankToSpell := make([]string, len(spellRanking))
	for i, z := range spellRanking {
		spellID := z.Member.(string)
		spellRank[spellID] = i + 1 // 转换为1-based
		spellRankScore[spellID] = z.Score
		rankToSpell[i] = spellID
	}

	// b.2. 提前解析可选字段

	// 决断率相关
	var totalStats user.UserStats
	if report.TotalVotes >= MinVotesForDecisionRate {
		totalStatsJSON, err := totalStatsCmd.Result()
		// user.TotalStatsKey 对应的值默认存在
		if err != nil {
			return nil, fmt.Errorf("获取 totalStatsJSON 的值时出错: %w", err)
		}
		err = json.Unmarshal([]byte(totalStatsJSON), &totalStats)
		if err != nil {
			return nil, fmt.Errorf("解析 totalStatsJSON 时出错: %w", err)
		}
	}

	// c. 获取用户投票历史
	var userVotes []userVoteRecord
	if err := database.DB.Model(&vote.Vote{}).
		Where("user_identifier = ? AND id <= ?", userID, lastVoteID).
		Order("id asc").
		Find(&userVotes).Error; err != nil {
		return nil, fmt.Errorf("查询用户投票历史时出错: %w", err)
	}

	// d. 填充必选字段
	report.TotalVotes = userStats.Wins + userStats.Draw + userStats.Skip
	report.Choices = ChoiceCounts{
		Wins: userStats.Wins,
		Draw: userStats.Draw,
		Skip: userStats.Skip,
	}

	if totalVoters > 0 {
		report.VoteRankPercent = float64(userRank) / float64(totalVoters)
	} else {
		report.VoteRankPercent = 1.0
	}

	// e. 填充可选字段

	// 决断率
	if report.TotalVotes >= MinVotesForDecisionRate {
		decisionRate := calculateDecisionRate(userStats)
		report.DecisionRate = &decisionRate

		communityDecisionRate := calculateDecisionRate(totalStats)
		report.CommunityDecisionRate = &communityDecisionRate
	}

	// 投票倾向
	if userStats.Wins >= MinWinsForTendency {
		consistencyIndex, err := calculateCommunityConsistencyIndex(userVotes, spellRank)
		if err != nil {
			return nil, err
		}
		report.CommunityConsistencyIndex = &consistencyIndex

		upsetTendency, err := calculateUpsetTendency(userVotes, spellRankScore)
		if err != nil {
			return nil, err
		}
		report.UpsetTendency = &upsetTendency
	}

	// 趣味高光时刻
	mostChosen, err := calculateMostChosenSpell(userVotes)
	if err != nil {
		return nil, err
	}
	report.MostChosen = mostChosen

	spellWinRates := calcualteWinRateForSpells(userVotes)

	if len(userVotes) >= int(float64(spell.GetSpellCount())*TotalVotesToSpellsRatioForWinRate) {
		highestWinRate, err := calculateHighestWinRateSpell(spellWinRates)
		if err != nil {
			return nil, err
		}
		report.HighestWinRate = highestWinRate
	}

	chosenOne, err := calculateChosenOne(spellWinRates, spellRankScore)
	if err != nil {
		return nil, err
	}
	report.ChosenOne = chosenOne

	nemesis, err := calculateNemesis(spellWinRates, spellRankScore)
	if err != nil {
		return nil, err
	}
	report.Nemesis = nemesis

	mostSubversive, err := calculateMostSubversiveVote(userVotes, spellRank)
	if err != nil {
		return nil, err
	}
	report.MostSubversive = mostSubversive

	// 里程碑与记录
	firstVote, err := calculateFirstVote(userVotes)
	if err != nil {
		return nil, err
	}
	report.FirstVote = firstVote

	milestones, err := calculateMilestones(userVotes)
	if err != nil {
		return nil, err
	}
	report.Milestones = milestones

	report.BusiestDay = calculateBusiestDay(userVotes)

	firstEncounterTop, err := calculateFirstEncounterTop(userVotes, spellRank, rankToSpell)
	if err != nil {
		return nil, err
	}
	report.FirstEncounterTop = firstEncounterTop

	firstEncounterBottom, err := calculateFirstEncounterBottom(userVotes, spellRank, rankToSpell)
	if err != nil {
		return nil, err
	}
	report.FirstEncounterBottom = firstEncounterBottom

	return report, nil
}

// generateReportFromMirrorRepo 从内存镜像生成报告（Redis降级时使用）。
func generateReportFromMirrorRepo(userID string) (*UserReport, error) {
	unlock, err := mirrorRepo.ensureAndLock()
	if err != nil {
		return nil, fmt.Errorf("无法确保或锁定内存仓库: %w", err)
	}
	defer unlock()

	report := &UserReport{
		UserID:      userID,
		GeneratedAt: mirrorRepo.snapshotTime,
	}

	// a. 从内存仓库准备数据
	userStats, ok := mirrorRepo.userStats[userID]
	if !ok {
		// 如果快照中没有该用户，则返回一个空的报告
		report.VoteRankPercent = 1.0
		return report, nil
	}

	// b. 获取用户投票历史
	var userVotes []userVoteRecord
	if err := database.DB.Model(&vote.Vote{}).
		Where("user_identifier = ? AND id <= ?", userID, mirrorRepo.snapshotVoteID).
		Order("id asc").
		Find(&userVotes).Error; err != nil {
		return nil, fmt.Errorf("查询用户投票历史时出错: %w", err)
	}

	// c. 填充必选字段
	report.TotalVotes = userStats.Wins + userStats.Draw + userStats.Skip
	report.Choices = ChoiceCounts{
		Wins: userStats.Wins,
		Draw: userStats.Draw,
		Skip: userStats.Skip,
	}

	userRank := mirrorRepo.userRank[userID]
	totalVoters := mirrorRepo.totalVoters
	if totalVoters > 0 {
		report.VoteRankPercent = float64(userRank) / float64(totalVoters)
	} else {
		report.VoteRankPercent = 1.0
	}

	// d. 填充可选字段

	// 决断率
	if report.TotalVotes >= MinVotesForDecisionRate {
		decisionRate := calculateDecisionRate(userStats)
		report.DecisionRate = &decisionRate

		communityDecisionRate := calculateDecisionRate(mirrorRepo.totalStats)
		report.CommunityDecisionRate = &communityDecisionRate
	}

	// 投票倾向
	if userStats.Wins >= MinWinsForTendency {
		consistencyIndex, err := calculateCommunityConsistencyIndex(userVotes, mirrorRepo.spellRank)
		if err != nil {
			return nil, err
		}
		report.CommunityConsistencyIndex = &consistencyIndex

		upsetTendency, err := calculateUpsetTendency(userVotes, mirrorRepo.spellRankScore)
		if err != nil {
			return nil, err
		}
		report.UpsetTendency = &upsetTendency
	}

	// 趣味高光时刻
	mostChosen, err := calculateMostChosenSpell(userVotes)
	if err != nil {
		return nil, err
	}
	report.MostChosen = mostChosen

	spellWinRates := calcualteWinRateForSpells(userVotes)

	if len(userVotes) >= int(float64(spell.GetSpellCount())*TotalVotesToSpellsRatioForWinRate) {
		highestWinRate, err := calculateHighestWinRateSpell(spellWinRates)
		if err != nil {
			return nil, err
		}
		report.HighestWinRate = highestWinRate
	}

	chosenOne, err := calculateChosenOne(spellWinRates, mirrorRepo.spellRankScore)
	if err != nil {
		return nil, err
	}
	report.ChosenOne = chosenOne

	nemesis, err := calculateNemesis(spellWinRates, mirrorRepo.spellRankScore)
	if err != nil {
		return nil, err
	}
	report.Nemesis = nemesis

	mostSubversive, err := calculateMostSubversiveVote(userVotes, mirrorRepo.spellRank)
	if err != nil {
		return nil, err
	}
	report.MostSubversive = mostSubversive

	// 里程碑与记录
	firstVote, err := calculateFirstVote(userVotes)
	if err != nil {
		return nil, err
	}
	report.FirstVote = firstVote

	milestones, err := calculateMilestones(userVotes)
	if err != nil {
		return nil, err
	}
	report.Milestones = milestones

	report.BusiestDay = calculateBusiestDay(userVotes)

	firstEncounterTop, err := calculateFirstEncounterTop(userVotes, mirrorRepo.spellRank, mirrorRepo.rankToSpell)
	if err != nil {
		return nil, err
	}
	report.FirstEncounterTop = firstEncounterTop

	firstEncounterBottom, err := calculateFirstEncounterBottom(userVotes, mirrorRepo.spellRank, mirrorRepo.rankToSpell)
	if err != nil {
		return nil, err
	}
	report.FirstEncounterBottom = firstEncounterBottom

	return report, nil
}
