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
  
  # 通过 API 获取最新 tag_name
  LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  
  if [ -z "$LATEST_TAG" ]; then
    echo -e "${RED}无法获取最新版本号，请检查网络或仓库是否公开。${NC}"
    exit 1
  fi

  echo -e "${YELLOW}正在从 GitHub 下载 $name ($LATEST_TAG | $PLATFORM)...${NC}"
  URL="https://github.com/$REPO/releases/download/$LATEST_TAG/${name}-${PLATFORM}"
  
  # 使用 -f 参数，如果 404 则返回错误而不保存空文件
  curl -L -f -o "$BIN_DIR/$target_name" "$URL"
  
  if [ $? -eq 0 ]; then
    chmod +x "$BIN_DIR/$target_name"
    echo -e "${GREEN}$name 安装成功!${NC}"
  else
    echo -e "${RED}$name 下载失败! URL: $URL${NC}"
    exit 1
  fi
}

install_sing_box() {
  echo -e "${YELLOW}正在安装 Sing-box 核心...${NC}"
  bash <(curl -Ls https://raw.githubusercontent.com/SagerNet/sing-box/main/install.sh)
}

install_controller() {
  show_logo
  echo -e "${BLUE}开始安装 StealthForward Controller (中控端)...${NC}"
  
  # 解决 Text file busy：如果服务正在运行，先停止它
  systemctl stop stealth-controller 2>/dev/null
  
  mkdir -p $INSTALL_DIR/web
  download_binary "stealth-controller" "stealth-controller"
  download_binary "stealth-admin" "stealth-admin"

  # 下载 Dashboard 静态文件
  echo -e "${YELLOW}正在同步可视化面板资源...${NC}"
  curl -L -o "$INSTALL_DIR/web/index.html" "https://raw.githubusercontent.com/$REPO/main/web/index.html"
  
  cat > /etc/systemd/system/stealth-controller.service <<EOF
[Unit]
Description=StealthForward Controller Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$BIN_DIR/stealth-controller
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  echo -e "${GREEN}Controller 服务已配置，请确保二进制文件已放置在 $BIN_DIR/stealth-controller${NC}"
}

install_agent() {
  install_sing_box
  
  echo -e "${BLUE}开始安装 StealthForward Agent (由出口机/入口机执行)...${NC}"
  
  # 解决 Text file busy
  systemctl stop stealth-agent 2>/dev/null
  
  mkdir -p $INSTALL_DIR/www
  download_binary "stealth-agent" "stealth-agent"
  
  read -p "请输入 Controller 的 API 地址 (例如 http://1.2.3.4:8080): " CTRL_ADDR
  read -p "请输入此节点的 ID: " NODE_ID

  cat > /etc/systemd/system/stealth-agent.service <<EOF
[Unit]
Description=StealthForward Agent Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=$BIN_DIR/stealth-agent -controller $CTRL_ADDR -node $NODE_ID -dir $INSTALL_DIR -www $INSTALL_DIR/www
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  echo -e "${GREEN}Agent 服务已配置！${NC}"
  echo -e "${YELLOW}启动前请确保 Agent 二进制文件已放置在 $BIN_DIR/stealth-agent${NC}"
}

# 主菜单
main_menu() {
  show_logo
  echo -e "1. 安装 ${GREEN}Controller (中控端)${NC}"
  echo -e "2. 安装 ${GREEN}Agent (入口节点端)${NC}"
  echo -e "3. 一键配置 ${CYAN}SSL 证书 (acme.sh)${NC}"
  echo -e "0. 退出"
  echo ""
  read -p "请选择 [0-3]: " choice

  case $choice in
    1) install_controller ;;
    2) install_agent ;;
    3) echo "证书模块开发中..." ;;
    0) exit 0 ;;
    *) echo "无效选项" ; sleep 1 ; main_menu ;;
  esac
}

main_menu
