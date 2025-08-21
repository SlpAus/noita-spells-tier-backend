package spell

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/redis/go-redis/v9"
)

func ConfigureModule(mode config.AppMode) {
	loadAlgorithmConsts(mode)
	initHandlerMode(mode)
}

// PrimeCachedDB 负责初始化spell模块的数据库和内存仓库
func PrimeCachedDB() error {
	// 1. 迁移数据库表结构
	if err := migrateDB(); err != nil {
		return err
	}
	// 2. 从数据库加载静态数据到内存仓库
	if err := InitializeRepository(); err != nil {
		return err
	}
	// 3. 将动态数据预热到Redis，并初始化权重树
	if err := WarmupCache(); err != nil {
		return err
	}
	return nil
}

// migrateDB 负责自动迁移数据库表结构
func migrateDB() error {
	if err := database.DB.AutoMigrate(&Spell{}); err != nil {
		return fmt.Errorf("无法迁移spell表: %w", err)
	}
	fmt.Println("Spell数据库表迁移成功。")
	return nil
}

// WarmupCache 从SQLite加载动态数据到Redis，并根据这些数据重建内存中的权重树
// 注意：此函数不包含锁，调用方需要确保在安全的时机（如单线程启动或重建大范围锁下）调用。
func WarmupCache() error {
	var spellsInDB []Spell
	if err := database.DB.Find(&spellsInDB).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取法术数据: %w", err)
	}

	pipe := database.RDB.Pipeline()
	// 只清空动态数据的Redis键
	pipe.Del(database.Ctx, StatsKey, RankingKey)

	// 准备用于重建权重树的初始权重
	initialWeights := make([]float64, GetSpellCount())

	for _, spell := range spellsInDB {
		// 准备动态统计数据 (spell:stats Hash)
		stats := SpellStats{
			Score:     spell.Score,
			Total:     spell.Total,
			Win:       spell.Win,
			RankScore: spell.RankScore, // 增加新字段
		}
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, StatsKey, spell.SpellID, statsJSON)

		// 准备排名数据 (spell:ranking Sorted Set)
		pipe.ZAdd(database.Ctx, RankingKey, redis.Z{
			Score:  spell.RankScore, // 使用RankScore作为排名依据
			Member: spell.SpellID,
		})

		// 计算初始权重 (补充：这里是冷门优先算法的核心)
		index, ok := GetSpellIndexByID(spell.SpellID)
		if ok {
			initialWeights[index] = CalculateWeightForTotal(spell.Total)
		}
	}

	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热法术动态数据到Redis失败: %w", err)
	}

	// 在预热Redis后，使用正确的初始权重重建内存中的线段树
	if err := globalRepository.weightsTree.Rebuild(initialWeights); err != nil {
		return fmt.Errorf("无法使用初始权重重建线段树: %w", err)
	}

	fmt.Printf("成功预热 %d 条法术的动态数据到Redis，并重建了权重树。\n", len(spellsInDB))
	return nil
}
