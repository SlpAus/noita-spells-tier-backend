package spell

// 定义与法术相关的Redis键名
const (
	InfoKey     = "spell_info"
	StatsKey    = "spell_stats"
	RankingKey  = "spell_ranking"
)

// SpellInfo 定义了在Redis spell_info Hash中存储的法术静态数据
type SpellInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sprite      string `json:"sprite"`
	Type        int    `json:"type"`
}

// SpellStats 定义了在Redis spell_stats Hash中存储的法术动态数据
type SpellStats struct {
	Score float64 `json:"score"`
	Total int     `json:"total"`
	Win   int     `json:"win"`
}
