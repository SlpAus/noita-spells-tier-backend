package spell

import "gorm.io/gorm"

// Spell 定义了数据库中法术的数据结构
type Spell struct {
	// gorm.Model 包含 ID, CreatedAt, UpdatedAt, DeletedAt
	gorm.Model

	// SpellID 是法术在游戏中的唯一字符串ID, 例如 "BOMB"
	SpellID string `gorm:"uniqueIndex;not null"`

	// Name 是法术的中文名称, 例如 "炸弹"
	Name string

	// Description 是法术的中文描述
	Description string

	// Sprite 是指向法术图标的文件名, 例如 "bomb.png"
	Sprite string

	// Type 是法术的类型 (来自原始数据)
	Type int

	// --- 动态数据 ---

	// Score 是法术的原始ELO分数
	Score float64

	// Total 是法术参与的总场次
	Total float64

	// Win 是法术获胜的场次
	Win float64

	// Rank 是法术基于原始ELO分数的排名
	Rank int `gorm:"index"`

	// RankScore 是最终用于排名的、混合了ELO和胜率的动态分数
	RankScore float64
}
