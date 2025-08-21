package database

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(cfg config.SqliteConfig) {
	var err error

	// 根据配置动态构建包含性能优化的DSN字符串
	// cache_size单位是KiB，负值表示使用KiB。
	dsn := fmt.Sprintf("file:%s?journal_mode=WAL&cache=shared&cache_size=-%d", cfg.FileName, cfg.MaxCacheSizeKB)

	// GORM日志配置
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: 0,
			LogLevel:      logger.Silent,
			Colorful:      true,
		},
	)

	// 连接到SQLite数据库
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:      newLogger,
		PrepareStmt: true,
	})

	if err != nil {
		fmt.Println("连接数据库失败", err)
		panic(err)
	}

	fmt.Println("数据库连接成功！")
}

// --- 新增的辅助函数 ---

// IsDuplicateKeyError 检查一个错误是否是由于主键或唯一约束冲突引起的。
// 这是对gorm.ErrDuplicatedKey的一个简单、可读的包装。
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// GORM v2 推荐使用 errors.Is 来检查它定义的标准错误类型。
	// gorm.ErrDuplicatedKey 已经封装了所有数据库驱动中
	// 关于“唯一性冲突”的错误，我们无需再检查底层驱动的特定错误码。
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

// IsRetryableError 检查一个错误是否是由于数据库暂时锁定或繁忙等可恢复的原因引起的。
// 这类错误通常可以通过短暂等待后重试来解决。
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		// SQLITE_BUSY 和 SQLITE_LOCKED 是典型的“请稍后重试”的错误码
		return sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked
	}
	return false
}
