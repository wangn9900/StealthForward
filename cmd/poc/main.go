package main

import (
	"fmt"

	"github.com/wangn9900/StealthForward/internal/generator"
	"github.com/wangn9900/StealthForward/internal/models"
)

func main() {
	// 1. 妯℃嫙涓€涓甫 SNI 鍥炶惤鐨勫叆鍙ｈ妭鐐?(鍙傝€冪敤鎴锋埅鍥捐瘉鎹?
	entry := &models.EntryNode{
		ID:          1,
		Name:        "US-Entry-Fallback",
		Port:        8443,
		Domain:      "orange-cloudcone.2233006.xyz",
		Certificate: "/etc/stealthforward/certs/cert.crt",
		Key:         "/etc/stealthforward/certs/cert.key",
		Fallback:    "127.0.0.1:8080", // 鍏抽敭锛氭帰娴嬫祦閲忓洖钀藉埌 Nginx
		Security:    "xtls-vision",
	}

	// 2. 妯℃嫙钀藉湴鑺傜偣
	exit1 := models.ExitNode{
		ID:     101,
		Name:   "Malaysian-SS",
		Config: `{"type": "shadowsocks", "method": "aes-256-gcm", "password": "pass", "server": "1.2.3.4", "server_port": 8388}`,
	}

	// 3. 妯℃嫙閫昏緫杞彂瑙勫垯
	rules := []models.ForwardingRule{
		{
			UserID:      "ed296cba-53cd-45cb-812b-ffe09e7d7410", // 鎴浘涓殑 UUID
			UserEmail:   "alice@stealth.com",
			EntryNodeID: 1,
			ExitNodeID:  101,
		},
	}

	// 4. 鐢熸垚閰嶇疆
	config, err := generator.GenerateEntryConfig(entry, rules, []models.ExitNode{exit1})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("--- Generated Sing-box Config with SNI Fallback ---")
	fmt.Println(config)
}
