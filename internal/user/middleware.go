package user

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// CookieName 是在浏览器中设置的Cookie的名称
	CookieName = "user-id"
	// 一年
	CookieMaxAge = 365 * 24 * 60 * 60
	// UserIDKey 是在Gin上下文中传递用户ID时使用的键
	UserIDKey = "userID"
)

// EnsureUserCookieMiddleware 确保用户的浏览器中有一个user-id的cookie。
// 如果没有，它会生成一个新的临时ID并设置cookie。
func EnsureUserCookieMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userActivated bool
		// 尝试从请求头中获取Cookie
		userID, err := c.Cookie(CookieName)

		if err != nil {
			if err != http.ErrNoCookie {
				fmt.Printf("获取用户Cookie时发生错误: %v\n", err)
			}
		} else {
			userActivated, err = IsUserActivated(userID)
			if err != nil {
				// 如果验证过程中发生服务器内部错误
				fmt.Printf("验证用户 %s 时发生错误: %v\n", userID, err)
			}
		}

		// 如果Cookie不存在，则创建一个新的临时用户ID
		if userActivated != true {
			provisionalUserID, err := CreateProvisionalUser()
			if err != nil {
				fmt.Printf("创建临时用户ID时发生错误: %v\n", err)
			} else {
				// 在响应中设置新的HttpOnly Cookie
				c.SetCookie(CookieName, provisionalUserID, CookieMaxAge, "/", "", false, true)
			}
		}

		// 无论cookie是否存在或是否有效，都继续处理请求
		// 这个中间件不负责验证，只负责分发
		c.Next()
	}
}

// LoadUserMiddleware 读取cookie并将其值放入Gin上下文中。
// 它不验证cookie的有效性，也不创建新用户。
func LoadUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Cookie(CookieName) // 忽略错误，如果cookie不存在，userID将为空字符串
		c.Set(UserIDKey, userID)          // 将userID（可能为空）存入上下文
		c.Next()
	}
}
