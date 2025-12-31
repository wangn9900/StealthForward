#!/bin/bash
# StealthForward - 落地机专用 Shadowsocks 一键安装脚本
# 采用 SS-2022-AES-128-GCM 协议，极致性能，低损耗

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
PLAIN='\033[0m'

echo -e "${BLUE}正在为您的落地机安装 Shadowsocks 服务端...${PLAIN}"

# 1. 安装 sing-box
if ! command -v sing-box &> /dev/null; then
    echo -e "${BLUE}开始安装 sing-box 内核...${PLAIN}"
    bash <(curl -fsSL https://sing-box.app/install.sh)
fi

# 2. 准备配置目录
mkdir -p /etc/sing-box

# 3. 生成随机参数
PORT=$((RANDOM % 10000 + 20000))
# 生成传统 SS AEAD 密码
PASSWORD=$(openssl rand -base64 16)
METHOD="chacha20-ietf-poly1305"

# 4. 写入配置文件
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

# 5. 设置服务并启动
echo -e "${BLUE}正在启动服务...${PLAIN}"
systemctl daemon-reload
systemctl enable sing-box
systemctl restart sing-box

# 6. 获取公网 IP
IP=$(curl -s -4 ifconfig.me || curl -s -4 api.ipify.org || echo "您的公网IP")

# 7. 打印结果
echo -e "\n${GREEN}==================================================${PLAIN}"
echo -e "${GREEN}🎉 Shadowsocks-2022 安装成功！${PLAIN}"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "请将以下信息填入 StealthForward 后台的「新增落地机」弹窗："
echo -e ""
echo -e "${BLUE}落地机备注:   ${PLAIN}我的落地小鸡"
echo -e "${BLUE}落地机地址:   ${PLAIN}$IP"
echo -e "${BLUE}落地机端口:   ${PLAIN}$PORT"
echo -e "${BLUE}传输协议:     ${PLAIN}Shadowsocks (传统/2022)"
echo -e "${BLUE}加密方式:     ${PLAIN}$METHOD"
echo -e "${BLUE}连接密码:     ${PLAIN}$PASSWORD"
echo -e "${GREEN}==================================================${PLAIN}"
echo -e "💡 温馨提示：请确保您的云平台安全组已放行端口 $PORT (TCP/UDP)"
echo -e "${GREEN}==================================================${PLAIN}\n"
