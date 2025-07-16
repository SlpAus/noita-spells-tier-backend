package spell

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/go-redis/redis/v8"
)

// --- Service-Level Data Transfer Objects (DTOs) ---
// 这些结构体用于在服务层内部和向控制器层传递数据

// RankedSpellDTO 包含了排行榜API所需的所有数据
type RankedSpellDTO struct {
	ID    string
	Info  SpellInfo
	Stats SpellStats
}

// SpellImageDTO 包含了获取单个法术图片API所需的数据
type SpellImageDTO struct {
	ID   string
	Info SpellInfo
}

// --- Service Functions ---

// GetRankedSpells 从Redis中获取完整的、已排序的法术列表
func GetRankedSpells() ([]RankedSpellDTO, error) {
	// 1. 从Sorted Set获取所有法术ID，按分数从高到低排序
	spellIDs, err := database.RDB.ZRevRange(database.Ctx, RankingKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("无法从Redis获取排行榜ID: %w", err)
	}
	if len(spellIDs) == 0 {
		return []RankedSpellDTO{}, nil
	}

	// 2. 使用Pipeline一次性获取所有法术的静态和动态数据
	pipe := database.RDB.Pipeline()
	infoCmd := pipe.HMGet(database.Ctx, InfoKey, spellIDs...)
	statsCmd := pipe.HMGet(database.Ctx, StatsKey, spellIDs...)
	if _, err := pipe.Exec(database.Ctx); err != nil {
		return nil, fmt.Errorf("执行Redis Pipeline失败: %w", err)
	}
	infoJSONs, _ := infoCmd.Result()
	statsJSONs, _ := statsCmd.Result()

	// 3. 组合成DTO列表
	var rankedSpells []RankedSpellDTO
	for i, id := range spellIDs {
		var info SpellInfo
		var stats SpellStats
		if infoJSONs[i] != nil {
			_ = json.Unmarshal([]byte(infoJSONs[i].(string)), &info)
		}
		if statsJSONs[i] != nil {
			_ = json.Unmarshal([]byte(statsJSONs[i].(string)), &stats)
		}
		rankedSpells = append(rankedSpells, RankedSpellDTO{
			ID:    id,
			Info:  info,
			Stats: stats,
		})
	}
	return rankedSpells, nil
}

// GetSpellImageInfoByID 从Redis中获取单个法术的图片所需信息
func GetSpellImageInfoByID(spellID string) (*SpellImageDTO, error) {
	// 1. 从Hash中获取静态数据
	infoJSON, err := database.RDB.HGet(database.Ctx, InfoKey, spellID).Result()
	if err == redis.Nil {
		return nil, nil // 未找到
	}
	if err != nil {
		return nil, fmt.Errorf("无法从Redis获取法术 %s 的数据: %w", spellID, err)
	}

	// 2. 组合成DTO
	var info SpellInfo
	if err := json.Unmarshal([]byte(infoJSON), &info); err != nil {
		return nil, fmt.Errorf("无法解析法术 %s 的数据: %w", spellID, err)
	}
	return &SpellImageDTO{
		ID:   spellID,
		Info: info,
	}, nil
}
