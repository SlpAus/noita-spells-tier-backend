package user

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CookieName   = "user-id"
	CookieMaxAge = 365 * 24 * 60 * 60
	UserIDKey    = "userID"
)

// EnsureUserCookieMiddleware 确保用户的浏览器中有一个格式正确的user-id cookie。
// 如果没有或格式不正确，它会生成一个新的临时ID并设置cookie。
func EnsureUserCookieMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := c.Cookie(CookieName)

		// 如果Cookie不存在，或存在但格式不正确，则分发一个新的
		if err != nil || !IsValidUUID(userID) {
			if err != http.ErrNoCookie {
				fmt.Printf("检测到无效的用户Cookie: %s, err: %v\n", userID, err)
			}
			provisionalUserID, err := CreateProvisionalUser()
			if err != nil {
				fmt.Printf("创建临时用户ID时发生错误: %v\n", err)
			} else {
				c.SetCookie(CookieName, provisionalUserID, CookieMaxAge, "/", "", false, true)
			}
		}

		c.Next()
	}
}

// LoadUserMiddleware 读取cookie并将其值放入Gin上下文中。
func LoadUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Cookie(CookieName)
		c.Set(UserIDKey, userID)
		c.Next()
	}
}
