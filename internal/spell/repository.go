package spell

import (
	"fmt"
	"sync"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/tree"
)

// --- Redis-specific Definitions ---
// 这些定义现在归属于仓库，因为它们描述了仓库所管理的外部动态数据结构

const (
	// StatsKey 是一个Redis Hash，存储所有法术的动态统计数据
	StatsKey = "spell_stats"
	// RankingKey 是一个Redis Sorted Set，用于按分数实时排序法术
	RankingKey = "spell_ranking"
)

// SpellStats 定义了在Redis spell_stats Hash中存储的法术动态数据
type SpellStats struct {
	Score     float64 `json:"score"`
	Total     float64 `json:"total"`
	Win       float64 `json:"win"`
	RankScore float64 `json:"rankScore"` // 最终用于排名的动态分数
}

// --- In-memory Repository ---

// SpellInfo 持有法术的静态数据，在程序启动时加载到内存中
type SpellInfo struct {
	Name        string
	Description string
	Sprite      string
	Type        int
}

// repository 是spell模块的中央数据仓库
type repository struct {
	// 内存中的静态数据，启动后只读
	idToIndex   map[string]int
	indexToInfo []SpellInfo
	indexToID   []string

	// 用于“冷门优先”匹配算法的动态权重树
	weightsTree *tree.SegmentTree
	rwLock      sync.RWMutex
}

// globalRepository 是我们仓库的私有单例实例
var globalRepository *repository

// InitializeRepository 从SQLite加载静态法术数据，初始化内存仓库。
// 这个函数应该在应用启动时且仅调用一次。
func InitializeRepository() error {
	var spellsFromDB []Spell
	if err := database.DB.Order("id asc").Find(&spellsFromDB).Error; err != nil {
		return fmt.Errorf("无法从SQLite加载法术静态数据: %w", err)
	}

	size := len(spellsFromDB)
	if size == 0 {
		return fmt.Errorf("法术静态数据为空，无法初始化仓库")
	}

	globalRepository = &repository{
		idToIndex:   make(map[string]int, size),
		indexToInfo: make([]SpellInfo, size),
		indexToID:   make([]string, size),
	}

	for i, s := range spellsFromDB {
		globalRepository.idToIndex[s.SpellID] = i
		globalRepository.indexToID[i] = s.SpellID
		globalRepository.indexToInfo[i] = SpellInfo{
			Name:        s.Name,
			Description: s.Description,
			Sprite:      s.Sprite,
			Type:        s.Type,
		}
	}

	segTree, err := tree.NewSegmentTree(size)
	if err != nil {
		return fmt.Errorf("无法创建线段树: %w", err)
	}
	// 树的初始权重将在WarmupCache阶段根据动态数据进行重建
	globalRepository.weightsTree = segTree

	InitializeGaussianMatcher(size)

	fmt.Printf("法术仓库 (Repository) 初始化成功，加载了 %d 个法术。\n", size)
	return nil
}

// --- Public Methods for Concurrency Control ---

// RLockRepository 获取用于读取权重树的读锁。
func RLockRepository() {
	globalRepository.rwLock.RLock()
}

// RUnlockRepository 释放读锁。
func RUnlockRepository() {
	globalRepository.rwLock.RUnlock()
}

// LockRepository 获取用于写入权重树的写锁。
func LockRepository() {
	globalRepository.rwLock.Lock()
}

// UnlockRepository 释放写锁。
func UnlockRepository() {
	globalRepository.rwLock.Unlock()
}

// --- Public Methods for Data Access ---
// 这些方法是线程安全的，因为它们访问的是启动后只读的数据。

func GetSpellCount() int {
	if globalRepository == nil {
		return 0
	}
	return len(globalRepository.indexToInfo)
}

func GetSpellInfoByIndex(index int) (SpellInfo, bool) {
	if globalRepository == nil || index < 0 || index >= len(globalRepository.indexToInfo) {
		return SpellInfo{}, false
	}
	return globalRepository.indexToInfo[index], true
}

func GetSpellIDByIndex(index int) (string, bool) {
	if globalRepository == nil || index < 0 || index >= len(globalRepository.indexToID) {
		return "", false
	}
	return globalRepository.indexToID[index], true
}

func GetSpellIndexByID(id string) (int, bool) {
	if globalRepository == nil {
		return -1, false
	}
	index, ok := globalRepository.idToIndex[id]
	return index, ok
}

// --- Unsafe Methods for Internal Use ---
// 这些方法必须在手动获取锁之后才能被安全调用。

func GetTotalWeightUnsafe() float64 {
	return globalRepository.weightsTree.TotalSum()
}

func GetWeightUnsafe(index int) (float64, error) {
	return globalRepository.weightsTree.Query(index)
}

func GetWeightPrefixUnsafe(index int) (float64, error) {
	return globalRepository.weightsTree.PrefixSum(index)
}

func UpdateWeightUnsafe(index int, weight float64) error {
	return globalRepository.weightsTree.Update(index, weight)
}

func FindByWeightUnsafe(weight float64) (int, error) {
	return globalRepository.weightsTree.Find(weight)
}
