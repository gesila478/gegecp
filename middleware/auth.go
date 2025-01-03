package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"gegecp/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidateToken 验证token是否有效
func ValidateToken(token string) bool {
	if token == "" {
		return false
	}

	// 验证 token 是否有效
	hasher := md5.New()
	hasher.Write([]byte(config.GlobalConfig.Auth.Username + config.GlobalConfig.Auth.Password))
	expectedToken := hex.EncodeToString(hasher.Sum(nil))

	return token == expectedToken
}

// AuthRequired 认证中间件
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// 检查是否是WebSocket请求
		if c.Request.URL.Path == "/api/terminal/ws" {
			token = c.Query("token")
		} else {
			// 从Authorization头获取token
			auth := c.GetHeader("Authorization")
			if auth == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证token"})
				c.Abort()
				return
			}

			// 从 Bearer token 中提取token
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证格式"})
				c.Abort()
				return
			}

			token = parts[1]
		}

		if !ValidateToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
