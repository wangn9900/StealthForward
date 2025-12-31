package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/wangn9900/StealthForward/internal/agent"
)

func main() {
	// 1. 定义命令行参数
	controllerAddr := flag.String("controller", "http://your-controller-ip:8080", "Controller API address")
	nodeID := flag.Int("node", 1, "Node ID for this agent")
	syncInterval := flag.Int("interval", 60, "Sync interval in seconds")
	configDir := flag.String("dir", "/etc/sing-box", "Directory to store config.json")
	wwwDir := flag.String("www", "/etc/stealthforward/www", "Directory to store masquerade site")
	singboxPath := flag.String("sbpath", "/usr/bin/sing-box", "Path to sing-box binary")
	fallbackPort := flag.Int("fallback-port", 8080, "Port for the local masquerade server")
	adminToken := flag.String("token", "", "Admin token for controller authentication")
	useInternal := flag.Bool("internal", true, "Use internal sing-box core for accurate traffic stats")
	once := flag.Bool("once", false, "Run once and exit")

	flag.Parse()

	// 智能探测 Sing-box 路径
	if _, err := os.Stat(*singboxPath); os.IsNotExist(err) {
		candidates := []string{"/usr/local/bin/sing-box", "/usr/bin/sing-box"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				*singboxPath = c
				break
			}
		}
		// 如果还是没找到，尝试从 PATH 环境找
		if lp, err := exec.LookPath("sing-box"); err == nil {
			*singboxPath = lp
		}
	}

	log.Printf("StealthForward Agent starting for Node ID: %d", *nodeID)
	log.Printf("Sing-box path: %s", *singboxPath)

	// 2. 初始化 Agent
	ag := agent.NewAgent(agent.Config{
		ControllerAddr: *controllerAddr,
		NodeID:         *nodeID,
		LocalConfigDir: *configDir,
		MasqueradeDir:  *wwwDir,
		SingBoxPath:    *singboxPath,
		AdminToken:     *adminToken,
		UseInternal:    *useInternal,
	})

	// 3. 启动本地伪装服务器（用于 SNI 回落目的地）
	ag.StartMasqueradeServer(*fallbackPort)

	// 如果指定了 -once，运行一次后退出
	if *once {
		ag.RunOnce()
		return
	}

	// 4. 循环同步任务
	ticker := time.NewTicker(time.Duration(*syncInterval) * time.Second)
	defer ticker.Stop()

	// 启动时立即运行一次
	ag.RunOnce()

	for range ticker.C {
		ag.RunOnce()
	}
}
