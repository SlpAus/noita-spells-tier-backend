package database

import (
	"fmt"
	"sync"
)

// statusManager 负责线程安全地管理和提供系统的健康状态。
type statusManager struct {
	mu             sync.RWMutex
	isRedisHealthy bool
	lastKnownRunID string
}

// 全局的状态管理器实例
var globalStatus = &statusManager{
	isRedisHealthy: true, // 默认启动时是健康的
}

// IsRedisHealthy 返回当前Redis的健康状态。
func IsRedisHealthy() bool {
	globalStatus.mu.RLock()
	defer globalStatus.mu.RUnlock()
	return globalStatus.isRedisHealthy
}

// SetInitialRunID 在应用启动时，由main.go调用，用于设置初始的Redis run_id。
func SetInitialRunID(runID string) {
	globalStatus.mu.Lock()
	defer globalStatus.mu.Unlock()
	globalStatus.lastKnownRunID = runID
}

// UpdateStatus 用于线程安全地更新健康状态。
func UpdateStatus(isHealthy bool, newRunID string) {
	globalStatus.mu.Lock()
	defer globalStatus.mu.Unlock()

	// 只有当状态发生变化时才打印日志
	if globalStatus.isRedisHealthy != isHealthy {
		globalStatus.isRedisHealthy = isHealthy
		if isHealthy {
			fmt.Println("健康检查: Redis服务状态已更新为 [可用]")
		} else {
			fmt.Println("健康检查警告: Redis服务状态已更新为 [不可用]")
		}
	}

	// 只有在健康状态下，才更新已知的run_id
	if isHealthy {
		globalStatus.lastKnownRunID = newRunID
	}
}

// GetLastKnownRunID 用于线程安全地获取已知的run_id。
func GetLastKnownRunID() string {
	globalStatus.mu.RLock()
	defer globalStatus.mu.RUnlock()
	return globalStatus.lastKnownRunID
}
