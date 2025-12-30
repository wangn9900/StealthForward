package generator

import (
	"encoding/json"
	"github.com/nasstoki/stealthforward/internal/models"
)

// SingBoxConfig 定义了简化的 sing-box 配置文件结构
type SingBoxConfig struct {
	Log       interface{}   `json:"log"`
	Inbounds  []interface{} `json:"inbounds"`
	Outbounds []interface{} `json:"outbounds"`
	Route     interface{}   `json:"route"`
}

// GenerateEntryConfig 生成入口节点的 Sing-box 配置
func GenerateEntryConfig(entry *models.EntryNode, rules []models.ForwardingRule, exits []models.ExitNode) (string, error) {
	config := SingBoxConfig{
		Log: map[string]interface{}{
			"level": "info",
		},
	}

	// 1. 构建 Inbound (VLESS + Vision + Fallback)
	vlessInbound := map[string]interface{}{
		"type":        "vless",
		"tag":         "vless-in",
		"listen":      "::",
		"listen_port": entry.Port,
		"sniff":       true,
		"users":       []interface{}{},
	}

	// 添加用户到 Inbound
	users := []map[string]interface{}{}
	for _, rule := range rules {
		// 这里强制开启 xtls-rprx-vision 流控
		users = append(users, map[string]interface{}{
			"uuid":  rule.UserID,
			"email": rule.UserEmail,
			"flow":  "xtls-rprx-vision",
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
	}

	// 核心防御：回落设置 (SNI 回落)
	vlessInbound["dest"] = entry.Fallback

	config.Inbounds = append(config.Inbounds, vlessInbound)

	// 2. 构建 Outbounds (落地节点)
	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)

		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	// 3. 构建 Routing (根据 User Email 分流)
	rulesList := []interface{}{}
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

	config.Route = map[string]interface{}{
		"rules": rulesList,
	}

	res, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(res), nil
}
