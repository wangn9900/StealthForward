package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// --- Auth ---

// LoginHandler 简单的登录接口
func LoginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 1. 验证密码
	// 优先从数据库 SystemSetting 获取密码，如果没设，回退到环境变量
	adminToken := os.Getenv("STEALTH_ADMIN_TOKEN") // 环境变量既是 Token 也是初始密码
	// TODO: 未来可以在 SystemSetting 里存加盐的密码 Hash

	// 简单起见，这里假设用户是在用 "admin" 和 环境变量里的 Token 登录
	// 实际生产环境应该生出 Session Token，但这里我们直接让前端存这个 Token 即可
	if req.Username == "admin" && req.Password == adminToken {
		c.JSON(http.StatusOK, gin.H{
			"token": adminToken, // 直接返回，前端存起来放到 Header Authorization
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
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
