package models

import "gorm.io/gorm"

// Metadata 定义了存储系统元数据的键值对表结构
// 我们使用一个通用的键值结构，以便未来可以灵活地添加新的元数据
type Metadata struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex;not null"` // 元数据的键，例如 "last_sync_timestamp"
	Value string // 元数据的值
}
