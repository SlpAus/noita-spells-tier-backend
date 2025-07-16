package main

import (
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/routes"
	"github.com/SlpAus/noita-spells-tier-backend/services"
	"github.com/SlpAus/noita-spells-tier-backend/utils" // *** 新增导入 ***
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// *** 新增调用：在所有操作之前生成密钥 ***
	utils.GenerateSecretKey()

	// 1. 初始化数据库连接
	database.InitDB()
	database.InitRedis()

	// 2. 执行应用启动初始化流程
	services.InitializeApplication(true)

	// 3. 创建Gin引擎
	r := gin.Default()

	// 4. 配置CORS中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 5. 配置静态文件路由
	r.Static("/images/spells", "./assets/data/ui_gfx/gun_actions")
	r.Static("/images/borders", "./assets/spell_borders")

	// 6. 注册API路由
	routes.SetupRoutes(r)

	// TODO: 在这里设置优雅停机和后台定时任务

	// 7. 启动服务器
	err := r.Run(":8080")
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
