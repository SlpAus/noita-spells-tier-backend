package api

import (
	"github.com/SlpAus/noita-spells-tier-backend/internal/report"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
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

			spellRoutes.GET("/pair", user.EnsureUserCookieMiddleware(), spell.GetSpellPair)
		}

		// 投票相关的路由
		api.POST("/vote", user.LoadUserMiddleware(), vote.SubmitVote)

		// 报告相关的路由
		api.GET("/report", user.LoadUserMiddleware(), report.GetReport)
	}
}
