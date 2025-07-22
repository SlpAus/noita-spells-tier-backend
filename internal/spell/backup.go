package spell

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"github.com/go-redis/redis/v8"
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
	lastVoteIDCmd := pipe.Get(database.Ctx, metadata.LastProcessedVoteIDKey)
	statsMapCmd := pipe.HGetAll(database.Ctx, StatsKey)
	sortedIDsCmd := pipe.ZRevRange(database.Ctx, RankingKey, 0, -1)
	_, err := pipe.Exec(database.Ctx)

	if err != nil {
		if err == redis.Nil {
			fmt.Println("备份调度器: 关键元数据不存在 (可能尚未处理任何投票)，跳过备份。")
			return nil
		}
		// 对于所有其他错误（网络问题、事务失败等），则报告为严重故障
		return fmt.Errorf("无法从Redis原子地获取快照数据: %w", err)
	}

	lastVoteIDStr, err := lastVoteIDCmd.Result()
	if err != nil {
		if err == redis.Nil {
			fmt.Println("备份调度器: 关键元数据不存在 (可能尚未处理任何投票)，跳过备份。")
			return nil
		}
		return fmt.Errorf("获取 lastVoteIDCmd 的结果时失败: %w", err)
	}
	lastVoteID, err := strconv.ParseUint(lastVoteIDStr, 10, 32)
	if err != nil {
		return fmt.Errorf("解析 lastVoteID 失败: %w", err)
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

	// 2. 将快照数据持久化到SQLite
	return database.DB.Transaction(func(tx *gorm.DB) error {
		for i, spellID := range sortedSpellIDs {
			rank := i + 1
			statsJSON, ok := statsMap[spellID]
			if !ok {
				// 如果在stats哈希表中找不到ID，说明数据可能不一致，跳过此条记录
				fmt.Printf("备份警告: 在stats哈希表中找不到ID为 %s 的法术，跳过备份。\n", spellID)
				continue
			}

			var stats SpellStats
			if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
				fmt.Printf("备份警告: 解析法术 %s 的数据失败，跳过备份: %v\n", spellID, err)
				continue
			}

			err := tx.Model(&Spell{}).Where("spell_id = ?", spellID).Updates(map[string]interface{}{
				"score": stats.Score,
				"total": stats.Total,
				"win":   stats.Win,
				"rank":  rank, // 写入排名
			}).Error
			if err != nil {
				return fmt.Errorf("更新法术 %s 的数据失败: %w", spellID, err)
			}
		}

		// 3. 在同一个事务中，更新持久化的元数据检查点
		if err := metadata.SetLastSnapshotVoteID(tx, uint(lastVoteID)); err != nil {
			return fmt.Errorf("更新元数据检查点失败: %w", err)
		}

		return nil
	})
}
