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

// 定义常用 Key 常量
const (
	ConfigKeyAwsAccessKeyID     = "aws.access_key_id"
	ConfigKeyAwsSecretAccessKey = "aws.secret_access_key"
	ConfigKeyAwsDefaultRegion   = "aws.default_region" // 默认区域
	ConfigKeyCfApiToken         = "cloudflare.api_token"
	ConfigKeyCfDefaultZone      = "cloudflare.default_zone" // 默认域名 (2233006.xyz)
)
