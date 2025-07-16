package vote

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
)

func MigrateDB() error {
	err := database.DB.AutoMigrate(&Vote{})
	if err != nil {
		return fmt.Errorf("投票数据库迁移失败: %w", err)
	}

	fmt.Println("投票数据库迁移成功！")
	return nil
}

func PrimeDB() error {
	if err := MigrateDB(); err != nil {
		return err
	}

	return nil
}