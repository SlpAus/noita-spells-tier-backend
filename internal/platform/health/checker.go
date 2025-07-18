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
func InitializeRunID() {
	fmt.Println("正在获取初始Redis Run ID...")
	runID, err := getRedisRunID()
	if err != nil {
		panic(fmt.Sprintf("无法在启动时获取Redis Run ID，请检查Redis服务: %v", err))
	}
	database.SetInitialRunID(runID)
	fmt.Printf("获取初始Redis Run ID成功: %s\n", runID)
}

// triggerAtomicRebuild 执行一次原子的、自校验的缓存重建。
// 它确保只有在重建期间Redis没有再次重启的情况下，才认为重建成功。
func triggerAtomicRebuild(idBeforeRebuild string) bool {
	fmt.Println("健康检查: 正在触发缓存热重建...")
	err := startup.RebuildCache()
	if err != nil {
		fmt.Printf("健康检查错误: 缓存热重建失败: %v\n", err)
		return false
	}

	// 重建后，再次检查run_id以确认原子性
	idAfterRebuild, err := getRedisRunID()
	if err != nil {
		fmt.Println("健康检查错误: 缓存重建后无法连接到Redis，重建无效。")
		return false
	}

	if idBeforeRebuild != idAfterRebuild {
		fmt.Printf("健康检查错误: 缓存重建期间检测到Redis再次重启 (run_id: %s -> %s)。重建无效。\n", idBeforeRebuild, idAfterRebuild)
		return false
	}

	fmt.Println("健康检查: 缓存热重建成功并通过原子性校验。")
	return true
}

// PerformCheck 执行一次完整的健康检查和可能的修复操作。
func PerformCheck() {
	currentRunID, err := getRedisRunID()
	if err != nil {
		// 无法连接到Redis，直接标记为不可用
		database.UpdateStatus(false, "")
		return
	}

	lastKnownRunID := database.GetLastKnownRunID()

	if currentRunID != lastKnownRunID {
		// 检测到Redis重启，触发原子重建
		rebuildSuccess := triggerAtomicRebuild(currentRunID)
		if rebuildSuccess {
			// 只有重建成功，才更新状态为可用，并更新已知的run_id
			database.UpdateStatus(true, currentRunID)
		} else {
			// 重建失败，保持不可用状态
			database.UpdateStatus(false, "")
		}
	} else {
		// run_id未变，说明服务健康
		database.UpdateStatus(true, currentRunID)
	}
}

// StartRedisHealthCheck 启动一个后台Goroutine来定期、阻塞式地执行健康检查。
func StartRedisHealthCheck() {
	fmt.Println("Redis高级健康检查器已启动。")
	// 使用 time.Timer 实现阻塞式循环
	timer := time.NewTimer(checkInterval)
	defer timer.Stop()

	for {
		<-timer.C                  // 等待定时器触发
		PerformCheck()             // 执行检查
		timer.Reset(checkInterval) // 重置定时器，从现在开始重新计时
	}
}
