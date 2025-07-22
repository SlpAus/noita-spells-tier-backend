package vote

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
)

// PrimeDB 负责初始化vote模块的数据库部分，并驱动user模块的初始化
func PrimeDB() error {
	if err := database.DB.AutoMigrate(&Vote{}); err != nil {
		return fmt.Errorf("无法迁移vote表: %w", err)
	}
	fmt.Println("Vote数据库表迁移成功。")

	var userIDs []string
	err := database.DB.Model(&Vote{}).Where("user_identifier != ?", "").Distinct("user_identifier").Pluck("user_identifier", &userIDs).Error
	if err != nil {
		return fmt.Errorf("无法从vote表提取用户ID: %w", err)
	}

	if err := user.BatchCreateUsers(userIDs); err != nil {
		return fmt.Errorf("将用户同步到user模块失败: %w", err)
	}

	return nil
}

// StartVoteProcessor 初始化并启动全局的VoteProcessor
// 它现在接收两个handle来管理其复杂的关闭逻辑
func StartVoteProcessor(gracefulHandle, forcefulHandle *lifecycle.Handle) error {
	startID, err := metadata.GetLastSnapshotVoteID()
	if err != nil {
		return fmt.Errorf("无法获取启动Vote Processor所需的快照ID: %w", err)
	}

	initializeProcessor(startID)
	go startProcessor(gracefulHandle, forcefulHandle) // 在一个新的Goroutine中启动它

	return nil
}
