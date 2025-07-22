package spell

import "gorm.io/gorm"

// Spell 定义了数据库中法术的数据结构
type Spell struct {
	// gorm.Model 包含 ID, CreatedAt, UpdatedAt, DeletedAt
	gorm.Model

	// SpellID 是法术在游戏中的唯一字符串ID, 例如 "BOMB"
	// 我们将使用它作为业务逻辑中的主键
	SpellID string `gorm:"uniqueIndex;not null" json:"id"`

	// Name 是法术的中文名称, 例如 "炸弹"
	Name string `json:"name"`

	// Description 是法术的中文描述
	Description string `json:"description"`

	// Sprite 是指向法术图标的完整相对路径, 例如 "assets/data/ui_gfx/gun_actions/bomb.png"
	Sprite string `json:"sprite"`

	// Type 是法术的类型 (来自原始数据)
	Type int `json:"type"`

	// --- 以下是用于排名的字段 ---

	// Score 是法术的ELO分数，默认为1500
	Score float64 `json:"score"`

	// Total 是法术参与的总场次
	Total int `json:"total"`

	// Win 是法术获胜的场次
	Win int `json:"win"`

	// Rank 是法术的排名
	Rank int `gorm:"index"`
}
