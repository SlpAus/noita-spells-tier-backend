package main

import (
	"time"

	"github.com/Qiuarctica/isaac-ranking-backend/database"
	"github.com/Qiuarctica/isaac-ranking-backend/routes"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	database.InitDB()
	database.SeedDB()

	r := gin.Default()

	// 配置 CORS 中间件
	r.Use(cors.New(cors.Config{
		// AllowOrigins:     []string{"http://114.55.238.72:8088/"}, // 允许的前端地址
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// 配置静态文件路由
	r.Static("/assets", "./assets")

	routes.ItemRoutes(r.Group(""))
	routes.VoteRoutes(r.Group(""))
	routes.RankRoutes(r.Group(""))

	r.Run(":8080")
}
