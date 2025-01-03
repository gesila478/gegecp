package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

// 处理系统信息请求
func HandleSystemInfo(c *gin.Context) {
	// CPU信息
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cpuInfo, err := cpu.Info()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var cpuModel string
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}

	// 内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 磁盘信息
	diskInfo, err := disk.Usage("/")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var cpuPercentValue float64
	if len(cpuPercent) > 0 {
		cpuPercentValue = cpuPercent[0]
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu": gin.H{
			"percent": cpuPercentValue,
			"model":   cpuModel,
		},
		"memory": gin.H{
			"total": memInfo.Total,
			"used":  memInfo.Used,
			"free":  memInfo.Free,
		},
		"disk": gin.H{
			"total": diskInfo.Total,
			"used":  diskInfo.Used,
			"free":  diskInfo.Free,
		},
	})
}
