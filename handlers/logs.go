package handlers

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func GetSystemLogs(c *gin.Context) {
	lines := c.DefaultQuery("lines", "100")
	cmd := exec.Command("journalctl", "-n", lines, "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, string(output))
}

func GetServiceLogs(c *gin.Context) {
	service := c.Query("service")
	lines := c.DefaultQuery("lines", "100")

	if service == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "服务名不能为空"})
		return
	}

	cmd := exec.Command("journalctl", "-u", service, "-n", lines, "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, string(output))
}
