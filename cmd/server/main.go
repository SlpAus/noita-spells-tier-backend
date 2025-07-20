package main

import (
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/api"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/health"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/startup"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote" // *** 新增导入 ***
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	token.GenerateSecretKey()
	database.InitDB()
	database.InitRedis()
	health.InitializeRunID()

	if err := startup.InitializeApplication(); err != nil {
		panic(fmt.Sprintf("应用初始化失败，无法启动: %v", err))
	}

	fmt.Println("正在执行启动后健康检查...")
	health.PerformCheck()

	// 启动所有后台工作进程
	go health.StartRedisHealthCheck()
	go user.StartActivationWorker()
	if err := vote.StartVoteProcessor(); err != nil { // *** 新增调用 ***
		panic(fmt.Sprintf("启动Vote Processor失败: %v", err))
	}

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Static("/images/spells", "./assets/data/ui_gfx/gun_actions")
	r.Static("/images/borders", "./assets/spell_borders")

	api.SetupRoutes(r)

	// TODO: 在这里设置优雅停机和后台定时任务

	fmt.Println("服务器已准备就绪，开始监听 :8080")
	if err := r.Run(":8080"); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
