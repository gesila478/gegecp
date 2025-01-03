#!/bin/bash

echo "开始编译..."
go build

echo "准备部署文件..."
# 创建临时目录
TEMP_DIR=$(mktemp -d)
mkdir -p "$TEMP_DIR/static"
mkdir -p "$TEMP_DIR/templates"

# 复制需要的文件到临时目录
cp gegecp "$TEMP_DIR/"
cp -r static/* "$TEMP_DIR/static/"
cp -r templates/* "$TEMP_DIR/templates/"

echo "开始传输文件..."
sshpass -p 'andyou' rsync -av \
    "$TEMP_DIR/" root@192.168.100.21:/opt/gegecp/

echo "清理临时文件..."
rm -rf "$TEMP_DIR"

echo "重启服务..."
sshpass -p 'andyou' ssh root@192.168.100.21 "systemctl restart gegecp"