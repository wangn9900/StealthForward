#!/bin/bash
# ============================================
#  StealthForward License Server 一键部署
#  用法: curl -fsSL https://raw.githubusercontent.com/.../install.sh | bash
# ============================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
echo "╔══════════════════════════════════════════╗"
echo "║  StealthForward License Server 一键部署   ║"
echo "╚══════════════════════════════════════════╝"
echo -e "${NC}"

# 配置
INSTALL_DIR="/opt/stealth-license"
SERVICE_NAME="stealth-license"
GITHUB_REPO="wangn9900/StealthForward"
PORT=9000

# 生成随机密钥
ADMIN_TOKEN=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
LICENSE_SECRET=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 64 | head -n 1)

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo -e "${RED}不支持的架构: $ARCH${NC}"; exit 1 ;;
esac

echo -e "${YELLOW}[1/4] 创建安装目录...${NC}"
mkdir -p $INSTALL_DIR
cd $INSTALL_DIR

echo -e "${YELLOW}[2/4] 下载授权服务器...${NC}"
# 先尝试从Release下载预编译版本
LATEST_TAG=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v3.6.23"
fi

BINARY_URL="https://github.com/$GITHUB_REPO/releases/download/$LATEST_TAG/license-server-$ARCH"
echo -e "${CYAN}正在下载: $BINARY_URL${NC}"

# 尝试下载最新二进制包
echo -e "${CYAN}正在尝试下载最新 Release 版本: ${LATEST_TAG}${NC}"

if curl -L -f -o $INSTALL_DIR/license-server.new "$BINARY_URL" 2>/dev/null; then
    mv $INSTALL_DIR/license-server.new $INSTALL_DIR/license-server
    chmod +x $INSTALL_DIR/license-server
    echo -e "${GREEN}下载预编译版本成功！${NC}"
else
    echo -e "${YELLOW}预编译版本下载失败（可能Github Action还在构建中），自动切换为源码编译安装最新版...${NC}"
    
    # 检查Go是否安装
    if ! command -v go &> /dev/null; then
        echo -e "${YELLOW}正在安装 Go 编译器...${NC}"
        curl -L -o /tmp/go.tar.gz "https://go.dev/dl/go1.21.5.linux-$ARCH.tar.gz"
        rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
        rm /tmp/go.tar.gz
    fi
    
    # 下载源码并编译
    cd /tmp
    rm -rf StealthForward
    git clone --depth 1 https://github.com/$GITHUB_REPO.git StealthForward
    cd StealthForward/license-server
    go build -ldflags="-s -w" -o $INSTALL_DIR/license-server .
    cd /
    rm -rf /tmp/StealthForward
    echo -e "${GREEN}源码编译成功！${NC}"
fi

echo -e "${YELLOW}[3/4] 配置系统服务...${NC}"
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

echo -e "${YELLOW}[4/4] 启动服务...${NC}"
systemctl daemon-reload
systemctl enable $SERVICE_NAME > /dev/null 2>&1
systemctl restart $SERVICE_NAME

# 等待启动
sleep 2

# 检查服务状态
if systemctl is-active --quiet $SERVICE_NAME; then
    STATUS="${GREEN}运行中${NC}"
else
    STATUS="${RED}启动失败${NC}"
fi

# 获取公网IP
PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s ipinfo.io/ip 2>/dev/null || echo "你的服务器IP")

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║               ✅ 安装完成！                                   ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${CYAN}📍 管理界面:${NC} http://${PUBLIC_IP}:${PORT}"
echo -e "${CYAN}🔑 管理员Token:${NC} ${ADMIN_TOKEN}"
echo -e "${CYAN}📊 服务状态:${NC} $STATUS"
echo ""
echo -e "${YELLOW}⚠️  请立即保存上述 Token！只显示一次！${NC}"
echo ""
echo -e "${CYAN}常用命令:${NC}"
echo "  查看状态: systemctl status $SERVICE_NAME"
echo "  查看日志: journalctl -u $SERVICE_NAME -f"
echo "  重启服务: systemctl restart $SERVICE_NAME"
echo ""
