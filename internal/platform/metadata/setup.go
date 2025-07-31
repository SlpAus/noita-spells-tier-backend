package metadata

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
)

// migrateDB 负责自动迁移数据库表结构
func migrateDB() error {
	if err := database.DB.AutoMigrate(&Metadata{}); err != nil {
		return fmt.Errorf("无法迁移metadata表: %w", err)
	}
	fmt.Println("Metadata数据库表迁移成功。")
	return nil
}

// WarmupCache 从SQLite加载元数据并预热到Redis。
func WarmupCache() error {
	fmt.Println("正在预热Metadata缓存...")
	// 1. 获取持久化的快照Vote ID和总投票数
	lastSnapshotVoteID, err := GetLastSnapshotVoteID(database.DB)
	if err != nil {
		return fmt.Errorf("无法从SQLite读取last_snapshot_vote_id: %w", err)
	}
	snapshotTotalVotes, err := GetSnapshotTotalVotes(database.DB)
	if err != nil {
		return fmt.Errorf("无法从SQLite读取snapshot_total_votes: %w", err)
	}

	// 2. 使用Pipeline将这些值写入Redis，作为实时计数器的初始值
	pipe := database.RDB.Pipeline()
	pipe.Set(database.Ctx, RedisLastProcessedVoteIDKey, lastSnapshotVoteID, 0)
	pipe.Set(database.Ctx, RedisTotalVotesKey, snapshotTotalVotes, 0)
	_, err = pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热元数据到Redis失败: %w", err)
	}

	fmt.Println("Metadata缓存预热成功。")
	return nil
}

// PrimeCachedDB 是metadata模块的初始化总入口
func PrimeCachedDB() error {
	if err := migrateDB(); err != nil {
		return err
	}
	if err := WarmupCache(); err != nil {
		return err
	}
	return nil
}
