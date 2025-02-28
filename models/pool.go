package models

import "gorm.io/gorm"

// 道具池
type Pool struct {
	gorm.Model
	Name  string `json:"name"`
	Items []Item `gorm:"many2many:item_pools"`
}
