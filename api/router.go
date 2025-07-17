package api

import (
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user" // *** 新增导入 ***
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 注册项目的所有API路由
func SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		// 法术相关的路由组 /api/spells
		spellRoutes := api.Group("/spells")
		{
			spellRoutes.GET("/ranking", spell.GetRanking)
			spellRoutes.GET("/:id", spell.GetSpellByID)

			// *** 新增/修改部分 ***
			// 为 /pair 路由应用 EnsureUserCookieMiddleware
			// 确保每个请求获取法术对的用户，浏览器里都有一个（可能是临时的）user-id
			spellRoutes.GET("/pair", user.EnsureUserCookieMiddleware(), spell.GetSpellPair)
		}

		// 投票相关的路由 /api/vote
		// 为 /vote 路由应用 LoadUserMiddleware
		// 它会把请求中的 user-id 加载到上下文中，供后续的handler使用
		api.POST("/vote", user.LoadUserMiddleware(), vote.SubmitVote)

		// TODO: 将来这里会有 /reports 相关的路由
	}
}
