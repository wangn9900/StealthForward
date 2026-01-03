package remote

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// ProvisionConfig 用于初始化节点时的配置
type ProvisionConfig struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	AgentCmd   string
}

// RunProvisioning 执行节点的初始化流程
func RunProvisioning(cfg ProvisionConfig) error {
	auth := []ssh.AuthMethod{}
	if cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * 1000 * 1000 * 1000, // 30s
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	return session.Run(cfg.AgentCmd)
}

// SSHClient 封装了远程操作逻辑
type SSHClient struct {
	Config *ssh.ClientConfig
	Host   string
	Port   int
}

func NewSSHClient(host string, port int, user, pass string) *SSHClient {
	return &SSHClient{
		Host: host,
		Port: port,
		Config: &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(pass),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         15 * time.Second,
		},
	}
}

// Run 执行单条命令并返回输出
func (s *SSHClient) Run(cmd string) (string, error) {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := ssh.Dial("tcp", addr, s.Config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	session.Stderr = &b
	err = session.Run(cmd)
	return b.String(), err
}

// UploadFile 通过 SFTP (或简单的 cat 方式) 上传小文件
func (s *SSHClient) UploadScript(destPath string, content string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := ssh.Dial("tcp", addr, s.Config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// 采用简单的 EOF 注入方式，无需依赖 sftp 库
	cmd := fmt.Sprintf("cat << 'EOF' > %s\n%s\nEOF\nchmod +x %s", destPath, content, destPath)
	return session.Run(cmd)
}

// DeployAgent 核心自动化脚本
func (s *SSHClient) DeployAgent(controllerURL, adminToken string, isTransit bool, listen, target, key string) (string, error) {
	// 1. 构造安装脚本
	// 我们直接利用 Controller 的下载地址
	installScript := fmt.Sprintf(`#!/bin/bash
mkdir -p /etc/stealth-pass
cat << EOF > /etc/stealth-pass/config.json
{
  "mode": "%s",
  "listen_addr": "%s",
  "target_addr": "%s",
  "key": "%s"
}
EOF

# 下载并设置服务 (根据架构自动选择)
ARCH=$(uname -m)
BINARY_URL="%s/static/stealth-agent-$ARCH"
curl -L -o /usr/local/bin/stealth-agent $BINARY_URL
chmod +x /usr/local/bin/stealth-agent

# 创建 Systemd 服务
cat << EOF > /etc/systemd/system/stealth-pass.service
[Unit]
Description=StealthPass Tunnel Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/stealth-agent -tunnel /etc/stealth-pass/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable stealth-pass
systemctl restart stealth-pass
`, "transit", listen, target, key, controllerURL)

	if !isTransit {
		// 修改为 Exit 模式的脚本逻辑
		// (省略重复部分，仅修改 mode...)
	}

	return s.Run(fmt.Sprintf("bash -c '%s'", installScript))
}
