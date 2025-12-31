package generator

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/wangn9900/StealthForward/internal/models"
)

// SingBoxConfig 最简配置结构，适配魔改内核
type SingBoxConfig struct {
	Log       interface{}       `json:"log"`
	DNS       interface{}       `json:"dns,omitempty"`
	Inbounds  []interface{}     `json:"inbounds"`
	Outbounds []interface{}     `json:"outbounds"`
	Route     interface{}       `json:"route"`
	Provision map[string]string `json:"provision,omitempty"`
}

func GenerateEntryConfig(entry *models.EntryNode, rules []models.ForwardingRule, exits []models.ExitNode) (string, error) {
	config := SingBoxConfig{
		Log: map[string]interface{}{
			"level": "error",
		},
		Provision: make(map[string]string),
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

	// 注入证书
	certPath := entry.Certificate
	if certPath == "" {
		certPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.crt"
	}
	keyPath := entry.Key
	if keyPath == "" {
		keyPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.key"
	}

	if entry.CertBody != "" && entry.KeyBody != "" {
		config.Provision[certPath] = entry.CertBody
		config.Provision[keyPath] = entry.KeyBody
	}

	// Inbound
	vlessInbound := map[string]interface{}{
		"type":                       "vless",
		"tag":                        "vless-in",
		"listen":                     "::",
		"listen_port":                entry.Port,
		"sniff":                      true,
		"sniff_override_destination": true,
	}

	// 分流回落
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

	// 关键：针对 Tox/V2bX 的单数 fallback
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
		"certificate_path": certPath,
		"key_path":         keyPath,
		"min_version":      "1.2",
	}
	config.Inbounds = append(config.Inbounds, vlessInbound)

	// Outbounds
	config.Outbounds = append(config.Outbounds, map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	})

	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)
		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	config.Outbounds = append(config.Outbounds, map[string]interface{}{"type": "block", "tag": "block"})

	// Routing
	config.Route = map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{"ip_cidr": []string{"127.0.0.1/32", "::1/128"}, "outbound": "direct"},
			map[string]interface{}{"protocol": "dns", "outbound": "direct"},
		},
		"final": "direct",
	}

	res, _ := json.MarshalIndent(config, "", "  ")
	return string(res), nil
}
