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
	log.Printf("[Provision] Starting automation for %s:%d (User: %s)...", cfg.Host, cfg.Port, cfg.User)

	// 1. 等待 SSH 端口就绪
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
		Timeout:         15 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	// 3. 执行初始化脚本矩阵 (使用 sudo 确保权限)
	log.Printf("[Provision] Executing BBR & System optimization...")
	setupScript := `
		sudo bash -c '
		echo "[$(date)] Starting configuration..." >> /var/log/stealth-init.log
		# BBR
		echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
		echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
		
		# TCP Keepalive (防止 SSH 3分钟断连)
		echo "net.ipv4.tcp_keepalive_time=60" >> /etc/sysctl.conf
		echo "net.ipv4.tcp_keepalive_intvl=15" >> /etc/sysctl.conf
		echo "net.ipv4.tcp_keepalive_probes=5" >> /etc/sysctl.conf
		
		sysctl -p >> /var/log/stealth-init.log 2>&1
		
		# RLimit
		echo "* soft nofile 65535" >> /etc/security/limits.conf
		echo "* hard nofile 65535" >> /etc/security/limits.conf
		echo "root soft nofile 65535" >> /etc/security/limits.conf
		echo "root hard nofile 65535" >> /etc/security/limits.conf
		
		# Acme.sh for SSL (确保 Agent 能申请证书)
		if [ ! -f "/root/.acme.sh/acme.sh" ]; then
			echo "[$(date)] Installing acme.sh..." >> /var/log/stealth-init.log
			curl https://get.acme.sh | sh >> /var/log/stealth-init.log 2>&1
		fi
		
		echo "[$(date)] System optimized." >> /var/log/stealth-init.log
		'
	`
	if err := runCommand(client, setupScript); err != nil {
		return fmt.Errorf("system optimization failed: %w", err)
	}

	// 4. 执行 Agent 安装 (带有 sudo 权限)
	log.Printf("[Provision] Executing Agent setup...")
	agentScript := fmt.Sprintf("sudo bash -c '%s'", cfg.AgentCmd)
	if err := runCommand(client, agentScript); err != nil {
		return fmt.Errorf("agent setup failed: %w", err)
	}

	log.Printf("[Provision] Success for %s!", cfg.Host)
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
