package user

import (
	"sync"
)

// --- Redis 键名常量 ---

const (
	// StatsKey 是一个 Redis Hash 的键，用于存储每个用户的详细统计信息。
	// Field: 用户的UUID 或 TotalStatsKey
	// Value: UserStats 结构体的JSON序列化字符串
	StatsKey = "user:stats"

	// RankingKey 是一个 Redis Sorted Set 的键，用于存储用户的投票数排名。
	// Score: 用户的总投票数 (Wins + Draw + Skip)
	// Member: 用户的UUID
	RankingKey = "user:ranking"

	// DirtySetKey 是一个 Redis Set 的键，用于存储自上次快照以来，
	// 统计数据发生变化的用户UUID。用于增量备份。
	DirtySetKey = "user:dirty"

	// ProcessingDirtySetKey 是一个 Redis Set 的键
	// 保留它，只在备份逻辑中被使用
	ProcessingDirtySetKey = "user:dirty:processing"
)

// --- 特殊键与常量 ---

const (
	// TotalStatsKey 是在 StatsKey (Hash) 中使用的一个特殊字段，
	// 用于存储所有投票（包括匿名投票）的社区总体统计数据。
	// 区别于用户的UUID，不使用UUID格式。
	TotalStatsKey = "_total_"
)

// --- Redis 数据结构 ---

// UserStats 定义了在 Redis 的 user:stats 哈希表中，
// 以JSON格式存储的用户统计数据结构。
type UserStats struct {
	Wins int `json:"wins"`
	Draw int `json:"draw"`
	Skip int `json:"skip"`
}

// --- 并发控制 ---

// repoMutex 是一个模块内部的、不导出的全局读写锁，
// 用于保护对本模块管理的Redis键的并发访问。
var repoMutex sync.RWMutex

// LockRepository 封装了对模块全局锁的写锁定操作。
func LockRepository() {
	repoMutex.Lock()
}

// UnlockRepository 封装了对模块全局锁的写解锁操作。
func UnlockRepository() {
	repoMutex.Unlock()
}

// RLockRepository 封装了对模块全局锁的读锁定操作。
func RLockRepository() {
	repoMutex.RLock()
}

// RUnlockRepository 封装了对模块全局锁的读解锁操作。
func RUnlockRepository() {
	repoMutex.RUnlock()
}
