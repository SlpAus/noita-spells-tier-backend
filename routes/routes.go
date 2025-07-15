package routes

import (
	"github.com/SlpAus/noita-spells-tier-backend/controllers"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 注册项目的所有API路由
func SetupRoutes(router *gin.Engine) {
	// 创建一个 /api 的路由组
	api := router.Group("/api")
	{
		// 法术相关的路由组 /api/spells
		spellRoutes := api.Group("/spells")
		{
			spellRoutes.GET("/pair", controllers.GetSpellPair)
			spellRoutes.GET("/ranking", controllers.GetRanking)
			spellRoutes.GET("/:id", controllers.GetSpellByID)
		}

		// 投票相关的路由 /api/vote
		api.POST("/vote", controllers.SubmitVote)

		// 个性化报告相关的路由 /api/reports
		reportRoutes := api.Group("/reports")
		{
			// 使用路径参数:identifier来获取特定用户的报告
			reportRoutes.GET("/:identifier", controllers.GetUserReport)
		}
	}
}
