package main

import (
	"fmt"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/api"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/health"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/startup"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	token.GenerateSecretKey()
	database.InitDB()
	database.InitRedis()

	// 1. *** 新增：阻塞式获取初始Run ID ***
	health.InitializeRunID()

	// 2. 执行应用首次启动初始化流程
	if err := startup.InitializeApplication(); err != nil {
		panic(fmt.Sprintf("应用初始化失败，无法启动: %v", err))
	}

	// 3. *** 新增：阻塞式执行一次启动后健康检查 ***
	fmt.Println("正在执行启动后健康检查...")
	health.PerformCheck()

	// 4. 异步启动后台的持续健康检查器
	go health.StartRedisHealthCheck()

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
