package vote

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/go-redis/redis/v8"
)

// ApplyIncrementalVotes 在缓存重建时，处理自上次快照以来的所有新投票
func ApplyIncrementalVotes() error {
	lastSnapshotVoteID, err := metadata.GetLastSnapshotVoteID(database.DB)
	if err != nil {
		return fmt.Errorf("无法获取上一次快照的vote ID: %w", err)
	}

	const batchSize = 10000

	var incrementalVotes []Vote
	if err := database.DB.Where("id > ?", lastSnapshotVoteID).Order("id asc").Limit(batchSize).Find(&incrementalVotes).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取增量投票: %w", err)
	}

	if len(incrementalVotes) == 0 {
		fmt.Println("没有新的投票记录需要处理。")
		return nil
	}

	fmt.Printf("正在处理 %d 条自上次快照以来的新投票...\n", len(incrementalVotes))

	// 1. 加写锁，保护对Redis和内存仓库的联合更新
	spell.LockRepository()
	defer spell.UnlockRepository()

	// 2. 一次性从Redis获取所有法术的当前统计数据到内存中
	statsMapJSON, err := database.RDB.HGetAll(database.Ctx, spell.StatsKey).Result()
	if err != nil {
		return fmt.Errorf("无法从Redis获取完整的法术统计数据: %w", err)
	}
	inMemoryStats := make(map[string]spell.SpellStats)
	for id, jsonStr := range statsMapJSON {
		var stats spell.SpellStats
		if err := json.Unmarshal([]byte(jsonStr), &stats); err == nil {
			inMemoryStats[id] = stats
		}
	}

	// 3. 在内存中批量计算所有增量投票
	var lastProcessedID uint = 0
	var totalVotesIncrement float64 = 0
	for {
		for _, vote := range incrementalVotes {
			if vote.Result != ResultSkip {
				statsA, okA := inMemoryStats[vote.SpellA_ID]
				statsB, okB := inMemoryStats[vote.SpellB_ID]
				if !okA || !okB {
					return fmt.Errorf("法术对 (%s , %s) 不存在", vote.SpellA_ID, vote.SpellB_ID)
				}

				switch vote.Result {
				case ResultAWins:
					statsA.Score, statsB.Score = calculateElo(vote.Multiplier, statsA.Score, statsB.Score)
					statsA.Win += vote.Multiplier
					statsA.Total += vote.Multiplier
					statsB.Total += vote.Multiplier
				case ResultBWins:
					statsB.Score, statsA.Score = calculateElo(vote.Multiplier, statsB.Score, statsA.Score)
					statsB.Win += vote.Multiplier
					statsB.Total += vote.Multiplier
					statsA.Total += vote.Multiplier
				case ResultDraw:
					statsA.Total += vote.Multiplier
					statsB.Total += vote.Multiplier
				}
				inMemoryStats[vote.SpellA_ID] = statsA
				inMemoryStats[vote.SpellB_ID] = statsB
				totalVotesIncrement += vote.Multiplier
			}
			lastProcessedID = vote.ID
		}

		if len(incrementalVotes) < batchSize {
			break
		}

		incrementalVotes = incrementalVotes[:0]
		if err := database.DB.Where("id > ?", lastProcessedID).Order("id asc").Limit(batchSize).Find(&incrementalVotes).Error; err != nil {
			return fmt.Errorf("无法从SQLite读取增量投票: %w", err)
		}
	}

	// 4. 批量计算完成后，一次性重置ELO追踪器
	eloTrackerTx := globalEloTracker.BeginUpdate()
	defer eloTrackerTx.RollbackUnlessCommitted()

	allScores := make([]float64, 0, len(inMemoryStats))
	for _, stats := range inMemoryStats {
		allScores = append(allScores, stats.Score)
	}
	globalEloTracker.Reset(eloTrackerTx, allScores)

	// 5. 使用更新后的边界，为所有法术计算新的RankScore并更新权重树
	for id, stats := range inMemoryStats {
		stats.RankScore = CalculateRankScore(eloTrackerTx, stats.Score, stats.Total, stats.Win)
		inMemoryStats[id] = stats
		index, ok := spell.GetSpellIndexByID(id)
		if ok {
			spell.UpdateWeightUnsafe(index, spell.CalculateWeightForTotal(stats.Total))
		}
	}

	eloTrackerTx.Commit()

	// 6. 使用Pipeline一次性将所有更新后的数据写回Redis
	pipe := database.RDB.TxPipeline()
	newRanking := make([]*redis.Z, 0, len(inMemoryStats))
	for id, stats := range inMemoryStats {
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, spell.StatsKey, id, statsJSON)
		newRanking = append(newRanking, &redis.Z{Score: stats.RankScore, Member: id})
	}
	pipe.ZAdd(database.Ctx, spell.RankingKey, newRanking...)
	if totalVotesIncrement > 0 {
		pipe.IncrByFloat(database.Ctx, metadata.RedisTotalVotesKey, totalVotesIncrement)
	}
	if lastProcessedID > 0 {
		pipe.Set(database.Ctx, metadata.RedisLastProcessedVoteIDKey, lastProcessedID, 0)
	}

	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("批量更新Redis失败: %w", err)
	}

	// 7. 更新VoteProcessor的内部状态
	if lastProcessedID > 0 {
		globalVoteProcessor.processMutex.Lock()
		globalVoteProcessor.lastProcessedVoteID = lastProcessedID
		globalVoteProcessor.processMutex.Unlock()
		fmt.Printf("增量投票处理完成，Vote Processor将从 ID %d 继续。\n", lastProcessedID)
	}

	// 8. 触发一次新的快照
	fmt.Println("增量恢复完成，正在触发一次新的数据快照...")
	if err := spell.CreateConsistentSnapshotInDB(context.Background()); err != nil {
		fmt.Printf("警告: 增量恢复后的快照创建失败: %v\n", err)
	}

	return nil
}
