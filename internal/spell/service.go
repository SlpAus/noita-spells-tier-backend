package spell

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
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

// PairDataDTO 是 GetNewSpellPair 服务返回给控制器的最终数据包。
// 它包含了生成API响应所需的所有原始信息。
type PairDataDTO struct {
	SpellAInfo SpellInfo
	SpellBInfo SpellInfo
	Payload    token.TokenPayload
	Signature  string
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

// GetNewSpellPair 是获取法术对的核心业务逻辑
func GetNewSpellPair(excludeA, excludeB string) (*PairDataDTO, error) {
	// 1. 从Redis的排名表(Sorted Set)中获取所有法术ID
	allSpellIDs, err := database.RDB.ZRange(database.Ctx, RankingKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("无法从Redis获取所有法术ID: %w", err)
	}
	if len(allSpellIDs) < 2 {
		return nil, errors.New("数据库中法术数量不足")
	}

	// 2. 创建一个可供抽样的ID列表，并排除指定的法术
	selectableIDs := make([]string, 0, len(allSpellIDs))
	excludeMap := map[string]bool{excludeA: true, excludeB: true}
	for _, id := range allSpellIDs {
		if !excludeMap[id] {
			selectableIDs = append(selectableIDs, id)
		}
	}
	if len(selectableIDs) < 2 {
		return nil, errors.New("排除后剩余法术数量不足")
	}

	// 3. 简单随机抽样：打乱列表并取前两个
	// Go 1.20+版本后，rand.Seed已无需手动调用
	rand.Shuffle(len(selectableIDs), func(i, j int) {
		selectableIDs[i], selectableIDs[j] = selectableIDs[j], selectableIDs[i]
	})
	selectedIDs := selectableIDs[:2]
	idA, idB := selectedIDs[0], selectedIDs[1]

	// 4. 使用Pipeline批量获取这两个法术的静态信息(SpellInfo)
	pipe := database.RDB.Pipeline()
	infoACmd := pipe.HGet(database.Ctx, InfoKey, idA)
	infoBCmd := pipe.HGet(database.Ctx, InfoKey, idB)
	_, err = pipe.Exec(database.Ctx)
	if err != nil {
		return nil, fmt.Errorf("无法从Redis批量获取法术对数据: %w", err)
	}

	// 5. 解析数据
	var infoA, infoB SpellInfo
	_ = json.Unmarshal([]byte(infoACmd.Val()), &infoA)
	_ = json.Unmarshal([]byte(infoBCmd.Val()), &infoB)

	// 6. 生成PairID和签名
	pairID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("无法生成PairID: %w", err)
	}
	payload := token.TokenPayload{
		PairID:   pairID.String(),
		SpellAID: idA,
		SpellBID: idB,
	}
	signature, err := token.GenerateVoteSignature(payload)
	if err != nil {
		return nil, fmt.Errorf("无法生成投票签名: %w", err)
	}

	// 7. 组合最终的响应DTO
	return &PairDataDTO{
		SpellAInfo: infoA,
		SpellBInfo: infoB,
		Payload:    payload,
		Signature:  signature,
	}, nil
}
