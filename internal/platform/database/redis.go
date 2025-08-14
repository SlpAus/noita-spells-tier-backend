package database

import (
	"context"
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

// RDB 是一个全局的Redis客户端实例，供项目其他部分使用
var RDB *redis.Client

// Ctx 是一个全局的上下文，用于Redis操作
var Ctx = context.Background()

// InitRedis 初始化与Redis数据库的连接
func InitRedis(cfg config.RedisConfig) {
	// 创建一个新的Redis客户端
	// 使用从配置文件加载的参数
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 使用Ping命令来测试连接是否成功
	_, err := RDB.Ping(Ctx).Result()
	if err != nil {
		// 如果连接失败，程序将panic并退出，打印出错误信息
		panic("无法连接到Redis: " + err.Error())
	}

	fmt.Println("Redis 连接成功！")
}
