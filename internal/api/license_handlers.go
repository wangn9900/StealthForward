package api

import (
	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/license"
	"github.com/wangn9900/StealthForward/internal/models"
)

// LicenseInfoResponse 授权信息响应
type LicenseInfoResponse struct {
	Level     string         `json:"level"`
	ExpiresAt string         `json:"expires_at"`
	Limits    license.Limits `json:"limits"`
	Usage     LicenseUsage   `json:"usage"`
}

// LicenseUsage 当前使用量
type LicenseUsage struct {
	Entries int64 `json:"entries"`
	Exits   int64 `json:"exits"`
}

// GetLicenseInfoHandler 获取授权信息
func GetLicenseInfoHandler(c *gin.Context) {
	info := license.GetInfo()

	// 如果跳过了授权验证，返回管理员模式
	if info == nil {
		c.JSON(200, gin.H{
			"level":      "admin",
			"expires_at": "永久",
			"limits": license.Limits{
				Protocols:    []string{"*"},
				MaxEntries:   999999,
				MaxExits:     999999,
				CloudEnabled: true,
			},
			"usage": LicenseUsage{
				Entries: countEntries(),
				Exits:   countExits(),
			},
		})
		return
	}

	c.JSON(200, LicenseInfoResponse{
		Level:     info.Level,
		ExpiresAt: info.ExpiresAt.Format("2006-01-02"),
		Limits:    info.Limits,
		Usage: LicenseUsage{
			Entries: countEntries(),
			Exits:   countExits(),
		},
	})
}

// countEntries 统计入口节点数
func countEntries() int64 {
	var count int64
	database.DB.Model(&models.EntryNode{}).Count(&count)
	return count
}

// countExits 统计落地节点数
func countExits() int64 {
	var count int64
	database.DB.Model(&models.ExitNode{}).Count(&count)
	return count
}

// CheckProtocolAllowed 检查协议是否允许（供其他handler调用）
func CheckProtocolAllowed(c *gin.Context, protocol string) bool {
	if !license.IsProtocolAllowed(protocol) {
		c.JSON(403, gin.H{
			"error": "当前授权等级不支持 " + protocol + " 协议，请升级到Pro版",
		})
		return false
	}
	return true
}

// CheckCanAddEntry 检查是否可以添加入口节点
func CheckCanAddEntry(c *gin.Context) bool {
	if !license.CanAddEntry(int(countEntries())) {
		limits := license.GetLimits()
		c.JSON(403, gin.H{
			"error": "已达入口节点上限，请升级授权",
			"limit": limits.MaxEntries,
		})
		return false
	}
	return true
}

// CheckCanAddExit 检查是否可以添加落地节点
func CheckCanAddExit(c *gin.Context) bool {
	if !license.CanAddExit(int(countExits())) {
		limits := license.GetLimits()
		c.JSON(403, gin.H{
			"error": "已达落地节点上限，请升级授权",
			"limit": limits.MaxExits,
		})
		return false
	}
	return true
}

// CheckCloudEnabled 检查云功能是否启用
func CheckCloudEnabled(c *gin.Context) bool {
	if !license.IsCloudEnabled() {
		c.JSON(403, gin.H{
			"error": "当前授权等级不支持云平台功能，请升级到Pro版",
		})
		return false
	}
	return true
}
