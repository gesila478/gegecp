package handlers

import (
	"linux-panel/config"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 验证旧密码
	if req.OldPassword != config.GlobalConfig.Auth.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
		return
	}

	// 更新配置文件中的密码
	config.GlobalConfig.Auth.Password = req.NewPassword

	// 将更新后的配置写入文件
	configFile, err := os.OpenFile("config/config.yaml", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开配置文件"})
		return
	}
	defer configFile.Close()

	encoder := yaml.NewEncoder(configFile)
	if err := encoder.Encode(config.GlobalConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置文件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码修改成功",
		"status":  "success",
	})
}
