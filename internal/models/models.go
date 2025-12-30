package models

import "time"

// EntryNode 代表入口服务器（海外机）
type EntryNode struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	IP           string    `json:"ip"`
	Port         int       `json:"port"`           // 通常为 443 或 8443
	Domain       string    `json:"domain"`         // 用于 TLS
	Certificate  string    `json:"certificate"`    // 证书文件路径
	Key          string    `json:"key"`            // 私钥文件路径
	Fallback     string    `json:"fallback"`       // 回落地址，例如 "127.0.0.1:8080"
	TargetExitID uint      `json:"target_exit_id"` // 一键中转映射的落地节点 ID
	Protocol     string    `json:"protocol"`       // vless
	Security     string    `json:"security"`       // xtls-vision
	CreatedAt    time.Time `json:"created_at"`
}

// ExitNode 代表落地服务器（小鸡）
type ExitNode struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"` // shadowsocks, vmess, vless
	Config    string    `json:"config"`   // 存储具体的协议配置 (JSON string)
	CreatedAt time.Time `json:"created_at"`
}

// ForwardingRule 定义了转发映射关系
type ForwardingRule struct {
	ID          uint   `json:"id"`
	UserID      string `json:"user_id"`    // 对应 VLESS 的 UUID
	UserEmail   string `json:"user_email"` // 对应 VLESS 的 Email，用于识别流量
	EntryNodeID uint   `json:"entry_node_id"`
	ExitNodeID  uint   `json:"exit_node_id"`
	Enabled     bool   `json:"enabled"`
}
