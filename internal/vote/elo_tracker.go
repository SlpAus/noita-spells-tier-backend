package vote

import (
	"fmt"
	"sync"
)

type eloTrackerData struct {
	minScore float64
	minCount int
	maxScore float64
	maxCount int
}

// eloTracker 是一个线程安全的对象，用于追踪系统中所有法术的最低和最高ELO分数。
type eloTracker struct {
	mu   sync.RWMutex
	data eloTrackerData
}

// eloTrackerTx 代表一次对eloTracker的事务性更新操作。
// 它持有一个原始对象的快照，并负责在操作结束时根据提交状态进行回滚或解锁。
type eloTrackerTx struct {
	target    *eloTracker    // 指向要被修改的原始eloTracker实例
	backup    eloTrackerData // 创建事务时，原始实例的数据快照（仅数据，不含锁）
	committed bool           // 标记事务是否已被提交
}

// globalEloTracker 是一个私有的、全局的eloTracker实例。
var globalEloTracker = &eloTracker{}

// Reset 从一个给定的分数切片（无序）中初始化或重置追踪器。
func (et *eloTracker) Reset(tx *eloTrackerTx, scores []float64) error {
	if len(scores) < 1 {
		return fmt.Errorf("初始化ELO追踪器所需的分数列表长度至少为1")
	}

	if tx == nil || tx.target != et {
		et.mu.Lock()
		defer et.mu.Unlock()
	}

	et.data.minScore = scores[0]
	et.data.minCount = 0
	et.data.maxScore = scores[0]
	et.data.maxCount = 0

	// 计算最高分和最低分的数量
	for _, score := range scores {
		if score < et.data.minScore {
			et.data.minScore = score
			et.data.minCount = 1
			continue
		}
		if score == et.data.minScore {
			et.data.minCount++
		}
		if score == et.data.maxScore {
			et.data.maxCount++
			continue
		}
		if score > et.data.maxScore {
			et.data.maxScore = score
			et.data.maxCount = 1
		}
	}

	fmt.Printf("ELO追踪器已重置: Min(%.2f, %d个), Max(%.2f, %d个)\n", et.data.minScore, et.data.minCount, et.data.maxScore, et.data.maxCount)
	return nil
}

// Update 在一次投票后，根据分数变化更新追踪器。
// 如果这次更新导致了最低分或最高分被“清空”，或者引入了新的最低分或最高分，则返回true，表示需要进行全局重建。
func (et *eloTracker) Update(tx *eloTrackerTx, oldScore, newScore float64) bool {
	if oldScore == newScore {
		return false // 分数未变，无需操作
	}

	if tx == nil || tx.target != et {
		et.mu.Lock()
		defer et.mu.Unlock()
	}

	boundaryChanged := false

	// --- 1. 处理旧分数 ---
	if oldScore == et.data.minScore {
		et.data.minCount--
		if et.data.minCount == 0 {
			// 最低分持有者已全部改变分数，边界发生变化
			boundaryChanged = true
		}
	} else if oldScore == et.data.maxScore {
		et.data.maxCount--
		if et.data.maxCount == 0 {
			// 最高分持有者已全部改变分数，边界发生变化
			boundaryChanged = true
		}
	}

	// 如果边界已发生变化，我们立刻返回，让外部逻辑触发重建
	if boundaryChanged {
		// fmt.Printf("ELO边界发生变化: count(%.2f) -> 0。需要重建。\n", oldScore)
		return true
	}

	// --- 2. 处理新分数 (仅在边界未发生变化时) ---
	if newScore < et.data.minScore {
		// 新的分数成为了新的最低分，边界发生变化
		boundaryChanged = true
	} else if newScore == et.data.minScore {
		et.data.minCount++
	} else if newScore > et.data.maxScore {
		// 新的分数成为了新的最高分，边界发生变化
		boundaryChanged = true
	} else if newScore == et.data.maxScore {
		et.data.maxCount++
	}

	// if boundaryChanged {
	// 	fmt.Printf("ELO边界发生变化: old(%.2f) -> new(%.2f)。需要重建。\n", oldScore, newScore)
	// }

	return boundaryChanged
}

// GetMinMax 线程安全地返回当前追踪的最低和最高ELO分数。
func (et *eloTracker) GetMinMax(tx *eloTrackerTx) (min float64, max float64) {
	if tx == nil || tx.target != et {
		et.mu.RLock()
		defer et.mu.RUnlock()
	}
	return et.data.minScore, et.data.maxScore
}

// BeginUpdate 开始一次对eloTracker的事务性更新。
// 它会获取对原始tracker的“写锁”，并返回一个事务句柄。
// 调用者必须在函数结束时使用defer调用返回的句柄的RollbackUnlessCommitted()方法。
func (et *eloTracker) BeginUpdate() *eloTrackerTx {
	// 1. 获取一个排他性的写锁，在整个事务期间持有它
	et.mu.Lock()

	// 2. 创建一个事务句柄，其中包含了指向原始对象的指针和一份数据备份
	return &eloTrackerTx{
		target:    et,
		backup:    et.data,
		committed: false,
	}
}

// Commit 标记本次事务为成功。
// 这个方法应该在所有关联操作（如Redis写入）都成功后，在函数返回前调用。
func (tx *eloTrackerTx) Commit() {
	tx.committed = true
	tx.target.mu.Unlock()
	tx.target = nil
}

// RollbackUnlessCommitted 是一个用于defer调用的关键方法。
// 它会检查事务是否被提交。如果未提交，则用备份的数据恢复原始对象。
// 无论如何，它最终都会释放原始对象上的写锁。
func (tx *eloTrackerTx) RollbackUnlessCommitted() {
	if tx.committed {
		return
	}

	defer func() {
		tx.target.mu.Unlock()
		tx.target = nil
	}()

	// 使用备份的数据覆盖被修改的原始对象
	tx.target.data = tx.backup
	fmt.Println("eloTracker: 事务未提交，状态已自动回滚。")
}
