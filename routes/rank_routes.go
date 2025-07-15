package routes

import (
	"github.com/SlpAus/noita-spells-tier-backend/controllers"
	"github.com/gin-gonic/gin"
)

func RankRoutes(router *gin.RouterGroup) {
	router.GET("/api/rank/getRanking", controllers.GetRanking)
	router.GET("/api/rank/getItemRank", controllers.GetItemRank)
	router.GET("/api/rank/getMyRank", controllers.GetMyRank)
}
