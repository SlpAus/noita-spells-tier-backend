package user

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
)

// PrimeDB 负责初始化user模块的数据库部分，仅迁移表结构。
// 缓存预热将由更高层的逻辑（例如vote模块）在同步完用户数据后触发。
func PrimeDB() error {
	if err := migrateDB(); err != nil {
		return err
	}
	return nil
}

// migrateDB 负责自动迁移数据库表结构
func migrateDB() error {
	if err := database.DB.AutoMigrate(&User{}); err != nil {
		return fmt.Errorf("无法迁移user表: %w", err)
	}
	fmt.Println("User数据库表迁移成功。")
	return nil
}

// WarmupCache 从SQLite加载所有已知的用户UUID，并预热到Redis的Set中
func WarmupCache() error {
	var users []User
	// 1. 从SQLite读取所有用户的UUID
	if err := database.DB.Select("uuid").Find(&users).Error; err != nil {
		return fmt.Errorf("无法从SQLite读取用户UUID: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("无现有用户数据，无需预热用户缓存。")
		return nil
	}

	// 2. 将UUID转换为interface{}切片以用于SAdd
	userUUIDs := make([]interface{}, len(users))
	for i, u := range users {
		userUUIDs[i] = u.UUID
	}

	// 3. 使用Pipeline批量将所有UUID添加到Redis的Set中
	pipe := database.RDB.Pipeline()
	// 先清空旧的缓存，确保数据一致性
	pipe.Del(database.Ctx, KnownUsersKey)
	pipe.SAdd(database.Ctx, KnownUsersKey, userUUIDs...)

	_, err := pipe.Exec(database.Ctx)
	if err != nil {
		return fmt.Errorf("预热用户UUID到Redis失败: %w", err)
	}

	fmt.Printf("成功预热 %d 个用户UUID到Redis。\n", len(users))
	return nil
}
