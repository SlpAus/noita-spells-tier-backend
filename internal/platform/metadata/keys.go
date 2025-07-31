package metadata

// --- SQLite Keys ---
// These keys are used for the 'key' column in the 'metadata' SQLite table.
const (
	// LastSnapshotVoteIDKey stores the ID of the last vote record that was included
	// in the last successful spell stats snapshot.
	LastSnapshotVoteIDKey = "last_snapshot_vote_id"

	// TotalVotesKey stores the total number of processed votes (excluding skips)
	// as of the last successful snapshot.
	SnapshotTotalVotesKey = "snapshot_total_votes"
)

// --- Redis Keys ---
// These keys are used for storing metadata in Redis.
const (
	// RedisLastProcessedVoteIDKey is a Redis String that stores the ID of the last vote
	// successfully processed by the VoteProcessor. It's the live checkpoint.
	RedisLastProcessedVoteIDKey = "meta:last_processed_vote_id"

	// RedisTotalVotesKey is a Redis String (used as a counter) that stores the live
	// total number of processed votes (excluding skips).
	RedisTotalVotesKey = "meta:total_votes"
)
