package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/process"
)

// 处理进程列表请求
func HandleProcessList(c *gin.Context) {
	processes, err := process.Processes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var processList []gin.H
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cpu, _ := p.CPUPercent()
		memory, _ := p.MemoryInfo()
		status, _ := p.Status()

		processList = append(processList, gin.H{
			"pid":    p.Pid,
			"name":   name,
			"cpu":    cpu,
			"memory": memory.RSS,
			"status": status,
		})
	}

	c.JSON(http.StatusOK, processList)
}

// 处理进程终止请求
func HandleProcessKill(c *gin.Context) {
	pidStr := c.PostForm("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的进程ID"})
		return
	}

	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := proc.Kill(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "进程已终止"})
}
