package vote

import (
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/go-redis/redis/v8"
)

const eloKFactor = 32

// voteProcessor 是一个单一写入者，负责按顺序处理投票事件并更新Redis
type voteProcessor struct {
	voteChan            chan Model
	lastProcessedVoteID uint
	buffer              *voteMinHeap
	mu                  sync.Mutex
}

// globalVoteProcessor 是一个私有的、全局的VoteProcessor实例
var globalVoteProcessor = voteProcessor{
	voteChan: make(chan Model, 10000),
}

// initializeProcessor 初始化全局的voteProcessor实例
func initializeProcessor(startID uint) {
	globalVoteProcessor.lastProcessedVoteID = startID
	h := &voteMinHeap{}
	heap.Init(h)
	globalVoteProcessor.buffer = h
}

// StartProcessor 启动VoteProcessor的主处理循环和巡查员
func startProcessor() {
	fmt.Println("投票处理器 (Vote Processor) 已启动。")
	go globalVoteProcessor.runPatroller()
	globalVoteProcessor.runMainLoop()
}

// SubmitVoteToQueue 供Handler调用的方法，用于提交新的投票任务，返回是否成功
func submitVoteToQueue(vote Model) {
	select {
	case globalVoteProcessor.voteChan <- vote:
	default:
		fmt.Printf("警告: 投票处理队列已满，暂时放弃实时处理 vote ID: %d\n", vote.ID)
	}
}

// runMainLoop 是处理器的主事件循环
func (vp *voteProcessor) runMainLoop() {
	for {
		nextVote := vp.getNextContinuousVote()

		// 检查Redis健康状态
		if !database.IsRedisHealthy() {
			fmt.Println("Vote Processor: 检测到Redis不可用或正在重建，暂停处理...")
			time.Sleep(5 * time.Second) // 与健康检查器同步休眠
			// 将取出的任务放回暂存区，以便在Redis恢复后能被重新处理
			vp.mu.Lock()
			heap.Push(vp.buffer, nextVote)
			vp.mu.Unlock()
			continue
		}

		// 处理投票，现在包含了精细化的重试逻辑
		err := vp.applyVoteToRedisWithRetry(nextVote)
		if err != nil {
			// 可能是Redis不健康了
			fmt.Printf("错误: 处理 vote ID %d 失败，已放回队列: %v\n", nextVote.ID, err)
			continue
		}

		// 只有在成功处理后才更新ID
		vp.mu.Lock()
		vp.lastProcessedVoteID = nextVote.ID
		vp.mu.Unlock()
	}
}

// getNextContinuousVote 是一个阻塞函数，它会一直等待直到获取到下一个连续的投票
func (vp *voteProcessor) getNextContinuousVote() Model {
	for {
		vp.mu.Lock()
		// 丢弃所有过时的堆顶元素
		for vp.buffer.Len() > 0 && (*vp.buffer)[0].ID <= vp.lastProcessedVoteID {
			heap.Pop(vp.buffer)
		}

		// 检查暂存区是否有我们需要的下一个投票
		if vp.buffer.Len() > 0 && (*vp.buffer)[0].ID == vp.lastProcessedVoteID+1 {
			vote := heap.Pop(vp.buffer).(Model)
			vp.mu.Unlock()
			return vote
		}
		vp.mu.Unlock()

		// 从主channel中等待，或在超时后重新检查暂存区
		select {
		case vote := <-vp.voteChan:
			vp.mu.Lock()
			if vote.ID <= vp.lastProcessedVoteID {
				vp.mu.Unlock()
				continue // 收到的是过时的投票，直接丢弃
			}
			if vote.ID == vp.lastProcessedVoteID+1 {
				vp.mu.Unlock()
				return vote // 正好是下一个，直接处理
			}
			// 收到的投票太新，放入暂存区
			heap.Push(vp.buffer, vote)
			vp.mu.Unlock()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// applyVoteToRedisWithRetry 包含了您设计的、带有指数退避和健康检查的重试逻辑
func (vp *voteProcessor) applyVoteToRedisWithRetry(vote Model) error {
	initialDelay := 8 * time.Millisecond
	maxDelay := 2 * time.Second

	delay := initialDelay
	for delay < maxDelay { // 短循环重试
		err := vp.applyVoteToRedis(vote)
		if err == nil {
			return nil // 成功
		}
		time.Sleep(delay)
		delay *= 2
	}

	// 进入长循环告警模式
	for {
		// 每次重试前都检查健康状态
		if !database.IsRedisHealthy() {
			fmt.Printf("Vote Processor: 在重试期间检测到Redis不可用，将 vote ID %d 放回队列。\n", vote.ID)
			// 将任务放回暂存区，并由外层循环处理休眠
			vp.mu.Lock()
			heap.Push(vp.buffer, vote)
			vp.mu.Unlock()
			return errors.New("redis became unhealthy during retry")
		}

		fmt.Printf("告警: Redis持续写入失败，将在%v后重试 vote ID %d\n", maxDelay, vote.ID)
		time.Sleep(maxDelay)
		err := vp.applyVoteToRedis(vote)
		if err == nil {
			return nil // 最终成功
		}
	}
}

// runPatroller 启动一个后台巡查员，定期检查数据库中是否有被遗漏的投票
func (vp *voteProcessor) runPatroller() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !database.IsRedisHealthy() {
			continue // 如果Redis不健康，则跳过本次巡查
		}

		vp.mu.Lock()
		startID := vp.lastProcessedVoteID
		bufferMinID := uint(0)
		if vp.buffer.Len() > 0 {
			bufferMinID = (*vp.buffer)[0].ID
		}
		vp.mu.Unlock()

		var missedVotes []Model
		query := database.DB.Where("id > ?", startID)
		if bufferMinID > 0 {
			query = query.Where("id < ?", bufferMinID)
		}
		query.Order("id asc").Limit(10000).Find(&missedVotes)

		if len(missedVotes) > 0 {
			fmt.Printf("巡查员: 发现 %d 条被遗漏的投票，正在提交处理...\n", len(missedVotes))
			for _, vote := range missedVotes {
				submitVoteToQueue(vote)
			}
		}
	}
}

// applyVoteToRedis 将单个投票的计算结果原子地更新到Redis
func (vp *voteProcessor) applyVoteToRedis(vote Model) error {
	if vote.Result == ResultSkip {
		// 对于跳过的投票，我们依然需要更新检查点
		return database.RDB.Set(database.Ctx, metadata.LastProcessedVoteIDKey, vote.ID, 0).Err()
	}

	// 1. 获取当前统计数据
	keys := []string{vote.SpellA_ID, vote.SpellB_ID}
	statsJSONs, err := database.RDB.HMGet(database.Ctx, spell.StatsKey, keys...).Result()
	if err != nil {
		return fmt.Errorf("无法从Redis获取法术统计数据: %w", err)
	}
	if statsJSONs[0] == nil || statsJSONs[1] == nil {
		fmt.Printf("警告: 处理 vote ID %d 时发现法术不存在 (%s or %s)，跳过此投票。\n", vote.ID, vote.SpellA_ID, vote.SpellB_ID)
		// 即使法术不存在，我们依然更新检查点，虽然这不太可能
		return database.RDB.Set(database.Ctx, metadata.LastProcessedVoteIDKey, vote.ID, 0).Err()
	}

	var statsA, statsB spell.SpellStats
	_ = json.Unmarshal([]byte(statsJSONs[0].(string)), &statsA)
	_ = json.Unmarshal([]byte(statsJSONs[1].(string)), &statsB)

	// 2. 计算新数据
	switch vote.Result {
	case ResultAWins:
		statsA.Score, statsB.Score = calculateElo(statsA.Score, statsB.Score)
		statsA.Win++
		statsA.Total++
		statsB.Total++
	case ResultBWins:
		statsB.Score, statsA.Score = calculateElo(statsB.Score, statsA.Score)
		statsB.Win++
		statsB.Total++
		statsA.Total++
	case ResultDraw:
		statsA.Total++
		statsB.Total++
	}

	// 3. 使用Pipeline将所有写操作打包成一个原子事务
	pipe := database.RDB.TxPipeline()
	statsAJSON, _ := json.Marshal(statsA)
	statsBJSON, _ := json.Marshal(statsB)
	pipe.HSet(database.Ctx, spell.StatsKey, vote.SpellA_ID, statsAJSON)
	pipe.HSet(database.Ctx, spell.StatsKey, vote.SpellB_ID, statsBJSON)
	pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsA.Score, Member: vote.SpellA_ID})
	pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsB.Score, Member: vote.SpellB_ID})
	// *** 新增：原子地更新检查点 ***
	pipe.Set(database.Ctx, metadata.LastProcessedVoteIDKey, vote.ID, 0)

	_, err = pipe.Exec(database.Ctx)
	return err
}

func calculateElo(winnerScore, loserScore float64) (float64, float64) {
	expectedWinner := 1.0 / (1.0 + math.Pow(10, (loserScore-winnerScore)/400.0))
	newWinnerScore := winnerScore + eloKFactor*(1-expectedWinner)
	newLoserScore := loserScore - eloKFactor*expectedWinner
	return newWinnerScore, newLoserScore
}

// voteMinHeap 实现了 container/heap 接口
type voteMinHeap []Model

func (h voteMinHeap) Len() int            { return len(h) }
func (h voteMinHeap) Less(i, j int) bool  { return h[i].ID < h[j].ID }
func (h voteMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *voteMinHeap) Push(x interface{}) { *h = append(*h, x.(Model)) }
func (h *voteMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
