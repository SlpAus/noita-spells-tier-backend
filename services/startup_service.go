package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/models"
	"github.com/go-redis/redis/v8"
)

// SpellInfo 定义了在Redis spell_info Hash中存储的法术静态数据
type SpellInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sprite      string `json:"sprite"`
	Type        int    `json:"type"`
}

// SpellStats 定义了在Redis spell_stats Hash中存储的法术动态数据
type SpellStats struct {
	Score float64 `json:"score"`
	Total int     `json:"total"`
	Win   int     `json:"win"`
}

// warmUpSpellCache 从SQLite加载法术数据并预热到Redis
func warmUpSpellCache() error {
	var spells []models.Spell
	if err := database.DB.Find(&spells).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取法术数据: %w", err)
	}

	pipe := database.RDB.Pipeline()
	pipe.Del(database.Ctx, SpellInfoKey, SpellStatsKey, SpellRankingKey)

	for _, spell := range spells {
		// 1. 准备静态数据 (spell_info Hash)
		info := SpellInfo{
			Name:        spell.Name,
			Description: spell.Description,
			Sprite:      spell.Sprite,
			Type:        spell.Type,
		}
		infoJSON, _ := json.Marshal(info)
		pipe.HSet(database.Ctx, SpellInfoKey, spell.SpellID, infoJSON)

		// 2. 准备动态数据 (spell_stats Hash)
		stats := SpellStats{
			Score: spell.Score,
			Total: spell.Total,
			Win:   spell.Win,
		}
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, SpellStatsKey, spell.SpellID, statsJSON)

		// 3. 准备排名数据 (spell_ranking Sorted Set)
		pipe.ZAdd(database.Ctx, SpellRankingKey, &redis.Z{
			Score:  spell.Score,
			Member: spell.SpellID,
		})
	}

	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热法术数据到Redis失败: %w", err)
	}

	fmt.Printf("成功预热 %d 条法术的完整数据到Redis。\n", len(spells))
	return nil
}

// warmUpIPVotes ... (此函数不变)
func warmUpIPVotes() error {
	var votes []models.Vote
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	if err := database.DB.Where("created_at > ?", twentyFourHoursAgo).Find(&votes).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取投票记录: %w", err)
	}
	if len(votes) == 0 {
		fmt.Println("最近24小时内无投票记录，无需预热IP数据。")
		return nil
	}
	pipe := database.RDB.Pipeline()
	ipVoteMap := make(map[string][]*redis.Z)
	for _, vote := range votes {
		if vote.UserIdentifier != "" {
			key := IPVoteKeyPrefix + vote.UserIdentifier
			timestamp := float64(vote.CreatedAt.UnixNano())
			ipVoteMap[key] = append(ipVoteMap[key], &redis.Z{Score: timestamp, Member: timestamp})
		}
	}
	for key, members := range ipVoteMap {
		pipe.ZAdd(database.Ctx, key, members...)
		pipe.Expire(database.Ctx, key, 24*time.Hour)
	}
	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热IP投票数据到Redis失败: %w", err)
	}
	fmt.Printf("成功预热 %d 个IP的投票数据到Redis。\n", len(ipVoteMap))
	return nil
}

// InitializeApplication ... (此函数不变)
func InitializeApplication(flushCache bool) {
	fmt.Println("开始应用初始化...")
	if flushCache {
		if err := database.RDB.FlushDB(database.Ctx).Err(); err != nil {
			panic("清空Redis缓存失败: " + err.Error())
		}
		fmt.Println("Redis缓存已清空。")
	}
	if err := warmUpSpellCache(); err != nil {
		panic(err)
	}
	if err := warmUpIPVotes(); err != nil {
		fmt.Printf("警告: 预热IP投票数据时发生错误: %v\n", err)
	}
	fmt.Println("应用初始化完成！")
}
