#!/bin/bash
# ============================================
#  StealthForward License Server 一键部署
#  运行: curl -fsSL https://你的域名/install-license.sh | bash
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
ADMIN_TOKEN=$(openssl rand -hex 16)
LICENSE_SECRET=$(openssl rand -hex 32)

# 检测系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo -e "${RED}不支持的架构: $ARCH${NC}"; exit 1 ;;
esac

echo -e "${YELLOW}[1/5] 安装依赖...${NC}"
apt-get update -qq
apt-get install -y -qq curl wget git golang-go > /dev/null 2>&1 || true

echo -e "${YELLOW}[2/5] 创建安装目录...${NC}"
mkdir -p $INSTALL_DIR
cd $INSTALL_DIR

echo -e "${YELLOW}[3/5] 下载并编译授权服务器...${NC}"
# 下载源码并编译
if [ -d "src" ]; then rm -rf src; fi
git clone --depth 1 https://github.com/$GITHUB_REPO.git src 2>/dev/null
cd src/license-server
go build -ldflags="-s -w" -o $INSTALL_DIR/license-server . 2>/dev/null
cd $INSTALL_DIR
rm -rf src

echo -e "${YELLOW}[4/5] 配置系统服务...${NC}"
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

echo -e "${YELLOW}[5/5] 启动服务...${NC}"
systemctl daemon-reload
systemctl enable $SERVICE_NAME > /dev/null 2>&1
systemctl restart $SERVICE_NAME

# 等待启动
sleep 2

# 获取公网IP
PUBLIC_IP=$(curl -s ifconfig.me 2>/dev/null || curl -s ipinfo.io/ip 2>/dev/null || echo "你的服务器IP")

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║               ✅ 安装完成！                                   ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${CYAN}📍 管理界面:${NC} http://${PUBLIC_IP}:${PORT}"
echo -e "${CYAN}🔑 管理员Token:${NC} ${ADMIN_TOKEN}"
echo ""
echo -e "${YELLOW}⚠️  请立即保存上述信息！Token只显示一次！${NC}"
echo ""
echo -e "${CYAN}常用命令:${NC}"
echo "  查看状态: systemctl status $SERVICE_NAME"
echo "  查看日志: journalctl -u $SERVICE_NAME -f"
echo "  重启服务: systemctl restart $SERVICE_NAME"
echo ""
