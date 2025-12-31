package models

import "time"

// EntryNode 代表入口服务器（海外机）
type EntryNode struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`           // 通常为 443 或 8443
	Domain       string `json:"domain"`         // 用于 TLS
	Certificate  string `json:"certificate"`    // 证书文件路径
	Key          string `json:"key"`            // 私钥文件路径
	CertBody     string `json:"cert_body"`      // 证书内容备份 (用于换机无感恢复)
	KeyBody      string `json:"key_body"`       // 私钥内容备份
	Fallback     string `json:"fallback"`       // 回落地址，例如 "127.0.0.1:8080"
	TargetExitID uint   `json:"target_exit_id"` // 默认的一键转落地节点 ID（作为备用）
	Protocol     string `json:"protocol"`       // vless
	Security     string `json:"security"`       // xtls-vision

	// V2Board 同步配置（全局默认）
	V2boardURL    string `json:"v2board_url"`     // V2Board API 地址
	V2boardKey    string `json:"v2board_key"`     // 通讯密钥
	V2boardNodeID int    `json:"v2board_node_id"` // 默认节点 ID
	V2boardType   string `json:"v2board_type"`    // v2ray, shadowsocks, trojan

	CreatedAt time.Time `json:"created_at"`
}

// NodeMapping 定义了同一入口下不同 V2Board 节点到不同落地机的映射关系
type NodeMapping struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	EntryNodeID   uint      `json:"entry_node_id"`   // 关联入口节点
	V2boardNodeID int       `json:"v2board_node_id"` // V2Board 那边的节点 ID
	TargetExitID  uint      `json:"target_exit_id"`  // 对应的落地节点 ID
	V2boardType   string    `json:"v2board_type"`    // 节点类型
	CreatedAt     time.Time `json:"created_at"`
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

// ForwardingRule 定义了最终的转发映射关系 (用户级)
type ForwardingRule struct {
	ID          uint   `json:"id"`
	UserID      string `json:"user_id"`     // 对应 VLESS 的 UUID
	V2boardUID  uint   `json:"v2board_uid"` // 对应 V2Board 的用户 ID，用于上报流量
	UserEmail   string `json:"user_email"`  // 对应 VLESS 的 Email，用于识别流量
	EntryNodeID uint   `json:"entry_node_id"`
	ExitNodeID  uint   `json:"exit_node_id"`
	Enabled     bool   `json:"enabled"`
}

// UserTraffic 代表单个用户的流量统计
type UserTraffic struct {
	UserEmail string `json:"user_email"`
	Upload    int64  `json:"upload"`
	Download  int64  `json:"download"`
}

// NodeTrafficReport 节点上报的流量汇总
type NodeTrafficReport struct {
	NodeID  uint          `json:"node_id"`
	Traffic []UserTraffic `json:"traffic"`
}
