package vote

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
)

// initializeEloTracker 从Redis获取所有法术的ELO分数，并用它们来初始化全局的eloTracker。
func initializeEloTracker() error {
	// 1. 从Redis的spell_stats Hash中获取所有法术的统计数据
	statsMapJSON, err := database.RDB.HGetAll(database.Ctx, spell.StatsKey).Result()
	if err != nil {
		return fmt.Errorf("无法从Redis获取法术统计数据: %w", err)
	}

	if len(statsMapJSON) == 0 {
		fmt.Println("ELO追踪器: 无法术数据，跳过初始化。")
		return nil
	}

	// 2. 提取所有的Score值
	scores := make([]float64, 0, len(statsMapJSON))
	for _, jsonStr := range statsMapJSON {
		var stats spell.SpellStats
		err := json.Unmarshal([]byte(jsonStr), &stats)
		if err != nil {
			return fmt.Errorf("解析从Redis获取的JSON时出错: %w", err)
		}
		scores = append(scores, stats.Score)
	}

	// 3. 使用分数列表重置eloTracker
	// 我们传入nil作为事务句柄，因为它是在单线程的启动流程中被调用的
	return globalEloTracker.Reset(nil, scores)
}

// PrimeModule 负责初始化vote模块的所有部分：数据库、用户同步和辅助组件。
func PrimeModule() error {
	// 1. 迁移自己的表结构
	if err := database.DB.AutoMigrate(&Vote{}); err != nil {
		return fmt.Errorf("无法迁移vote表: %w", err)
	}
	fmt.Println("Vote数据库表迁移成功。")

	// 2. 从自己的表中提取所有唯一的用户ID，并驱动user模块的初始化
	var userIDs []string
	err := database.DB.Model(&Vote{}).Where("user_identifier != ?", "").Distinct("user_identifier").Pluck("user_identifier", &userIDs).Error
	if err != nil {
		return fmt.Errorf("无法从vote表提取用户ID: %w", err)
	}
	if err := user.BatchCreateUsers(userIDs); err != nil {
		return fmt.Errorf("将用户同步到user模块失败: %w", err)
	}

	// 3. 初始化内部辅助组件
	if err := InitializeReplayDefense(); err != nil {
		return fmt.Errorf("初始化重放检测器失败: %w", err)
	}
	if err := RebuildIPVoteCache(); err != nil {
		return fmt.Errorf("初始化IP统计器失败: %w", err)
	}
	if err := initializeEloTracker(); err != nil {
		return fmt.Errorf("初始化ELO追踪器失败: %w", err)
	}

	return nil
}

// StartVoteProcessor 初始化并启动全局的VoteProcessor
func StartVoteProcessor(gracefulHandle, forcefulHandle *lifecycle.Handle) error {
	startID, err := metadata.GetLastSnapshotVoteID(database.DB)
	if err != nil {
		return fmt.Errorf("无法获取启动Vote Processor所需的快照ID: %w", err)
	}

	initializeProcessor(startID)
	go startProcessor(gracefulHandle, forcefulHandle)

	return nil
}

// RebuildAndApplyVotes 重置内部的两个辅助组件，并处理上次快照以来的增量投票
func RebuildAndApplyVotes() error {
	if err := InitializeReplayDefense(); err != nil {
		return fmt.Errorf("重置重放检测器失败: %w", err)
	}
	if err := RebuildIPVoteCache(); err != nil {
		return fmt.Errorf("重置IP统计器失败: %w", err)
	}

	if err := ApplyIncrementalVotes(); err != nil {
		return fmt.Errorf("处理增量投票失败: %w", err)
	}

	return nil
}
