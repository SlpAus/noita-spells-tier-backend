package routes

import (
	"github.com/Qiuarctica/isaac-ranking-backend/controllers"
	"github.com/gin-gonic/gin"
)

func VoteRoutes(router *gin.RouterGroup) {
	router.POST("/api/vote/sendVoting", controllers.SendVoting)
	router.POST("/api/v1/vote/send", controllers.SendVotingResult)
}
