package metadata

import (
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Generic Accessors ---

// GetValue retrieves a value for a given key from the metadata table.
func GetValue(db *gorm.DB, key string) (string, error) {
	var meta Metadata
	err := db.Where("key = ?", key).First(&meta).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If the key doesn't exist, return an empty string, which is a valid default.
			return "", nil
		}
		return "", err
	}
	return meta.Value, nil
}

// SetValue creates or updates a value for a given key within a transaction.
func SetValue(db *gorm.DB, key, value string) error {
	// Use GORM's OnConflict clause for an efficient and atomic "upsert" operation.
	// It will update the 'value' column if a record with the same 'key' already exists.
	meta := Metadata{
		Key:   key,
		Value: value,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&meta).Error
}

// --- Specific Helpers for Type Conversion ---

// GetLastSnapshotVoteID is a helper that retrieves and parses the last snapshot vote ID.
func GetLastSnapshotVoteID(db *gorm.DB) (uint, error) {
	valueStr, err := GetValue(db, LastSnapshotVoteIDKey)
	if err != nil {
		return 0, err
	}
	if valueStr == "" {
		return 0, nil
	}
	id, err := strconv.ParseUint(valueStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("无法解析元数据 '%s' 的值: %w", LastSnapshotVoteIDKey, err)
	}
	return uint(id), nil
}

// SetLastSnapshotVoteID is a helper that formats and sets the last snapshot vote ID.
func SetLastSnapshotVoteID(db *gorm.DB, voteID uint) error {
	valueStr := strconv.FormatUint(uint64(voteID), 10)
	return SetValue(db, LastSnapshotVoteIDKey, valueStr)
}

// GetSnapshotTotalVotes is a helper that retrieves and parses the total votes count.
func GetSnapshotTotalVotes(db *gorm.DB) (float64, error) {
	valueStr, err := GetValue(db, SnapshotTotalVotesKey)
	if err != nil {
		return 0.0, err
	}
	if valueStr == "" {
		return 0.0, nil
	}
	count, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("无法解析元数据 '%s' 的值: %w", SnapshotTotalVotesKey, err)
	}
	return count, nil
}

// SetSnapshotTotalVotes is a helper that formats and sets the total votes count.
func SetSnapshotTotalVotes(db *gorm.DB, count float64) error {
	valueStr := strconv.FormatFloat(count, 'f', -1, 64)
	return SetValue(db, SnapshotTotalVotesKey, valueStr)
}
