package main

import (
	"flag"
	"log"
	"time"

	"github.com/nasstoki/stealthforward/internal/agent"
)

func main() {
	// 1. 定义命令行参数
	controllerAddr := flag.String("controller", "http://your-controller-ip:8080", "Controller API address")
	nodeID := flag.Int("node", 1, "Node ID for this agent")
	syncInterval := flag.Int("interval", 60, "Sync interval in seconds")
	configDir := flag.String("dir", "/etc/stealthforward", "Directory to store config.json")
	wwwDir := flag.String("www", "/etc/stealthforward/www", "Directory to store masquerade site")
	singboxPath := flag.String("sbpath", "/usr/bin/sing-box", "Path to sing-box binary")
	fallbackPort := flag.Int("fallback-port", 8080, "Port for the local masquerade server")

	flag.Parse()

	log.Printf("StealthForward Agent starting for Node ID: %d", *nodeID)
	log.Printf("Connecting to Controller: %s", *controllerAddr)

	// 2. 初始化 Agent
	ag := agent.NewAgent(agent.Config{
		ControllerAddr: *controllerAddr,
		NodeID:         *nodeID,
		LocalConfigDir: *configDir,
		MasqueradeDir:  *wwwDir,
		SingBoxPath:    *singboxPath,
	})

	// 3. 启动本地伪装服务器（用于 SNI 回落目的地）
	ag.StartMasqueradeServer(*fallbackPort)

	// 4. 循环同步任务
	ticker := time.NewTicker(time.Duration(*syncInterval) * time.Second)
	defer ticker.Stop()

	// 启动时立即运行一次
	ag.RunOnce()

	for range ticker.C {
		ag.RunOnce()
	}
}
