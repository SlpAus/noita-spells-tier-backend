package vote

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/go-redis/redis/v8"
)

const eloKFactor = 32

// calculateElo 计算对战后的新ELO分数
func calculateElo(winnerScore, loserScore float64) (newWinnerScore, newLoserScore float64) {
	expectedWinner := 1.0 / (1.0 + math.Pow(10, (loserScore-winnerScore)/400.0))
	newWinnerScore = winnerScore + eloKFactor*(1-expectedWinner)
	newLoserScore = loserScore - eloKFactor*expectedWinner
	return
}

// ProcessVote 是处理投票的核心函数，保证了Redis和SQLite的原子性操作
func ProcessVote(spellAID, spellBID string, result VoteResult, userID string) error {
	// 1. *** 新增验证 ***
	// 在所有操作开始前，验证投票结果是否合法
	switch result {
	case ResultAWins, ResultBWins, ResultDraw:
		// 合法，继续执行
	case ResultSkip:
		// 对于跳过的投票，我们只记录，不进行任何分数更新
		return persistVoteRecord(spellAID, spellBID, result, userID)
	default:
		// 不合法，直接返回错误
		return fmt.Errorf("无效的投票结果: %s", result)
	}

	// 2. 使用Redis的WATCH来监视法术数据
	err := database.RDB.Watch(database.Ctx, func(tx *redis.Tx) error {
		// 3. 获取两个法术当前的统计数据
		keys := []string{spellAID, spellBID}
		statsJSONs, err := tx.HMGet(database.Ctx, spell.StatsKey, keys...).Result()
		if err != nil {
			return fmt.Errorf("无法从Redis获取法术统计数据: %w", err)
		}
		if statsJSONs[0] == nil || statsJSONs[1] == nil {
			return errors.New("一个或多个法术不存在")
		}

		var statsA, statsB spell.SpellStats
		_ = json.Unmarshal([]byte(statsJSONs[0].(string)), &statsA)
		_ = json.Unmarshal([]byte(statsJSONs[1].(string)), &statsB)

		oldStatsA, oldStatsB := statsA, statsB

		// 4. 根据投票结果，计算新的分数和统计数据
		switch result {
		case ResultAWins:
			statsA.Score, statsB.Score = calculateElo(statsA.Score, statsB.Score)
			statsA.Win++
			statsA.Total++
			statsB.Total++
		case ResultBWins:
			statsB.Score, statsA.Score = calculateElo(statsB.Score, statsA.Score)
			statsB.Win++
			statsB.Total++
			statsA.Total++
		case ResultDraw:
			statsA.Total++
			statsB.Total++
		}

		// 5. 使用Pipeline在同一个事务中执行所有Redis写操作
		_, err = tx.Pipelined(database.Ctx, func(pipe redis.Pipeliner) error {
			newStatsAJSON, _ := json.Marshal(statsA)
			newStatsBJSON, _ := json.Marshal(statsB)
			pipe.HSet(database.Ctx, spell.StatsKey, spellAID, newStatsAJSON)
			pipe.HSet(database.Ctx, spell.StatsKey, spellBID, newStatsBJSON)
			pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsA.Score, Member: spellAID})
			pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsB.Score, Member: spellBID})
			return nil
		})
		if err != nil {
			return fmt.Errorf("执行Redis Pipeline失败: %w", err)
		}

		// 6. Redis事务成功后，尝试将投票记录写入SQLite
		err = persistVoteRecord(spellAID, spellBID, result, userID)
		if err != nil {
			fmt.Printf("警告: SQLite写入失败，正在回滚Redis更改: %v\n", err)
			revertRedisChanges(spellAID, spellBID, oldStatsA, oldStatsB)
			return fmt.Errorf("无法持久化投票记录，操作已回滚: %w", err)
		}

		return nil
	}, spell.StatsKey)

	return err
}

// persistVoteRecord 将单条投票记录写入SQLite
func persistVoteRecord(spellAID, spellBID string, result VoteResult, userID string) error {
	newVote := Vote{
		SpellA_ID:      spellAID,
		SpellB_ID:      spellBID,
		Result:         result,
		UserIdentifier: userID,
	}
	return database.DB.Create(&newVote).Error
}

// revertRedisChanges 执行补偿事务，将Redis中的数据恢复到之前的状态
func revertRedisChanges(spellAID, spellBID string, oldStatsA, oldStatsB spell.SpellStats) {
	pipe := database.RDB.Pipeline()
	oldStatsAJSON, _ := json.Marshal(oldStatsA)
	oldStatsBJSON, _ := json.Marshal(oldStatsB)
	pipe.HSet(database.Ctx, spell.StatsKey, spellAID, oldStatsAJSON)
	pipe.HSet(database.Ctx, spell.StatsKey, spellBID, oldStatsBJSON)
	pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: oldStatsA.Score, Member: spellAID})
	pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: oldStatsB.Score, Member: spellBID})
	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		fmt.Printf("严重错误: Redis补偿事务执行失败: %v\n", err)
	}
}

// ValidatePairID 是用于验证pairId的占位符函数
func ValidatePairID(pairID string) bool {
	fmt.Printf("警告: PairID验证被跳过 (开发模式)。PairID: %s\n", pairID)
	return true
}
