package user

import (
	"time"

	"gorm.io/gorm"
)

// User 定义了用户在SQLite数据库中的持久化模型。
// 它只存储最核心的、作为快照基础的数据。
type User struct {
	// UUID 是用户的主键，来自客户端Cookie。
	UUID string `gorm:"primarykey;type:varchar(36)"`

	// WinsCount 记录了用户做出选择（投票给A或B）的总次数。
	WinsCount int

	// DrawCount 记录了用户选择双输的总次数。
	DrawCount int

	// SkipCount 记录了用户选择跳过的总次数。
	SkipCount int

	// 部分gorm.Model，由GORM自动管理
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TotalStats 定义了在SQLite中持久化的社区总体统计数据。
// 这张表中应该只有一条记录。
type TotalStats struct {
	gorm.Model
	WinsCount int
	DrawCount int
	SkipCount int
}
