package main

import (
	"flag"
	"log"
	"time"

	"github.com/wangn9900/StealthForward/internal/agent"
)

func main() {
	// 1. 定义命令行参�?
	controllerAddr := flag.String("controller", "http://your-controller-ip:8080", "Controller API address")
	nodeID := flag.Int("node", 1, "Node ID for this agent")
	syncInterval := flag.Int("interval", 60, "Sync interval in seconds")
	configDir := flag.String("dir", "/etc/sing-box", "Directory to store config.json")
	wwwDir := flag.String("www", "/etc/stealthforward/www", "Directory to store masquerade site")
	singboxPath := flag.String("sbpath", "/usr/bin/sing-box", "Path to sing-box binary")
	fallbackPort := flag.Int("fallback-port", 8080, "Port for the local masquerade server")
	adminToken := flag.String("token", "", "Admin token for controller authentication")
	once := flag.Bool("once", false, "Run once and exit")

	flag.Parse()

	log.Printf("StealthForward Agent starting for Node ID: %d", *nodeID)
	log.Printf("Connecting to Controller: %s", *controllerAddr)

	// 2. 初始�?Agent
	ag := agent.NewAgent(agent.Config{
		ControllerAddr: *controllerAddr,
		NodeID:         *nodeID,
		LocalConfigDir: *configDir,
		MasqueradeDir:  *wwwDir,
		SingBoxPath:    *singboxPath,
		AdminToken:     *adminToken,
	})

	// 3. 启动本地伪装服务器（用于 SNI 回落目的地）
	ag.StartMasqueradeServer(*fallbackPort)

	// 如果指定�?-once，运行一次后退�?
	if *once {
		ag.RunOnce()
		return
	}

	// 4. 循环同步任务
	ticker := time.NewTicker(time.Duration(*syncInterval) * time.Second)
	defer ticker.Stop()

	// 启动时立即运行一�?
	ag.RunOnce()

	for range ticker.C {
		ag.RunOnce()
	}
}
