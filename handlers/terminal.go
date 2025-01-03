package handlers

import (
	"fmt"
	// "linux-panel/config"
	"net/http"
	"strings"
	"time"

	"io"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// var fileLogger = config.GetLogger()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TerminalWS(c *gin.Context) {
	// 先升级WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 获取并验证参数
	host := c.Query("host")
	username := c.Query("username")
	password := c.Query("password")

	// 参数验证
	var errMsg string
	switch {
	case host == "":
		errMsg = "错误: 未提供主机地址"
	case username == "":
		errMsg = "错误: 未提供用户名"
	case password == "":
		errMsg = "错误: 未提供密码"
	}

	if errMsg != "" {
		conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}

	// 确保主机地址包含端口
	if !strings.Contains(host, ":") {
		host = host + ":22"
	}

	// 发送连接信息
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("正在连接到 %s@%s ...\n", username, host)))

	// 创建SSH客户端配置
	sshConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 30,
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"arcfour256", "arcfour128", "arcfour",
				"aes128-cbc", "3des-cbc",
			},
			KeyExchanges: []string{
				"curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256",
				"ecdh-sha2-nistp384",
				"ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha1",
			},
			MACs: []string{
				"hmac-sha2-256-etm@openssh.com",
				"hmac-sha2-256",
				"hmac-sha1",
				"hmac-sha1-96",
			},
		},
	}

	// 连接SSH服务器
	sshConn, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		errMsg := fmt.Sprintf("SSH连接失败: %v", err)
		if strings.Contains(err.Error(), "unable to authenticate") {
			errMsg = fmt.Sprintf("SSH认证失败: 用户名[%s]或密码错误\n原始错误: %v", username, err)
		} else if strings.Contains(err.Error(), "connection refused") {
			errMsg = fmt.Sprintf("SSH连接被拒绝: 请检查服务器[%s]是否开启SSH服务(端口22)\n原始错误: %v", host, err)
		} else if strings.Contains(err.Error(), "i/o timeout") {
			errMsg = fmt.Sprintf("SSH连接超时: 请检查网络连接和防火墙设置，目标主机[%s]\n原始错误: %v", host, err)
		} else if strings.Contains(err.Error(), "no supported methods remain") {
			errMsg = fmt.Sprintf("SSH认证方法不支持: 服务器可能不允许密码认证，用户[%s]\n原始错误: %v", username, err)
		}
		conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}
	defer sshConn.Close()

	// 创建SSH会话
	session, err := sshConn.NewSession()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("创建SSH会话失败: %v", err)))
		return
	}
	defer session.Close()

	// 创建管道用于输入输出
	stdin, err := session.StdinPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("创建输入管道失败: %v", err)))
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("创建输出管道失败: %v", err)))
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("创建错误输出管道失败: %v", err)))
		return
	}

	// 设置环境变量
	envVars := []struct {
		key, value string
	}{
		{"TERM", "xterm-256color"},
		{"LANG", "en_US.UTF-8"},
		{"LC_ALL", "en_US.UTF-8"},
		{"SHELL", "/bin/bash"},
		{"PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
	}

	for _, env := range envVars {
		if err := session.Setenv(env.key, env.value); err != nil {
			continue
		}
	}

	// 请求伪终端
	if err := session.RequestPty("xterm-256color", 40, 80, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
		ssh.IUTF8:         1,
	}); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("请求伪终端失败: %v", err)))
		return
	}

	// 启动shell
	if err := session.Shell(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("启动shell失败: %v", err)))
		return
	}

	// 发送连接成功消息
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[32m=== 成功连接到 %s@%s ===\x1b[0m\r\n", username, host)))

	// 处理WebSocket消息
	go func() {
		defer func() {
			session.Close()
			sshConn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("WebSocket连接异常: %v", err)))
				}
				return
			}
			_, err = stdin.Write(message)
			if err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("写入SSH失败: %v", err)))
				return
			}
		}
	}()

	// 处理SSH输出
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("读取SSH输出失败: %v", err)))
				}
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// 处理SSH错误输出
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("读取SSH错误输出失败: %v", err)))
				}
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// 等待会话结束
	if err := session.Wait(); err != nil {
		if err != io.EOF {
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("SSH会话异常结束: %v", err)))
		}
	}
}
