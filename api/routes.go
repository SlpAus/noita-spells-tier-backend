package api

import (
	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
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
			// TODO: 将来这里会是 GetSpellPair
		}

		// 投票相关的路由 /api/vote
		api.POST("/vote", vote.SubmitVote)

		// TODO: 将来这里会有 /reports 和 /user 相关的路由
	}
}
