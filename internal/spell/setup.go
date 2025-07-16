package spell

import (
	"encoding/json"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/go-redis/redis/v8"
)

func MigrateDB() error {
	err := database.DB.AutoMigrate(&Spell{})
	if err != nil {
		return fmt.Errorf("法术数据库迁移失败: %w", err)
	}

	fmt.Println("法术数据库迁移成功！")
	return nil
}

// WarmupCache 从SQLite加载法术数据并预热到Redis
func WarmupCache() error {

	var spellsInDB []Spell
	if err := database.DB.Find(&spellsInDB).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取法术数据: %w", err)
	}

	pipe := database.RDB.Pipeline()
	pipe.Del(database.Ctx, InfoKey, StatsKey, RankingKey)

	for _, spell := range spellsInDB {
		info := SpellInfo{ Name: spell.Name, Description: spell.Description, Sprite: spell.Sprite, Type: spell.Type, }
		infoJSON, _ := json.Marshal(info)
		pipe.HSet(database.Ctx, InfoKey, spell.SpellID, infoJSON)

		stats := SpellStats{ Score: spell.Score, Total: spell.Total, Win: spell.Win, }
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, StatsKey, spell.SpellID, statsJSON)

		pipe.ZAdd(database.Ctx, RankingKey, &redis.Z{ Score: spell.Score, Member: spell.SpellID, })
	}

	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热法术数据到Redis失败: %w", err)
	}

	fmt.Printf("成功预热 %d 条法术的完整数据到Redis。\n", len(spellsInDB))
	return nil
}

func PrimeCachedDB() error {
	if err := MigrateDB(); err != nil {
		return err
	}

	if err := WarmupCache(); err != nil {
		return err
	}
	
	return nil
}