package vote

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/redis/go-redis/v9"
)

// IPVoteCompensator 封装了一次IP计数增加操作的回滚逻辑。
// 它被设计为在业务流程失败时，通过defer语句安全地执行补偿。
type IPVoteCompensator struct {
	ip        string
	member    string
	committed bool
}

const (
	// ipVoteKeyPrefix 是Redis中有序集合的键名前缀
	ipVoteKeyPrefix = "ip_votes:"
	// ipVoteWindow 定义了IP投票计数的时间窗口
	ipVoteWindow = 24 * time.Hour
	// ipVoteTTL 是每个IP记录在Redis中的生存时间，比窗口稍长以作缓冲
	ipVoteTTL = 25 * time.Hour
)

var (
	ipMutex sync.RWMutex // 借用读写锁的概念，IncrementIPVoteCount可以并发执行
)

// deleteKeysByPrefix 是一个辅助函数，用于安全地删除key
func deleteKeysByPrefix(ctx context.Context, rdb *redis.Client, prefix string) error {
	var cursor uint64
	matchPattern := prefix + "*"
	const batchSize = 500 // 每次SCAN和DEL的数量

	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, matchPattern, batchSize).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// GenerateUniqueID 根据给定的时间生成一个16字节的、抗冲突的ID，并将其编码为Base64字符串。
// 结构: [ 8字节纳秒时间戳 (Big Endian) | 8字节随机数 ]
func generateUniqueID(t time.Time) (string, error) {
	// 准备一个16字节的缓冲区
	b := make([]byte, 16)

	// 1. 写入8字节的纳秒时间戳
	timestamp := uint64(t.UnixNano())
	binary.BigEndian.PutUint64(b[0:8], timestamp)

	// 2. 写入8字节的随机数
	_, err := rand.Read(b[8:16])
	if err != nil {
		return "", err
	}

	// 3. 使用URL安全的Base64编码，没有padding，更紧凑
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RebuildIPVoteCache 从SQLite重建过去ipVoteWindow内的IP投票缓存。
// 这个方法也用于应用启动时的初始化。
func RebuildIPVoteCache() error {
	fmt.Println("正在从SQLite重建IP投票频率缓存...")

	ipMutex.Lock()
	defer ipMutex.Unlock()

	// 1. 从SQLite中获取ipVoteWindow内的投票记录
	var recentVotes []Vote
	beginTime := time.Now().Add(-ipVoteWindow)
	err := database.DB.Model(&Vote{}).Where("vote_time > ?", beginTime).Select("user_ip", "vote_time").Find(&recentVotes).Error
	if err != nil {
		return fmt.Errorf("无法从SQLite读取近期投票: %w", err)
	}

	if len(recentVotes) == 0 {
		fmt.Println("IP频率限制：无近期投票数据需要恢复。")
		return nil
	}

	// 我们将相同IP的记录分组，以减少Pipeline的调用次数
	ipVoteMap := make(map[string][]redis.Z)
	for _, vote := range recentVotes {
		if vote.UserIP != "" {
			key := ipVoteKeyPrefix + vote.UserIP
			timestamp := float64(vote.VoteTime.UnixMicro())
			memberID, err := generateUniqueID(vote.VoteTime)
			if err != nil {
				fmt.Printf("生成 memberID 失败: %v\n", err)
				continue
			}
			ipVoteMap[key] = append(ipVoteMap[key], redis.Z{Score: timestamp, Member: memberID})
		}
	}

	// 2. 安全地删除所有旧的IP记录
	if err := deleteKeysByPrefix(database.Ctx, database.RDB, ipVoteKeyPrefix); err != nil {
		return fmt.Errorf("删除旧的IP键失败: %w", err)
	}
	fmt.Println("已删除所有旧的IP缓存记录。")

	// 3. 批量将记录写回Redis
	pipe := database.RDB.Pipeline()
	for key, members := range ipVoteMap {
		pipe.ZAdd(database.Ctx, key, members...)
		pipe.Expire(database.Ctx, key, ipVoteTTL)
	}
	if _, err := pipe.Exec(database.Ctx); err != nil {
		return fmt.Errorf("批量写回IP投票数据到Redis失败: %w", err)
	}

	fmt.Printf("IP频率限制：成功从SQLite恢复了 %d 个IP的投票数据到缓存。\n", len(ipVoteMap))
	return nil
}

// IncrementIPVoteCount 在Redis中为一个IP原子地记录一次新的投票，并返回其在过去ipVoteWindow内的总投票数。
// 返回最新的计数值和一个补偿句柄，用于在业务流程失败时回滚此次计数增加。当返回error时，补偿句柄为nil。
func IncrementIPVoteCount(ip string, voteTime time.Time) (int64, *IPVoteCompensator, error) {
	if ip == "" {
		return 0, nil, errors.New("投票缺少IP")
	}

	if net.ParseIP(ip) == nil {
		return 0, nil, errors.New("投票IP无效")
	}

	key := ipVoteKeyPrefix + ip
	// 1. 计算ipVoteWindow前的时间戳，作为清理的边界
	minTimestamp := float64(voteTime.Add(-ipVoteWindow).UnixMicro())

	// 2. 生成本次投票的Score和Member
	scoreTime := float64(voteTime.UnixMicro())
	memberID, err := generateUniqueID(voteTime)
	if err != nil {
		return 0, nil, fmt.Errorf("生成 memberID 失败: %w\n", err)
	}

	// 不使用defer自动管理，成功路径上ipMutex的读锁范围会拓展到SQLite操作结束后
	ipMutex.RLock()

	if !database.IsRedisHealthy() {
		ipMutex.RUnlock()
		return 0, nil, errors.New("服务暂时不可用，无法获取投票频率")
	}

	// 3. 使用Redis事务(TxPipeline)来保证所有操作的原子性
	pipe := database.RDB.TxPipeline()
	// a. 移除所有旧记录
	pipe.ZRemRangeByScore(database.Ctx, key, "-inf", fmt.Sprintf("(%f", minTimestamp))
	// b. 添加新记录
	pipe.ZAdd(database.Ctx, key, redis.Z{Score: scoreTime, Member: memberID})
	// c. 刷新过期时间
	pipe.Expire(database.Ctx, key, ipVoteTTL)
	// d. 获取更新后的总数
	countCmd := pipe.ZCard(database.Ctx, key)

	// 4. 执行事务
	_, err = pipe.Exec(database.Ctx)
	if err != nil {
		ipMutex.RUnlock()
		return 0, nil, fmt.Errorf("执行IP计数事务失败: %w", err)
	}

	// 5. 返回最新的计数值
	count, err := countCmd.Result()
	if err != nil {
		database.RDB.ZRem(database.Ctx, key, memberID)
		ipMutex.RUnlock()
		return 0, nil, fmt.Errorf("获取IP计数结果失败: %w", err)
	}

	return count, &IPVoteCompensator{ip: ip, member: memberID}, nil
}

// Commit 标记上层业务事务已成功，阻止后续的回滚操作。
// 这个方法应该在整个业务流程（例如，SQLite写入等）都成功后调用。
func (c *IPVoteCompensator) Commit() {
	c.committed = true
}

// RollbackUnlessCommitted 是一个用于defer调用的关键方法。
// 如果Commit()没有被调用，它会自动执行对Redis的补偿操作（删除之前添加的成员）。
func (c *IPVoteCompensator) RollbackUnlessCommitted() {
	// 如果事务已被提交，则无需做任何事
	defer ipMutex.RUnlock()

	if c.committed {
		return
	}

	if !database.IsRedisHealthy() {
		// 只记录错误，此时主流程已经失败了
		fmt.Printf("严重警告: IP投票计数补偿操作时Redis不健康。 IP: %s, Member: %s", c.ip, c.member)
	}

	// 执行补偿：从有序集合中移除本次投票对应的成员
	key := ipVoteKeyPrefix + c.ip

	err := database.RDB.ZRem(database.Ctx, key, c.member).Err()
	if err != nil {
		fmt.Printf("严重警告: IP投票计数补偿操作失败! IP: %s, Member: %s, 错误: %v\n", c.ip, c.member, err)
	}
}
