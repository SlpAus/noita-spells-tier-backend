package health

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/startup"
)

const (
	checkInterval = 5 * time.Second
	pingTimeout   = 2 * time.Second
)

// getRedisRunID 从Redis服务器信息中提取run_id
func getRedisRunID() (string, error) {
	ctx, cancel := context.WithTimeout(database.Ctx, pingTimeout)
	defer cancel()
	info, err := database.RDB.Info(ctx, "server").Result()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`run_id:([a-f0-9]+)`)
	matches := re.FindStringSubmatch(info)
	if len(matches) < 2 {
		return "", fmt.Errorf("无法在Redis INFO中找到run_id")
	}
	return matches[1], nil
}

// InitializeRunID 在应用启动时执行一次，获取并设置初始的run_id。
// 如果失败，将直接panic。
func InitializeRunID() {
	fmt.Println("正在获取初始Redis Run ID...")
	runID, err := getRedisRunID()
	if err != nil {
		panic(fmt.Sprintf("无法在启动时获取Redis Run ID，请检查Redis服务: %v", err))
	}
	globalStatus.SetInitialRunID(runID)
	fmt.Printf("获取初始Redis Run ID成功: %s\n", runID)
}

// PerformCheck 执行一次完整的健康检查和可能的状态转换/修复操作。
// 这个函数现在是阻塞的，可以被main.go和后台循环共同调用。
func PerformCheck() {
	// 1. 探测当前状态
	runID, err := getRedisRunID()
	isCurrentlyConnected := (err == nil)

	// 2. 评估状态并获取行动指令
	needsRebuild := globalStatus.Assess(isCurrentlyConnected, runID)

	// 3. 根据指令执行操作
	if needsRebuild {
		fmt.Println("健康检查: 正在触发缓存热重建...")
		rebuildErr := startup.RebuildCache()

		// *** 新增逻辑：重建后再次检查run_id ***
		runIDAfterRebuild, errAfterRebuild := getRedisRunID()
		if errAfterRebuild != nil {
			// 如果在重建后立刻就无法连接，说明重建是无效的
			fmt.Println("健康检查错误: 缓存重建后无法连接到Redis，标记为重建失败。")
			globalStatus.MarkRebuildComplete(false, "")
			return
		}

		// 4. 将操作结果和最新的run_id反馈给状态管理器
		globalStatus.MarkRebuildComplete(rebuildErr == nil, runIDAfterRebuild)
	}
}

// StartRedisHealthCheck 启动一个后台Goroutine来定期驱动健康状态机
func StartRedisHealthCheck() {
	fmt.Println("Redis高级健康检查器已启动。")
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		PerformCheck()
	}
}
