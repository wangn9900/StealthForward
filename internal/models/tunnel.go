package models

import (
	"time"

	"gorm.io/gorm"
)

// ... existing code ...

// Plan 订阅套餐
type Plan struct {
	gorm.Model
	Name         string  `json:"name"`
	Price        float64 `json:"price"`
	TrafficLimit int64   `json:"traffic_limit"` // Bytes
	MaxRules     int     `json:"max_rules"`
	DurationDays int     `json:"duration_days"`
	Description  string  `json:"description"`
}

// Order 订单记录
type Order struct {
	gorm.Model
	OrderNo       string  `json:"order_no"`
	UserID        uint    `json:"user_id"`
	PlanID        uint    `json:"plan_id"`
	Amount        float64 `json:"amount"`
	Type          string  `json:"type"`   // recharge, consumption
	Status        string  `json:"status"` // pending, completed, cancelled
	PaymentMethod string  `json:"payment_method"`
}

// User 代表商用转发系统的用户
type User struct {
	gorm.Model
	Username     string    `json:"username" gorm:"unique"`
	Balance      float64   `json:"balance"`       // 钱包余额 (CNY)
	TotalTraffic int64     `json:"total_traffic"` // 总额度 (Bytes)
	UsedTraffic  int64     `json:"used_traffic"`  // 已用流量 (已经乘过倍率的)
	MaxRules     int       `json:"max_rules"`     // 最大规则数
	ExpiredAt    time.Time `json:"expired_at"`    // 到期时间
	AutoRenewal  bool      `json:"auto_renewal"`  // 是否自动续费
}

// UltraTunnelUserStats 用于返回给前端的概览 (兼容旧接口)
type UltraTunnelUserStats struct {
	Balance      float64   `json:"balance"`
	UsedTraffic  int64     `json:"used_traffic"`
	TotalTraffic int64     `json:"total_traffic"`
	ExpiredAt    time.Time `json:"expired_at"`
	MaxRules     int       `json:"max_rules"`
	CurrentRules int       `json:"current_rules"`
}

// UltraTunnelNode 代表一个独立的中转机节点
type UltraTunnelNode struct {
	gorm.Model
	Name         string `json:"name"`
	PublicAddr   string `json:"public_addr"`   // 公网访问地址 (中转机 IP/域名)
	InternalAddr string `json:"internal_addr"` // 内部连接地址 (通常同上)
	SSHHost      string `json:"ssh_host"`
	SSHPort      int    `json:"ssh_port"`
	SSHUser      string `json:"ssh_user"`
	SSHPass      string `json:"ssh_pass"`
	Status       string `json:"status"` // online, offline, deploying
}

// UltraTunnelLine 管理员定义的线路池 (例如：深港专线)
type UltraTunnelLine struct {
	gorm.Model
	Name          string  `json:"name"`
	TransitNodeID uint    `json:"transit_node_id"` // 入口机 ID
	ExitNodeID    uint    `json:"exit_node_id"`    // 出口机 ID
	Price         float64 `json:"price"`           // 单价/倍率
	IsPublic      bool    `json:"is_public"`       // 是否公开给所有用户
}

// UltraTunnelRule 代表一个转发规则
type UltraTunnelRule struct {
	gorm.Model
	UserID     uint   `json:"user_id"` // 属于哪个用户
	Name       string `json:"name"`
	LineID     uint   `json:"line_id"`      // 使用哪条线路
	NodeID     uint   `json:"node_id"`      // 入口机 (Transit) ID
	ExitNodeID uint   `json:"exit_node_id"` // 出口机 (Exit) ID
	ListenPort int    `json:"listen_port"`  // 入口机监听端口
	TunnelPort int    `json:"tunnel_port"`  // 出口机监听端口
	LocalDest  string `json:"local_dest"`   // 最终目标地址
	Key        string `json:"key"`          // 隧道加密 Key
	Upload     int64  `json:"upload"`
	Download   int64  `json:"download"`
	Status     bool   `json:"status"`
}
