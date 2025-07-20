package startup

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
)

// InitializeApplication 是应用首次启动时执行的总入口
func InitializeApplication() error {
	fmt.Println("开始应用首次初始化...")

	// 首次启动时，我们需要迁移数据库并预热缓存
	if err := metadata.PrimeDB(); err != nil {
		return err
	}
	if err := user.PrimeCachedDB(); err != nil {
		return err
	}
	if err := spell.PrimeCachedDB(); err != nil {
		return err
	}
	if err := vote.PrimeDB(); err != nil {
		return err
	}

	fmt.Println("应用初始化完成！")
	return nil
}

// RebuildCache 是一个专门用于在运行时热重建Redis缓存的函数
func RebuildCache() error {
	fmt.Println("开始缓存热重建...")

	// 1. 重建user缓存
	if err := user.WarmupCache(); err != nil {
		return err
	}
	// 2. 重建spell缓存（加载上一次的快照）
	if err := spell.WarmupCache(); err != nil {
		return err
	}
	// 3. *** 新增：处理自上次快照以来的所有增量投票 ***
	if err := vote.ApplyIncrementalVotes(); err != nil {
		return err
	}

	fmt.Println("缓存热重建完成！")
	return nil
}
