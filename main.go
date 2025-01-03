package main

import (
	"encoding/json"
	"fmt"
	"gegecp/config"
	"gegecp/handlers"
	"gegecp/middleware"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var fileLogger *log.Logger

func init() {
	// 创建日志目录
	logDir := "/var/log/gegecp"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// log.Fatal("无法创建日志目录:", err)
	}

	// 打开日志文件
	logFile, err := os.OpenFile(filepath.Join(logDir, "panel.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// log.Fatal("无法打开日志文件:", err)
	}

	// 初始化文件日志记录器
	fileLogger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.LstdFlags)

	// 设置gin的日志输出
	gin.DefaultWriter = io.MultiWriter(os.Stdout, logFile)
	gin.DefaultErrorWriter = io.MultiWriter(os.Stderr, logFile)
}

// SSHClientConfig SSH客户端配置
type SSHClientConfig struct {
	User       string
	Password   string
	Host       string
	PrivateKey string
	Timeout    time.Duration
}

// SSHClient SSH客户端结构
type SSHClient struct {
	config    *SSHClientConfig
	client    *ssh.Client
	session   *ssh.Session
	wsConn    *websocket.Conn
	writeLock sync.Mutex
}

// 初始化SSH客户端
func newSSHClient(config *SSHClientConfig) (*SSHClient, error) {
	// 如果主机地址不包含端口，则添加默认端口22
	host := config.Host
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	var authMethods []ssh.AuthMethod

	// 如果提供了私钥，优先使用私钥认证
	if config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// 如果提供了密码，添加密码认证
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	// 设置超时时间
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %v", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	return &SSHClient{
		config:  config,
		client:  client,
		session: session,
	}, nil
}

// 启动终端会话
func (s *SSHClient) startTerminal() error {
	// 设置终端模式
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := s.session.RequestPty("xterm-256color", 40, 80, modes); err != nil {
		return fmt.Errorf("请求PTY失败: %v", err)
	}

	// 创建管道用于输入输出
	stdin, err := s.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建stdin管道失败: %v", err)
	}

	stdout, err := s.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %v", err)
	}

	stderr, err := s.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建stderr管道失败: %v", err)
	}

	// 启动shell
	if err := s.session.Shell(); err != nil {
		return fmt.Errorf("启动shell失败: %v", err)
	}

	// 处理输入
	go func() {
		for {
			_, message, err := s.wsConn.ReadMessage()
			if err != nil {
				return
			}
			stdin.Write(message)
		}
	}()

	// 处理输出
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				return
			}
			s.writeLock.Lock()
			err = s.wsConn.WriteMessage(websocket.TextMessage, buf[:n])
			s.writeLock.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// 处理错误输出
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			s.writeLock.Lock()
			err = s.wsConn.WriteMessage(websocket.TextMessage, buf[:n])
			s.writeLock.Unlock()
			if err != nil {
				return
			}
		}
	}()

	return nil
}

// 关闭连接
func (s *SSHClient) Close() {
	if s.session != nil {
		s.session.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
	if s.wsConn != nil {
		s.wsConn.Close()
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 处理SSH终端连接
func handleSSHTerminal(c *gin.Context) {
	// fileLogger.Printf("=== WebSocket连接请求 ===")
	// fileLogger.Printf("URL: %s", c.Request.URL.String())
	// fileLogger.Printf("请求头: %+v", c.Request.Header)

	// 验证认证头
	auth := c.GetHeader("Authorization")
	if auth == "" {
		// fileLogger.Printf("WebSocket连接失败: 未提供认证头")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证token"})
		return
	}

	// 从 Bearer token 中提取token
	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		// fileLogger.Printf("无效的认证格式")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证格式"})
		return
	}

	token := parts[1]
	if !middleware.ValidateToken(token) {
		// fileLogger.Printf("无效的token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
		return
	}
	// fileLogger.Printf("Token验证成功: %s", token)

	// 升级HTTP连接为WebSocket
	// fileLogger.Printf("开始升级WebSocket连接...")
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// fileLogger.Printf("WebSocket升级失败: %v\n错误类型: %T\n堆栈: %+v", err, err, err)
		return
	}
	defer ws.Close()
	// fileLogger.Printf("WebSocket连接升级成功")

	// 获取连接参数
	host := c.Query("host")
	// fileLogger.Printf("主机地址: %s", host)

	// 如果是本地连接，直接启动本地shell
	if host == "localhost" || host == "127.0.0.1" {
		// fileLogger.Printf("启动本地终端会话")
		handleLocalTerminal(ws)
		return
	}

	// 远程SSH连接
	user := c.Query("user")
	password := c.Query("password")
	// fileLogger.Printf("尝试建立SSH连接: %s@%s", user, host)

	config := &SSHClientConfig{
		Host:     host,
		User:     user,
		Password: password,
		Timeout:  10 * time.Second,
	}

	sshClient, err := newSSHClient(config)
	if err != nil {
		// fileLogger.Printf("SSH连接失败: %v", err)
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH连接失败: %v\n", err)))
		return
	}
	defer sshClient.Close()

	sshClient.wsConn = ws
	if err := sshClient.startTerminal(); err != nil {
		// fileLogger.Printf("启动终端失败: %v", err)
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("启动终端失败: %v\n", err)))
		return
	}

	// fileLogger.Printf("SSH终端会话已建立: %s@%s", user, host)
	if err := sshClient.session.Wait(); err != nil {
		// fileLogger.Printf("会话结束: %v", err)
	}
}

// 处理本地终端
func handleLocalTerminal(ws *websocket.Conn) {
	// fileLogger.Printf("开始创建本地终端进程")

	// 检测可用的shell
	shells := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"}
	var shellPath string
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			shellPath = shell
			break
		}
	}

	if shellPath == "" {
		ws.WriteMessage(websocket.TextMessage, []byte("错误: 未找到可用的shell\n"))
		return
	}

	cmd := exec.Command(shellPath)

	// 设置环境变量
	defaultEnv := os.Environ()
	cmd.Env = append(defaultEnv,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=en_US.UTF-8", // 先使用英文环境确保基本功能
	)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		// fileLogger.Printf("启动终端失败: %v\n错误类型: %T\n堆栈: %+v", err, err, err)
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("启动终端失败: %v\n", err)))
		return
	}
	defer ptmx.Close()
	// fileLogger.Printf("本地终端进程创建成功")

	// 设置初始终端大小
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: 40,
		Cols: 80,
	}); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("设置终端大小失败: %v\n", err)))
	}

	// 处理WebSocket输入
	go func() {
		defer cmd.Process.Kill()
		for {
			messageType, message, err := ws.ReadMessage()
			if err != nil {
				// fileLogger.Printf("读取WebSocket消息失败: %v", err)
				return
			}

			// 检查是否是调整大小的消息
			if messageType == websocket.TextMessage {
				var resizeMsg struct {
					Type string `json:"type"`
					Cols uint16 `json:"cols"`
					Rows uint16 `json:"rows"`
				}
				if err := json.Unmarshal(message, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					// fileLogger.Printf("调整终端大小: %dx%d", resizeMsg.Cols, resizeMsg.Rows)
					pty.Setsize(ptmx, &pty.Winsize{
						Rows: resizeMsg.Rows,
						Cols: resizeMsg.Cols,
					})
					continue
				}
			}

			// 普通输入消息
			if _, err = ptmx.Write(message); err != nil {
				// fileLogger.Printf("写入终端失败: %v", err)
				return
			}
		}
	}()

	// 处理终端输出
	buf := make([]byte, 8192) // 增大缓冲区
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				// fileLogger.Printf("读取终端输出失败: %v", err)
				ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("终端错误: %v\n", err)))
			}
			return
		}
		if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
			// fileLogger.Printf("发送终端输出失败: %v", err)
			return
		}
	}
}

func main() {
	// 设置为生产模式
	gin.SetMode(gin.ReleaseMode)

	// 初始化路由
	r := gin.Default()

	// 设置受信任的代理
	r.SetTrustedProxies([]string{"127.0.0.1"})

	// 加载配置文件
	if err := config.LoadConfig("config/config.yaml"); err != nil {
		// log.Fatal("加载配置文件失败:", err)
	}

	// 设置静态文件路由
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// 首页路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Linux Panel",
		})
	})

	// API路由组
	api := r.Group("/api")
	{
		// 登录路由 - 不需要认证
		api.POST("/login", handlers.Login)

		// 需要认证的路由组
		auth := api.Group("/")
		auth.Use(middleware.AuthRequired())
		{
			// 终端相关路由
			auth.GET("/terminal/ws", handlers.TerminalWS)

			// 系统信息
			auth.GET("/system/info", handlers.HandleSystemInfo)

			// 进程管理
			auth.GET("/process/list", handlers.HandleProcessList)
			auth.POST("/process/kill", handlers.HandleProcessKill)

			// 文件管理
			auth.GET("/files/list", handlers.HandleFilesList)
			auth.POST("/files/upload", handlers.HandleFileUpload)
			auth.GET("/files/download", handlers.HandleFileDownload)
			auth.DELETE("/files/delete", handlers.HandleFileDelete)
			auth.GET("/files/read", handlers.HandleFileRead)
			auth.POST("/files/save", handlers.HandleFileSave)
			auth.POST("/files/chmod", handlers.HandleFileChmod)

			// 收藏管理
			auth.GET("/favorites", handlers.GetFavorites)
			auth.POST("/favorites", handlers.UpdateFavorites)

			// 用户相关
			auth.POST("/user/change-password", handlers.ChangePassword)

			// 其他API路由...
		}
	}

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", config.GlobalConfig.Server.Host, config.GlobalConfig.Server.Port)
	// fileLogger.Printf("服务器启动在: %s", addr)
	r.Run(addr)
}
