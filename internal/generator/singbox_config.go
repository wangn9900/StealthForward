package generator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wangn9900/StealthForward/internal/models"
)

// SingBoxConfig 适配魔改内核 Tox/V2bX 的最简结构
type SingBoxConfig struct {
	Log       interface{}   `json:"log"`
	DNS       interface{}   `json:"dns,omitempty"`
	Inbounds  []interface{} `json:"inbounds"`
	Outbounds []interface{} `json:"outbounds"`
	Route     interface{}   `json:"route"`
}

func GenerateEntryConfig(entry *models.EntryNode, rules []models.ForwardingRule, exits []models.ExitNode) (string, error) {
	config := SingBoxConfig{
		Log: map[string]interface{}{
			"level": "error",
		},
		DNS: map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address": "1.1.1.1",
					"tag":     "cf",
					"detour":  "direct",
				},
			},
			"strategy": "prefer_ipv4",
		},
	}

	// 统一证书路径
	certPath := entry.Certificate
	if certPath == "" {
		certPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.crt"
	}
	keyPath := entry.Key
	if keyPath == "" {
		keyPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.key"
	}

	// 1. Inbound (VLESS) - 适配魔改内核的 Inbound Tag 分流
	inboundTag := fmt.Sprintf("node_%d", entry.ID) // 对齐魔改版 node_X 格式
	vlessInbound := map[string]interface{}{
		"type":                       "vless",
		"tag":                        inboundTag,
		"listen":                     "::",
		"listen_port":                entry.Port,
		"sniff":                      true,
		"sniff_override_destination": true,
	}

	// 回落配置
	fallbackHost := "127.0.0.1"
	fallbackPort := 80
	if entry.Fallback != "" {
		if strings.Contains(entry.Fallback, ":") {
			parts := strings.Split(entry.Fallback, ":")
			fallbackHost = parts[0]
			p, _ := strconv.Atoi(parts[1])
			fallbackPort = p
		} else {
			fallbackHost = entry.Fallback
		}
	}
	vlessInbound["fallback"] = map[string]interface{}{
		"server":      fallbackHost,
		"server_port": fallbackPort,
	}

	// 用户信息 (魔改内核依然需要 UUID 列表)
	users := []map[string]interface{}{}
	for _, rule := range rules {
		users = append(users, map[string]interface{}{
			"uuid": rule.UserID,
			"flow": "xtls-rprx-vision",
		})
	}
	vlessInbound["users"] = users

	vlessInbound["tls"] = map[string]interface{}{
		"enabled":          true,
		"server_name":      entry.Domain,
		"certificate_path": certPath,
		"key_path":         keyPath,
		"min_version":      "1.2",
	}
	config.Inbounds = append(config.Inbounds, vlessInbound)

	// 2. Outbounds
	config.Outbounds = append(config.Outbounds, map[string]interface{}{
		"tag":  "direct",
		"type": "direct",
	})

	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)

		// 适配 Shadowsocks 格式
		if exit.Protocol == "ss" {
			exitOutbound["type"] = "shadowsocks"
			if cipher, ok := exitOutbound["cipher"]; ok {
				exitOutbound["method"] = cipher
			}
			if port, ok := exitOutbound["port"]; ok {
				exitOutbound["server_port"] = port
			}
			if addr, ok := exitOutbound["address"]; ok {
				exitOutbound["server"] = addr
			}
			exitOutbound["tcp_fast_open"] = false
			exitOutbound["multiplex"] = map[string]interface{}{
				"enabled": false,
				"padding": true,
			}
		}

		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}
	config.Outbounds = append(config.Outbounds, map[string]interface{}{"tag": "block", "type": "block"})

	// 3. Routing Rules - 基于 Inbound Tag 进行分流
	routingRules := []interface{}{
		map[string]interface{}{"ip_cidr": []string{"127.0.0.1/32"}, "outbound": "direct"},
		map[string]interface{}{"protocol": "dns", "outbound": "direct"},
	}

	// 核心分流：根据入站标签甩到落地点
	for _, rule := range rules {
		for _, e := range exits {
			if e.ID == rule.ExitNodeID {
				routingRules = append(routingRules, map[string]interface{}{
					"inbound":  []string{inboundTag},
					"outbound": "out-" + e.Name,
				})
				break
			}
		}
	}

	config.Route = map[string]interface{}{
		"rules": routingRules,
		"final": "direct",
	}

	res, _ := json.MarshalIndent(config, "", "  ")
	return string(res), nil
}
