package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// activationQueue 是一个带缓冲的channel，用作异步处理用户激活请求的任务队列。
var activationQueue = make(chan string, 1000) // 缓冲100个请求

// IsValidUUID 验证一个字符串是否是合法的、非未来的v7 UUID。
func IsValidUUID(uuidStr string) bool {
	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil || parsedUUID.Version() != 7 {
		return false
	}
	// 验证时间戳
	uuidTimestamp := parsedUUID.Time()
	sec, nsec := uuidTimestamp.UnixTime()
	uuidTime := time.Unix(sec, nsec)
	return uuidTime.Before(time.Now())
}

func CreateProvisionalUser() (string, error) {
	newUUID, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("无法生成UUID v7: %w", err)
	}
	return newUUID.String(), nil
}

// BatchCreateUsers 接收一个用户ID列表，并以幂等的方式将它们写入SQLite。
// 这是由vote模块在初始化时调用的。
func BatchCreateUsers(userIDs []string) error {
	if len(userIDs) == 0 {
		return WarmupCache()
	}
	usersToCreate := make([]User, len(userIDs))
	for i, id := range userIDs {
		usersToCreate[i] = User{UUID: id}
	}
	// 使用 GORM 的 OnConflict Do Nothing 来实现幂等插入
	err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&usersToCreate).Error
	if err != nil {
		return fmt.Errorf("批量创建用户失败: %w", err)
	}
	err = WarmupCache()
	if err != nil {
		return err
	}
	fmt.Printf("用户模块：成功同步 %d 个来自vote表的用户ID。\n", len(userIDs))
	return nil
}

// QueueUserActivationIfValid 验证UUID格式，如果合法，则非阻塞地将其提交到后台激活队列。
func QueueUserActivationIfValid(uuidStr string) bool {
	// 在提交前，先进行一次快速的格式校验
	if !IsValidUUID(uuidStr) {
		return false
	}
	// 尝试向channel发送任务，如果队列已满，则非阻塞地放弃，防止阻塞handler
	select {
	case activationQueue <- uuidStr:
		// 成功提交
	default:
		fmt.Printf("警告: 用户激活队列已满，暂时放弃激活请求: %s\n", uuidStr)
	}
	return true
}

// StartActivationWorker 启动一个单一写入者Goroutine来处理用户激活。
// 它现在接收一个lifecycle.Handle来管理其生命周期。
func StartActivationWorker(handle *lifecycle.Handle) {
	defer handle.Close() // 确保在退出时通知管理器
	fmt.Println("用户激活后台工作进程已启动。")

	for {
		select {
		case <-handle.Done():
			fmt.Println("用户激活后台工作进程正在关闭...")
			return
		case userID := <-activationQueue:
			processActivation(handle, userID)
		}
	}
}

// processActivation 是单一写入者的核心逻辑
func processActivation(handle *lifecycle.Handle, userID string) {
	select {
	case <-handle.Done():
		return
	default:
	}

	if database.IsRedisHealthy() {
		// 1. 快速检查Redis缓存，如果用户已存在，则直接结束
		exists, err := database.RDB.SIsMember(database.Ctx, KnownUsersKey, userID).Result()
		if err == nil && exists {
			return // 用户已激活，无需操作
		}
		if err != nil && err != redis.Nil {
			// 即使检查失败，我们依然继续尝试写入，以SQLite为准
		}
	}

	select {
	case <-handle.Done():
		return
	default:
	}

	// 2. 尝试写入SQLite，带有阻塞和指数退避的重试逻辑
	exists, err := writeToSQLiteWithRetry(handle, userID)
	if exists {
		return
	}
	if err != nil {
		if !(err == context.Canceled || err == context.DeadlineExceeded) {
			fmt.Printf("严重错误: SQLite写入用户 %s 失败，放弃激活: %v\n", userID, err)
		}
		return
	}

	select {
	case <-handle.Done():
		return
	default:
	}

	// 3. 尝试写入Redis，带有阻塞和指数退避的重试逻辑
	writeToRedisWithRetry(handle, userID)
}

// --- 重试逻辑辅助函数 ---

func writeToSQLiteWithRetry(handle *lifecycle.Handle, userID string) (bool, error) {
	const maxRetries = 5
	baseDelay := 50 * time.Millisecond
	maxDelay := 1 * time.Minute

	for i := 0; i < maxRetries; i++ {
		err := database.DB.Create(&User{UUID: userID}).Error
		if err == nil {
			return false, nil // 成功写入
		}
		// 如果错误是因为主键冲突，说明另一个进程已经写入成功，这不是一个错误
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return true, nil
		}

		if sleepErr := handle.Sleep(baseDelay); sleepErr != nil {
			return false, sleepErr // 休眠被中断
		}
	}
	fmt.Printf("SQLite写入用户 %s 失败\n", userID)

	for baseDelay < maxDelay {
		err := database.DB.Create(&User{UUID: userID}).Error
		if err == nil {
			return false, nil
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return true, nil
		}

		if sleepErr := handle.Sleep(baseDelay); sleepErr != nil {
			return false, sleepErr
		}
		baseDelay *= 2
	}

	// 进入长循环告警模式
	for {
		err := database.DB.Create(&User{UUID: userID}).Error
		if err == nil {
			return false, nil
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return true, nil
		}
		fmt.Printf("告警: SQLite持续写入失败，将在%v后重试用户 %s\n", userID, maxDelay)
		if sleepErr := handle.Sleep(maxDelay); sleepErr != nil {
			return false, sleepErr
		}
	}
}

func writeToRedisWithRetry(handle *lifecycle.Handle, userID string) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 1 * time.Minute

	for {
		// 每次写入前都检查Redis健康状态
		if !database.IsRedisHealthy() {
			fmt.Printf("Redis当前不可用，用户 %s 的激活将等待健康检查恢复...\n", userID)
			if sleepErr := handle.Sleep(5 * time.Second); sleepErr != nil { // 与健康检查同步睡眠
				return
			}
			baseDelay = 100 * time.Millisecond // 重置退避
			continue
		}

		err := database.RDB.SAdd(database.Ctx, KnownUsersKey, userID).Err()
		if err == nil {
			return // 成功写入
		}

		fmt.Printf("Redis写入用户 %s 失败: %v。将在 %v 后重试。\n", userID, err, baseDelay)
		if sleepErr := handle.Sleep(baseDelay); sleepErr != nil {
			return
		}
		if baseDelay < maxDelay {
			baseDelay *= 2
			if baseDelay >= maxDelay {
				baseDelay = maxDelay // 达到上限后进入长循环
				fmt.Printf("告警: Redis持续写入失败，已进入长循环重试模式 (用户: %s)\n", userID)
			}
		}
	}
}
