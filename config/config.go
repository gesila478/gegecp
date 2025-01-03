package config

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"
)

var (
	GlobalConfig Config
	logger       *log.Logger
	loggerOnce   sync.Once
)

// GetLogger 返回全局日志记录器
func GetLogger() *log.Logger {
	loggerOnce.Do(func() {
		// 创建日志目录
		logDir := "./log"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// 尝试使用临时目录
			logDir = os.TempDir()
		}

		// 打开日志文件
		logFile, err := os.OpenFile(filepath.Join(logDir, "panel.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// 如果无法打开文件，仅使用标准输出
			logger = log.New(os.Stdout, "[PANEL] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		} else {
			// 只输出到文件
			logger = log.New(logFile, "[PANEL] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
		}
	})
	return logger
}

// Config 配置结构
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Auth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return err
	}

	// 记录配置加载信息
	if logger != nil {
		// logger.Printf("=== 配置文件加载完成 ===")
		// logger.Printf("配置文件路径: %s", path)
		// logger.Printf("用户名: %s", GlobalConfig.Auth.Username)
		// logger.Printf("密码哈希: %s", GlobalConfig.Auth.Password)
	}

	return nil
}

// getCurrentDir 获取当前工作目录
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}
