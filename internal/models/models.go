package models

import "time"

// EntryNode 代表入口服务器（海外机）
type EntryNode struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`           // 通常�?443 �?8443
	Domain       string `json:"domain"`         // 用于 TLS
	Certificate  string `json:"certificate"`    // 证书文件路径
	Key          string `json:"key"`            // 私钥文件路径
	Fallback     string `json:"fallback"`       // 回落地址，例�?"127.0.0.1:8080"
	TargetExitID uint   `json:"target_exit_id"` // 一键中转映射的落地节点 ID
	Protocol     string `json:"protocol"`       // vless
	Security     string `json:"security"`       // xtls-vision

	// V2Board 同步配置
	V2boardURL    string `json:"v2board_url"`     // V2Board API 地址
	V2boardKey    string `json:"v2board_key"`     // 通讯密钥
	V2boardNodeID int    `json:"v2board_node_id"` // V2Board 正式节点�?ID
	V2boardType   string `json:"v2board_type"`    // v2ray, shadowsocks, trojan

	CreatedAt time.Time `json:"created_at"`
}

// ExitNode 代表落地服务器（小鸡�?
type ExitNode struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"` // shadowsocks, vmess, vless
	Config    string    `json:"config"`   // 存储具体的协议配�?(JSON string)
	CreatedAt time.Time `json:"created_at"`
}

// ForwardingRule 定义了转发映射关�?
type ForwardingRule struct {
	ID          uint   `json:"id"`
	UserID      string `json:"user_id"`    // 对应 VLESS �?UUID
	UserEmail   string `json:"user_email"` // 对应 VLESS �?Email，用于识别流�?
	EntryNodeID uint   `json:"entry_node_id"`
	ExitNodeID  uint   `json:"exit_node_id"`
	Enabled     bool   `json:"enabled"`
}
