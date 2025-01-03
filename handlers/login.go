package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"gegecp/config"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("解析请求失败: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 打印接收到的用户名和密码（仅用于调试）
	fmt.Printf("\n=== 登录请求详情 ===\n")
	fmt.Printf("接收到的用户名: [%s] (长度: %d)\n", req.Username, len(req.Username))
	fmt.Printf("接收到的密码哈希: [%s] (长度: %d)\n", req.Password, len(req.Password))
	fmt.Printf("配置文件中的用户名: [%s] (长度: %d)\n", config.GlobalConfig.Auth.Username, len(config.GlobalConfig.Auth.Username))
	fmt.Printf("配置文件中的密码哈希: [%s] (长度: %d)\n", config.GlobalConfig.Auth.Password, len(config.GlobalConfig.Auth.Password))

	// 验证用户名和密码
	if req.Username != config.GlobalConfig.Auth.Username {
		fmt.Printf("用户名不匹配\n")
		fmt.Printf("用户名比较: [%s] != [%s]\n", req.Username, config.GlobalConfig.Auth.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 直接比较密码哈希
	if req.Password != config.GlobalConfig.Auth.Password {
		fmt.Printf("密码哈希不匹配\n")
		fmt.Printf("密码哈希比较: [%s] != [%s]\n", req.Password, config.GlobalConfig.Auth.Password)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成 token
	hasher := md5.New()
	hasher.Write([]byte(config.GlobalConfig.Auth.Username + config.GlobalConfig.Auth.Password))
	token := hex.EncodeToString(hasher.Sum(nil))

	fmt.Printf("登录成功，生成token: %s\n", token)

	c.JSON(http.StatusOK, gin.H{
		"token":    token,
		"username": req.Username,
		"message":  "登录成功",
		"status":   "success",
	})
}

// 退出登录
func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "退出成功",
		"status":  "success",
	})
}
