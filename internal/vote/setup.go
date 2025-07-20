package vote

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
)

// PrimeDB 负责初始化vote模块的数据库部分
func PrimeDB() error {
	if err := database.DB.AutoMigrate(&Model{}); err != nil {
		return fmt.Errorf("无法迁移vote表: %w", err)
	}
	fmt.Println("Vote数据库表迁移成功。")
	return nil
}

// StartVoteProcessor 初始化并启动全局的VoteProcessor
func StartVoteProcessor() error {
	startID, err := metadata.GetLastSnapshotVoteID()
	if err != nil {
		return fmt.Errorf("无法获取启动Vote Processor所需的快照ID: %w", err)
	}

	initializeProcessor(startID)
	go startProcessor() // 在一个新的Goroutine中启动它

	return nil
}
