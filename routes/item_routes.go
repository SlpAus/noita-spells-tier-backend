package routes

import (
	"github.com/SlpAus/noita-spells-tier-backend/controllers"
	"github.com/gin-gonic/gin"
)

func ItemRoutes(router *gin.RouterGroup) {
	router.GET("/api/item/getItems", controllers.GetItems)
}
