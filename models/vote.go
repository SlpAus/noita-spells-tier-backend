package models

import "gorm.io/gorm"

type Vote struct {
	gorm.Model
	Winner uint    `json:"winner" gorm:"not null;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Loser  uint    `json:"loser" gorm:"not null;index;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Weight float64 `json:"weight"`
	IP     string  `json:"ip" gorm:"not null"`
}
