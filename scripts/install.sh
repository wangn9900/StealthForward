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
  echo -e "${YELLOW}正在安装 Sing-box 魔改版 (支持 VLESS Fallback)...${NC}"
  
  # 先停止服务，避免文件被占用
  systemctl stop sing-box 2>/dev/null
  
  # 从 StealthForward Release 下载魔改版 sing-box
  SB_URL="https://github.com/$REPO/releases/latest/download/sing-box-mod-$ARCH"
  SB_PATH="/usr/local/bin/sing-box"
  
  echo -e "${CYAN}正在下载魔改版 Sing-box...${NC}"
  curl -Lo "$SB_PATH" "$SB_URL"
  
  # 关键：验证下载是否成功（文件至少 10MB）
  MIN_SIZE=$((10 * 1024 * 1024))  # 10MB
  if [ -f "$SB_PATH" ]; then
    FILE_SIZE=$(stat -c%s "$SB_PATH" 2>/dev/null || stat -f%z "$SB_PATH" 2>/dev/null || echo 0)
    if [ "$FILE_SIZE" -lt "$MIN_SIZE" ]; then
      echo -e "${RED}错误: Sing-box 下载失败或文件损坏 (大小: ${FILE_SIZE} 字节)!${NC}"
      echo -e "${YELLOW}可能原因: 新版本正在编译中，请稍后重试或手动下载。${NC}"
      echo -e "${CYAN}手动下载命令: curl -Lo /usr/local/bin/sing-box https://github.com/$REPO/releases/download/v1.3.20/sing-box-mod-$ARCH${NC}"
      rm -f "$SB_PATH"
      exit 1
    fi
  else
    echo -e "${RED}Sing-box 下载失败!${NC}"
    exit 1
  fi
  
  chmod +x "$SB_PATH"
  echo -e "${GREEN}魔改版 Sing-box 安装成功!${NC}"
  echo -e "${CYAN}版本: $($SB_PATH version 2>/dev/null | head -n 1 || echo '魔改版')${NC}"

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
  # 1. 安装 Nginx（用于托管伪装页和申请证书）
  if command -v nginx &> /dev/null; then
    echo -e "${GREEN}检测到 Nginx 已安装，跳过安装步骤。${NC}"
  else
    echo -e "${YELLOW}正在安装 Nginx (用于伪装页和证书申请)...${NC}"
    if command -v apt-get &> /dev/null; then
      apt-get update && apt-get install -y nginx
    elif command -v yum &> /dev/null; then
      yum install -y nginx
    fi
  fi
  systemctl enable nginx
  systemctl start nginx
  
  # 2. 安装魔改版 Sing-box
  install_sing_box
  
  show_logo
  echo -e "${BLUE}开始安装 StealthForward Agent (入口节点端)...${NC}"
  
  systemctl stop stealth-agent 2>/dev/null
  
  mkdir -p $INSTALL_DIR/www
  download_binary "stealth-agent" "stealth-agent"
  
  # 3. 生成并部署伪装页到 Nginx
  echo -e "${CYAN}正在部署伪装页到 Nginx...${NC}"
  if [ -d "/var/www/html" ]; then
    # 如果 Agent 已生成伪装页，复制过去
    if [ -f "$INSTALL_DIR/www/index.html" ]; then
      cp "$INSTALL_DIR/www/index.html" /var/www/html/index.html
    fi
  fi
  
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

uninstall_controller() {
  echo -e "${RED}正在卸载 StealthForward Controller...${NC}"
  systemctl stop stealth-controller 2>/dev/null
  systemctl disable stealth-controller 2>/dev/null
  rm -f /etc/systemd/system/stealth-controller.service
  systemctl daemon-reload
  
  rm -f $BIN_DIR/stealth-controller
  rm -f $BIN_DIR/stealth-admin
  # 注意：这里我们询问是否保留数据库
  read -p "是否删除所有数据库和配置数据(不可逆)? [y/N]: " del_data
  if [[ "$del_data" =~ ^[Yy]$ ]]; then
    rm -rf $INSTALL_DIR
    echo -e "${YELLOW}已清除所有配置数据和 SQLite 数据库。${NC}"
  fi
  
  echo -e "${GREEN}Controller 卸载完成！${NC}"
}

uninstall_agent() {
  echo -e "${RED}正在卸载 StealthForward Agent 及相关组件...${NC}"
  
  # 1. 停止并删除 Agent 服务
  systemctl stop stealth-agent 2>/dev/null
  systemctl disable stealth-agent 2>/dev/null
  rm -f /etc/systemd/system/stealth-agent.service
  rm -f $BIN_DIR/stealth-agent
  
  # 2. 停止并删除 Sing-box
  echo -e "${YELLOW}清理 Sing-box 核心...${NC}"
  systemctl stop sing-box 2>/dev/null
  systemctl disable sing-box 2>/dev/null
  rm -f /etc/systemd/system/sing-box.service
  rm -f /usr/local/bin/sing-box
  rm -rf /etc/sing-box
  
  # 3. 清理 Nginx 和伪装网站
  read -p "是否卸载 Nginx 并清除伪装网站数据? [y/N]: " del_nginx
  if [[ "$del_nginx" =~ ^[Yy]$ ]]; then
    systemctl stop nginx 2>/dev/null
    if command -v apt-get &> /dev/null; then
      apt-get purge -y nginx nginx-common
      apt-get autoremove -y
    elif command -v yum &> /dev/null; then
      yum remove -y nginx
    fi
    rm -rf /var/www/html/*
    echo -e "${YELLOW}Nginx 及伪装页已清除。${NC}"
  fi
  
  # 4. 清理主目录
  rm -rf $INSTALL_DIR
  systemctl daemon-reload
  
  echo -e "${GREEN}Agent 及其关联组件已彻底清除！${NC}"
}

main_menu() {
  show_logo
  echo -e "1. 安装 ${GREEN}Controller (中控端)${NC}"
  echo -e "2. 安装 ${GREEN}Agent (入口节点端)${NC}"
  echo -e "--------------------------------"
  echo -e "3. ${RED}卸载 Controller${NC}"
  echo -e "4. ${RED}卸载 Agent (包含清理 Sing-box/Nginx)${NC}"
  echo -e "--------------------------------"
  echo -e "0. 退出"
  echo ""
  read -p "请选择 [0-4]: " choice

  case $choice in
    1) install_controller ;;
    2) install_agent ;;
    3) uninstall_controller ;;
    4) uninstall_agent ;;
    0) exit 0 ;;
    *) echo "无效选项" ; sleep 1 ; main_menu ;;
  esac
}

main_menu
