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
			"level": "debug",
		},
	}

	// 1. 构建 Inbound (VLESS + Vision + Fallback)
	vlessInbound := map[string]interface{}{
		"type":        "vless",
		"tag":         "vless-in",
		"listen":      "::",
		"listen_port": entry.Port,
		"sniff": map[string]interface{}{
			"enabled":              true,
			"override_destination": true,
		},
		"users": []interface{}{},
	}

	// 添加用户到 Inbound
	users := []map[string]interface{}{}
	for _, rule := range rules {
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
		"alpn":             []string{"http/1.1", "h2"},
		"detour":           "fallback-in", // 握手失败回落到这个标签
	}

	config.Inbounds = append(config.Inbounds, vlessInbound)

	// 2. 增加回落 Inbound (配合 detour)
	// 这个 Inbound 负责把非代理流量直接指向我们的本地伪装服务器
	fallbackInbound := map[string]interface{}{
		"type":   "http",
		"tag":    "fallback-in",
		"listen": "127.0.0.1",
		// 注意：sing-box 1.10+ 这里通常指向另一个本地监听的服务
	}
	// 更稳妥的做法：直接用 redirect
	_ = fallbackInbound

	// 重新调整：既然目标是 Fallback 地址，我们用一个特殊的 redirect 或者增加一个 listen
	// 下面是 sing-box 最标准的写法：
	config.Inbounds = append(config.Inbounds, map[string]interface{}{
		"type": "redirect",
		"tag":  "fallback-in",
		"dest": entry.Fallback,
	})

	// 2. 构建 Outbounds (落地节点)
	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)

		// 映射协议类型到 sing-box type
		sbType := exit.Protocol
		if sbType == "ss" {
			sbType = "shadowsocks"
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
