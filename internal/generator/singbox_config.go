package generator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wangn9900/StealthForward/internal/models"
)

// SingBoxConfig 绝不包含任何会让魔改内核崩溃的 experimental 或 hosts 字段
type SingBoxConfig struct {
	Log       interface{}   `json:"log"`
	DNS       interface{}   `json:"dns,omitempty"`
	Route     interface{}   `json:"route"`
	Outbounds []interface{} `json:"outbounds"`
	Inbounds  []interface{} `json:"inbounds"` // 用户列表最长，挪到最后，方便用户查看
}

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
			},
			"strategy": "prefer_ipv4",
		},
	}

	// 证书路径
	certPath := entry.Certificate
	if certPath == "" {
		certPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.crt"
	}
	keyPath := entry.Key
	if keyPath == "" {
		keyPath = "/etc/stealthforward/certs/" + entry.Domain + "/cert.key"
	}

	// Inbound - 采用 node_ID 格式，且彻底禁用 override_destination 解决公网环路
	inboundTag := fmt.Sprintf("node_%d", entry.ID)
	vlessInbound := map[string]interface{}{
		"type":        "vless",
		"tag":         inboundTag,
		"listen":      "::",
		"listen_port": entry.Port,
		"sniff":       true, // 保留嗅探用于协议识别，但绝不重定向目的地
	}

	// 回落
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

	users := []map[string]interface{}{}
	for _, rule := range rules {
		// 直接使用数据库中的 UserEmail 作为唯一标识，不再二次拼接前缀
		users = append(users, map[string]interface{}{
			"name": rule.UserEmail,
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

	// Outbounds
	config.Outbounds = append(config.Outbounds, map[string]interface{}{"tag": "direct", "type": "direct"})

	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound) // 适配 Shadowsocks 格式

		if exit.Protocol == "ss" {
			exitOutbound["type"] = "shadowsocks"
			if cipher, ok := exitOutbound["cipher"]; ok {
				exitOutbound["method"] = cipher
			}
			// 修正端口逻辑：优先尝试 server_port，其次尝试 port
			finalPort := exitOutbound["server_port"]
			if exitOutbound["server_port"] == nil || exitOutbound["server_port"] == float64(0) {
				if exitOutbound["port"] != nil {
					finalPort = exitOutbound["port"]
				}
			}
			exitOutbound["server_port"] = finalPort

			if addr, ok := exitOutbound["address"]; ok {
				exitOutbound["server"] = addr
			}

			// 移除可能引起兼容性问题的冗余字段 (sing-box 官方字段为 server, server_port, method, password)
			delete(exitOutbound, "address")
			delete(exitOutbound, "port")
			delete(exitOutbound, "cipher")

			exitOutbound["tcp_fast_open"] = false
		}

		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	config.Outbounds = append(config.Outbounds, map[string]interface{}{"tag": "block", "type": "block"})

	// Routing - 恢复最稳健的显式路由逻辑 (不再使用 Final 兜底，确保 IP 物理一致)
	routingRules := []interface{}{
		map[string]interface{}{"ip_cidr": []string{"127.0.0.1/32", "::1/128"}, "outbound": "direct"},
		map[string]interface{}{"protocol": "dns", "outbound": "direct"},
	}

	// 按落地节点分组生成规则
	exitToUsers := make(map[uint][]string)
	for _, rule := range rules {
		if rule.ExitNodeID != 0 {
			exitToUsers[rule.ExitNodeID] = append(exitToUsers[rule.ExitNodeID], rule.UserEmail)
		}
	}

	for exitID, tags := range exitToUsers {
		var exitName string
		for _, e := range exits {
			if e.ID == exitID {
				exitName = e.Name
				break
			}
		}
		if exitName != "" && len(tags) > 0 {
			routingRules = append(routingRules, map[string]interface{}{
				"user":     tags,
				"outbound": "out-" + exitName,
			})
		}
	}

	// 兜底出站：如果没有任何规则匹配，则优先走该节点的默认落地机，若没配则走系统第一个可用落地机
	finalOutbound := "block"
	if entry.TargetExitID != 0 {
		for _, e := range exits {
			if e.ID == entry.TargetExitID {
				finalOutbound = "out-" + e.Name
				break
			}
		}
	}
	// 如果还是 block，且有出口，强行指向第一个出口作为抢救
	if finalOutbound == "block" && len(exits) > 0 {
		finalOutbound = "out-" + exits[0].Name
	}

	config.Route = map[string]interface{}{
		"rules": routingRules,
		"final": finalOutbound,
	}

	res, _ := json.MarshalIndent(config, "", "  ")
	return string(res), nil
}
