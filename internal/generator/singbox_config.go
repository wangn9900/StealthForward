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
				map[string]interface{}{"address": "1.1.1.1", "tag": "cf"},
				map[string]interface{}{"address": "8.8.8.8", "tag": "google"},
			},
		},
	}

	// 1. 构建 Inbound (VLESS + Vision + Fallback)
	vlessInbound := map[string]interface{}{
		"type":                       "vless",
		"tag":                        "vless-in",
		"listen":                     "::",
		"listen_port":                entry.Port,
		"sniff":                      true,
		"sniff_override_destination": true,
		"users":                      []interface{}{},
	}

	// 添加用户到 Inbound
	users := []map[string]interface{}{}
	for _, rule := range rules {
		users = append(users, map[string]interface{}{
			"uuid": rule.UserID,
			"name": rule.UserEmail,
			"flow": "xtls-rprx-vision",
		})
	}
	vlessInbound["users"] = users

	// 证书与安全配置
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
	// 按照用户图示：先放 direct，再放代理，最后放 block
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
			// 增加专业参数映射
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

	// 3. 构建 Routing (增加专业分流规则)
	rulesList := []interface{}{
		// 路由规则 1: 确保本地回落流量永不走代理 (修复 ERR_EMPTY_RESPONSE)
		map[string]interface{}{
			"ip_cidr":  []string{"127.0.0.1/32", "::1/128"},
			"outbound": "direct",
		},
		// 路由规则 2: DNS 流量直连或走专有出口
		map[string]interface{}{
			"protocol": "dns",
			"outbound": "direct",
		},
	}

	// 记录默认出口名称
	var defaultExitName string
	if entry.TargetExitID != 0 {
		for _, e := range exits {
			if e.ID == entry.TargetExitID {
				defaultExitName = "out-" + e.Name
				break
			}
		}
	}

	// 路由规则 3: 用户自定义映射
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

	// 最终路由配置
	routeConfig := map[string]interface{}{
		"rules": rulesList,
	}

	// 如果有默认绑定落地，其余流量走默认落地
	if defaultExitName != "" {
		routeConfig["final"] = defaultExitName
	} else {
		routeConfig["final"] = "direct"
	}

	config.Route = routeConfig

	res, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(res), nil
}
