package user

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/redis/go-redis/v9"
)

// PrimeCachedDB 是user模块在应用启动时调用的主设置函数。
// 它负责迁移数据库表，并调用WarmupCache来初始化Redis缓存。
func PrimeCachedDB() (err error) {
	// 迁移 users 表
	if err := migrateDB(); err != nil {
		return err
	}

	// 初始化缓存
	if err = WarmupCache(); err != nil {
		return fmt.Errorf("user模块缓存预热失败: %w", err)
	}

	return nil
}

// migrateDB 负责自动迁移数据库表结构
func migrateDB() error {
	if err := database.DB.AutoMigrate(&User{}); err != nil {
		return fmt.Errorf("无法迁移user表: %w", err)
	}
	fmt.Println("User数据库表迁移成功。")
	return nil
}

// WarmupCache 从SQLite数据库中读取所有用户数据，并用其重建Redis中的缓存。
// 这个过程是破坏性的，会先清空旧的缓存数据。
// 注意：此函数不包含锁，调用方需要确保在安全的时机（如单线程启动或重建大范围锁下）调用。
func WarmupCache() error {
	fmt.Println("开始预热user模块缓存...")

	// 1. 清空所有相关的Redis键
	pipe := database.RDB.Pipeline()
	pipe.Del(database.Ctx, StatsKey)
	pipe.Del(database.Ctx, RankingKey)
	pipe.Del(database.Ctx, DirtySetKey)
	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("清空旧的user缓存失败: %w", err)
	}
	fmt.Println("旧的user缓存已清空.")

	// 2. 初始化社区总票数累加器
	totalStats := UserStats{}

	// 3. 分批从SQLite读取数据并写入Redis
	const batchSize = 10000

	var batch []User
	lastID := ""
	for {
		// 从数据库中分批读取用户
		if err := database.DB.Order("uuid asc").Limit(batchSize).Where("uuid > ?", lastID).Find(&batch).Error; err != nil {
			return fmt.Errorf("从数据库分批读取用户失败 (uuid > %s): %w", lastID, err)
		}

		// 如果没有更多用户，则结束循环
		if len(batch) == 0 {
			break
		}

		// 准备当前批次写入Redis的数据
		statsPayload := make(map[string]interface{})
		rankingPayload := make([]redis.Z, 0, len(batch))

		for _, user := range batch {
			// 准备 user:stats (Hash) 的数据
			stats := UserStats{
				Wins: user.WinsCount,
				Draw: user.DrawCount,
				Skip: user.SkipCount,
			}
			statsJSON, err := json.Marshal(stats)
			if err != nil {
				// 理论上这个错误不应该发生
				return fmt.Errorf("警告：序列化用户 %s 的统计数据失败: %w", user.UUID, err)
			}
			statsPayload[user.UUID] = string(statsJSON)

			// 准备 user:ranking (Sorted Set) 的数据
			totalVotes := float64(user.WinsCount + user.DrawCount + user.SkipCount)
			rankingPayload = append(rankingPayload, redis.Z{
				Score:  totalVotes,
				Member: user.UUID,
			})

			// 累加到社区总票数
			totalStats.Wins += user.WinsCount
			totalStats.Draw += user.DrawCount
			totalStats.Skip += user.SkipCount
		}

		// 使用Pipeline写入当前批次的数据
		if len(statsPayload) > 0 {
			pipe := database.RDB.Pipeline()
			pipe.HSet(database.Ctx, StatsKey, statsPayload)
			pipe.ZAdd(database.Ctx, RankingKey, rankingPayload...)
			if _, err := pipe.Exec(database.Ctx); err != nil {
				return fmt.Errorf("写入批次到Redis失败 (uuid > %s): %w", lastID, err)
			}
		}

		// 更新 lastID 为当前批次的最后一条记录的ID
		lastID = batch[len(batch)-1].UUID

		batch = batch[:0]
	}

	// 4. 将最终的社区总票数写入Redis
	totalStatsJSON, err := json.Marshal(totalStats)
	if err != nil {
		return fmt.Errorf("序列化社区总统计数据失败: %w", err)
	}
	if err := database.RDB.HSet(database.Ctx, StatsKey, TotalStatsKey, string(totalStatsJSON)).Err(); err != nil {
		return fmt.Errorf("写入社区总统计数据到Redis失败: %w", err)
	}

	fmt.Println("user模块缓存预热完成.")
	return nil
}
