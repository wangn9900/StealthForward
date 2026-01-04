#!/bin/bash
# StealthForward License Server 安装脚本

set -e

# 配置
INSTALL_DIR="/opt/stealth-license"
SERVICE_NAME="stealth-license"
PORT="${PORT:-9000}"
ADMIN_TOKEN="${ADMIN_TOKEN:-$(openssl rand -hex 16)}"
LICENSE_SECRET="${LICENSE_SECRET:-$(openssl rand -hex 32)}"

echo "=========================================="
echo "  StealthForward License Server Installer"
echo "=========================================="

# 创建目录
mkdir -p $INSTALL_DIR

# 下载最新版本 (需要替换为实际的下载链接)
echo "📦 下载授权服务器..."
# 如果有预编译好的二进制，可以从 GitHub Release 下载
# wget -O $INSTALL_DIR/license-server https://github.com/xxx/releases/xxx
# chmod +x $INSTALL_DIR/license-server

# 或者从源码编译
if command -v go &> /dev/null; then
    echo "🔨 从源码编译..."
    cd /tmp
    git clone --depth 1 https://github.com/wangn9900/StealthForward.git sf-temp 2>/dev/null || true
    cd sf-temp/license-server
    go build -o $INSTALL_DIR/license-server .
    cd /
    rm -rf /tmp/sf-temp
else
    echo "❌ 未找到 Go 编译器，请先安装 Go 或使用预编译二进制"
    exit 1
fi

# 创建 systemd 服务
echo "⚙️ 创建系统服务..."
cat > /etc/systemd/system/$SERVICE_NAME.service << EOF
[Unit]
Description=StealthForward License Server
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/license-server
Environment=PORT=$PORT
Environment=ADMIN_TOKEN=$ADMIN_TOKEN
Environment=LICENSE_SECRET=$LICENSE_SECRET
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl restart $SERVICE_NAME

echo ""
echo "=========================================="
echo "  ✅ 安装完成！"
echo "=========================================="
echo ""
echo "📍 服务地址: http://$(curl -s ifconfig.me):$PORT"
echo "🔑 管理员Token: $ADMIN_TOKEN"
echo ""
echo "请保存上述信息！"
echo ""
echo "管理命令:"
echo "  查看状态: systemctl status $SERVICE_NAME"
echo "  查看日志: journalctl -u $SERVICE_NAME -f"
echo "  重启服务: systemctl restart $SERVICE_NAME"
