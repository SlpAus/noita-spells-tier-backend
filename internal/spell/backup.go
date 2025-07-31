package spell

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"gorm.io/gorm"
)

const backupInterval = 10 * time.Minute // 定时备份频率

// StartBackupScheduler 启动一个后台Goroutine来定期执行数据库备份
// 它现在接收一个lifecycle.Handle来管理其生命周期
func StartBackupScheduler(handle *lifecycle.Handle) {
	defer handle.Close() // 确保在退出时通知管理器
	fmt.Println("法术数据备份调度器已启动。")

	for {
		// 使用可中断的休眠来代替ticker。
		// 这使得整个循环可以在收到停机信号时立刻从休眠中唤醒并退出。
		if err := handle.Sleep(backupInterval); err != nil {
			fmt.Printf("备份调度器: 休眠被中断，正在关闭... (%v)\n", err)
			return
		}

		if !database.IsRedisHealthy() {
			fmt.Println("备份调度器: 检测到Redis不可用，跳过本次备份。")
			continue
		}

		fmt.Println("备份调度器: 正在执行定时备份...")
		if err := CreateConsistentSnapshotInDB(handle.Ctx()); err != nil {
			// 如果错误是由于停机信号导致的，则静默退出
			if err != context.Canceled && err != context.DeadlineExceeded {
				fmt.Printf("备份调度器错误: 执行快照备份失败: %v\n", err)
			}
		} else {
			fmt.Println("备份调度器: 快照备份成功。")
		}
	}
}

// CreateConsistentSnapshotInDB 执行一次原子的、一致的快照备份
func CreateConsistentSnapshotInDB(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err() // 如果已收到信号，则放弃操作
	default:
	}

	// 1. 使用原子事务(TxPipeline)从Redis获取快照
	pipe := database.RDB.TxPipeline()
	lastVoteIDCmd := pipe.Get(database.Ctx, metadata.RedisLastProcessedVoteIDKey)
	totalVotesCmd := pipe.Get(database.Ctx, metadata.RedisTotalVotesKey)
	statsMapCmd := pipe.HGetAll(database.Ctx, StatsKey)
	sortedIDsCmd := pipe.ZRevRange(database.Ctx, RankingKey, 0, -1)
	_, err := pipe.Exec(database.Ctx)

	if err != nil {
		return fmt.Errorf("无法从Redis原子地获取快照数据: %w", err)
	}

	lastVoteIDUint64, err := lastVoteIDCmd.Uint64()
	if err != nil {
		return fmt.Errorf("获取 lastVoteIDUint64 的结果时失败: %w", err)
	}
	lastVoteID := uint(lastVoteIDUint64)

	totalVotes, err := totalVotesCmd.Float64()
	if err != nil {
		return fmt.Errorf("获取 totalVotes 的结果时失败: %w", err)
	}

	statsMap, err := statsMapCmd.Result()
	if err != nil {
		return fmt.Errorf("获取 statsMap 的结果时失败: %w", err)
	}
	sortedSpellIDs, err := sortedIDsCmd.Result()
	if err != nil {
		return fmt.Errorf("获取 sortedSpellIDs 的结果时失败: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err() // 如果在读取Redis后收到了信号，则放弃写入
	default:
	}

	lastSnapshotVoteID, err := metadata.GetLastSnapshotVoteID(database.DB)
	if err != nil {
		return fmt.Errorf("获取 lastSnapshotVoteID 失败: %w", err)
	}
	snapshotTotalVotes, err := metadata.GetSnapshotTotalVotes(database.DB)
	if err != nil {
		return fmt.Errorf("获取 snapshotTotalVotes 失败: %w", err)
	}

	// 无需备份
	if lastVoteID == lastSnapshotVoteID {
		return nil
	}

	// 只需要更新 LastSnapshotVoteID
	if totalVotes == snapshotTotalVotes {
		return metadata.SetLastSnapshotVoteID(database.DB, lastVoteID)
	}

	// 2. 将快照数据持久化到SQLite
	return database.DB.Transaction(func(tx *gorm.DB) error {
		for i, spellID := range sortedSpellIDs {
			rank := i + 1
			statsJSON, ok := statsMap[spellID]
			if !ok {
				fmt.Printf("备份警告: 在stats哈希表中找不到ID为 %s 的法术，跳过备份。\n", spellID)
				continue
			}

			var stats SpellStats
			if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
				fmt.Printf("备份警告: 解析法术 %s 的数据失败，跳过备份: %v\n", spellID, err)
				continue
			}

			err := tx.Model(&Spell{}).Where("spell_id = ?", spellID).Updates(map[string]interface{}{
				"score":      stats.Score,
				"total":      stats.Total,
				"win":        stats.Win,
				"rank":       rank,
				"rank_score": stats.RankScore, // 增加新字段
			}).Error
			if err != nil {
				return fmt.Errorf("更新法术 %s 的数据失败: %w", spellID, err)
			}
		}

		// 3. 在同一个事务中，更新持久化的元数据
		if err := metadata.SetLastSnapshotVoteID(tx, lastVoteID); err != nil {
			return fmt.Errorf("更新元数据 LastSnapshotVoteID 失败: %w", err)
		}
		if err := metadata.SetSnapshotTotalVotes(tx, totalVotes); err != nil {
			return fmt.Errorf("更新元数据 SnapshotTotalVotes 失败: %w", err)
		}

		return nil
	})
}
