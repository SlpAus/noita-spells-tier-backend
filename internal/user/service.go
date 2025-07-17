package user

import (
	"errors"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateProvisionalUser 生成一个临时的、尚未持久化的新用户UUID。
// 这个UUID将被设置到cookie中，但此时尚未被“认证”。
func CreateProvisionalUser() (string, error) {
	newUUID, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("无法生成UUID v7: %w", err)
	}
	return newUUID.String(), nil
}

// IsUserActivated 检查一个给定的UUID是否已经被认证（即存在于我们的持久化系统中）。
// 它只查询Redis缓存，以获得最高性能。
func IsUserActivated(uuidStr string) (bool, error) {
	if uuidStr == "" {
		return false, nil
	}
	exists, err := database.RDB.SIsMember(database.Ctx, KnownUsersKey, uuidStr).Result()
	if err != nil {
		return false, fmt.Errorf("检查Redis用户缓存时出错: %w", err)
	}
	return exists, nil
}

// ActivateUser 将一个临时的UUID正式持久化到数据库和缓存中。
// 这个操作是原子性的，如果缓存写入失败，数据库写入将被回滚。
func ActivateUser(uuidStr string) error {
	// 首先检查该用户是否已经被激活，避免重复写入
	activated, err := IsUserActivated(uuidStr)
	if err != nil {
		return err
	}
	if activated {
		return nil // 用户已存在，无需操作
	}

	// 开启一个SQLite事务
	tx := database.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("无法开始数据库事务: %w", tx.Error)
	}
	// 使用defer来确保事务在函数结束时总能被处理（提交或回滚）
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback() // 如果发生panic，回滚事务
		}
	}()

	// 在事务中创建数据库记录
	newUser := User{UUID: uuidStr}
	if err := tx.Create(&newUser).Error; err != nil {
		tx.Rollback()
		// 如果是因为记录已存在而出错，这不是一个真正的错误
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return fmt.Errorf("无法在SQLite中创建新用户: %w", err)
	}

	// 尝试将新UUID添加到Redis缓存中
	if err := database.RDB.SAdd(database.Ctx, KnownUsersKey, uuidStr).Err(); err != nil {
		// 如果Redis写入失败，回滚SQLite的写入，保证数据一致性
		tx.Rollback()
		return fmt.Errorf("无法将新用户 %s 添加到Redis缓存: %w", uuidStr, err)
	}

	// 所有操作都成功，提交事务
	return tx.Commit().Error
}
