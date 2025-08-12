package user

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IsValidUUID 验证一个字符串是否是合法的、非未来的v7 UUID。
func IsValidUUID(uuidStr string) bool {
	parsedUUID, err := uuid.Parse(uuidStr)
	if err != nil || parsedUUID.Version() != 7 {
		return false
	}
	// 验证时间戳
	uuidTimestamp := parsedUUID.Time()
	sec, nsec := uuidTimestamp.UnixTime()
	uuidTime := time.Unix(sec, nsec)
	return uuidTime.Before(time.Now())
}

func CreateProvisionalUser() (string, error) {
	newUUID, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("无法生成UUID v7: %w", err)
	}
	return newUUID.String(), nil
}
