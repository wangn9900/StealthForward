package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// SingBoxConfig 绝不包含任何会让魔改内核崩溃的 experimental 或 hosts 字段
type SingBoxConfig struct {
	Log       interface{}   `json:"log"`
	DNS       interface{}   `json:"dns,omitempty"`
	Route     interface{}   `json:"route"`
	Outbounds []interface{} `json:"outbounds"`
	Inbounds  []interface{} `json:"inbounds"`
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
					"tag":     "dns-local",
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

	// 获取所有 NodeMapping
	var mappings []models.NodeMapping
	database.DB.Where("entry_node_id = ?", entry.ID).Find(&mappings)

	// 构建端口到 Mapping 的映射（多端口分流核心逻辑）
	portToMapping := make(map[int]*models.NodeMapping)
	for i := range mappings {
		m := &mappings[i]
		if m.Port > 0 {
			portToMapping[m.Port] = m
		}
	}

	// 构建端口到用户的映射
	portToUsers := make(map[int][]map[string]interface{})
	defaultPortUsers := []map[string]interface{}{}

	for _, rule := range rules {
		user := map[string]interface{}{
			"name": rule.UserEmail,
			"uuid": rule.UserID,
			"flow": "xtls-rprx-vision",
		}

		// 从 UserEmail (n20-xxx) 提取节点 ID，找到对应的端口
		assignedPort := entry.Port // 默认端口
		if strings.HasPrefix(rule.UserEmail, "n") && strings.Contains(rule.UserEmail, "-") {
			idPart := strings.Split(rule.UserEmail, "-")[0][1:]
			if v2bNodeID, err := strconv.Atoi(idPart); err == nil {
				// 查找这个节点 ID 对应的 Mapping
				for _, m := range mappings {
					if m.V2boardNodeID == v2bNodeID && m.Port > 0 {
						assignedPort = m.Port
						break
					}
				}
			}
		}

		if assignedPort == entry.Port {
			defaultPortUsers = append(defaultPortUsers, user)
		} else {
			portToUsers[assignedPort] = append(portToUsers[assignedPort], user)
		}
	}

	// Determine default protocol type
	defaultType := entry.V2boardType
	if defaultType == "" {
		defaultType = "vless"
	} else if defaultType == "v2ray" {
		defaultType = "vmess"
	}

	// 创建默认端口的 inbound
	defaultInboundTag := fmt.Sprintf("node_%d", entry.ID)
	defaultInbound := map[string]interface{}{
		"type":          defaultType,
		"tag":           defaultInboundTag,
		"listen":        "::",
		"listen_port":   entry.Port,
		"sniff":         true,
		"sniff_timeout": "100ms", // 核心加速点：缩短等待时间，实现首屏秒开
		"fallback": map[string]interface{}{
			"server":      fallbackHost,
			"server_port": fallbackPort,
		},
		"users": defaultPortUsers,
		"tls": map[string]interface{}{
			"enabled":          true,
			"server_name":      entry.Domain,
			"certificate_path": certPath,
			"key_path":         keyPath,
			"min_version":      "1.2",
		},
	}
	config.Inbounds = append(config.Inbounds, defaultInbound)

	// 为每个独立端口创建 inbound
	var ports []int
	for p := range portToUsers {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	for _, port := range ports {
		users := portToUsers[port]
		inboundType := "vless"
		if m, ok := portToMapping[port]; ok && m.V2boardType != "" {
			inboundType = m.V2boardType
		}
		if inboundType == "v2ray" {
			inboundType = "vmess"
		}

		inboundTag := fmt.Sprintf("node_%d_port_%d", entry.ID, port)
		inbound := map[string]interface{}{
			"type":          inboundType,
			"tag":           inboundTag,
			"listen":        "::",
			"listen_port":   port,
			"sniff":         true,
			"sniff_timeout": "100ms",
			"fallback": map[string]interface{}{
				"server":      fallbackHost,
				"server_port": fallbackPort,
			},
			"users": users,
			"tls": map[string]interface{}{
				"enabled":          true,
				"server_name":      entry.Domain,
				"certificate_path": certPath,
				"key_path":         keyPath,
				"min_version":      "1.2",
			},
		}
		config.Inbounds = append(config.Inbounds, inbound)
	}

	// Outbounds
	config.Outbounds = append(config.Outbounds, map[string]interface{}{"tag": "direct", "type": "direct"})

	for _, exit := range exits {
		var exitOutbound map[string]interface{}
		json.Unmarshal([]byte(exit.Config), &exitOutbound)
		if exit.Protocol == "ss" {
			exitOutbound["type"] = "shadowsocks"
			if cipher, ok := exitOutbound["cipher"]; ok {
				exitOutbound["method"] = cipher
			}
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

			delete(exitOutbound, "address")
			delete(exitOutbound, "port")
			delete(exitOutbound, "cipher")
			exitOutbound["tcp_fast_open"] = false
		}

		exitOutbound["tag"] = "out-" + exit.Name
		config.Outbounds = append(config.Outbounds, exitOutbound)
	}

	config.Outbounds = append(config.Outbounds, map[string]interface{}{"tag": "block", "type": "block"})

	// Routing - 按端口分流
	routingRules := []interface{}{
		map[string]interface{}{"ip_cidr": []string{"127.0.0.1/32"}, "outbound": "direct"},
		map[string]interface{}{"protocol": "dns", "outbound": "direct"},
	}

	var mappingPorts []int
	for p := range portToMapping {
		mappingPorts = append(mappingPorts, p)
	}
	sort.Ints(mappingPorts)

	for _, port := range mappingPorts {
		m := portToMapping[port]
		inboundTag := fmt.Sprintf("node_%d_port_%d", entry.ID, port)
		var exitName string
		for _, e := range exits {
			if e.ID == m.TargetExitID {
				exitName = e.Name
				break
			}
		}
		if exitName != "" {
			routingRules = append(routingRules, map[string]interface{}{
				"inbound":  []string{inboundTag},
				"outbound": "out-" + exitName,
			})
		}
	}

	defaultExitTag := "block"
	if entry.TargetExitID != 0 {
		for _, e := range exits {
			if e.ID == entry.TargetExitID {
				defaultExitTag = "out-" + e.Name
				break
			}
		}
	}

	config.Route = map[string]interface{}{
		"rules": routingRules,
		"final": defaultExitTag,
	}

	res, _ := json.MarshalIndent(config, "", "  ")
	return string(res), nil
}
