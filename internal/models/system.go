package models

import "time"

// SystemSetting 存储全局配置 (Key-Value)
type SystemSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex"` // 例如 aws_access_key_id
	Value     string    `json:"value"`                  // 配置值
	Category  string    `json:"category"`               // 分类: aws, cloudflare, system
	UpdatedAt time.Time `json:"updated_at"`
}

// CloudAccount 存储多个云账号信息
type CloudAccount struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`                        // 账号备注名
	Provider  string    `json:"provider"`                    // aws, cloudflare
	AccessKey string    `json:"access_key"`                  // AWS Access Key ID 或 CF Token
	SecretKey string    `json:"secret_key"`                  // AWS Secret Access Key (CF 可为空)
	UsageHash string    `json:"usage_hash" gorm:"index"`     // 用于简单去重或查找
	Enabled   bool      `json:"enabled" gorm:"default:true"` // 是否启用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SSHKey 存储用于拉起 Agent 的全局 SSH 私钥
type SSHKey struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name"`        // 密钥名 (如 aws-global)
	User       string    `json:"user"`        // 登录用户名 (如 root, ubuntu)
	KeyContent string    `json:"key_content"` // 私钥 Base64 或明文
	UpdatedAt  time.Time `json:"updated_at"`
}
