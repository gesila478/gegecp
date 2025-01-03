package handlers

import (
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func GetNetworkStats(c *gin.Context) {
	cmd := exec.Command("netstat", "-i")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, string(output))
}

func GetConnections(c *gin.Context) {
	cmd := exec.Command("netstat", "-ntu")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.String(http.StatusOK, string(output))
}
