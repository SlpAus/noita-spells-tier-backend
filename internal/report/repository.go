package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	// CacheKey 是一个 Redis Hash 的键，用于缓存序列化后的用户报告。
	// Field: 用户的UUID
	// Value: UserReport 结构体的JSON序列化字符串
	CacheKey = "report:cache"
)

// GetReportCache 从Redis缓存中获取用户报告。
func GetReportCache(userID string) (*UserReport, error) {
	result, err := database.RDB.HGet(database.Ctx, CacheKey, userID).Result()
	if err == redis.Nil {
		return nil, nil // 缓存未命中，是正常情况，不返回错误
	}
	if err != nil {
		return nil, err // 其他Redis错误
	}

	var report UserReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// SetReportCache 将用户报告存入Redis缓存。
func SetReportCache(report *UserReport, expire time.Duration) error {
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}

	// 使用Pipeline来原子地设置值和过期时间
	pipe := database.RDB.Pipeline()
	pipe.HSet(database.Ctx, CacheKey, report.UserID, data)
	pipe.HExpire(database.Ctx, CacheKey, expire, report.UserID)
	_, err = pipe.Exec(database.Ctx)
	return err
}

// --- 内存仓库 (用于Redis降级) ---

type inMemoryRepository struct {
	mu             sync.RWMutex
	isLoaded       bool
	snapshotTime   time.Time
	snapshotVoteID uint
	userStats      map[string]user.UserStats
	userRank       map[string]int64
	totalStats     user.UserStats
	totalVoters    int64
	spellRank      map[string]int // 1-based
	spellRankScore map[string]float64
	rankToSpell    []string // 0-based
}

var mirrorRepo = &inMemoryRepository{}

// ensureAndLock 确保内存仓库已填充数据，并返回一个用于defer的解锁函数。
// 这是实现双重检查锁定的核心。
func (r *inMemoryRepository) ensureAndLock() (func(), error) {
	for {
		r.mu.RLock()
		if r.isLoaded {
			return r.mu.RUnlock, nil
		}
		r.mu.RUnlock()

		err := func() error {
			r.mu.Lock()
			defer r.mu.Unlock()
			// 再次检查，因为在获取写锁的过程中，可能已有其他goroutine完成了填充
			if !r.isLoaded {
				if err := r.populate(); err != nil {
					return err
				}
				r.isLoaded = true
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
}

// populate 从SQLite快照中读取数据并填充内存仓库。
// 这个方法必须在持有写锁的情况下被调用。
func (r *inMemoryRepository) populate() error {
	var snapshotTime time.Time
	var snapshotVoteID uint
	var users []user.User
	var totalStatsRecord user.TotalStats
	var spells []struct {
		SpellID   string
		RankScore float64
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		// 1. 获取快照元数据
		snapshotTime, err = metadata.GetLastSnapshotTime(tx)
		if err != nil {
			return fmt.Errorf("无法获取快照时间: %w", err)
		}
		snapshotVoteID, err = metadata.GetLastSnapshotVoteID(tx)
		if err != nil {
			return fmt.Errorf("无法获取快照时最后处理的VoteID: %w", err)
		}

		// 2. 加载用户数据
		if err = tx.Find(&users).Error; err != nil {
			return fmt.Errorf("无法从SQLite加载用户快照数据: %w", err)
		}

		// 3. 加载总统计数据
		// First会返回ErrRecordNotFound，我们忽略这个错误，因为表可能为空
		if err = tx.First(&totalStatsRecord).Error; err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("无法从SQLite加载总统计数据: %w", err)
		}

		// 4. 加载法术数据
		if err = tx.Model(&spell.Spell{}).Order("rank asc").Find(&spells).Error; err != nil {
			return fmt.Errorf("无法从SQLite加载法术快照数据: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	r.snapshotTime = snapshotTime
	r.snapshotVoteID = snapshotVoteID

	// 4. 处理用户数据
	r.userStats = make(map[string]user.UserStats, len(users))
	for _, u := range users {
		r.userStats[u.UUID] = user.UserStats{
			Wins: u.WinsCount,
			Draw: u.DrawCount,
			Skip: u.SkipCount,
		}
	}

	r.totalStats = user.UserStats{
		Wins: totalStatsRecord.WinsCount,
		Draw: totalStatsRecord.DrawCount,
		Skip: totalStatsRecord.SkipCount,
	}

	//  5. 处理法术数据
	r.spellRank = make(map[string]int, len(spells))
	r.spellRankScore = make(map[string]float64, len(spells))
	r.rankToSpell = make([]string, len(spells))
	for i, s := range spells {
		r.spellRank[s.SpellID] = i + 1
		r.spellRankScore[s.SpellID] = s.RankScore
		r.rankToSpell[i] = s.SpellID
	}

	// 6. 计算排名
	type userWithTotal struct {
		uuid  string
		total int
	}
	sortedUsers := make([]userWithTotal, 0, len(users))
	for _, u := range users {
		sortedUsers = append(sortedUsers, userWithTotal{
			uuid:  u.UUID,
			total: u.WinsCount + u.DrawCount + u.SkipCount,
		})
	}
	// 按总票数降序排序
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].total > sortedUsers[j].total
	})

	r.userRank = make(map[string]int64, len(sortedUsers))
	for i, u := range sortedUsers {
		r.userRank[u.uuid] = int64(i)
	}
	r.totalVoters = int64(len(sortedUsers))

	return nil
}
