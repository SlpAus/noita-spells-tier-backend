package database

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

var Rdb *redis.Client
var Ctx = context.Background()

func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // Redis 服务器地址
	})

	// 测试连接
	_, err := Rdb.Ping(Ctx).Result()
	if err != nil {
		fmt.Println("无法连接到 Redis 服务器:", err)
	} else {
		fmt.Println("成功连接到 Redis 服务器")
	}
}
