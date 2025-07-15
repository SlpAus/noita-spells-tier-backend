package main

import (
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/database"
	"github.com/SlpAus/noita-spells-tier-backend/routes" // 我们将所有路由注册逻辑放入此包
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化数据库和Redis (如果需要)
	database.InitDB()
	// database.InitRedis() // 如果暂时不用Redis可以先注释掉

	// 2. 创建Gin引擎
	r := gin.Default()

	// 3. 配置CORS中间件 (您的配置已经很好了，这里稍作完善)
	r.Use(cors.New(cors.Config{
		// 允许的前端地址
		AllowOrigins: []string{"http://localhost:3000"},
		// 允许的HTTP方法
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		// 允许的请求头
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		// 暴露给前端的响应头
		ExposeHeaders: []string{"Content-Length"},
		// 是否允许携带Cookies
		AllowCredentials: true,
		// 预检请求(OPTIONS)的缓存时间
		MaxAge: 12 * time.Hour,
	}))

	// 4. 配置静态文件路由 (解答您的问题)
	// 分别为不同的资源路径设置路由，这样更清晰、更安全
	// 前端请求 /images/spells/bomb.png 时，会从 ./assets/data/ui_gfx/gun_actions/ 目录中寻找 bomb.png
	r.Static("/images/spells", "./assets/data/ui_gfx/gun_actions")
	// 前端请求 /images/borders/projectile.png 时，会从 ./assets/spell_borders/ 目录中寻找 projectile.png
	r.Static("/images/borders", "./assets/spell_borders")

	// 5. 注册API路由 (解答您的问题)
	// 我们将所有API路由的注册逻辑都封装到 routes 包中，让main.go保持整洁
	routes.SetupRoutes(r)

	// 6. 启动服务器
	// 最好处理一下潜在的错误
	err := r.Run(":8080")
	if err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
