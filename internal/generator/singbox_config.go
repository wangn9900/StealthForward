package generator

import (
	"encoding/json"

	"github.com/nasstoki/stealthforward/internal/models"
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
	vlessInbound := map[string]interface{}{
		"type":                       "vless",
		"tag":                        "vless-in",
		"listen":                     "::",
		"listen_port":                entry.Port,
		"sniff":                      true,
		"sniff_override_destination": true,
		"users":                      []interface{}{},
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
	// 按照用户图示顺序：direct -> proxies -> block
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

	// 3. 构建 Routing (包含基础分流规则)
	rulesList := []interface{}{
		// A. 本地与 DNS 强制直连
		map[string]interface{}{
			"ip_cidr":  []string{"127.0.0.1/32", "::1/128"},
			"outbound": "direct",
		},
		map[string]interface{}{
			"protocol": "dns",
			"outbound": "direct",
		},
	}

	// C. 用户自定义映射 (多对一或多对多)
	// 记录入口节点的默认绑定落地
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

	// 最终路由策略
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
