package handlers

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func ListServices(c *gin.Context) {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, string(output))
}

func ServiceControl(c *gin.Context) {
	service := c.Query("service")
	action := c.Query("action") // start, stop, restart, status

	if service == "" || action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "服务名和操作不能为空"})
		return
	}

	cmd := exec.Command("systemctl", action, service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": string(output)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "操作成功",
		"output":  string(output),
	})
}
