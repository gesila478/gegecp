package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"gegecp/config"
	"net/http"

	"github.com/gin-contrib/sessions"
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

	// 验证用户名和密码
	if req.Username != config.GlobalConfig.Auth.Username {
		fmt.Printf("用户名不匹配\n")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if req.Password != config.GlobalConfig.Auth.Password {
		fmt.Printf("密码哈希不匹配\n")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成 token
	hasher := md5.New()
	hasher.Write([]byte(req.Username + req.Password))
	token := hex.EncodeToString(hasher.Sum(nil))

	// 设置session
	session := sessions.Default(c)
	session.Set("username", req.Username)
	session.Set("token", token)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败"})
		return
	}

	fmt.Printf("登录成功，用户名: %s, token: %s\n", req.Username, token)

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"message": "登录成功",
		"status":  "success",
	})
}
