package api

import (
	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/config"
	"github.com/SlpAus/noita-spells-tier-backend/internal/report"
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	"github.com/gin-gonic/gin"
)

// SetupRoutes 注册项目的所有API路由
func SetupRoutes(router *gin.Engine, cfg config.AppConfig) {
	api := router.Group("/api")
	{
		var spellRoutes *gin.RouterGroup
		switch cfg.Mode {
		case config.AppModeSpell:
			spellRoutes = api.Group("/spells")
		case config.AppModePerk:
			spellRoutes = api.Group("/perks")
		}

		// 路由在模式下
		{
			// 候选人相关的路由组
			spellRoutes.GET("/ranking", spell.GetRanking)
			spellRoutes.GET("/:id", spell.GetSpellByID)
			spellRoutes.GET("/pair", user.EnsureUserCookieMiddleware(), spell.GetSpellPair)

			// 投票相关的路由
			spellRoutes.POST("/vote", user.LoadUserMiddleware(), vote.SubmitVote)

			// 报告相关的路由
			spellRoutes.GET("/report", user.LoadUserMiddleware(), report.GetReport)
		}
	}
}
