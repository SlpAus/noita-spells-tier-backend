package models

import "gorm.io/gorm"

// Type 为0时表示正常投票，为1时表示二者都输
type Vote struct {
	gorm.Model
	Winner uint    `json:"winner" gorm:"not null;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Loser  uint    `json:"loser" gorm:"not null;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Weight float64 `json:"weight"`
	IP     string  `json:"ip" gorm:"not null"`
	Type   int     `json:"type"`
}

type Pvote struct {
	gorm.Model
	Winner      uint   `json:"winner"`
	Loser       uint   `json:"loser"`
	Description string `json:"description"`
	IP          string `json:"ip"`
}
