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

	// 2. 构建 Outbounds (落地节点)
	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)

		// 映射协议类型到 sing-box type
		sbType := exit.Protocol
		if sbType == "ss" {
			sbType = "shadowsocks"
		}

		// 关键兼容性修复：映射 v2board/通用风格参数到 sing-box
		if port, ok := exitOutbound["port"]; ok {
			exitOutbound["server_port"] = port
		}
		if addr, ok := exitOutbound["address"]; ok {
			exitOutbound["server"] = addr
		}
		if cipher, ok := exitOutbound["cipher"]; ok {
			exitOutbound["method"] = cipher
		}

		exitOutbound["type"] = sbType
		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	// 3. 构建 Routing (优先根据规则，最后根据默认绑定)
	rulesList := []interface{}{}

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

	// 最终路由规则
	routeConfig := map[string]interface{}{
		"rules": rulesList,
	}

	// 如果有默认绑定落地，则除了显式规则外的所有流量默认走该落地
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
