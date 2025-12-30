package main

import (
	"fmt"

	"github.com/nasstoki/stealthforward/internal/generator"
	"github.com/nasstoki/stealthforward/internal/models"
)

func main() {
	// 1. 模拟一个带 SNI 回落的入口节点 (参考用户截图证据)
	entry := &models.EntryNode{
		ID:          1,
		Name:        "US-Entry-Fallback",
		Port:        8443,
		Domain:      "orange-cloudcone.2233006.xyz",
		Certificate: "/etc/stealthforward/certs/cert.crt",
		Key:         "/etc/stealthforward/certs/cert.key",
		Fallback:    "127.0.0.1:8080", // 关键：探测流量回落到 Nginx
		Security:    "xtls-vision",
	}

	// 2. 模拟落地节点
	exit1 := models.ExitNode{
		ID:     101,
		Name:   "Malaysian-SS",
		Config: `{"type": "shadowsocks", "method": "aes-256-gcm", "password": "pass", "server": "1.2.3.4", "server_port": 8388}`,
	}

	// 3. 模拟逻辑转发规则
	rules := []models.ForwardingRule{
		{
			UserID:      "ed296cba-53cd-45cb-812b-ffe09e7d7410", // 截图中的 UUID
			UserEmail:   "alice@stealth.com",
			EntryNodeID: 1,
			ExitNodeID:  101,
		},
	}

	// 4. 生成配置
	config, err := generator.GenerateEntryConfig(entry, rules, []models.ExitNode{exit1})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("--- Generated Sing-box Config with SNI Fallback ---")
	fmt.Println(config)
}
