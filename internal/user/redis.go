package user

// 定义与用户相关的Redis键名
const (
	// KnownUsersKey 是一个Set，用于快速查找一个UUID是否是已知的、合法的用户。
	// Key: known_users
	// Member: User UUID (e.g., "018f4e2a-....")
	KnownUsersKey = "known_users"
)
