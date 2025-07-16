package database

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// RDB 是一个全局的Redis客户端实例，供项目其他部分使用
var RDB *redis.Client

// Ctx 是一个全局的上下文，用于Redis操作
var Ctx = context.Background()

// InitRedis 初始化与Redis数据库的连接
func InitRedis() {
	// 创建一个新的Redis客户端
	// Addr: Redis服务器的地址和端口
	// Password: 如果您的Redis没有密码，则留空
	// DB: 使用哪个数据库，0是默认数据库
	RDB = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// 使用Ping命令来测试连接是否成功
	_, err := RDB.Ping(Ctx).Result()
	if err != nil {
		// 如果连接失败，程序将panic并退出，打印出错误信息
		panic("无法连接到Redis: " + err.Error())
	}

	fmt.Println("Redis 连接成功！")
}
