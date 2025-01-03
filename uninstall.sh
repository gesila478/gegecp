#!/bin/bash

# 设置颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}请使用 root 用户运行此脚本${NC}"
    exit 1
fi

echo -e "${GREEN}开始卸载 Linux 管理面板...${NC}"

# 停止并删除服务
echo "停止服务..."
systemctl stop linux-panel
systemctl disable linux-panel
rm -f /etc/systemd/system/linux-panel.service
systemctl daemon-reload

# 删除安装目录
echo "删除安装文件..."
rm -rf /opt/linux-panel

# 删除日志文件
echo "删除日志文件..."
rm -rf /var/log/linux-panel
rm -f /var/log/linux-panel-install.log

# 删除临时文件
echo "清理临时文件..."
rm -rf /tmp/tmp.* 2>/dev/null
rm -rf /tmp/linux-panel* 2>/dev/null

# 清理 Go 缓存和编译文件
echo "清理 Go 缓存..."
if command -v go &> /dev/null; then
    go clean -cache -modcache
fi
rm -rf ~/go/pkg/mod/linux-panel* 2>/dev/null

# 删除防火墙规则
echo "删除防火墙规则..."
if command -v ufw &> /dev/null; then
    # Ubuntu 使用 ufw
    ufw delete allow 8080/tcp
    ufw reload
elif command -v firewall-cmd &> /dev/null; then
    # CentOS/RHEL 使用 firewalld
    firewall-cmd --permanent --remove-port=8080/tcp
    firewall-cmd --reload
else
    # 使用 iptables
    iptables -D INPUT -p tcp --dport 8080 -j ACCEPT
    iptables-save > /etc/iptables/rules.v4 || true
fi

# 清理环境变量（如果有）
if grep -q "linux-panel" /etc/profile; then
    echo "清理环境变量..."
    sed -i '/linux-panel/d' /etc/profile
    source /etc/profile
fi

# 清理系统缓存
echo "清理系统缓存..."
sync
echo 3 > /proc/sys/vm/drop_caches

# 验证清理结果
echo "验证清理结果..."
if [ -d "/opt/linux-panel" ]; then
    echo -e "${RED}警告: 安装目录仍然存在${NC}"
fi

if [ -f "/etc/systemd/system/linux-panel.service" ]; then
    echo -e "${RED}警告: 服务文件仍然存在${NC}"
fi

if systemctl is-active linux-panel &>/dev/null; then
    echo -e "${RED}警告: 服务仍在运行${NC}"
fi

# 最后的清理确认
echo "执行最终清理..."
killall -9 panel 2>/dev/null || true
rm -rf /opt/linux-panel 2>/dev/null
systemctl reset-failed linux-panel 2>/dev/null

echo -e "${GREEN}卸载完成！${NC}"
echo "如果需要重新安装，请运行安装脚本" 