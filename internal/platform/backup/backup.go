package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const backupInterval = 10 * time.Minute // 定时备份频率

var backupMutex sync.Mutex // 避免意外竞态

// StartBackupScheduler 启动一个后台Goroutine来定期执行数据库备份
// 它现在接收一个lifecycle.Handle来管理其生命周期
func StartBackupScheduler(handle *lifecycle.Handle) {
	defer handle.Close() // 确保在退出时通知管理器
	fmt.Println("法术数据备份调度器已启动。")

	for {
		// 使用可中断的休眠来代替ticker。
		// 这使得整个循环可以在收到停机信号时立刻从休眠中唤醒并退出。
		if err := handle.Sleep(backupInterval); err != nil {
			fmt.Printf("备份调度器: 休眠被中断，正在关闭...\n")
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
func CreateConsistentSnapshotInDB(ctx context.Context) (err error) {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	var lastVoteIDCmd *redis.StringCmd
	var totalVotesCmd *redis.StringCmd
	var statsMapCmd *redis.MapStringStringCmd
	var sortedIDsCmd *redis.StringSliceCmd

	var dirtyUserIDs []string
	var dirtyUserStats []interface{}

	transferred, err := func() (bool, error) {
		// user 模块在两批Redis操作期间保持锁定，确保dirtyUserIDs和dirtyUserStats不撕裂
		user.LockRepository()
		defer user.UnlockRepository()

		dirtySetExists, err := database.RDB.Exists(ctx, user.DirtySetKey).Result()
		if err != nil {
			return false, fmt.Errorf("无法检查Redis中 DirtySetKey 是否存在: %w", err)
		}

		// 1. 使用原子事务(TxPipeline)从Redis获取快照
		pipe := database.RDB.TxPipeline()
		lastVoteIDCmd = pipe.Get(database.Ctx, metadata.RedisLastProcessedVoteIDKey)
		totalVotesCmd = pipe.Get(database.Ctx, metadata.RedisTotalVotesKey)
		statsMapCmd = pipe.HGetAll(database.Ctx, spell.StatsKey)
		sortedIDsCmd = pipe.ZRevRange(database.Ctx, spell.RankingKey, 0, -1)
		dirtyUserIDsCmd := pipe.SMembers(database.Ctx, user.DirtySetKey)
		if dirtySetExists > 0 {
			pipe.Rename(database.Ctx, user.DirtySetKey, user.ProcessingDirtySetKey)
		}
		_, err = pipe.Exec(database.Ctx)

		if err != nil {
			return false, fmt.Errorf("无法从Redis原子地获取快照数据: %w", err)
		}
		// TxPipeline 成功后，transferred为true，代表 DirtySetKey 已被消费

		dirtyUserIDs, err = dirtyUserIDsCmd.Result()
		if err != nil {
			return true, fmt.Errorf("获取 dirtyUserIDs 的结果时失败: %w", err)
		}
		if len(dirtyUserIDs) > 0 {
			dirtyUserStats, err = database.RDB.HMGet(database.Ctx, user.StatsKey, dirtyUserIDs...).Result()
			if err != nil {
				return true, fmt.Errorf("获取 dirtyUserStats 的结果时失败: %w", err)
			}
		}

		return true, nil
	}()

	if transferred {
		defer func() {
			if err != nil {
				pipe := database.RDB.TxPipeline()
				pipe.SUnionStore(database.Ctx, user.DirtySetKey, user.DirtySetKey, user.ProcessingDirtySetKey)
				pipe.Del(database.Ctx, user.ProcessingDirtySetKey)
				pipe.Exec(database.Ctx)
			} else {
				database.RDB.Del(database.Ctx, user.ProcessingDirtySetKey)
			}
		}()
	}

	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 2. 准备将写入SQLite的数据
	lastVoteIDUint64, err := lastVoteIDCmd.Uint64()
	if err != nil {
		return fmt.Errorf("获取 lastVoteIDUint64 的结果时失败: %w", err)
	}
	lastVoteID := uint(lastVoteIDUint64)

	lastSnapshotVoteID, err := metadata.GetLastSnapshotVoteID(database.DB)
	if err != nil {
		return fmt.Errorf("获取 lastSnapshotVoteID 失败: %w", err)
	}
	// 无需备份
	if lastVoteID == lastSnapshotVoteID {
		return nil
	}

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
	spellsToUpsert := make([]spell.Spell, 0, len(sortedSpellIDs))
	for i, spellID := range sortedSpellIDs {
		rank := i + 1
		statsJSON, ok := statsMap[spellID]
		if !ok {
			return fmt.Errorf("备份警告: 在stats哈希表中找不到ID为 %s 的法术.\n", spellID)
		}

		var stats spell.SpellStats
		if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
			return fmt.Errorf("备份警告: 解析法术 %s 的数据失败: %w\n", spellID, err)
		}

		spellToUpdate := spell.Spell{
			SpellID:   spellID, // 额外包含主键
			Score:     stats.Score,
			Total:     stats.Total,
			Win:       stats.Win,
			Rank:      rank,
			RankScore: stats.RankScore,
		}

		spellsToUpsert = append(spellsToUpsert, spellToUpdate)
	}

	usersToUpsert := make([]user.User, 0, len(dirtyUserIDs))
	for i, userID := range dirtyUserIDs {
		userStatsJSON := dirtyUserStats[i].(string)

		var userStats user.UserStats
		if err := json.Unmarshal([]byte(userStatsJSON), &userStats); err != nil {
			return fmt.Errorf("警告：解析用户 %s 的统计数据JSON失败: %w\n", userID, err)
		}

		userToUpsert := user.User{
			UUID:      userID,
			WinsCount: userStats.Wins,
			DrawCount: userStats.Draw,
			SkipCount: userStats.Skip,
		}
		usersToUpsert = append(usersToUpsert, userToUpsert)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 3. 将快照数据持久化到SQLite
	const maxRetry = 3
	const delay = 50 * time.Millisecond
	for i := 0; i < maxRetry; i++ {
		err = database.DB.Transaction(func(tx *gorm.DB) error {
			// a. 持久化spell模块的数据
			// OnConflict 模拟 UPDATE 操作
			// 冲突的判断依据是spell_id，模拟主键唯一
			err = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "spell_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"score", "total", "win", "rank", "rank_score"}),
			}).Create(&spellsToUpsert).Error

			if err != nil {
				return fmt.Errorf("批量更新法术数据失败: %w", err)
			}

			// b. 持久化user模块的数据
			// 使用 OnConflict 执行 UPSERT 操作
			// 如果UUID已存在，则更新统计字段和updated_at；否则，插入新行。
			err = tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "uuid"}},
				DoUpdates: clause.AssignmentColumns([]string{"wins_count", "draw_count", "skip_count", "updated_at"}),
			}).Create(&usersToUpsert).Error

			if err != nil {
				return fmt.Errorf("持久化用户数据失败: %w", err)
			}

			// c. 更新metadata模块的元数据
			if err := metadata.SetLastSnapshotVoteID(tx, lastVoteID); err != nil {
				return fmt.Errorf("更新元数据 LastSnapshotVoteID 失败: %w", err)
			}
			if err := metadata.SetSnapshotTotalVotes(tx, totalVotes); err != nil {
				return fmt.Errorf("更新元数据 SnapshotTotalVotes 失败: %w", err)
			}

			return nil
		})

		if err == nil || !database.IsRetryableError(err) {
			break
		}
	}
	return err
}
