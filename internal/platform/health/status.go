package health

import (
	"fmt"
	"sync"
)

// State 定义了系统健康状态的枚举类型
type State int

const (
	StateHealthy State = iota
	StateDegraded
	StateRebuilding
)

// statusManager 负责线程安全地管理和提供系统的健康状态。
type statusManager struct {
	mu             sync.RWMutex
	currentState   State
	lastKnownRunID string
}

var globalStatus = &statusManager{
	currentState: StateHealthy,
}

// GetState 返回当前的系统健康状态。
func GetState() State {
	globalStatus.mu.RLock()
	defer globalStatus.mu.RUnlock()
	return globalStatus.currentState
}

// SetInitialRunID 在应用启动时，由main.go调用，用于设置初始的Redis run_id。
func (sm *statusManager) SetInitialRunID(runID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastKnownRunID = runID
}

// Assess a new health check result and decide the next state.
func (sm *statusManager) Assess(isCurrentlyConnected bool, newRunID string) (needsRebuild bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	switch sm.currentState {
	case StateHealthy:
		if !isCurrentlyConnected {
			sm.currentState = StateDegraded
			fmt.Println("健康检查: Redis连接丢失，系统状态 -> [降级]")
		} else if sm.lastKnownRunID != "" && sm.lastKnownRunID != newRunID {
			sm.currentState = StateRebuilding
			needsRebuild = true
			fmt.Printf("健康检查: 检测到Redis重启 (run_id: %s -> %s)，系统状态 -> [重建中]\n", sm.lastKnownRunID, newRunID)
		}
	case StateDegraded:
		if isCurrentlyConnected {
			if sm.lastKnownRunID != "" && sm.lastKnownRunID != newRunID {
				sm.currentState = StateRebuilding
				needsRebuild = true
				fmt.Printf("健康检查: Redis已恢复但检测到重启 (run_id: %s -> %s)，系统状态 -> [重建中]\n", sm.lastKnownRunID, newRunID)
			} else {
				sm.currentState = StateHealthy
				fmt.Println("健康检查: Redis连接已恢复，系统状态 -> [健康]")
			}
		}
	case StateRebuilding:
		if !isCurrentlyConnected {
			sm.currentState = StateDegraded
			fmt.Println("健康检查: 在缓存重建期间Redis连接再次丢失，系统状态 -> [降级]")
		} else {
			// *** 已应用您的修正 ***
			// 如果连接是好的，但我们仍处于重建状态，说明上次重建失败了。
			needsRebuild = true
			fmt.Println("健康检查: 系统处于[重建中]状态，将再次尝试重建缓存...")
		}
	}

	if isCurrentlyConnected {
		sm.lastKnownRunID = newRunID
	}

	return needsRebuild
}

// MarkRebuildComplete should be called after a rebuild attempt.
func (sm *statusManager) MarkRebuildComplete(success bool, runIDAfterRebuild string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentState != StateRebuilding {
		return
	}

	// *** 新增逻辑：检查重建期间Redis是否再次重启 ***
	if success && sm.lastKnownRunID != runIDAfterRebuild {
		fmt.Printf("健康检查错误: 缓存重建期间检测到Redis再次重启 (run_id: %s -> %s)。重建无效，保持[重建中]状态。\n", sm.lastKnownRunID, runIDAfterRebuild)
		sm.lastKnownRunID = runIDAfterRebuild // 更新为最新的run_id
		return
	}

	if success {
		sm.currentState = StateHealthy
		fmt.Println("健康检查: 缓存重建成功，系统状态 -> [健康]")
	} else {
		fmt.Println("健康检查错误: 缓存重建失败，系统状态保持 [重建中] 以待重试")
	}
}
