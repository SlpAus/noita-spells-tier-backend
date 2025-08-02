package spell

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/redis/go-redis/v9"
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

// PairSpellDTO 包含了组成一个法术对的单个法术的完整信息，包括其即时排名
type PairSpellDTO struct {
	Info        SpellInfo
	CurrentRank int64
}

// PairDataDTO 是 GetNewSpellPair 服务返回给控制器的最终数据包
type PairDataDTO struct {
	SpellA    PairSpellDTO
	SpellB    PairSpellDTO
	Payload   token.TokenPayload
	Signature string
}

// --- 服务降级辅助函数---
func getRankedSpellsFromDB() ([]RankedSpellDTO, error) {
	var spellsFromDB []Spell
	// 从SQLite快照中读取，并按已存入的Rank字段排序
	if err := database.DB.Order("rank asc").Find(&spellsFromDB).Error; err != nil {
		return nil, err
	}
	var dtos []RankedSpellDTO
	for _, s := range spellsFromDB {
		dtos = append(dtos, RankedSpellDTO{
			ID:    s.SpellID,
			Info:  SpellInfo{Name: s.Name, Description: s.Description, Sprite: s.Sprite, Type: s.Type},
			Stats: SpellStats{Score: s.Score, Total: s.Total, Win: s.Win, RankScore: s.RankScore},
		})
	}
	return dtos, nil
}

// --- Service Functions ---

// GetRankedSpells 从Redis中获取完整的、已排序的法术列表
func GetRankedSpells() ([]RankedSpellDTO, error) {
	if !database.IsRedisHealthy() {
		return getRankedSpellsFromDB()
	}

	// 1. 从Sorted Set获取所有法术ID，按分数从高到低排序
	spellIDs, err := database.RDB.ZRevRange(database.Ctx, RankingKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("无法从Redis获取排行榜ID: %w", err)
	}
	if len(spellIDs) == 0 {
		return []RankedSpellDTO{}, nil
	}

	// 2. 从Redis批量获取动态统计数据
	statsJSONs, err := database.RDB.HMGet(database.Ctx, StatsKey, spellIDs...).Result()
	if err != nil {
		return nil, fmt.Errorf("无法从Redis批量获取法术数据: %w", err)
	}

	// 3. 组合来自内存仓库的静态数据和来自Redis的动态数据
	var rankedSpells []RankedSpellDTO
	for i, id := range spellIDs {
		index, ok := GetSpellIndexByID(id)
		if !ok {
			continue // 如果内存仓库中没有这个ID，跳过
		}
		info, _ := GetSpellInfoByIndex(index)

		var stats SpellStats
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

// GetSpellImageInfoByID 从内存仓库中获取单个法术的图片所需信息
func GetSpellImageInfoByID(spellID string) (*SpellImageDTO, error) {
	// 静态信息现在直接从内存读取，不再需要服务降级
	index, ok := GetSpellIndexByID(spellID)
	if !ok {
		return nil, nil // 使用nil来表示未找到
	}
	info, _ := GetSpellInfoByIndex(index)

	return &SpellImageDTO{
		ID:   spellID,
		Info: info,
	}, nil
}

// GetNewSpellPair 实现了包含“冷门优先”和“实力接近”的智能匹配算法
func GetNewSpellPair(excludeA, excludeB string) (*PairDataDTO, error) {
	if !database.IsRedisHealthy() {
		return nil, errors.New("服务暂时不可用，请稍后重试")
	}

	handleExcludes := (excludeA != "" && excludeB != "")

	var excludeIndexA, excludeIndexB int
	if handleExcludes {
		var ok bool
		excludeIndexA, ok = GetSpellIndexByID(excludeA)
		if !ok {
			return nil, fmt.Errorf("排除法术A不存在: %v", excludeA)
		}
		excludeIndexB, ok = GetSpellIndexByID(excludeB)
		if !ok {
			return nil, fmt.Errorf("排除法术B不存在: %v", excludeB)
		}
	}

	var candidateID1, candidateID2 string
	var candidateRank1, candidateRank2 int64

	err := func() error {
		// --- 阶段一: 选择第一候选法术 (冷门优先) ---
		RLockRepository()
		defer RUnlockRepository()

		totalWeight := GetTotalWeightUnsafe()

		var weightA, weightB float64
		if handleExcludes {
			weightA, _ = GetWeightUnsafe(excludeIndexA)
			totalWeight -= weightA
			weightB, _ = GetWeightUnsafe(excludeIndexB)
			totalWeight -= weightB
		}

		if totalWeight <= 0 {
			return errors.New("排除后无可用法术")
		}

		randomWeight := rand.Float64() * totalWeight
		if handleExcludes {
			if excludeIndexA > excludeIndexB {
				excludeIndexA, excludeIndexB = excludeIndexB, excludeIndexA
				weightA, weightB = weightB, weightA
			}
			prefixA, _ := GetWeightPrefixUnsafe(excludeIndexA - 1)
			if randomWeight >= prefixA {
				randomWeight += weightA
			}
			prefixB, _ := GetWeightPrefixUnsafe(excludeIndexB - 1)
			if randomWeight >= prefixB {
				randomWeight += weightB
			}
		}

		candidateIndex1, err := FindByWeightUnsafe(randomWeight)
		if err != nil {
			return fmt.Errorf("查找第一候选法术失败: %w", err)
		}
		// 处理浮点误差
		spellCount := GetSpellCount()
		if handleExcludes {
			for safe := 0; ; safe++ {
				if candidateIndex1 != excludeIndexA && candidateIndex1 != excludeIndexB {
					break
				}
				if safe >= 2 {
					return errors.New("排除后无法选出第一候选法术")
				}
				candidateIndex1 = (candidateIndex1 + 1) % spellCount
			}
		}

		candidateID1, _ = GetSpellIDByIndex(candidateIndex1)

		// --- 阶段二: 选择第二候选法术 (实力接近) ---
		// 1. 获取所需数据
		pipe := database.RDB.Pipeline()
		rank1Cmd := pipe.ZRevRank(database.Ctx, RankingKey, candidateID1)
		var rankACmd, rankBCmd *redis.IntCmd
		if handleExcludes {
			rankACmd = pipe.ZRevRank(database.Ctx, RankingKey, excludeA)
			rankBCmd = pipe.ZRevRank(database.Ctx, RankingKey, excludeB)
		}
		totalVotesCmd := pipe.Get(database.Ctx, metadata.RedisTotalVotesKey)
		_, err = pipe.Exec(database.Ctx)
		if err != nil {
			return errors.New("查询法术排名失败")
		}

		candidateRank1, err = rank1Cmd.Result()
		if err != nil {
			return fmt.Errorf("查找排除法术排名失败: %w", err)
		}
		var rankA, rankB int64
		if handleExcludes {
			rankA, err = rankACmd.Result()
			if err != nil {
				return fmt.Errorf("查找排除法术排名失败: %w", err)
			}
			rankB, err = rankBCmd.Result()
			if err != nil {
				return fmt.Errorf("查找排除法术排名失败: %w", err)
			}
		}
		totalVotes, err := totalVotesCmd.Float64()
		if err != nil {
			return errors.New("获取总投票数失败")
		}

		// 2. 计算混合比例和总权重
		mixtureFactor := globalGaussianMatcher.GetMixtureFactor(totalVotes)
		minMixedWeight := globalGaussianMatcher.GetMixedPrefixSum(0-int(candidateRank1), mixtureFactor)
		maxMixedWeight := globalGaussianMatcher.GetMixedPrefixSum((spellCount-1)-int(candidateRank1), mixtureFactor)
		totalMixedWeight := maxMixedWeight - minMixedWeight

		// 3. 处理排除
		var excludedRanks []int
		if handleExcludes {
			excludedRanks = append(excludedRanks, int(rankA))
			excludedRanks = append(excludedRanks, int(rankB))

			for _, r := range excludedRanks {
				totalMixedWeight -= globalGaussianMatcher.GetMixedWeight(r-int(candidateRank1), mixtureFactor)
			}
		}

		// 4. 加权随机抽样
		randWeight2 := rand.Float64()*totalMixedWeight + minMixedWeight

		// 5. 调整随机数以跳过排除区间
		if handleExcludes {
			sort.Ints(excludedRanks)
			for _, r := range excludedRanks {
				rankDiff := r - int(candidateRank1)
				var preRankDiff int
				if rankDiff == 1 {
					preRankDiff = -1
				} else {
					preRankDiff = rankDiff - 1
				}
				prefix := globalGaussianMatcher.GetMixedPrefixSum(preRankDiff, mixtureFactor)
				if randWeight2 >= prefix {
					randWeight2 += globalGaussianMatcher.GetMixedWeight(rankDiff, mixtureFactor)
				}
			}
		}

		// 6. 二分查找排名差距
		targetRankDiff := globalGaussianMatcher.FindRankOffsetByMixedPrefixSum(randWeight2, mixtureFactor)
		candidateRank2 = candidateRank1 + int64(targetRankDiff)
		candidateRank2 = max(0, min(int64(spellCount-1), candidateRank2))

		// 处理浮点误差
		excludedRanks = append(excludedRanks, int(candidateRank1))
		for safe := 0; ; safe++ {
			isValid := true
			for _, r := range excludedRanks {
				if int(candidateRank2) == r {
					isValid = false
					break
				}
			}
			if isValid {
				break
			}
			if !handleExcludes && safe >= 1 || handleExcludes && safe >= 3 {
				return errors.New("无法选出第二候选法术")
			}
			candidateRank2 = (candidateRank2 + 1) % int64(spellCount)
		}

		// 7. 从排名获取ID
		candidateIDs2, err := database.RDB.ZRevRange(database.Ctx, RankingKey, candidateRank2, candidateRank2).Result()
		if err != nil {
			return fmt.Errorf("无法从排名获取第二候选法术: %w", err)
		}
		candidateID2 = candidateIDs2[0]

		return nil
	}()

	if err != nil {
		return nil, err
	}

	// --- 后续流程 ---
	if rand.Float32() > 0.5 {
		candidateID1, candidateID2 = candidateID2, candidateID1
		candidateRank1, candidateRank2 = candidateRank2, candidateRank1
	}

	indexA, _ := GetSpellIndexByID(candidateID1)
	infoA, _ := GetSpellInfoByIndex(indexA)

	indexB, _ := GetSpellIndexByID(candidateID2)
	infoB, _ := GetSpellInfoByIndex(indexB)

	spellA := PairSpellDTO{Info: infoA, CurrentRank: candidateRank1 + 1}
	spellB := PairSpellDTO{Info: infoB, CurrentRank: candidateRank2 + 1}

	pairID, _ := uuid.NewV7()
	payload := token.TokenPayload{PairID: pairID.String(), SpellAID: candidateID1, SpellBID: candidateID2}
	signature, _ := token.GenerateVoteSignature(payload)
	return &PairDataDTO{SpellA: spellA, SpellB: spellB, Payload: payload, Signature: signature}, nil
}
