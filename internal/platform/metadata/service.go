package metadata

import (
	"strconv"
	"sync"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"gorm.io/gorm"
)

const (
	// lastSnapshotVoteIDKey 是存储在数据库中的键名
	lastSnapshotVoteIDKey = "last_snapshot_vote_id"
)

var (
	// 使用一个互斥锁来保护对metadata表的并发写操作
	mu sync.Mutex
)

// GetLastSnapshotVoteID 从数据库中获取最后一次快照时对应的vote ID。
// 如果从未设置过，则返回0。
func GetLastSnapshotVoteID() (uint, error) {
	mu.Lock()
	defer mu.Unlock()

	var meta Model
	err := database.DB.Where("key = ?", lastSnapshotVoteIDKey).First(&meta).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果记录不存在，这是正常情况（首次运行），返回0
			return 0, nil
		}
		// 其他数据库错误
		return 0, err
	}

	// 将字符串值转换为uint
	id, err := strconv.ParseUint(meta.Value, 10, 32)
	if err != nil {
		// 如果值格式不正确，返回0并记录错误
		return 0, err
	}

	return uint(id), nil
}

// SetLastSnapshotVoteID 将最后一次快照的vote ID写入数据库。
// 这个操作是“upsert”（更新或插入）。
func SetLastSnapshotVoteID(voteID uint) error {
	mu.Lock()
	defer mu.Unlock()

	valueStr := strconv.FormatUint(uint64(voteID), 10)
	meta := Model{
		Key:   lastSnapshotVoteIDKey,
		Value: valueStr,
	}

	// GORM的 .Save() 方法会智能地执行更新（如果主键或唯一索引存在）或插入
	// 为了确保是upsert，我们使用更明确的 .Clauses(clause.OnConflict)
	// 但为了简单和兼容性，我们先查后写
	var existing Model
	err := database.DB.Where("key = ?", lastSnapshotVoteIDKey).First(&existing).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 插入新记录
			return database.DB.Create(&meta).Error
		}
		return err
	}

	// 更新现有记录
	return database.DB.Model(&existing).Update("value", valueStr).Error
}
