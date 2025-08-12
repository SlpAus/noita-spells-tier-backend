package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/api"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/backup"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/database"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/health"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/shutdown"
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/startup"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/token"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// --- 1. 初始设置 ---
	token.GenerateSecretKey()
	database.InitDB()
	database.InitRedis()
	health.InitializeRunID()

	// --- 2. 数据库和缓存初始化 ---
	if err := startup.InitializeApplication(); err != nil {
		panic(fmt.Sprintf("应用初始化失败，无法启动: %v", err))
	}

	fmt.Println("正在执行启动后健康检查...")
	health.PerformCheck()

	// --- 3. 创建生命周期和停机管理器 ---
	gracefulManager := lifecycle.NewManager()
	forcefulManager := lifecycle.NewManager()
	shutdownCoordinator := shutdown.NewCoordinator(gracefulManager, forcefulManager)

	// --- 4. 启动所有后台工作进程 ---
	// 为每个需要优雅停机的后台服务注册，并获取其生命周期句柄

	// TODO: 不再使用user.StartActivationWorker
	// userHandle, err := gracefulManager.NewServiceHandle("UserActivationWorker")
	// if err != nil {
	// 	panic(err)
	// }
	// go user.StartActivationWorker(userHandle)

	backupHandle, err := gracefulManager.NewServiceHandle("BackupScheduler")
	if err != nil {
		panic(err)
	}
	go backup.StartBackupScheduler(backupHandle)

	voteGracefulHandle, err := gracefulManager.NewServiceHandle("VoteProcessor")
	if err != nil {
		panic(err)
	}
	voteForcefulHandle, err := forcefulManager.NewServiceHandle("VoteProcessor")
	if err != nil {
		panic(err)
	}
	if err := vote.StartVoteProcessor(voteGracefulHandle, voteForcefulHandle); err != nil {
		panic(fmt.Sprintf("启动 Vote Processor 失败: %v", err))
	}

	healthHandle, err := forcefulManager.NewServiceHandle("HealthChecker")
	if err != nil {
		panic(err)
	}
	go health.StartRedisHealthCheck(healthHandle)

	// --- 5. 创建并配置Web服务器 ---
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

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// --- 6. 启动Web服务器并等待停机信号 ---
	go func() {
		fmt.Println("服务器已准备就绪，开始监听 :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("无法启动Gin服务器: " + err.Error())
		}
	}()

	// 这一步是阻塞的，程序将在这里等待，直到收到关闭信号
	shutdownCoordinator.ListenForSignalsAndShutdown(server)
}
