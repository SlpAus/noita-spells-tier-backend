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
}

type ItemResponse struct {
	ItemID  uint   `json:"id"`
	Name    string `json:"name"`
	Url     string `json:"url"`
	Quality uint   `json:"quality"`
}
