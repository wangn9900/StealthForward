package generator

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/wangn9900/StealthForward/internal/models"
)

// SingBoxConfig 定义了简化的 sing-box 配置文件结构
type SingBoxConfig struct {
	Log       interface{}   `json:"log"`
	DNS       interface{}   `json:"dns,omitempty"`
	Inbounds  []interface{} `json:"inbounds"`
	Outbounds []interface{} `json:"outbounds"`
	Route     interface{}   `json:"route"`
}

// GenerateEntryConfig 生成入口节点的 Sing-box 配置
func GenerateEntryConfig(entry *models.EntryNode, rules []models.ForwardingRule, exits []models.ExitNode) (string, error) {
	config := SingBoxConfig{
		Log: map[string]interface{}{
			"level": "debug",
		},
		DNS: map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address": "1.1.1.1",
					"tag":     "cf",
					"detour":  "direct",
				},
				map[string]interface{}{
					"address": "8.8.8.8",
					"tag":     "google",
					"detour":  "direct",
				},
			},
			"strategy": "prefer_ipv4",
		},
	}

	// 1. 构建 Inbound (VLESS + Vision)
	// 恢复 fallback 字段：即便官方原版不支持，如果有自研内核（如 V2bX 的）则能识别
	// 如果是官方内核，我们通过下面的逻辑确保它不会 Crash
	vlessInbound := map[string]interface{}{
		"type":                       "vless",
		"tag":                        "vless-in",
		"listen":                     "::",
		"listen_port":                entry.Port,
		"sniff":                      true,
		"sniff_override_destination": true,
	}

	// 注入“自研特色”回落配置
	fallbackHost := "127.0.0.1"
	fallbackPort := 80 // Nginx 托管伪装页
	if entry.Fallback != "" {
		parts := strings.Split(entry.Fallback, ":")
		if len(parts) == 2 {
			fallbackHost = parts[0]
			p, _ := strconv.Atoi(parts[1])
			fallbackPort = p
		} else {
			fallbackHost = entry.Fallback
		}
	}

	// 注意：这里使用单数 fallback，这是 V2bX 自研内核的特征标识
	vlessInbound["fallback"] = map[string]interface{}{
		"server":      fallbackHost,
		"server_port": fallbackPort,
	}

	users := []map[string]interface{}{}
	for _, rule := range rules {
		users = append(users, map[string]interface{}{
			"uuid": rule.UserID,
			"name": rule.UserEmail,
			"flow": "xtls-rprx-vision",
		})
	}
	vlessInbound["users"] = users

	vlessInbound["tls"] = map[string]interface{}{
		"enabled":          true,
		"server_name":      entry.Domain,
		"certificate_path": entry.Certificate,
		"key_path":         entry.Key,
		"min_version":      "1.2",
		"alpn":             []string{"http/1.1", "h2"},
	}
	config.Inbounds = append(config.Inbounds, vlessInbound)

	// 2. 构建 Outbounds
	config.Outbounds = append(config.Outbounds, map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	})

	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)

		sbType := exit.Protocol
		if sbType == "ss" {
			sbType = "shadowsocks"
			exitOutbound["tcp_fast_open"] = false
			if _, ok := exitOutbound["multiplex"]; !ok {
				exitOutbound["multiplex"] = map[string]interface{}{
					"enabled": false,
					"padding": true,
				}
			}
		}

		if port, ok := exitOutbound["port"]; ok {
			exitOutbound["server_port"] = port
		}
		if addr, ok := exitOutbound["address"]; ok {
			exitOutbound["server"] = addr
		}
		if cipher, ok := exitOutbound["cipher"]; ok {
			exitOutbound["method"] = cipher
		}
		if pwd, ok := exitOutbound["password"]; ok {
			exitOutbound["password"] = pwd
		}
		if uuid, ok := exitOutbound["uuid"]; ok {
			exitOutbound["uuid"] = uuid
		}

		exitOutbound["type"] = sbType
		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	config.Outbounds = append(config.Outbounds, map[string]interface{}{
		"type": "block",
		"tag":  "block",
	})

	// 3. 构建 Routing
	rulesList := []interface{}{
		// 关键：来源是 127.0.0.1 的流量（回落流量）直接走 direct，不转发
		map[string]interface{}{
			"source_ip_cidr": []string{"127.0.0.1/32", "::1/128"},
			"outbound":       "direct",
		},
		map[string]interface{}{
			"ip_cidr":  []string{"127.0.0.1/32", "::1/128"},
			"outbound": "direct",
		},
		map[string]interface{}{
			"protocol": "dns",
			"outbound": "direct",
		},
	}

	var defaultExitName string
	if entry.TargetExitID != 0 {
		for _, e := range exits {
			if e.ID == entry.TargetExitID {
				defaultExitName = "out-" + e.Name
				break
			}
		}
	}

	for _, rule := range rules {
		var targetExitName string
		for _, e := range exits {
			if e.ID == rule.ExitNodeID {
				targetExitName = "out-" + e.Name
				break
			}
		}
		if targetExitName != "" {
			rulesList = append(rulesList, map[string]interface{}{
				"user":     []string{rule.UserEmail},
				"outbound": targetExitName,
			})
		}
	}

	routeConfig := map[string]interface{}{
		"rules": rulesList,
		"final": "direct",
	}
	if defaultExitName != "" {
		routeConfig["final"] = defaultExitName
	}
	config.Route = routeConfig

	res, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(res), nil
}
