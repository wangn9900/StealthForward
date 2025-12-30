#!/bin/bash

# StealthForward 一键安装脚本
# 支持 OS: Ubuntu, Debian, CentOS 7+

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# 检查权限
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}错误: 请使用 root 权限运行此脚本。${NC}"
  exit 1
fi

show_logo() {
  clear
  echo -e "${CYAN}"
  echo "  ____  _             _ _   _     _____                            _ "
  echo " / ___|| |_ ___  __ _| | |_| |__ |  ___|__  _ __ __      ____ _ _ __ __| |"
  echo " \___ \| __/ _ \/ _\` | | __| '_ \| |_ / _ \| '__\ \ /\ / / _\` | '__/ _\` |"
  echo "  ___) | ||  __/ (_| | | |_| | | |  _| (_) | |   \ V  V / (_| | | | (_| |"
  echo " |____/ \__\___|\__,_|_|\__|_| |_|_|  \___/|_|    \_/\_/ \__,_|_|  \__,_|"
  echo -e "${NC}"
  echo -e "${PURPLE}--- 隐形转发面板 (StealthForward) | 海外入口专属优化 ---${NC}"
  echo ""
}

# 核心变量
REPO="wangn9900/StealthForward"
INSTALL_DIR="/etc/stealthforward"
BIN_DIR="/usr/local/bin"

# 自动检测架构
ARCH=$(uname -m)
case $ARCH in
  x86_64)  PLATFORM="amd64" ;;
  aarch64) PLATFORM="arm64" ;;
  *)       echo -e "${RED}不支持的架构: $ARCH${NC}"; exit 1 ;;
esac

download_binary() {
  local name=$1
  local target_name=$2
  echo -e "${YELLOW}正在探测最新版本...${NC}"
  
  LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  
  if [ -z "$LATEST_TAG" ]; then
    echo -e "${RED}无法获取最新版本号，请检查网络。${NC}"
    exit 1
  fi

  echo -e "${YELLOW}正在下载 $name ($LATEST_TAG | $PLATFORM)...${NC}"
  URL="https://github.com/$REPO/releases/download/$LATEST_TAG/${name}-${PLATFORM}"
  
  curl -L -f -o "$BIN_DIR/$target_name" "$URL"
  
  if [ $? -eq 0 ]; then
    chmod +x "$BIN_DIR/$target_name"
    echo -e "${GREEN}$name 安装成功!${NC}"
  else
    echo -e "${RED}$name 下载失败!${NC}"
    exit 1
  fi
}

install_sing_box() {
  echo -e "${YELLOW}正在检查/安装 Sing-box 核心...${NC}"
  
  # 强制运行官方安装脚本以确保最新版
  bash <(curl -Ls https://raw.githubusercontent.com/SagerNet/sing-box/main/install.sh)
  
  # 如果官方脚本没成功，或者想确保最新，可以考虑在这里增加强制下载二进制逻辑
  # 但通常官方脚本是最稳的，关键是确保它运行了
  
  SB_PATH=$(which sing-box)
  if [ -z "$SB_PATH" ]; then
    SB_PATH="/usr/local/bin/sing-box"
  fi

  echo -e "${CYAN}当前 Sing-box 路径: $SB_PATH | 版本: $($SB_PATH version | head -n 1)${NC}"

  cat > /etc/systemd/system/sing-box.service <<EOF
[Unit]
Description=sing-box Service
After=network.target nss-lookup.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/sing-box
ExecStart=$SB_PATH run -c /etc/sing-box/config.json
Restart=on-failure
RestartSec=10
LimitNOFILE= infinity

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable sing-box
  echo -e "${GREEN}Sing-box 核心与服务安装完成！${NC}"
}

install_controller() {
  show_logo
  echo -e "${BLUE}开始安装 StealthForward Controller (中控端)...${NC}"
  
  systemctl stop stealth-controller 2>/dev/null
  
  mkdir -p $INSTALL_DIR/web
  download_binary "stealth-controller" "stealth-controller"
  download_binary "stealth-admin" "stealth-admin"

  echo -e "${YELLOW}正在同步可视化面板资源...${NC}"
  curl -L -f -o "$INSTALL_DIR/web/index.html" "https://raw.githubusercontent.com/$REPO/main/web/index.html"
  
  # 使用 'EOF' (带引号) 防止变量被提前解析
  cat > /etc/systemd/system/stealth-controller.service <<'EOF'
[Unit]
Description=StealthForward Controller Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/stealthforward
ExecStart=/usr/local/bin/stealth-controller
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable stealth-controller
  systemctl start stealth-controller
  echo -e "${GREEN}Controller 安装并启动成功！${NC}"
  echo -e "${CYAN}面板地址: http://你的公网IP:8080/dashboard${NC}"
}

install_agent() {
  echo -e "${YELLOW}正在安装 Nginx 及其依赖 (Sniproxy 方案必需)...${NC}"
  apt-get update && apt-get install -y nginx libnginx-mod-stream || yum install -y nginx nginx-mod-stream
  systemctl enable nginx
  systemctl start nginx

  install_sing_box
  
  show_logo
  echo -e "${BLUE}开始安装 StealthForward Agent (入口节点端)...${NC}"
  
  systemctl stop stealth-agent 2>/dev/null
  
  mkdir -p $INSTALL_DIR/www
  download_binary "stealth-agent" "stealth-agent"
  
  read -p "请输入 Controller API 地址 [http://127.0.0.1:8080]: " CTRL_ADDR
  CTRL_ADDR=${CTRL_ADDR:-http://127.0.0.1:8080}
  read -p "请输入当前节点 ID [1]: " NODE_ID
  NODE_ID=${NODE_ID:-1}
  read -p "请输入管理口令 (STEALTH_ADMIN_TOKEN) [留空则无需鉴权]: " CTRL_TOKEN

  cat > /etc/systemd/system/stealth-agent.service <<EOF
[Unit]
Description=StealthForward Agent Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=$BIN_DIR/stealth-agent -controller $CTRL_ADDR -node $NODE_ID -dir /etc/sing-box -www $INSTALL_DIR/www -token "$CTRL_TOKEN" -fallback-port 8081
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable stealth-agent
  systemctl start stealth-agent
  echo -e "${GREEN}Agent 已安装并在后台运行!${NC}"
}

main_menu() {
  show_logo
  echo -e "1. 安装 ${GREEN}Controller (中控端)${NC}"
  echo -e "2. 安装 ${GREEN}Agent (入口节点端)${NC}"
  echo -e "0. 退出"
  echo ""
  read -p "请选择 [0-2]: " choice

  case $choice in
    1) install_controller ;;
    2) install_agent ;;
    0) exit 0 ;;
    *) echo "无效选项" ; sleep 1 ; main_menu ;;
  esac
}

main_menu
