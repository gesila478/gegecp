package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"gegecp/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var fileLogger = config.GetLogger()

// ValidateToken 验证token是否有效
func ValidateToken(token string) bool {
	fileLogger.Printf("\n=== ValidateToken 开始 ===")
	fileLogger.Printf("输入token: [%s] (长度: %d)", token, len(token))

	if token == "" {
		fileLogger.Printf("token为空，验证失败")
		return false
	}

	// 验证 token 是否有效
	hasher := md5.New()
	hasher.Write([]byte(config.GlobalConfig.Auth.Username + config.GlobalConfig.Auth.Password))
	expectedToken := hex.EncodeToString(hasher.Sum(nil))

	fileLogger.Printf("验证信息:")
	fileLogger.Printf("- 用户名: [%s]", config.GlobalConfig.Auth.Username)
	// fileLogger.Printf("- 密码哈希: [%s]", config.GlobalConfig.Auth.Password)
	// fileLogger.Printf("- 期望的token: [%s]", expectedToken)
	// fileLogger.Printf("- 实际的token: [%s]", token)
	fileLogger.Printf("- 验证结果: %v", token == expectedToken)

	return token == expectedToken
}

// AuthRequired 认证中间件
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// fileLogger.Printf("\n=== 认证中间件开始 ===")
		// fileLogger.Printf("请求路径: %s", c.Request.URL.Path)
		// fileLogger.Printf("请求方法: %s", c.Request.Method)
		// fileLogger.Printf("请求头: %+v", c.Request.Header)

		var token string

		// 检查是否是WebSocket请求
		if c.Request.URL.Path == "/api/terminal/ws" {
			// fileLogger.Printf("WebSocket请求，从URL参数获取token")
			token = c.Query("token")
			// fileLogger.Printf("URL参数中的token: [%s]", token)
		} else {
			// 从Authorization头获取token
			auth := c.GetHeader("Authorization")
			// fileLogger.Printf("Authorization头: [%s]", auth)
			if auth == "" {
				// fileLogger.Printf("未提供Authorization头")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证token"})
				c.Abort()
				return
			}

			// 从 Bearer token 中提取token
			parts := strings.Split(auth, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				// fileLogger.Printf("无效的Authorization格式: [%s]", auth)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证格式"})
				c.Abort()
				return
			}

			token = parts[1]
		}

		fileLogger.Printf("待验证的token: [%s]", token)
		if !ValidateToken(token) {
			fileLogger.Printf("token验证失败")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
			c.Abort()
			return
		}

		fileLogger.Printf("token验证成功")
		c.Next()
	}
}
