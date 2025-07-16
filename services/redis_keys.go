package services

// 定义所有在Redis中使用的键名常量
const (
	// SpellInfoKey 是一个Hash，存储所有法术的静态数据 (name, description, sprite, type)
	SpellInfoKey = "spell_info"

	// SpellStatsKey 是一个Hash，存储所有法술的动态数据 (score, total, win)
	SpellStatsKey = "spell_stats"

	// SpellRankingKey 是一个Sorted Set，用于按分数实时排序法术
	SpellRankingKey = "spell_ranking"

	// IPVoteKeyPrefix 是有序集合的前缀，用于IP频率限制
	IPVoteKeyPrefix = "ip_votes:"
)
