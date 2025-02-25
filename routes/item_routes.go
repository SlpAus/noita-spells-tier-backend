package routes

import (
	"github.com/Qiuarctica/isaac-ranking-backend/controllers"
	"github.com/gin-gonic/gin"
)

func ItemRoutes(router *gin.RouterGroup) {
	router.GET("/api/item/getItems", controllers.GetItems)
}
