package metadata

import "gorm.io/gorm"

// Metadata 定义了存储系统元数据的键值对表结构
type Metadata struct {
	// gorm.Model 包含 ID, CreatedAt, UpdatedAt, DeletedAt
	gorm.Model

	// Key 是元数据的唯一键，例如 "last_snapshot_vote_id"
	Key string `gorm:"uniqueIndex;not null;type:varchar(255)"`

	// Value 存储元数据的值
	Value string `gorm:"type:varchar(255)"`
}
