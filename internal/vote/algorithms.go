package vote

import "math"

// --- 算法常量 ---

const (
	// calculateMultiplierForCount 使用的常量
	GracePeriodThreshold       = 200
	HarshPenaltyThreshold      = 600
	MultiplierAtHarshThreshold = 0.5
	CutoffMultiplier           = 0.01
	DecaySlope                 = (MultiplierAtHarshThreshold - 1.0) / (HarshPenaltyThreshold - GracePeriodThreshold)

	// eloKFactor 是ELO算法中的K值，它决定了每次对战后分数变化的大小。
	eloKFactor = 32

	// rankScoreEloWeightBase 是计算归一化ELO分占比的基础值 (1.2)
	rankScoreEloWeightBase = 1.2
	// rankScoreEloWeightDecay 是计算归一化ELO分占比的衰减率 (0.01)
	rankScoreEloWeightDecay = 0.01
)

// --- Multiplier计算 ---

// calculateMultiplierForCount 根据同IP一定时间内的投票数计算该票的Multiplier
func calculateMultiplierForCount(count int64) float64 {
	if count <= GracePeriodThreshold {
		return 1.0
	}

	if count <= HarshPenaltyThreshold {
		return 1.0 + DecaySlope*float64(count-GracePeriodThreshold)
	}

	return CutoffMultiplier
}

// --- ELO计算 ---

// calculateElo 计算对战后的新ELO分数。
func calculateElo(winnerScore, loserScore, multiplier float64) (newWinnerScore, newLoserScore float64) {
	expectedWinner := 1.0 / (1.0 + math.Pow(10, (loserScore-winnerScore)/400.0))
	newWinnerScore = winnerScore + eloKFactor*(1-expectedWinner)*multiplier
	newLoserScore = loserScore - eloKFactor*expectedWinner*multiplier
	return
}

// --- 动态排名分数 (RankScore) 计算 ---

// calculateEloWeight 根据法术的总场次数，计算其归一化ELO分数在最终RankScore中的占比。
func calculateEloWeight(total float64) float64 {
	// 占比 = 1.2 - 0.01 * n
	weight := rankScoreEloWeightBase - rankScoreEloWeightDecay*total
	// 将结果限制在 [0, 1] 区间内
	return max(0.0, min(1.0, weight))
}

// CalculateRankScore 计算最终用于排名的动态分数。
// 它混合了归一化的ELO分数和原始胜率。
func CalculateRankScore(tx *eloTrackerTx, score, total, win float64) float64 {
	// 1. 根据总场数计算ELO分数的混合权重
	eloWeight := calculateEloWeight(total)

	// 2. 计算归一化的ELO分数
	var normalizedElo float64
	if eloWeight > 0.0 {
		// 获取当前的ELO分数范围
		minScore, maxScore := globalEloTracker.GetMinMax(tx)

		if maxScore == minScore {
			// 如果所有分数都相同，则归一化ELO为0.5
			normalizedElo = 0.5
		} else {
			normalizedElo = (score - minScore) / (maxScore - minScore)
		}
	}

	// 3. 计算原始胜率
	var winRate float64
	if eloWeight < 1.0 {
		if total == 0 {
			// 0场/0胜记为50%胜率
			winRate = 0.5
		} else {
			winRate = win / total
		}
	}

	// 4. 加权混合得到最终的RankScore
	// RankScore = 归一化ELO * 权重 + 胜率 * (1 - 权重)
	rankScore := normalizedElo*eloWeight + winRate*(1-eloWeight)

	return rankScore
}
