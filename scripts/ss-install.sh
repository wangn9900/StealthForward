#!/bin/bash
# StealthForward - 落地机专用 Shadowsocks 一键安装脚本 (隔离增强版)
# 设计初衷：与 V2bX/Xray 等后端完美共存，不冲突，不覆盖

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

echo -e "${BLUE}==================================================${PLAIN}"
echo -e "${BLUE}     StealthForward 落地机助手 (多后端共存版)     ${PLAIN}"
echo -e "${BLUE}==================================================${PLAIN}"

# 1. 选择加密方式
echo -e "请选择加密方式:"
echo -e "  ${GREEN}1)${PLAIN} chacha20-ietf-poly1305 (默认)"
echo -e "  ${GREEN}2)${PLAIN} 2022-blake3-aes-128-gcm"
echo -e "  ${GREEN}3)${PLAIN} aes-256-gcm"
read -p "请输入序号 [1-3, 默认 1]: " choice
case $choice in
    2) METHOD="2022-blake3-aes-128-gcm" ;;
    3) METHOD="aes-256-gcm" ;;
    *) METHOD="chacha20-ietf-poly1305" ;;
esac

# 2. 智能探测 Sing-box
SB_BIN="/usr/local/bin/sing-box"
if command -v sing-box &> /dev/null; then
    SB_BIN=$(command -v sing-box)
    echo -e "${GREEN}检测到系统中已存在 Sing-box 内核: $SB_BIN${PLAIN}"
    echo -e "${YELLOW}将直接复用现有内核，不会重复安装，确保不影响您的 V2bX 等业务。${PLAIN}"
else
    echo -e "${BLUE}未检测到 Sing-box，正在进行轻量化安装...${PLAIN}"
    bash <(curl -fsSL https://sing-box.app/install.sh)
fi

# 3. 隔离配置环境 (使用 stealth-ss 命名空间)
CONF_DIR="/etc/stealth-ss"
CONF_FILE="$CONF_DIR/config.json"
mkdir -p $CONF_DIR

# 4. 生成隔离参数
PORT=$((RANDOM % 10000 + 20000))
PASSWORD=$(openssl rand -base64 16)

# 5. 写入独立配置文件
cat > $CONF_FILE <<EOF
{
  "log": { "level": "error" },
  "inbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      "listen": "::",
      "listen_port": $PORT,
      "method": "$METHOD",
      "password": "$PASSWORD"
    }
  ],
  "outbounds": [{ "type": "direct", "tag": "direct" }]
}
EOF

# 6. 创建隔离的 Systemd 服务 (stealth-ss)
cat > /etc/systemd/system/stealth-ss.service <<EOF
[Unit]
Description=StealthForward SS Exit Service
After=network.target nss-lookup.target

[Service]
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
ExecStart=$SB_BIN run -c $CONF_FILE
Restart=on-failure
RestartSec=10
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target
EOF

# 7. 启动隔离服务
echo -e "${BLUE}正在启动隔离转发服务...${PLAIN}"
systemctl daemon-reload
systemctl enable stealth-ss
systemctl restart stealth-ss

# 8. 获取公网 IP
IP=$(curl -s -4 ifconfig.me || curl -s -4 api.ipify.org || echo "您的公网IP")

# 9. 打印结果
echo -e "\n${GREEN}==================================================${PLAIN}"
echo -e "${GREEN}🎉 落地机服务已启动 (已与 V2bX 隔离共存) ${PLAIN}"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "该服务独立运行，不会修改或停止您的原有 Sing-box/V2bX 配置。"
echo -e ""
echo -e "${BLUE}落地机地址:   ${PLAIN}$IP"
echo -e "${BLUE}落地机端口:   ${PLAIN}$PORT"
echo -e "${BLUE}加密方式:     ${PLAIN}$METHOD"
echo -e "${BLUE}连接密码:     ${PLAIN}$PASSWORD"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "💡 防火墙请放行单独的 $PORT 端口 (TCP/UDP)"
echo -e "${GREEN}==================================================${PLAIN}\n"
