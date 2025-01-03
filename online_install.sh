#!/bin/bash

# 设置颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}开始安装 Linux 管理面板......${NC}"

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}请使用 root 用户运行此脚本${NC}"
    exit 1
fi

# 清理旧的安装
echo "清理旧的安装..."
systemctl stop linux-panel 2>/dev/null || true
systemctl disable linux-panel 2>/dev/null || true
rm -f /etc/systemd/system/linux-panel.service
rm -rf /opt/linux-panel
systemctl daemon-reload

# 下载安装脚本
echo "下载安装脚本..."
TMP_SCRIPT=$(mktemp)
curl -o "$TMP_SCRIPT" https://raw.githubusercontent.com/gesila478/Gegecp/refs/heads/main/install.sh

# 检查下载是否成功
if [ $? -ne 0 ]; then
    echo -e "${RED}下载安装脚本失败${NC}"
    rm -f "$TMP_SCRIPT"
    exit 1
fi

# 执行安装脚本
echo "执行安装脚本..."
bash "$TMP_SCRIPT"

# 清理临时文件
rm -f "$TMP_SCRIPT" 