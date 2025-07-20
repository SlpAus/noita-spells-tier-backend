package metadata

const (
	// LastProcessedVoteIDKey 是一个简单的Redis键 (String)，
	// 它存储了VoteProcessor已成功处理并更新到Redis的最后一个vote记录的ID。
	// 这是由vote模块写入，由spell模块读取的“检查点”。
	LastProcessedVoteIDKey = "meta:last_processed_vote_id"
)
