package vote

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/metadata"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"github.com/go-redis/redis/v8"
)

// voteMinHeap 实现了 container/heap 接口
type voteMinHeap []Vote

func (h voteMinHeap) Len() int            { return len(h) }
func (h voteMinHeap) Less(i, j int) bool  { return h[i].ID < h[j].ID }
func (h voteMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *voteMinHeap) Push(x interface{}) { *h = append(*h, x.(Vote)) }
func (h *voteMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// voteProcessor 是一个单一写入者，负责按顺序处理投票事件并更新Redis
type voteProcessor struct {
	voteChan            chan Vote
	lastProcessedVoteID uint
	buffer              *voteMinHeap
	processMutex        sync.Mutex
	isShutdown          bool
	shutdownMutex       sync.Mutex
}

// globalVoteProcessor 是一个私有的、全局的VoteProcessor实例
var globalVoteProcessor = voteProcessor{
	voteChan: make(chan Vote, 10000),
}

// initializeProcessor 初始化全局的voteProcessor实例
func initializeProcessor(startID uint) {
	globalVoteProcessor.lastProcessedVoteID = startID
	h := &voteMinHeap{}
	heap.Init(h)
	globalVoteProcessor.buffer = h
}

// StartProcessor 启动VoteProcessor的主处理循环和巡查员
func startProcessor(gracefulHandle, forcefulHandle *lifecycle.Handle) {
	defer gracefulHandle.Close()
	fmt.Println("投票处理器 (Vote Processor) 已启动。")

	// 立刻收集缺失的投票
	globalVoteProcessor.checkAndRequeueMissedVotes(gracefulHandle.Ctx())
	// 巡查员的生命周期与优雅关闭信号绑定
	patrollerCtx, patrollerCancel := context.WithCancel(gracefulHandle.Ctx())
	defer patrollerCancel() // 确保在主处理器退出时，巡查员也被关闭
	go globalVoteProcessor.runPatroller(patrollerCtx)

	globalVoteProcessor.runMainLoop(gracefulHandle, forcefulHandle)
}

// submitVoteToQueue 供Handler调用的方法，用于提交新的投票任务，返回是否成功
func submitVoteToQueue(vote Vote) {
	globalVoteProcessor.shutdownMutex.Lock()
	if globalVoteProcessor.isShutdown {
		globalVoteProcessor.shutdownMutex.Unlock()
		fmt.Printf("警告: 投票处理队列已满，放弃处理 vote ID: %d\n", vote.ID)
		return
	}
	select {
	case globalVoteProcessor.voteChan <- vote:
		globalVoteProcessor.shutdownMutex.Unlock()
	default:
		globalVoteProcessor.shutdownMutex.Unlock()
		fmt.Printf("警告: 投票处理队列已满，暂时放弃实时处理 vote ID: %d\n", vote.ID)
	}
}

// runMainLoop 是处理器的主事件循环，现在响应两阶段停机
func (vp *voteProcessor) runMainLoop(gracefulHandle, forcefulHandle *lifecycle.Handle) {
	for {
		select {
		case <-gracefulHandle.Done():
			// 收到第一停机信号，进入“排空队列”模式
			fmt.Println("Vote Processor: 收到优雅停机信号，正在处理剩余任务...")
			vp.drainQueue(forcefulHandle) // 使用强制停机handle来中断排空过程
			fmt.Println("Vote Processor: 优雅停机完成，主循环退出。")
			return
		default:
			// 正常处理流程
			vp.processSingleVote(gracefulHandle)
		}
	}
}

// drainQueue 在收到优雅停机信号后，尽力处理完暂存区和channel中的剩余任务
func (vp *voteProcessor) drainQueue(forcefulHandle *lifecycle.Handle) {
	vp.checkAndRequeueMissedVotes(forcefulHandle.Ctx())
	select {
	case <-forcefulHandle.Done():
		fmt.Println("Vote Processor: 收到强制停机信号，排空队列被中断。")
		return
	default:
	}

	// 然后关闭channel，不再接收新任务
	vp.shutdownMutex.Lock()
	vp.isShutdown = true
	close(vp.voteChan)
	vp.shutdownMutex.Unlock()

	// 将channel中所有剩余的任务都转移到暂存区
	for vote := range vp.voteChan {
		vp.processMutex.Lock()
		heap.Push(vp.buffer, vote)
		vp.processMutex.Unlock()
	}

	// 循环处理暂存区，直到它为空或收到强制关闭信号
	for {
		select {
		case <-forcefulHandle.Done():
			fmt.Println("Vote Processor: 收到强制停机信号，排空队列被中断。")
			return
		default:
		}

		vp.processMutex.Lock()
		if vp.buffer.Len() == 0 {
			vp.processMutex.Unlock()
			return // 队列已空，完成
		}
		// 我们只处理连续的任务
		if (*vp.buffer)[0].ID == vp.lastProcessedVoteID+1 {
			vote := heap.Pop(vp.buffer).(Vote)
			vp.processMutex.Unlock()
			// 在排空模式下，我们简化重试逻辑，如果失败则放弃
			if err := vp.applyVoteToRepository(vote); err == nil {
				vp.processMutex.Lock()
				vp.lastProcessedVoteID = vote.ID
				vp.processMutex.Unlock()
			} else {
				fmt.Printf("排空队列时处理 vote ID %d 失败，已放弃: %v\n", vote.ID, err)
			}
		} else {
			vp.processMutex.Unlock()
			// 如果不连续，说明有任务丢失，排空结束
			return
		}
	}
}

func (vp *voteProcessor) processSingleVote(gracefulHandle *lifecycle.Handle) {
	nextVote, err := vp.getNextContinuousVote(gracefulHandle)
	if err != nil {
		return
	}

	// 检查Redis健康状态
	if !database.IsRedisHealthy() {
		fmt.Println("Vote Processor: 检测到Redis不可用或正在重建，暂停处理...")
		gracefulHandle.Sleep(5 * time.Second) // 与健康检查器同步休眠
		// 将取出的任务放回暂存区，以便在Redis恢复后能被重新处理
		vp.processMutex.Lock()
		heap.Push(vp.buffer, nextVote)
		vp.processMutex.Unlock()
		return
	}

	select {
	case <-gracefulHandle.Done():
		return
	default:
	}

	// 处理投票，现在包含了精细化的重试逻辑
	err = vp.applyVoteToRepositoryWithRetry(gracefulHandle, nextVote)
	if err != nil {
		// 可能是Redis不健康了
		if err != context.Canceled && err != context.DeadlineExceeded {
			fmt.Printf("错误: 处理 vote ID %d 失败，已放回队列: %v\n", nextVote.ID, err)
		}
		// 将任务放回暂存区，并由外层循环处理休眠
		vp.processMutex.Lock()
		heap.Push(vp.buffer, nextVote)
		vp.processMutex.Unlock()
		return
	}

	// 只有在成功处理后才更新ID
	vp.processMutex.Lock()
	vp.lastProcessedVoteID = nextVote.ID
	vp.processMutex.Unlock()
}

// getNextContinuousVote 是一个阻塞函数，它会一直等待直到获取到下一个连续的投票
// 现在可以被gracefulHandle中断
func (vp *voteProcessor) getNextContinuousVote(gracefulHandle *lifecycle.Handle) (Vote, error) {
	for {
		vp.processMutex.Lock()
		// 丢弃所有过时的堆顶元素
		for vp.buffer.Len() > 0 && (*vp.buffer)[0].ID <= vp.lastProcessedVoteID {
			heap.Pop(vp.buffer)
		}

		// 检查暂存区是否有我们需要的下一个投票
		if vp.buffer.Len() > 0 && (*vp.buffer)[0].ID == vp.lastProcessedVoteID+1 {
			vote := heap.Pop(vp.buffer).(Vote)
			vp.processMutex.Unlock()
			return vote, nil
		}
		vp.processMutex.Unlock()

		// 从主channel中等待，或在超时后重新检查暂存区
		select {
		case <-gracefulHandle.Done():
			return Vote{}, gracefulHandle.Err()
		case vote := <-vp.voteChan:
			vp.processMutex.Lock()
			if vote.ID <= vp.lastProcessedVoteID {
				vp.processMutex.Unlock()
				continue // 收到的是过时的投票，直接丢弃
			}
			if vote.ID == vp.lastProcessedVoteID+1 {
				vp.processMutex.Unlock()
				return vote, nil // 正好是下一个，直接处理
			}
			// 收到的投票太新，放入暂存区
			heap.Push(vp.buffer, vote)
			vp.processMutex.Unlock()
		}
	}
}

// applyVoteToRepositoryWithRetry 包含了您设计的、带有指数退避和健康检查的重试逻辑
func (vp *voteProcessor) applyVoteToRepositoryWithRetry(gracefulHandle *lifecycle.Handle, vote Vote) error {
	initialDelay := 8 * time.Millisecond
	maxDelay := 2 * time.Second

	delay := initialDelay
	for delay < maxDelay { // 短循环重试
		err := vp.applyVoteToRepository(vote)
		if err == nil {
			return nil // 成功
		}
		if err = gracefulHandle.Sleep(delay); err != nil {
			return err
		}
		delay *= 2
	}

	// 进入长循环告警模式
	for {
		// 每次重试前都检查健康状态
		if !database.IsRedisHealthy() {
			return errors.New("redis became unhealthy during retry")
		}

		err := vp.applyVoteToRepository(vote)
		if err == nil {
			return nil // 最终成功
		}

		fmt.Printf("告警: Redis持续写入失败，将在%v后重试 vote ID %d\n", maxDelay, vote.ID)
		if err := gracefulHandle.Sleep(maxDelay); err != nil {
			return err
		}
	}
}

// runPatroller 启动一个后台巡查员，定期检查数据库中是否有被遗漏的投票
func (vp *voteProcessor) runPatroller(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			vp.checkAndRequeueMissedVotes(ctx)
		}
	}
}

func (vp *voteProcessor) checkAndRequeueMissedVotes(ctx context.Context) {
	if !database.IsRedisHealthy() {
		return // 如果Redis不健康，则跳过本次巡查
	}

	vp.processMutex.Lock()
	startID := vp.lastProcessedVoteID
	bufferMinID := uint(0)
	if vp.buffer.Len() > 0 {
		bufferMinID = (*vp.buffer)[0].ID
	}
	vp.processMutex.Unlock()

	select {
	case <-ctx.Done():
		return
	default:
	}

	var missedVotes []Vote
	query := database.DB.Where("id > ?", startID)
	if bufferMinID > 0 {
		query = query.Where("id < ?", bufferMinID)
	}
	query.Order("id asc").Limit(1000).Find(&missedVotes)

	if len(missedVotes) > 0 {
		vp.processMutex.Lock()
		currentID := vp.lastProcessedVoteID
		vp.processMutex.Unlock()
		if bufferMinID > 0 && currentID >= bufferMinID {
			return
		}

		fmt.Printf("巡查员: 发现 %d 条被遗漏的投票，正在提交处理...\n", len(missedVotes))
		for _, vote := range missedVotes {
			select {
			case <-ctx.Done():
				return
			default:
				if vote.ID > currentID {
					submitVoteToQueue(vote)
				}
			}
		}
	}
}

// applyVoteToRepository 将单个投票的计算结果原子地更新到Redis和内存仓库
func (vp *voteProcessor) applyVoteToRepository(vote Vote) error {
	if vote.Result == ResultSkip {
		// 对于跳过的投票，我们只需要原子地更新检查点
		return database.RDB.Set(database.Ctx, metadata.RedisLastProcessedVoteIDKey, vote.ID, 0).Err()
	}

	// 1. 加写锁，保护对Redis和内存权重树的联合更新
	spell.LockRepository()
	defer spell.UnlockRepository()

	vp.processMutex.Lock()
	currentID := vp.lastProcessedVoteID
	vp.processMutex.Unlock()
	if currentID > vote.ID {
		return nil
	}

	// 2. 从Redis获取当前统计数据
	keys := []string{vote.SpellA_ID, vote.SpellB_ID}
	statsJSONs, err := database.RDB.HMGet(database.Ctx, spell.StatsKey, keys...).Result()
	if err != nil {
		return fmt.Errorf("无法从Redis获取法术统计数据: %w", err)
	}
	if statsJSONs[0] == nil || statsJSONs[1] == nil {
		return fmt.Errorf("无法从Redis获取法术JSON数据")
	}

	var statsA, statsB spell.SpellStats
	_ = json.Unmarshal([]byte(statsJSONs[0].(string)), &statsA)
	_ = json.Unmarshal([]byte(statsJSONs[1].(string)), &statsB)
	oldScoreA, oldScoreB := statsA.Score, statsB.Score

	// 3. 计算新的ELO, Win, Total
	switch vote.Result {
	case ResultAWins:
		statsA.Score, statsB.Score = calculateElo(statsA.Score, statsB.Score, vote.Multiplier)
		statsA.Win += vote.Multiplier
		statsA.Total += vote.Multiplier
		statsB.Total += vote.Multiplier
	case ResultBWins:
		statsB.Score, statsA.Score = calculateElo(statsB.Score, statsA.Score, vote.Multiplier)
		statsB.Win += vote.Multiplier
		statsB.Total += vote.Multiplier
		statsA.Total += vote.Multiplier
	case ResultDraw:
		statsA.Total += vote.Multiplier
		statsB.Total += vote.Multiplier
	}

	eloTrackerTx := globalEloTracker.BeginUpdate()
	defer eloTrackerTx.RollbackUnlessCommitted()

	// 4. 检查ELO边界是否变化
	boundaryChanged := globalEloTracker.Update(eloTrackerTx, oldScoreA, statsA.Score) || globalEloTracker.Update(eloTrackerTx, oldScoreB, statsB.Score)

	// 5. 根据边界变化情况，选择性地更新或全局重建
	if boundaryChanged {
		err = rebuildAllRankScores(eloTrackerTx, vote, statsA, statsB)
	} else {
		// 6. 正常更新
		// 计算新的RankScore
		statsA.RankScore = CalculateRankScore(eloTrackerTx, statsA.Score, statsA.Total, statsA.Win)
		statsB.RankScore = CalculateRankScore(eloTrackerTx, statsB.Score, statsB.Total, statsB.Win)

		// 原子地写入Redis
		pipe := database.RDB.TxPipeline()
		statsAJSON, _ := json.Marshal(statsA)
		statsBJSON, _ := json.Marshal(statsB)
		pipe.HSet(database.Ctx, spell.StatsKey, vote.SpellA_ID, statsAJSON)
		pipe.HSet(database.Ctx, spell.StatsKey, vote.SpellB_ID, statsBJSON)
		pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsA.RankScore, Member: vote.SpellA_ID})
		pipe.ZAdd(database.Ctx, spell.RankingKey, &redis.Z{Score: statsB.RankScore, Member: vote.SpellB_ID})
		pipe.IncrByFloat(database.Ctx, metadata.RedisTotalVotesKey, vote.Multiplier)
		pipe.Set(database.Ctx, metadata.RedisLastProcessedVoteIDKey, vote.ID, 0)
		_, err = pipe.Exec(database.Ctx)
	}

	if err != nil {
		return err
	}

	// 更新内存权重树
	indexA, _ := spell.GetSpellIndexByID(vote.SpellA_ID)
	indexB, _ := spell.GetSpellIndexByID(vote.SpellB_ID)
	spell.UpdateWeightUnsafe(indexA, spell.CalculateWeightForTotal(statsA.Total))
	spell.UpdateWeightUnsafe(indexB, spell.CalculateWeightForTotal(statsB.Total))

	eloTrackerTx.Commit()
	return nil
}

// rebuildAllRankScores 在ELO边界变化时，执行全局的RankScore重算和批量更新
func rebuildAllRankScores(tx *eloTrackerTx, vote Vote, currentStatsA, currentStatsB spell.SpellStats) error {
	fmt.Println("检测到ELO边界变化，正在执行全局RankScore重建...")

	// 1. 获取所有法术的统计数据
	allStatsJSON, err := database.RDB.HGetAll(database.Ctx, spell.StatsKey).Result()
	if err != nil {
		return fmt.Errorf("无法获取所有法术统计数据以进行重建: %w", err)
	}

	updatedStats := make(map[string]spell.SpellStats)

	for id, statsJSON := range allStatsJSON {
		var stats spell.SpellStats
		_ = json.Unmarshal([]byte(statsJSON), &stats)
		updatedStats[id] = stats
	}

	// 2. 将当前投票的最新结果应用到全量数据中
	updatedStats[vote.SpellA_ID] = currentStatsA
	updatedStats[vote.SpellB_ID] = currentStatsB

	// 3. 重新计算所有法术的RankScore
	allScores := make([]float64, 0, len(allStatsJSON))
	for _, stats := range updatedStats {
		allScores = append(allScores, stats.Score)
	}

	// 重置ELO追踪器
	globalEloTracker.Reset(tx, allScores)

	// 现在用更新后的边界，为所有法术计算新的RankScore
	for id, stats := range updatedStats {
		stats.RankScore = CalculateRankScore(tx, stats.Score, stats.Total, stats.Win)
		updatedStats[id] = stats
	}

	// 4. 原子地将所有更新写回Redis
	pipe := database.RDB.TxPipeline()
	newRanking := make([]*redis.Z, 0, len(updatedStats))
	for id, stats := range updatedStats {
		statsJSON, _ := json.Marshal(stats)
		pipe.HSet(database.Ctx, spell.StatsKey, id, statsJSON)
		newRanking = append(newRanking, &redis.Z{Score: stats.RankScore, Member: id})
	}
	pipe.ZAdd(database.Ctx, spell.RankingKey, newRanking...) // 批量更新排名
	pipe.IncrByFloat(database.Ctx, metadata.RedisTotalVotesKey, vote.Multiplier)
	pipe.Set(database.Ctx, metadata.RedisLastProcessedVoteIDKey, vote.ID, 0)

	_, err = pipe.Exec(database.Ctx)
	return err
}
