package vote

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
)

// --- 数据模型 ---

// UsedPairID 定义了已使用的PairID在数据库中的存储结构
type UsedPairID struct {
	PairID    string `gorm:"primaryKey;type:varchar(36)"`
	CreatedAt time.Time
}

// --- 常量与全局变量 ---

const (
	bloomFilterKey = "pairid_bloom_filter"
	cacheSetKey    = "pairid_cache_set"

	bloomFilterErrorRate = 0.001
	bloomFilterCapacity  = 1000000
)

var (
	replayMutex sync.Mutex
)

// --- 核心功能 ---

// InitializeReplayDefense 擦除所有旧数据，并创建一个全新的、干净的防重放系统。
func InitializeReplayDefense() error {
	fmt.Println("正在初始化防重放攻击系统...")

	// 1. 擦除旧的Redis数据
	pipe := database.RDB.Pipeline()
	pipe.Del(database.Ctx, bloomFilterKey)
	pipe.Del(database.Ctx, cacheSetKey)
	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("擦除旧的Redis防重放数据失败: %w", err)
	}

	// 2. 擦除旧的SQLite数据
	if err := database.DB.AutoMigrate(&UsedPairID{}); err != nil {
		return fmt.Errorf("无法迁移PairID表: %w", err)
	}
	if err := database.DB.Exec("DELETE FROM used_pair_ids").Error; err != nil {
		return fmt.Errorf("擦除旧的SQLite PairID表失败: %w", err)
	}

	// 3. 创建一个新的布隆过滤器
	// BF.RESERVE [error_rate] [capacity]
	err := database.RDB.BFReserve(database.Ctx, bloomFilterKey, bloomFilterErrorRate, bloomFilterCapacity).Err()
	if err != nil {
		return fmt.Errorf("创建布隆过滤器失败: %w", err)
	}

	fmt.Println("防重放攻击系统初始化成功。")
	return nil
}

// CheckAndUsePairID 检查一个PairID是否是首次使用，如果是，则将其原子地记录到三层系统中。
// 返回值: isReplay bool, err error
func CheckAndUsePairID(pairID string) (bool, error) {
	// 1. 检查Redis健康状态
	if !database.IsRedisHealthy() {
		return false, errors.New("服务暂时不可用，无法验证投票")
	}

	// --- 只读检查 ---
	// Tier 1: 布隆过滤器
	existsInBF, err := database.RDB.BFExists(database.Ctx, bloomFilterKey, pairID).Result()
	if err != nil {
		return false, fmt.Errorf("查询布隆过滤器失败: %w", err)
	}

	if existsInBF {
		// Tier 2: Redis Set 缓存
		existsInSet, err := database.RDB.SIsMember(database.Ctx, cacheSetKey, pairID).Result()
		if err != nil {
			return false, fmt.Errorf("查询Redis Set缓存失败: %w", err)
		}

		if existsInSet {
			return true, nil // Set缓存确认是重放
		}
	}

	// --- 写入逻辑 ---
	return insertNewPairID(pairID)
}

// insertNewPairID 尝试将一个新的PairID原子地写入三层系统
func insertNewPairID(pairID string) (bool, error) {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	if !database.IsRedisHealthy() {
		return false, errors.New("服务暂时不可用，无法验证投票")
	}

	// 在持有锁之后，再次检查Set缓存，防止在等待锁的过程中ID已被其他请求插入
	isMember, _ := database.RDB.SIsMember(database.Ctx, cacheSetKey, pairID).Result()
	if isMember {
		return true, nil
	}

	// 1. 开启SQLite事务
	tx := database.DB.Begin()
	if tx.Error != nil {
		return false, fmt.Errorf("无法开始SQLite事务: %w", tx.Error)
	}
	defer tx.Rollback() // 默认回滚，只有在最后才提交

	// 2. 在事务中插入SQLite
	newID := UsedPairID{PairID: pairID}
	if err := tx.Create(&newID).Error; err != nil {
		if database.IsDuplicateKeyError(err) {
			// 这几乎是不可能的，说明Redis中的状态曾丢失
			// 尽管马上要触发重建了，我们可以信任SQLite
			return true, nil
		}
		return false, fmt.Errorf("写入SQLite失败: %w", err)
	}

	// 3. 开启Redis事务
	pipe := database.RDB.TxPipeline()
	pipe.BFAdd(database.Ctx, bloomFilterKey, pairID)
	pipe.SAdd(database.Ctx, cacheSetKey, pairID)
	_, err := pipe.Exec(database.Ctx)

	if err != nil {
		// Redis失败，SQLite事务将自动回滚
		return false, fmt.Errorf("写入Redis失败: %w", err)
	}

	const maxRetry = 3
	const delay = 50 * time.Millisecond
	// 4. Redis成功，尝试提交SQLite事务
	for i := 0; i < maxRetry; i++ { // 短间隔重试
		err := tx.Commit().Error
		if err == nil {
			return false, nil // 完美成功
		} else if !database.IsRetryableError(err) {
			break
		}
		time.Sleep(delay)
	}

	// 这是一个严重问题，SQLite提交失败但Redis已写入
	fmt.Printf("严重告警: SQLite提交失败但Redis已写入, PairID: %s\n", pairID)
	// 尽管这里出现内部不一致，应当以不存在的结果静默返回成功
	// 如果后续Redis不崩溃，则此PairID已不可再次使用，如果后续Redis崩溃，则无法阻止此PairID被重复使用
	return false, nil
}

// RecoverReplayDefense 从SQLite重建布隆过滤器和缓存
func RecoverReplayDefense() error {
	fmt.Println("正在从SQLite重建防重放攻击缓存...")

	replayMutex.Lock()
	defer replayMutex.Unlock()

	// 1. 擦除旧的Redis数据
	pipe := database.RDB.Pipeline()
	pipe.Del(database.Ctx, bloomFilterKey)
	pipe.Del(database.Ctx, cacheSetKey)
	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("擦除旧的Redis防重放数据失败: %w", err)
	}

	// 2. 重新创建布隆过滤器
	err := database.RDB.BFReserve(database.Ctx, bloomFilterKey, bloomFilterErrorRate, bloomFilterCapacity).Err()
	if err != nil {
		return fmt.Errorf("创建布隆过滤器失败: %w", err)
	}

	// 3. 从SQLite分批读取所有已存在的ID并处理
	const batchSize = 10000

	pairCount := 0
	var lastProcessedID string // 在字符串UUID上分页，按字母顺序
	var batch []string

	for i := 1; ; i++ {
		if err := database.DB.Model(&UsedPairID{}).Where("pair_id > ?", lastProcessedID).Order("pair_id asc").Limit(batchSize).Pluck("pair_id", &batch).Error; err != nil {
			return fmt.Errorf("分批从SQLite读取PairID失败 (batch %d): %w", i, err)
		}

		// 将string切片转换为interface{}切片
		interfaceBatch := make([]interface{}, len(batch))
		for j, id := range batch {
			interfaceBatch[j] = id
		}

		// 4. 将这一批次的ID写回Redis
		pipe := database.RDB.Pipeline()
		pipe.SAdd(database.Ctx, cacheSetKey, interfaceBatch...)
		pipe.BFMAdd(database.Ctx, bloomFilterKey, interfaceBatch...)
		if _, err := pipe.Exec(database.Ctx); err != nil {
			return fmt.Errorf("批量写回Redis失败 (batch %d): %w", i, err)
		}

		pairCount += len(batch)
		if len(batch) < batchSize {
			break
		}

		lastProcessedID = batch[len(batch)-1]
		batch = batch[:0]
	}

	fmt.Printf("防重放攻击：成功从SQLite恢复了 %d 个PairID到缓存。\n", pairCount)
	return nil
}
