package models

import "gorm.io/gorm"

type Item struct {
	gorm.Model         // 数据库
	ItemID     uint    `json:"id" gorm:"primaryKey"` // 道具
	Name       string  `json:"name"`                 // 名称
	Url        string  `json:"url"`                  // 道具图片URL
	Quality    uint    `json:"quality"`              // 道具品质
	Score      float64 `json:"score"`                // 道具分数(获胜次数)
	Total      float64 `json:"total"`                // 道具投票总数
	WinRate    float64 `json:"winrate"`              // 道具获胜率
	Lost       bool    `json:"lost"`                 // 能否被角色Lost获取
	Descrption string  `json:"description"`          // 道具描述
	WinCount   float64 `json:"wincount"`             // 获胜次数
	Pools      []Pool  `gorm:"many2many:item_pools"`
}

type ItemResponse struct {
	ItemID     uint   `json:"id"`
	Name       string `json:"name"`
	Url        string `json:"url"`
	Quality    uint   `json:"quality"`
	Descrption string `json:"description"`
	FilterNum  uint   `json:"filternum"` // 经过过滤后剩下的道具数量
}

type LastGetItem struct {
	gorm.Model
	UserID    string `json:"userid" gorm:"not null;index"` // 用户的cookie,用来指示用户
	Left      uint   `json:"left"`
	Right     uint   `json:"right"`
	FilterNum uint   `json:"filternum"`
}
