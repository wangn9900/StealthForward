#!/bin/bash
# StealthForward - 落地机专用 Shadowsocks 一键安装脚本 (交互增强版)

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

echo -e "${BLUE}==================================================${PLAIN}"
echo -e "${BLUE}          StealthForward 落地机安装助手          ${PLAIN}"
echo -e "${BLUE}==================================================${PLAIN}"

# 1. 选择加密方式
echo -e "请选择加密方式 (推荐使用 chacha20):"
echo -e "  ${GREEN}1)${PLAIN} chacha20-ietf-poly1305 (移动端优选/兼容性强)"
echo -e "  ${GREEN}2)${PLAIN} 2022-blake3-aes-128-gcm (SS-2022 现代标准)"
echo -e "  ${GREEN}3)${PLAIN} aes-256-gcm (经典大厂方案/硬件加速)"
read -p "请输入序号 [1-3, 默认 1]: " choice

case $choice in
    2)
        METHOD="2022-blake3-aes-128-gcm"
        ;;
    3)
        METHOD="aes-256-gcm"
        ;;
    *)
        METHOD="chacha20-ietf-poly1305"
        ;;
esac

echo -e "已选择加密方式: ${YELLOW}$METHOD${PLAIN}"

# 2. 安装 sing-box
if ! command -v sing-box &> /dev/null; then
    echo -e "${BLUE}开始安装 sing-box 内核...${PLAIN}"
    bash <(curl -fsSL https://sing-box.app/install.sh)
fi

# 3. 准备配置目录
mkdir -p /etc/sing-box

# 4. 生成随机参数
PORT=$((RANDOM % 10000 + 20000))
PASSWORD=$(openssl rand -base64 16)

# 5. 写入配置文件
cat > /etc/sing-box/config.json <<EOF
{
  "log": {
    "level": "error"
  },
  "inbounds": [
    {
      "type": "shadowsocks",
      "tag": "ss-in",
      "listen": "::",
      "listen_port": $PORT,
      "method": "$METHOD",
      "password": "$PASSWORD",
      "multiplex": {
        "enabled": false
      }
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    }
  ]
}
EOF

# 6. 设置服务并启动
systemctl daemon-reload
systemctl enable sing-box
systemctl restart sing-box

# 7. 获取公网 IP
IP=$(curl -s -4 ifconfig.me || curl -s -4 api.ipify.org || echo "您的公网IP")

# 8. 打印结果
echo -e "\n${GREEN}==================================================${PLAIN}"
echo -e "${GREEN}🎉 Shadowsocks 服务端安装成功！${PLAIN}"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "请将以下信息填入 StealthForward 后台的「新增落地机」弹窗："
echo -e ""
echo -e "${BLUE}落地机地址:   ${PLAIN}$IP"
echo -e "${BLUE}落地机端口:   ${PLAIN}$PORT"
echo -e "${BLUE}传输协议:     ${PLAIN}Shadowsocks (传统/2022)"
echo -e "${BLUE}加密方式:     ${PLAIN}$METHOD"
echo -e "${BLUE}连接密码:     ${PLAIN}$PASSWORD"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "💡 温馨提示：请确保防火墙已放行端口 $PORT (TCP/UDP)"
echo -e "${GREEN}==================================================${PLAIN}\n"
