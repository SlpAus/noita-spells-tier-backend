package startup

import (
	"context"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/backup"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/report"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
)

// InitializeApplication 是应用首次启动时执行的总入口
func InitializeApplication() error {
	fmt.Println("开始应用首次初始化...")

	if err := metadata.PrimeCachedDB(); err != nil {
		return err
	}
	if err := user.PrimeCachedDB(); err != nil {
		return err
	}
	if err := spell.PrimeCachedDB(); err != nil {
		return err
	}
	if err := vote.PrimeModule(); err != nil {
		return err
	}

	fmt.Println("应用初始化完成！")
	return nil
}

// RebuildCache 是一个专门用于在运行时热重建Redis缓存的函数
func RebuildCache() error {
	fmt.Println("开始缓存热重建...")

	if err := metadata.WarmupCache(); err != nil {
		return err
	}

	err := func() error {
		spell.LockRepository()
		defer spell.UnlockRepository()
		if err := spell.WarmupCache(); err != nil {
			return err
		}

		user.LockRepository()
		defer user.UnlockRepository()
		if err := user.WarmupCache(); err != nil {
			return err
		}

		if err := vote.RebuildAndApplyVotes(); err != nil {
			return err
		}
		return nil
	}()

	if err != nil {
		return err
	}

	// 触发一次新的快照
	fmt.Println("缓存热重建完成，正在触发一次新的数据快照...")
	if err := backup.CreateConsistentSnapshotInDB(context.Background()); err != nil {
		fmt.Printf("警告: 缓存热重建后的快照创建失败: %v\n", err)
	}
	fmt.Println("快照创建成功！")

	return nil
}

// HandleRedisRecovery 在Redis从不健康状态恢复时，执行必要的清理和恢复操作。
func HandleRedisRecovery() {
	fmt.Println("检测到Redis已恢复，正在执行恢复后操作...")
	report.ClearMirrorRepo()
	fmt.Println("恢复后操作完成。")
}
