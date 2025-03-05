package routes

import (
	"github.com/Qiuarctica/isaac-ranking-backend/controllers"
	"github.com/gin-gonic/gin"
)

func VoteRoutes(router *gin.RouterGroup) {
	router.POST("/api/vote/sendVoting", controllers.SendVoting)
	router.POST("/api/v1/vote/send", controllers.SendVotingResult)
	router.POST("/api/vote/Pvote", controllers.Pvote)
	router.GET("/api/vote/GetPvoteNum", controllers.PvoteNum)
	router.GET("/api/vote/GetPvoteRecords", controllers.GetPvoteRecord)
}
