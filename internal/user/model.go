package user

import (
	"time"

	"gorm.io/gorm"
)

// User 定义了用户的数据库结构
// 我们不使用gorm.Model，因为我们需要一个自定义的UUID作为主键
type User struct {
	// UUID 是用户的主键，使用v7版本的UUID
	UUID string `gorm:"primaryKey;type:varchar(36)"`

	// CreatedAt 和 UpdatedAt 由GORM自动管理
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
