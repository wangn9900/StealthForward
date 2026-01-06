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
	portToUsers := make(map[int][]models.ForwardingRule)
	defaultPortUsers := []models.ForwardingRule{}

	for _, rule := range rules {
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
			defaultPortUsers = append(defaultPortUsers, rule)
		} else {
			portToUsers[assignedPort] = append(portToUsers[assignedPort], rule)
		}
	}

	// 辅助函数：根据协议生成 User 配置
	generateUsers := func(protocol string, ruleList []models.ForwardingRule) []map[string]interface{} {
		var users []map[string]interface{}
		for _, r := range ruleList {
			var u map[string]interface{}
			switch protocol {
			case "trojan", "shadowsocks", "ss", "hysteria2":
				u = map[string]interface{}{
					"name":     r.UserEmail,
					"password": r.UserID,
				}
			case "vmess":
				u = map[string]interface{}{
					"name": r.UserEmail,
					"uuid": r.UserID,
				}
			case "anytls":
				// AnyTLS 使用 password 认证（UUID 作为密码）
				u = map[string]interface{}{
					"password": r.UserID,
				}
				if r.UserEmail != "" {
					u["name"] = r.UserEmail
				}
			default: // VLESS and others
				u = map[string]interface{}{
					"uuid": r.UserID,
				}
				if r.UserEmail != "" {
					u["name"] = r.UserEmail
				}
				// 仅当 VLESS 且传输层为 TCP 或空（默认）时才加 flow
				if entry.Transport == "" || entry.Transport == "tcp" {
					u["flow"] = "xtls-rprx-vision"
				}
			}
			users = append(users, u)
		}
		return users
	}

	// Determine default protocol type - 使用 entry.Protocol 而非 V2boardType
	// generateUsers 需要原始协议 ("anytls") 来决定是否加 flow
	// 但 sing-box 配置文件需要 "vless"
	defaultProtocolFn := entry.Protocol
	if defaultProtocolFn == "" {
		defaultProtocolFn = "vless" // 默认视为 VLESS (带 flow)
	}

	defaultProtocolType := defaultProtocolFn
	// AnyTLS 保持原生类型，不再映射成 vless
	if defaultProtocolType == "v2ray" {
		defaultProtocolType = "vmess"
	} else if defaultProtocolType == "ss" {
		defaultProtocolType = "shadowsocks"
	}

	// 创建默认端口的 inbound
	defaultInboundTag := fmt.Sprintf("node_%d", entry.ID)
	defaultInbound := map[string]interface{}{
		"type":          defaultProtocolType,
		"tag":           defaultInboundTag,
		"listen":        "::",
		"listen_port":   entry.Port,
		"sniff":         true,
		"sniff_timeout": "1s", // 放宽到 1s，牺牲极微小首包延迟，换取 100% 握手成功率与长连接稳定性
		"users":         generateUsers(defaultProtocolFn, defaultPortUsers),
	}

	// Reality 回落解析
	realityDestHost := entry.RealityFallback
	realityDestPort := 443
	if entry.RealityFallback != "" {
		if strings.Contains(entry.RealityFallback, ":") {
			parts := strings.Split(entry.RealityFallback, ":")
			realityDestHost = parts[0]
			if p, err := strconv.Atoi(parts[1]); err == nil {
				realityDestPort = p
			}
		}
	}

	// 根据协议类型决定是否需要 fallback
	// 只有 VLESS 和 Trojan 支持 fallback
	// 如果开启了 Reality，回落由 Reality Handshake 接管，不需要 inbound 层的 fallback
	if (defaultProtocolType == "vless" || defaultProtocolType == "trojan") && !entry.RealityEnabled {
		defaultInbound["fallback"] = map[string]interface{}{
			"server":      fallbackHost,
			"server_port": fallbackPort,
		}
	}

	// AnyTLS 需要 padding_scheme 配置
	if defaultProtocolType == "anytls" && entry.PaddingScheme != "" {
		defaultInbound["padding_scheme"] = entry.PaddingScheme
	}

	// TLS 配置
	tlsConfig := map[string]interface{}{
		"enabled":     true,
		"min_version": "1.2",
	}

	if entry.RealityEnabled {
		// Reality 模式
		tlsConfig["server_name"] = entry.RealityServerName
		tlsConfig["reality"] = map[string]interface{}{
			"enabled":     true,
			"handshake":   map[string]interface{}{"server": realityDestHost, "server_port": realityDestPort},
			"private_key": entry.RealityPrivateKey,
			"short_id":    []string{entry.RealityShortID},
		}

		// Reality 不需要本地证书路径
	} else {
		// 标准 TLS 模式
		tlsConfig["server_name"] = entry.Domain
		tlsConfig["certificate_path"] = certPath
		tlsConfig["key_path"] = keyPath
	}

	// Shadowsocks 不使用 TLS
	if defaultProtocolType != "shadowsocks" {
		defaultInbound["tls"] = tlsConfig
	}

	// gRPC/WS/H2 传输层配置 (仅适用于非 AnyTLS/Shadowsocks 协议)
	// AnyTLS 是纯 TLS 协议，不支持额外的传输层封装
	if defaultProtocolFn != "anytls" && defaultProtocolType != "shadowsocks" {
		if entry.Transport == "grpc" {
			serviceName := entry.GrpcService
			if serviceName == "" {
				serviceName = "grpc" // 默认 service name
			}
			defaultInbound["transport"] = map[string]interface{}{
				"type":         "grpc",
				"service_name": serviceName,
			}
		} else if entry.Transport == "ws" {
			defaultInbound["transport"] = map[string]interface{}{
				"type": "ws",
				"path": "/",
			}
		} else if entry.Transport == "h2" {
			defaultInbound["transport"] = map[string]interface{}{
				"type": "http",
			}
		}
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

		inboundProtocolType := inboundType
		// AnyTLS 保持原生类型，不再映射成 vless
		if inboundProtocolType == "v2ray" {
			inboundProtocolType = "vmess"
		} else if inboundProtocolType == "ss" {
			inboundProtocolType = "shadowsocks"
		}

		inboundTag := fmt.Sprintf("node_%d_port_%d", entry.ID, port)
		inbound := map[string]interface{}{
			"type":          inboundProtocolType,
			"tag":           inboundTag,
			"listen":        "::",
			"listen_port":   port,
			"sniff":         true,
			"sniff_timeout": "1s",
			"users":         generateUsers(inboundType, users),
			"tls":           tlsConfig,
		}

		// 只有在非 Reality 模式下，且协议为 VLESS 或 Trojan 时才添加本地伪装回落
		if !entry.RealityEnabled && (inboundProtocolType == "vless" || inboundProtocolType == "trojan") {
			inbound["fallback"] = map[string]interface{}{
				"server":      fallbackHost,
				"server_port": fallbackPort,
			}
		}

		// AnyTLS 需要 padding_scheme 配置
		if inboundProtocolType == "anytls" && entry.PaddingScheme != "" {
			inbound["padding_scheme"] = entry.PaddingScheme
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

			// --- 自愈逻辑：Shadowsocks 2022 强制校验 ---
			// 如果内核检测到 2022 协议但密码长度不对，会直接导致整个 Agent 崩溃。
			// 这里我们主动检测不合规的配置并跳过，宁可少一个节点，不要挂整个服务。
			if method, ok := exitOutbound["method"].(string); ok && strings.Contains(method, "2022-blake3") {
				if pwd, ok := exitOutbound["password"].(string); ok {
					// 所有的 2022 协议都要求 password 是 Base64 编码的 16/32 字节密钥
					// 简单起见，我们只能检查它是否像一个普通密码（比如长度<32）
					// 标准 32 bytes 密钥 base64 编码后长度约为 44 字符
					// 16 bytes 密钥 base64 编码后长度约为 24 字符
					if len(pwd) < 20 {
						// 记录日志或直接静默跳过
						// fmt.Printf("Skipping invalid SS-2022 node %d (%s): password too short for %s\n", exit.ID, exit.Name, method)
						continue
					}
				}
			}

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
