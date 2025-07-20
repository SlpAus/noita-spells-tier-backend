package spell

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"gorm.io/gorm"
)

const backupInterval = 10 * time.Minute // 定时备份频率

// StartBackupScheduler 启动一个后台Goroutine来定期执行数据库备份
func StartBackupScheduler() {
	fmt.Println("法术数据备份调度器已启动。")
	ticker := time.NewTicker(backupInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		// 使用正确的健康检查函数
		if !database.IsRedisHealthy() {
			fmt.Println("备份调度器: 检测到Redis不可用，跳过本次备份。")
			continue
		}

		fmt.Println("备份调度器: 正在执行定时备份...")
		if err := CreateConsistentSnapshotInDB(); err != nil {
			fmt.Printf("备份调度器错误: 执行快照备份失败: %v\n", err)
		} else {
			fmt.Println("备份调度器: 快照备份成功。")
		}
	}
}

// CreateConsistentSnapshotInDB 执行一次原子的、一致的快照备份
func CreateConsistentSnapshotInDB() error {
	// 1. 使用原子事务(TxPipeline)从Redis获取快照
	pipe := database.RDB.TxPipeline()
	lastVoteIDCmd := pipe.Get(database.Ctx, metadata.LastProcessedVoteIDKey)
	statsMapCmd := pipe.HGetAll(database.Ctx, StatsKey)
	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("无法从Redis原子地获取快照数据: %w", err)
	}

	lastVoteIDStr, err := lastVoteIDCmd.Result()
	if err != nil {
		fmt.Println("备份调度器: 尚未有投票被处理，跳过备份。")
		return nil
	}
	lastVoteID, _ := strconv.ParseUint(lastVoteIDStr, 10, 32)
	statsMap, _ := statsMapCmd.Result()

	// 2. 将快照数据持久化到SQLite
	return database.DB.Transaction(func(tx *gorm.DB) error {
		for spellID, statsJSON := range statsMap {
			var stats SpellStats
			if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
				continue
			}

			err := tx.Model(&Spell{}).Where("spell_id = ?", spellID).Updates(map[string]interface{}{
				"score": stats.Score,
				"total": stats.Total,
				"win":   stats.Win,
			}).Error

			if err != nil {
				return fmt.Errorf("更新法术 %s 的数据失败: %w", spellID, err)
			}
		}

		// 3. 在同一个事务中，更新持久化的元数据检查点
		if err := metadata.SetLastSnapshotVoteID(uint(lastVoteID)); err != nil {
			return fmt.Errorf("更新元数据检查点失败: %w", err)
		}

		return nil
	})
}
