package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

const (
	// 72小时，每1分钟一个数据点
	maxDataPoints = 72 * 60 // 4320个点
	dataFile      = "data/system_history.json"
)

type SystemMetric struct {
	Timestamp time.Time
	CPU       float64
	Memory    float64
	Disk      float64
	Network   float64
}

var (
	lastNetStats   map[string]net.IOCountersStat
	lastUpdateTime time.Time
	netStatsMutex  sync.Mutex

	// 历史数据相关
	historyData  []SystemMetric
	historyMutex sync.RWMutex
	lastSaveTime time.Time
)

func init() {
	lastNetStats = make(map[string]net.IOCountersStat)
	historyData = make([]SystemMetric, 0, maxDataPoints)

	// 加载历史数据
	loadHistoryData()

	// 启动定时保存任务
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			saveHistoryData()
		}
	}()
}

// 加载历史数据
func loadHistoryData() {
	historyMutex.Lock()
	defer historyMutex.Unlock()

	// 确保 data 目录存在
	if err := os.MkdirAll("data", 0755); err != nil {
		return
	}

	data, err := ioutil.ReadFile(dataFile)
	if err != nil {
		return
	}

	var loadedData []SystemMetric
	if err := json.Unmarshal(data, &loadedData); err != nil {
		return
	}

	// 只保留最近72小时的数据
	cutoffTime := time.Now().Add(-72 * time.Hour)
	for _, metric := range loadedData {
		if metric.Timestamp.After(cutoffTime) {
			historyData = append(historyData, metric)
		}
	}
}

// 保存历史数据
func saveHistoryData() {
	historyMutex.RLock()
	data := make([]SystemMetric, len(historyData))
	copy(data, historyData)
	historyMutex.RUnlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	// 创建备份文件
	backupFile := dataFile + ".bak"
	if _, err := os.Stat(dataFile); err == nil {
		os.Rename(dataFile, backupFile)
	}

	// 写入新文件
	if err := ioutil.WriteFile(dataFile, jsonData, 0644); err != nil {
		// 如果写入失败，恢复备份
		if _, err := os.Stat(backupFile); err == nil {
			os.Rename(backupFile, dataFile)
		}
		return
	}

	// 删除备份
	os.Remove(backupFile)
	lastSaveTime = time.Now()
}

// 添加新的度量数据
func addMetric(metric SystemMetric) {
	historyMutex.Lock()
	defer historyMutex.Unlock()

	// 删除超过72小时的数据
	cutoffTime := time.Now().Add(-72 * time.Hour)
	for len(historyData) > 0 && historyData[0].Timestamp.Before(cutoffTime) {
		historyData = historyData[1:]
	}

	historyData = append(historyData, metric)

	// 如果距离上次保存超过1分钟，触发保存
	if time.Since(lastSaveTime) > 1*time.Minute {
		go saveHistoryData()
	}
}

// 获取历史数据
func getHistoryData() []SystemMetric {
	historyMutex.RLock()
	defer historyMutex.RUnlock()

	result := make([]SystemMetric, len(historyData))
	copy(result, historyData)
	return result
}

// 获取网络速度
func getNetworkSpeed() (sent uint64, recv uint64, totalSent uint64, totalRecv uint64, err error) {
	netStatsMutex.Lock()
	defer netStatsMutex.Unlock()

	// 获取当前网络统计信息
	stats, err := net.IOCounters(false)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	currentTime := time.Now()

	// 如果是第一次获取，记录基准值并返回0
	if len(lastNetStats) == 0 {
		lastNetStats["all"] = stats[0]
		lastUpdateTime = currentTime
		return 0, 0, stats[0].BytesSent, stats[0].BytesRecv, nil
	}

	// 计算时间差（秒）
	duration := currentTime.Sub(lastUpdateTime).Seconds()
	if duration == 0 {
		return 0, 0, stats[0].BytesSent, stats[0].BytesRecv, nil
	}

	// 计算速度（字节/秒）
	sentSpeed := uint64(float64(stats[0].BytesSent-lastNetStats["all"].BytesSent) / duration)
	recvSpeed := uint64(float64(stats[0].BytesRecv-lastNetStats["all"].BytesRecv) / duration)

	// 更新基准值
	lastNetStats["all"] = stats[0]
	lastUpdateTime = currentTime

	return sentSpeed, recvSpeed, stats[0].BytesSent, stats[0].BytesRecv, nil
}

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

	// 网络信息
	sentSpeed, recvSpeed, totalSent, totalRecv, err := getNetworkSpeed()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var cpuPercentValue float64
	if len(cpuPercent) > 0 {
		cpuPercentValue = cpuPercent[0]
	}

	// 添加新的度量数据
	metric := SystemMetric{
		Timestamp: time.Now(),
		CPU:       cpuPercentValue,
		Memory:    float64(memInfo.Used) / float64(memInfo.Total) * 100,
		Disk:      float64(diskInfo.Used) / float64(diskInfo.Total) * 100,
		Network:   float64(sentSpeed+recvSpeed) / (1024 * 1024), // MB/s
	}
	addMetric(metric)

	// 获取历史数据
	history := getHistoryData()

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
		"network": gin.H{
			"sent":       totalSent,
			"recv":       totalRecv,
			"sent_speed": sentSpeed,
			"recv_speed": recvSpeed,
		},
		"history": history,
	})
}
