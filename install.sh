#!/bin/bash

# 错误处理
set -e

# 设置颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 设置日志文件
LOG_FILE="/var/log/gegecp-install.log"

# 日志函数
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

# 错误日志函数
log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $1" | tee -a "$LOG_FILE"
}

# 错误处理函数
handle_error() {
    log_error "安装过程中发生错误"
    echo -e "${RED}安装过程中发生错误，请检查日志: $LOG_FILE${NC}"
    exit 1
}

# 设置错误处理
trap 'handle_error' ERR

echo -e "${GREEN}开始安装 Linux 管理面板..............................${NC}"

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}请使用 root 用户运行此脚本${NC}"
    exit 1
fi

# 安装 curl
echo "正在安装 curl,wget..."
apt-get update
apt-get upgrade
apt-get install -y curl
apt-get install -y wget

# 检查系统
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$NAME
    VER=$VERSION_ID
else
    echo -e "${RED}无法确定操作系统类型${NC}"
    exit 1
fi

# 检查系统版本兼容性
SUPPORTED=false

if [[ "$OS" == *"Ubuntu"* ]]; then
    case "$VER" in
        "20.04"|"22.04"|"24.04")
            SUPPORTED=true
            ;;
    esac
elif [[ "$OS" == *"Debian"* ]]; then
    case "$VER" in
        "10"|"11"|"12")
            SUPPORTED=true
            ;;
    esac
fi

if [ "$SUPPORTED" = false ]; then
    echo -e "${RED}不支持的系统版本"
    echo "支持的系统版本："
    echo "Ubuntu: 20.04, 22.04, 24.04"
    echo "Debian: 10, 11, 12${NC}"
    exit 1
fi

echo -e "${GREEN}检测到系统: $OS $VER${NC}"

# 安装必要的依赖
echo "正在安装依赖..."
# 更新软件源
# apt-get update

# 针对不同系统版本安装依赖
if [[ "$OS" == *"Ubuntu"* ]]; then
    if [[ "$VER" == "20.04" ]]; then
        # Ubuntu 20.04 特定依赖
        apt-get install -y git software-properties-common apt-transport-https ca-certificates
    else
        # Ubuntu 22.04 和 24.04
        apt-get install -y git
    fi
elif [[ "$OS" == *"Debian"* ]]; then
    if [[ "$VER" == "9" ]]; then
        # Debian 9 特定依赖
        apt-get install -y git apt-transport-https ca-certificates gnupg2
    else
        # Debian 10, 11, 12
        apt-get install -y git
    fi
fi

# 检查并安装 Go 环境
echo "检查 Go 环境..."
if ! command -v go &> /dev/null; then
    log "正在安装 Go..."
    # 选择合适的 Go 版本
    GO_VERSION="1.23.4"
    
    # 对于较老的系统，使用兼容的 Go 版本
    if [[ "$OS" == *"Debian"* ]] && [[ "$VER" == "9" ]]; then
        GO_VERSION="1.19.13"
    fi
    
    # 定义下载镜像源
    MIRRORS=(
        "https://golang.google.cn/dl"
        "https://gomirrors.org/dl"
        "https://mirrors.aliyun.com/golang"
        "https://mirrors.ustc.edu.cn/golang"
    )
    
    # 下载函数，带有超时处理
    download_with_timeout() {
        local url=$1
        local output=$2
        local timeout=30  # 设置超时时间为30秒
        
        if wget --timeout=$timeout --tries=3 "$url" -O "$output" 2>/dev/null; then
            return 0
        else
            return 1
        fi
    }
    
    # 尝试从不同镜像源下载
    GO_DOWNLOADED=false
    for mirror in "${MIRRORS[@]}"; do
        echo "尝试从 $mirror 下载 Go..."
        if download_with_timeout "$mirror/go${GO_VERSION}.linux-amd64.tar.gz" "go${GO_VERSION}.linux-amd64.tar.gz"; then
            GO_DOWNLOADED=true
            echo "从 $mirror 下载成功"
            break
        else
            echo "从 $mirror 下载失败，尝试下一个镜像"
        fi
    done
    
    # 检查是否成功下载
    if [ "$GO_DOWNLOADED" = false ]; then
        echo -e "${RED}所有镜像源下载失败，请检查网络连接或手动安装 Go${NC}"
        exit 1
    fi
    
    # 删除可能存在的旧版本
    rm -rf /usr/local/go
    
    # 解压到 /usr/local
    tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    
    # 设置环境变量
    if ! grep -q "/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
        # 设置 GOPROXY 使用中国代理
        echo 'export GOPROXY=https://goproxy.cn,direct' >> /etc/profile
    fi
    source /etc/profile
    
    # 清理下载文件
    rm go${GO_VERSION}.linux-amd64.tar.gz
    
    echo "Go 安装完成"
else
    echo "Go 已安装，版本：$(go version)"
fi

# 验证 Go 安装
if ! command -v go &> /dev/null; then
    echo "Go 安装失败"
    exit 1
fi

# 创建工作目录
INSTALL_DIR="/opt/gegecp"
mkdir -p $INSTALL_DIR
cd $INSTALL_DIR

# 下载面板源码
log "正在下载面板源码..."
# 创建临时目录并克隆代码
TMP_DIR=$(mktemp -d)
echo "正在克隆最新代码..."
git clone --depth=1 --branch main https://github.com/gesila478/Gegecp.git "$TMP_DIR"
cd "$TMP_DIR"

# 确保获取最新代码
git fetch --depth=1 origin refs/heads/main
git reset --hard FETCH_HEAD

if [ $? -ne 0 ]; then
    echo -e "${RED}下载源码失败${NC}"
    exit 1
fi

# 编译
log "正在编译..."
# 删除可能存在的 go.mod 和 go.sum
rm -f go.mod go.sum
go mod init gegecp
go mod tidy
go get github.com/gin-gonic/gin
go get github.com/gorilla/websocket
go build -o panel

# 清空并准备安装目录
rm -rf "${INSTALL_DIR:?}"/*

# 创建必要的目录结构
mkdir -p "$INSTALL_DIR/templates"
mkdir -p "$INSTALL_DIR/static/css"
mkdir -p "$INSTALL_DIR/static/js"
mkdir -p "$INSTALL_DIR/static/js/lib"
mkdir -p "$INSTALL_DIR/config"

# 复制编译好的文件和前端文件到安装目录
cp "$TMP_DIR/panel" "$INSTALL_DIR/"

# 复制前端文件（使用 -f 强制覆盖）
echo "正在复制前端文件..."
cp -rf "$TMP_DIR/templates/"* "$INSTALL_DIR/templates/"
cp -rf "$TMP_DIR/static/"* "$INSTALL_DIR/static/"

# # 调试信息
# echo "源目录内容:"
# ls -la "$TMP_DIR/templates"
# ls -la "$TMP_DIR/static"
# echo "目标目录内容:"
# ls -la "$INSTALL_DIR/templates"
# ls -la "$INSTALL_DIR/static"

# # 下载第三方库到本地
# echo "正在下载第三方库..."
# cd "$INSTALL_DIR/static/js/lib"

# # 下载 Vue.js
# wget -O vue.min.js https://cdn.jsdelivr.net/npm/vue@2.6.14/dist/vue.min.js

# # 下载 Axios
# wget -O axios.min.js https://cdn.jsdelivr.net/npm/axios/dist/axios.min.js

# # 下载 CryptoJS
# wget -O crypto-js.min.js https://cdnjs.cloudflare.com/ajax/libs/crypto-js/4.1.1/crypto-js.min.js

# 下载Monaco Editor文件
# echo "正在下载Monaco Editor..."
# MONACO_VERSION="0.33.0"
MONACO_BASE="$INSTALL_DIR/static/js/lib/monaco-editor"
# mkdir -p "$MONACO_BASE/min/vs/base/worker"
# mkdir -p "$MONACO_BASE/min/vs/editor"
# mkdir -p "$MONACO_BASE/min/vs/basic-languages"

# 下载基础文件
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/loader.js" -o "$MONACO_BASE/min/vs/loader.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/editor/editor.main.js" -o "$MONACO_BASE/min/vs/editor/editor.main.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/editor/editor.main.css" -o "$MONACO_BASE/min/vs/editor/editor.main.css"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/editor/editor.main.nls.js" -o "$MONACO_BASE/min/vs/editor/editor.main.nls.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/base/worker/workerMain.js" -o "$MONACO_BASE/min/vs/base/worker/workerMain.js"

# 下载语言支持
# mkdir -p "$MONACO_BASE/min/vs/basic-languages/javascript"
# mkdir -p "$MONACO_BASE/min/vs/basic-languages/css"
# mkdir -p "$MONACO_BASE/min/vs/basic-languages/html"
# mkdir -p "$MONACO_BASE/min/vs/basic-languages/shell"

# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/basic-languages/javascript/javascript.js" -o "$MONACO_BASE/min/vs/basic-languages/javascript/javascript.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/basic-languages/css/css.js" -o "$MONACO_BASE/min/vs/basic-languages/css/css.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/basic-languages/html/html.js" -o "$MONACO_BASE/min/vs/basic-languages/html/html.js"
# curl -L "https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/basic-languages/shell/shell.js" -o "$MONACO_BASE/min/vs/basic-languages/shell/shell.js"

# 确保文件权限正确
chmod -R 644 "$MONACO_BASE"
find "$MONACO_BASE" -type d -exec chmod 755 {} \;

cd "$INSTALL_DIR"

# 生成随机密码
PASSWORD=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 7 | head -n 1)
DEFAULT_USER="admin"

# 使用 md5sum 生成密码哈希（不包含换行符）
PASS_MD5=$(echo -n "${PASSWORD}" | md5sum | cut -d ' ' -f1)

# 创建配置文件
cat > "$INSTALL_DIR/config/config.yaml" << EOF
server:
  port: 8080
  host: 0.0.0.0

auth:
  username: "${DEFAULT_USER}"
  password: "${PASS_MD5}"
EOF

# 保存密码信息到文件（仅保存一次）
cat > "$INSTALL_DIR/password.txt" << EOF
用户名: ${DEFAULT_USER}
密码: ${PASSWORD}
MD5: ${PASS_MD5}
EOF
chmod 600 "$INSTALL_DIR/password.txt"

# 设置正确的权限
chmod -R 755 "$INSTALL_DIR"
chown -R root:root "$INSTALL_DIR"

# 删除临时目录
rm -rf "$TMP_DIR"

# 创建服务文件
cat > /etc/systemd/system/gegecp.service << EOF
[Unit]
Description=Linux Management Panel
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/panel
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

# 日志设置
StandardOutput=append:$INSTALL_DIR/log/panel.log
StandardError=append:$INSTALL_DIR/log/error.log
SyslogIdentifier=gegecp

[Install]
WantedBy=multi-user.target
EOF

# 确保二进制文件有执行权限
chmod +x "$INSTALL_DIR/panel"

# 创建日志目录和文件
mkdir -p "$INSTALL_DIR/log"
touch "$INSTALL_DIR/log/panel.log"
touch "$INSTALL_DIR/log/error.log"
chmod 755 "$INSTALL_DIR/log"
chmod 644 "$INSTALL_DIR/log/panel.log"
chmod 644 "$INSTALL_DIR/log/error.log"

# 重启服务
echo "正在启动服务..."
systemctl daemon-reload
systemctl stop gegecp 2>/dev/null || true
sleep 1
systemctl enable gegecp
systemctl start gegecp

# 检查服务状态
echo "检查服务状态..."
if ! systemctl is-active --quiet gegecp; then
    echo -e "${RED}服务启动失败，请检查日志文件：${NC}"
    echo "systemctl status gegecp"
    echo "tail -f /var/log/gegecp/error.log"
    exit 1
fi

# 等待服务完全启动
echo "等待服务启动..."
sleep 2

# 检查端口是否正常监听
if ! netstat -tuln | grep -q ':8080 '; then
    echo -e "${RED}服务端口 8080 未正常监听，请检查日志文件：${NC}"
    echo "tail -f /var/log/gegecp/error.log"
    exit 1
fi

# 配置防火墙
echo "配置防火墙规则..."
if command -v ufw &> /dev/null; then
    # Ubuntu 使用 ufw
    echo "检测到 ufw 防火墙..."
    # 确保 ufw 已安装
    apt-get install -y ufw
    # 启用 ufw
    echo "y" | ufw enable
    # 允许 SSH 和面板端口
    ufw allow ssh
    ufw allow 8080/tcp
    ufw reload
    echo "ufw 防火墙配置完成"
elif command -v firewall-cmd &> /dev/null; then
    # CentOS/RHEL 使用 firewalld
    echo "检测到 firewalld 防火墙..."
    systemctl start firewalld
    systemctl enable firewalld
    firewall-cmd --permanent --add-port=8080/tcp
    firewall-cmd --reload
    echo "firewalld 防火墙配置完成"
else
    # 如果没有检测到防火墙，安装并配置 ufw
    echo "未检测到防火墙，正在安装 ufw..."
    apt-get install -y ufw
    echo "y" | ufw enable
    ufw allow ssh
    ufw allow 8080/tcp
    ufw reload
    echo "ufw 防火墙安装和配置完成"
fi

echo -e "${GREEN}安装完成！${NC}"
echo "面板访问地址: http://your-server-ip:8080"
echo "默认用户名: ${DEFAULT_USER}"
echo "默认密码: ${PASSWORD}"
echo -e "\n${GREEN}重要提示：${NC}"
echo "1. 如果更新后页面没有变化，请按 Ctrl+F5 强制刷新页面"
echo "2. 或者清除浏览器缓存后重新访问"
echo "3. 请登录后立即修改默认密码"

# 删除重复的密码保存代码
# echo "用户名: ${DEFAULT_USER}" > "$INSTALL_DIR/password.txt"
# echo "密码: ${DEFAULT_PASS}" >> "$INSTALL_DIR/password.txt"
# chmod 600 "$INSTALL_DIR/password.txt" 