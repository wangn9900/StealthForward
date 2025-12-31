#!/bin/bash
# StealthForward - 落地机专用 Shadowsocks 一键安装脚本 (全能版)
# 支持自定义端口、NAT 映射、多后端共存

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

function install_ss() {
    echo -e "${BLUE}==================================================${PLAIN}"
    echo -e "${BLUE}     StealthForward 落地机助手 (全能版)           ${PLAIN}"
    echo -e "${BLUE}==================================================${PLAIN}"

    # 1. 选择加密方式
    echo -e "1. 选择加密方式:"
    echo -e "  ${GREEN}1)${PLAIN} chacha20-ietf-poly1305 (默认)"
    echo -e "  ${GREEN}2)${PLAIN} 2022-blake3-aes-128-gcm"
    echo -e "  ${GREEN}3)${PLAIN} aes-256-gcm"
    read -p "请输入序号 [1-3, 默认 1]: " choice
    case $choice in
        2) METHOD="2022-blake3-aes-128-gcm" ;;
        3) METHOD="aes-256-gcm" ;;
        *) METHOD="chacha20-ietf-poly1305" ;;
    esac

    # 2. 自定义端口
    RANDOM_PORT=$((RANDOM % 10000 + 20000))
    echo -e "\n2. 配置监听端口 (NAT 机器请填写内网转发端口):"
    read -p "请输入端口 [默认 $RANDOM_PORT]: " PORT
    [ -z "$PORT" ] && PORT=$RANDOM_PORT

    # 3. 智能探测 Sing-box
    SB_BIN="/usr/local/bin/sing-box"
    if command -v sing-box &> /dev/null; then
        SB_BIN=$(command -v sing-box)
        echo -e "${GREEN}检测到系统中已存在 Sing-box 内核: $SB_BIN${PLAIN}"
    else
        echo -e "${BLUE}未检测到 Sing-box，正在进行轻量化安装...${PLAIN}"
        bash <(curl -fsSL https://sing-box.app/install.sh)
    fi

    # 4. 隔离配置环境
    CONF_DIR="/etc/stealth-ss"
    CONF_FILE="$CONF_DIR/config.json"
    mkdir -p $CONF_DIR

    # 5. 生成密钥
    PASSWORD=$(openssl rand -base64 16)

    # 6. 写入独立配置文件
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

    # 7. 创建并启动隔离服务
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

    systemctl daemon-reload
    systemctl enable stealth-ss
    systemctl restart stealth-ss

    # 8. 获取公网 IP
    IP=$(curl -s -4 ifconfig.me || curl -s -4 api.ipify.org || echo "您的公网IP")

    echo -e "\n${GREEN}==================================================${PLAIN}"
    echo -e "${GREEN}🎉 落地机服务已启动 (隔离共存模式) ${PLAIN}"
    echo -e "${GREEN}==================================================${PLAIN}"
    echo -e "${BLUE}落地机地址:   ${PLAIN}$IP"
    echo -e "${BLUE}内网监听端口: ${PLAIN}$PORT"
    echo -e "${BLUE}加密方式:     ${PLAIN}$METHOD"
    echo -e "${BLUE}连接密码:     ${PLAIN}$PASSWORD"
    echo -e "${GREEN}==================================================${PLAIN}"
    echo -e "${YELLOW}NAT 机器提醒：请确保已在服务商后台将公网端口映射至内网端 $PORT${PLAIN}"
    echo -e "${GREEN}==================================================${PLAIN}\n"
}

function uninstall_ss() {
    echo -e "${RED}正在卸载 StealthForward SS 落地服务...${PLAIN}"
    systemctl stop stealth-ss || true
    systemctl disable stealth-ss || true
    rm -f /etc/systemd/system/stealth-ss.service
    rm -rf /etc/stealth-ss
    systemctl daemon-reload
    echo -e "${GREEN}卸载完成！${PLAIN}"
}

# 脚本入口
case "$1" in
    uninstall)
        uninstall_ss
        ;;
    *)
        install_ss
        ;;
esac
