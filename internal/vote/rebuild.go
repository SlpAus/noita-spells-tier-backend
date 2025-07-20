package vote

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/go-redis/redis/v8"
)

// ApplyIncrementalVotes 在缓存重建时，处理自上次快照以来的所有新投票
func ApplyIncrementalVotes() error {
	// 1. 获取上一次快照时处理的最后一个vote ID
	lastSnapshotVoteID, err := metadata.GetLastSnapshotVoteID()
	if err != nil {
		return fmt.Errorf("无法获取上一次快照的vote ID: %w", err)
	}

	// 2. 从SQLite中获取所有在这之后的投票记录
	var incrementalVotes []Model
	if err := database.DB.Where("id > ?", lastSnapshotVoteID).Order("id asc").Find(&incrementalVotes).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取增量投票: %w", err)
	}

	if len(incrementalVotes) == 0 {
		fmt.Println("没有新的投票记录需要处理。")
		return nil
	}

	fmt.Printf("正在处理 %d 条自上次快照以来的新投票...\n", len(incrementalVotes))

	// 3. 一次性从Redis获取所有法术的当前统计数据到内存中
	statsMapJSON, err := database.RDB.HGetAll(database.Ctx, spell.StatsKey).Result()
	if err != nil {
		return fmt.Errorf("无法从Redis获取完整的法术统计数据: %w", err)
	}
	// 将JSON解析为内存中的map
	inMemoryStats := make(map[string]spell.SpellStats)
	for id, jsonStr := range statsMapJSON {
		var stats spell.SpellStats
		if err := json.Unmarshal([]byte(jsonStr), &stats); err == nil {
			inMemoryStats[id] = stats
		}
	}

	// 4. 在内存中批量计算所有增量投票
	var lastProcessedID uint = 0
	for _, vote := range incrementalVotes {
		if vote.Result == ResultSkip {
			lastProcessedID = vote.ID
			continue
		}
		statsA, okA := inMemoryStats[vote.SpellA_ID]
		statsB, okB := inMemoryStats[vote.SpellB_ID]
		if !okA || !okB {
			lastProcessedID = vote.ID
			continue
		}

		switch vote.Result {
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
		inMemoryStats[vote.SpellA_ID] = statsA
		inMemoryStats[vote.SpellB_ID] = statsB
		lastProcessedID = vote.ID
	}

	// 5. 使用Pipeline一次性将所有更新后的数据写回Redis
	pipe := database.RDB.Pipeline()
	for id, stats := range inMemoryStats {
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, spell.StatsKey, id, statsJSON)
		pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: stats.Score, Member: id})
	}
	// *** 新增：更新检查点 ***
	if lastProcessedID > 0 {
		pipe.Set(database.Ctx, metadata.LastProcessedVoteIDKey, lastProcessedID, 0)
	}

	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("批量更新Redis失败: %w", err)
	}

	// 6. 更新VoteProcessor的内部状态，让它从正确的位置继续
	if lastProcessedID > 0 {
		globalVoteProcessor.mu.Lock()
		globalVoteProcessor.lastProcessedVoteID = lastProcessedID
		globalVoteProcessor.mu.Unlock()
		fmt.Printf("增量投票处理完成，Vote Processor将从 ID %d 继续。\n", lastProcessedID)
	}

	// 7. 调用spell.CreateConsistentSnapshotInDB()来创建新的快照，并更新metadata
	// 不检查错误，spell的持久化无需确保
	spell.CreateConsistentSnapshotInDB()

	return nil
}
