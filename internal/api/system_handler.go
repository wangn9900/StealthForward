package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/license"
	"github.com/wangn9900/StealthForward/internal/models"
)

// 管理员密码硬编码的SHA256哈希（生产环境请修改）
// 默认密码: stealth@admin2024
// 生成方式: echo -n "stealth@admin2024" | sha256sum
const adminPasswordHash = "a8f5f167f44f4964e6c998dee827110c9d679f0fc7b8e9b7a0c7c7c8d8e4f1b2"

// --- Auth ---

// LoginHandler 支持两种登录方式：
// 1. 管理员登录：username=admin, password=管理员密码（本地验证，不依赖授权服务器）
// 2. License Key登录：license_key=SF-X-XXXX-XXXX（远程验证）
func LoginHandler(c *gin.Context) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		LicenseKey string `json:"license_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// ========== 方式1：管理员密码登录（本地验证，完全独立）==========
	if req.Username == "admin" && req.Password != "" {
		// 管理员密码验证（不依赖授权服务器）
		adminToken := os.Getenv("STEALTH_ADMIN_TOKEN")
		if adminToken == "" {
			adminToken = "stealth@admin2024" // 默认密码
		}

		if req.Password == adminToken {
			c.JSON(http.StatusOK, gin.H{
				"token":   adminToken,
				"role":    "admin",
				"level":   "admin",
				"message": "管理员登录成功",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "管理员密码错误"})
		return
	}

	// ========== 方式2：License Key 登录（远程验证）==========
	if req.LicenseKey != "" {
		// 验证License Key格式
		if !strings.HasPrefix(req.LicenseKey, "SF-") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的授权Key格式"})
			return
		}

		// 设置License Key并验证
		os.Setenv("STEALTH_LICENSE_KEY", req.LicenseKey)
		if err := license.Verify(); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "授权验证失败: " + err.Error(),
				"expired": strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "过期"),
			})
			return
		}

		info := license.GetInfo()
		c.JSON(http.StatusOK, gin.H{
			"token":      req.LicenseKey, // 用License Key作为Token
			"role":       "user",
			"level":      info.Level,
			"expires_at": info.ExpiresAt.Format("2006-01-02"),
			"limits":     info.Limits,
			"message":    "授权验证成功",
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "请输入管理员密码或授权Key"})
}

// --- System Config ---

// GetSystemConfigHandler 获取所有系统配置
func GetSystemConfigHandler(c *gin.Context) {
	var settings []models.SystemSetting
	if err := database.DB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转为 Map 方便前端使用
	configMap := make(map[string]string)
	for _, s := range settings {
		configMap[s.Key] = s.Value
	}

	// 补充默认值（如果数据库里没有）
	if _, ok := configMap[models.ConfigKeyAwsAccessKeyID]; !ok {
		configMap[models.ConfigKeyAwsAccessKeyID] = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if _, ok := configMap[models.ConfigKeyAwsDefaultRegion]; !ok {
		configMap[models.ConfigKeyAwsDefaultRegion] = os.Getenv("AWS_DEFAULT_REGION")
		if configMap[models.ConfigKeyAwsDefaultRegion] == "" {
			configMap[models.ConfigKeyAwsDefaultRegion] = "ap-northeast-1"
		}
	}
	// SecretKey 如果是从环境变量拿的，要不要脱敏？既然已经进来了，就明文给吧，方便修改

	c.JSON(http.StatusOK, gin.H{"config": configMap})
}

// UpdateSystemConfigHandler 批量更新配置
func UpdateSystemConfigHandler(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := database.DB.Begin()
	for k, v := range req {
		// Upsert
		var setting models.SystemSetting
		if err := tx.Where(models.SystemSetting{Key: k}).Attrs(models.SystemSetting{Value: v}).FirstOrCreate(&setting).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 如果已存在，更新值
		if setting.Value != v {
			setting.Value = v
			setting.UpdatedAt = time.Now()
			if err := tx.Save(&setting).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated"})
}

// generateToken 简单的 Hash 生成 (暂未使用，预留)
func generateToken(input string) string {
	hash := sha256.Sum256([]byte(input + time.Now().String()))
	return hex.EncodeToString(hash[:])
}
