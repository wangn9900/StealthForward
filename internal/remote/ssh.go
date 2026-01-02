package remote

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// ProvisionConfig 定义了远程初始化的核心参数
type ProvisionConfig struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	AgentCmd   string
}

// RunProvisioning 执行全自动初始化流程 (BBR + RLimit + Agent)
func RunProvisioning(cfg ProvisionConfig) error {
	log.Printf("[Provision] Starting automation for %s:%d...", cfg.Host, cfg.Port)

	// 1. 等待 SSH 端口就绪 (轮询最多 5 分钟)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if err := waitSSHReady(addr, 5*time.Minute); err != nil {
		return fmt.Errorf("SSH port timeout: %w", err)
	}

	// 2. 建立 SSH 连接
	signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	// 3. 执行初始化脚本矩阵
	// 第一步：开启 BBR 加速 & 优化内核参数
	bbrScript := `
		echo "net.core.default_qdisc=fq" | sudo tee -a /etc/sysctl.conf
		echo "net.ipv4.tcp_congestion_control=bbr" | sudo tee -a /etc/sysctl.conf
		sudo sysctl -p
		echo "[Remote] BBR Enabled."
	`
	if err := runCommand(client, bbrScript); err != nil {
		log.Printf("[Provision] BBR setup warning: %v", err)
	}

	// 第二步：系统优化 (RLimit)
	limitScript := `
		echo "* soft nofile 65535" | sudo tee -a /etc/security/limits.conf
		echo "* hard nofile 65535" | sudo tee -a /etc/security/limits.conf
		echo "root soft nofile 65535" | sudo tee -a /etc/security/limits.conf
		echo "root hard nofile 65535" | sudo tee -a /etc/security/limits.conf
		echo "[Remote] RLimit set to 65535."
	`
	if err := runCommand(client, limitScript); err != nil {
		log.Printf("[Provision] RLimit setup warning: %v", err)
	}

	// 第三步：静默安装 Agent
	log.Printf("[Provision] Executing Agent setup command...")
	if err := runCommand(client, cfg.AgentCmd); err != nil {
		return fmt.Errorf("agent setup failed: %w", err)
	}

	log.Printf("[Provision] All tasks completed for %s!", cfg.Host)
	return nil
}

func waitSSHReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for port %s", addr)
}

func runCommand(client *ssh.Client, script string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(script); err != nil {
		return fmt.Errorf("cmd failed: %s | err: %v | stderr: %s", script, err, stderr.String())
	}
	return nil
}
