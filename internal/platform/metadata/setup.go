package metadata

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
)

// PrimeDB 负责初始化metadata模块的数据库部分
func PrimeDB() error {
	if err := database.DB.AutoMigrate(&Metadata{}); err != nil {
		return fmt.Errorf("无法迁移metadata表: %w", err)
	}
	fmt.Println("Metadata数据库表迁移成功。")
	return nil
}
