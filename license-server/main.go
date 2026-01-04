package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ========== 数据模型 ==========

// License 授权许可
type License struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	LicenseKey       string     `json:"license_key" gorm:"uniqueIndex;size:64"`
	Level            string     `json:"level"` // basic, pro, admin
	CustomerName     string     `json:"customer_name"`
	CustomerEmail    string     `json:"customer_email"`
	BoundIP          string     `json:"bound_ip"`
	BoundFingerprint string     `json:"bound_fingerprint"`
	IsActive         bool       `json:"is_active" gorm:"default:true"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	LastVerifyAt     *time.Time `json:"last_verify_at"`
	LastVerifyIP     string     `json:"last_verify_ip"`
}

// VerifyLog 验证日志
type VerifyLog struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	LicenseID   uint      `json:"license_id"`
	ClientIP    string    `json:"client_ip"`
	Fingerprint string    `json:"fingerprint"`
	Result      string    `json:"result"` // success, expired, invalid, ip_mismatch
	CreatedAt   time.Time `json:"created_at"`
}

// ========== 授权等级配置 ==========

type Limits struct {
	Protocols    []string `json:"protocols"`
	MaxEntries   int      `json:"max_entries"`
	MaxExits     int      `json:"max_exits"`
	CloudEnabled bool     `json:"cloud_enabled"`
}

var levelLimits = map[string]Limits{
	"basic": {
		Protocols:    []string{"anytls"},
		MaxEntries:   10,
		MaxExits:     100,
		CloudEnabled: false,
	},
	"pro": {
		Protocols:    []string{"anytls", "vless", "vmess", "trojan", "shadowsocks", "hysteria2"},
		MaxEntries:   20,
		MaxExits:     200,
		CloudEnabled: true,
	},
	"admin": {
		Protocols:    []string{"*"},
		MaxEntries:   999999,
		MaxExits:     999999,
		CloudEnabled: true,
	},
}

// ========== API 请求/响应 ==========

type VerifyRequest struct {
	Key         string `json:"key"`
	IP          string `json:"ip"`
	Fingerprint string `json:"fingerprint"`
	Version     string `json:"version"`
}

type VerifyResponse struct {
	Valid     bool      `json:"valid"`
	Level     string    `json:"level,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Limits    *Limits   `json:"limits,omitempty"`
	Error     string    `json:"error,omitempty"`
	Signature string    `json:"signature,omitempty"`
}

type CreateLicenseRequest struct {
	Level         string `json:"level"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
	DurationDays  int    `json:"duration_days"`
}

// ========== 全局变量 ==========

var db *gorm.DB
var signSecret = "your-secret-key-change-in-production"

func main() {
	// 初始化数据库
	var err error
	db, err = gorm.Open(sqlite.Open("license.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// 自动迁移
	db.AutoMigrate(&License{}, &VerifyLog{})

	// 读取签名密钥
	if secret := os.Getenv("LICENSE_SECRET"); secret != "" {
		signSecret = secret
	}

	// 设置路由
	r := gin.Default()

	// 公开API
	r.POST("/api/v1/license/verify", verifyHandler)
	r.POST("/api/v1/license/heartbeat", heartbeatHandler)

	// 管理API (需要管理员Token)
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuth())
	{
		admin.GET("/licenses", listLicensesHandler)
		admin.POST("/licenses", createLicenseHandler)
		admin.PUT("/licenses/:id", updateLicenseHandler)
		admin.DELETE("/licenses/:id", deleteLicenseHandler)
		admin.POST("/licenses/:id/renew", renewLicenseHandler)
	}

	// 简易管理页面
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(200, adminPageHTML)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	log.Printf("License Server running on :%s", port)
	r.Run(":" + port)
}

// ========== 验证处理 ==========

func verifyHandler(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, VerifyResponse{Valid: false, Error: "invalid request"})
		return
	}

	// 查找License
	var license License
	if err := db.Where("license_key = ?", req.Key).First(&license).Error; err != nil {
		logVerify(0, c.ClientIP(), req.Fingerprint, "invalid")
		c.JSON(http.StatusOK, VerifyResponse{Valid: false, Error: "license_invalid"})
		return
	}

	// 检查是否激活
	if !license.IsActive {
		logVerify(license.ID, c.ClientIP(), req.Fingerprint, "disabled")
		c.JSON(http.StatusOK, VerifyResponse{Valid: false, Error: "license_disabled"})
		return
	}

	// 检查是否过期
	if time.Now().After(license.ExpiresAt) {
		logVerify(license.ID, c.ClientIP(), req.Fingerprint, "expired")
		c.JSON(http.StatusOK, VerifyResponse{
			Valid:     false,
			Error:     "license_expired",
			ExpiresAt: license.ExpiresAt,
		})
		return
	}

	// IP绑定检查（首次使用自动绑定）
	if license.BoundIP == "" {
		// 首次激活，绑定IP
		license.BoundIP = req.IP
		license.BoundFingerprint = req.Fingerprint
		db.Save(&license)
	} else if license.BoundIP != req.IP {
		// IP不匹配
		logVerify(license.ID, c.ClientIP(), req.Fingerprint, "ip_mismatch")
		c.JSON(http.StatusOK, VerifyResponse{
			Valid: false,
			Error: fmt.Sprintf("ip_mismatch: bound to %s", maskIP(license.BoundIP)),
		})
		return
	}

	// 更新最后验证时间
	now := time.Now()
	license.LastVerifyAt = &now
	license.LastVerifyIP = req.IP
	db.Save(&license)

	// 记录成功日志
	logVerify(license.ID, c.ClientIP(), req.Fingerprint, "success")

	// 获取等级限制
	limits := levelLimits[license.Level]

	// 生成签名
	sigData := fmt.Sprintf("%s|%s|%d", license.LicenseKey, license.Level, license.ExpiresAt.Unix())
	signature := signData(sigData)

	c.JSON(http.StatusOK, VerifyResponse{
		Valid:     true,
		Level:     license.Level,
		ExpiresAt: license.ExpiresAt,
		Limits:    &limits,
		Signature: signature,
	})
}

func heartbeatHandler(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid request"})
		return
	}

	var license License
	if err := db.Where("license_key = ?", req.Key).First(&license).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "invalid"})
		return
	}

	// 更新心跳
	now := time.Now()
	license.LastVerifyAt = &now
	db.Save(&license)

	c.JSON(http.StatusOK, gin.H{
		"ok":         license.IsActive && time.Now().Before(license.ExpiresAt),
		"next_check": 21600, // 6小时
	})
}

// ========== 管理API ==========

func adminAuth() gin.HandlerFunc {
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		adminToken = "admin123" // 默认密码，生产环境务必修改！
	}

	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token != adminToken {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func listLicensesHandler(c *gin.Context) {
	var licenses []License
	db.Order("created_at DESC").Find(&licenses)
	c.JSON(http.StatusOK, licenses)
}

func createLicenseHandler(c *gin.Context) {
	var req CreateLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Level == "" {
		req.Level = "basic"
	}
	if req.DurationDays == 0 {
		req.DurationDays = 30
	}

	// 生成License Key
	key := generateLicenseKey(req.Level)

	license := License{
		LicenseKey:    key,
		Level:         req.Level,
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,
		IsActive:      true,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().AddDate(0, 0, req.DurationDays),
	}

	if err := db.Create(&license).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, license)
}

func updateLicenseHandler(c *gin.Context) {
	id := c.Param("id")
	var license License
	if err := db.First(&license, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var update struct {
		Level         string `json:"level"`
		CustomerName  string `json:"customer_name"`
		CustomerEmail string `json:"customer_email"`
		IsActive      *bool  `json:"is_active"`
		BoundIP       string `json:"bound_ip"`
	}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if update.Level != "" {
		license.Level = update.Level
	}
	if update.CustomerName != "" {
		license.CustomerName = update.CustomerName
	}
	if update.CustomerEmail != "" {
		license.CustomerEmail = update.CustomerEmail
	}
	if update.IsActive != nil {
		license.IsActive = *update.IsActive
	}
	if update.BoundIP != "" {
		license.BoundIP = update.BoundIP
	}

	db.Save(&license)
	c.JSON(http.StatusOK, license)
}

func deleteLicenseHandler(c *gin.Context) {
	id := c.Param("id")
	db.Delete(&License{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func renewLicenseHandler(c *gin.Context) {
	id := c.Param("id")
	var license License
	if err := db.First(&license, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	var req struct {
		Days int `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days == 0 {
		req.Days = 30
	}

	// 从过期时间或当前时间（取较晚者）开始续期
	baseTime := license.ExpiresAt
	if time.Now().After(baseTime) {
		baseTime = time.Now()
	}
	license.ExpiresAt = baseTime.AddDate(0, 0, req.Days)
	license.IsActive = true

	db.Save(&license)
	c.JSON(http.StatusOK, gin.H{
		"message":    "续期成功",
		"expires_at": license.ExpiresAt,
	})
}

// ========== 辅助函数 ==========

func generateLicenseKey(level string) string {
	prefix := "SF"
	switch level {
	case "basic":
		prefix = "SF-B"
	case "pro":
		prefix = "SF-P"
	case "admin":
		prefix = "SF-A"
	}

	// 生成随机部分
	bytes := make([]byte, 12)
	rand.Read(bytes)
	randomPart := strings.ToUpper(hex.EncodeToString(bytes)[:16])

	// 格式化为 SF-B-XXXX-XXXX-XXXX-XXXX
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		prefix,
		randomPart[0:4],
		randomPart[4:8],
		randomPart[8:12],
		randomPart[12:16],
	)
}

func signData(data string) string {
	mac := hmac.New(sha256.New, []byte(signSecret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func maskIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	return ip[:len(ip)/2] + "***"
}

func logVerify(licenseID uint, clientIP, fingerprint, result string) {
	log := VerifyLog{
		LicenseID:   licenseID,
		ClientIP:    clientIP,
		Fingerprint: fingerprint,
		Result:      result,
		CreatedAt:   time.Now(),
	}
	db.Create(&log)
}

// ========== 管理页面 ==========

var adminPageHTML = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>StealthForward License Server</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            min-height: 100vh;
            color: #eee;
            padding: 2rem;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { margin-bottom: 2rem; color: #7c3aed; }
        .card {
            background: rgba(255,255,255,0.05);
            border-radius: 16px;
            padding: 1.5rem;
            margin-bottom: 1rem;
            border: 1px solid rgba(255,255,255,0.1);
        }
        input, select, button {
            padding: 0.75rem 1rem;
            border-radius: 8px;
            border: 1px solid rgba(255,255,255,0.2);
            background: rgba(255,255,255,0.1);
            color: #fff;
            margin-right: 0.5rem;
            margin-bottom: 0.5rem;
        }
        button {
            background: #7c3aed;
            cursor: pointer;
            border: none;
            font-weight: bold;
        }
        button:hover { background: #6d28d9; }
        .btn-danger { background: #dc2626; }
        .btn-success { background: #059669; }
        table { width: 100%; border-collapse: collapse; }
        th, td { 
            padding: 1rem; 
            text-align: left; 
            border-bottom: 1px solid rgba(255,255,255,0.1);
        }
        .badge {
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: bold;
        }
        .badge-basic { background: #3b82f6; }
        .badge-pro { background: #7c3aed; }
        .badge-admin { background: #dc2626; }
        .badge-active { background: #059669; }
        .badge-expired { background: #6b7280; }
        .code { 
            font-family: monospace; 
            background: rgba(0,0,0,0.3);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
        }
        #token-form { margin-bottom: 2rem; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 StealthForward License Server</h1>
        
        <div class="card" id="token-form">
            <input type="password" id="token" placeholder="管理员Token" style="width:300px">
            <button onclick="loadLicenses()">登录</button>
        </div>

        <div class="card">
            <h3 style="margin-bottom:1rem">创建新授权</h3>
            <select id="new-level">
                <option value="basic">Basic (基础版)</option>
                <option value="pro">Pro (专业版)</option>
                <option value="admin">Admin (管理员)</option>
            </select>
            <input type="text" id="new-name" placeholder="客户名称">
            <input type="email" id="new-email" placeholder="客户邮箱">
            <input type="number" id="new-days" placeholder="有效天数" value="30" style="width:100px">
            <button onclick="createLicense()">生成授权</button>
        </div>

        <div class="card">
            <h3 style="margin-bottom:1rem">授权列表</h3>
            <table>
                <thead>
                    <tr>
                        <th>License Key</th>
                        <th>等级</th>
                        <th>客户</th>
                        <th>状态</th>
                        <th>到期时间</th>
                        <th>绑定IP</th>
                        <th>操作</th>
                    </tr>
                </thead>
                <tbody id="license-list"></tbody>
            </table>
        </div>
    </div>

    <script>
    function getToken() {
        return document.getElementById('token').value;
    }

    async function loadLicenses() {
        try {
            const res = await fetch('/api/v1/admin/licenses?token=' + getToken());
            if (!res.ok) throw new Error('认证失败');
            const data = await res.json();
            renderLicenses(data);
        } catch (e) {
            alert(e.message);
        }
    }

    function renderLicenses(licenses) {
        const tbody = document.getElementById('license-list');
        tbody.innerHTML = licenses.map(l => {
            const isExpired = new Date(l.expires_at) < new Date();
            const statusBadge = isExpired 
                ? '<span class="badge badge-expired">已过期</span>'
                : (l.is_active ? '<span class="badge badge-active">有效</span>' : '<span class="badge badge-expired">已禁用</span>');
            
            return '<tr>' +
                '<td><span class="code">' + l.license_key + '</span></td>' +
                '<td><span class="badge badge-' + l.level + '">' + l.level.toUpperCase() + '</span></td>' +
                '<td>' + (l.customer_name || '-') + '</td>' +
                '<td>' + statusBadge + '</td>' +
                '<td>' + new Date(l.expires_at).toLocaleDateString() + '</td>' +
                '<td>' + (l.bound_ip || '未绑定') + '</td>' +
                '<td>' +
                    '<button class="btn-success" onclick="renewLicense(' + l.id + ')">续期30天</button>' +
                    '<button class="btn-danger" onclick="deleteLicense(' + l.id + ')">删除</button>' +
                '</td>' +
            '</tr>';
        }).join('');
    }

    async function createLicense() {
        const data = {
            level: document.getElementById('new-level').value,
            customer_name: document.getElementById('new-name').value,
            customer_email: document.getElementById('new-email').value,
            duration_days: parseInt(document.getElementById('new-days').value) || 30
        };
        
        const res = await fetch('/api/v1/admin/licenses?token=' + getToken(), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        
        if (res.ok) {
            const license = await res.json();
            alert('创建成功！\n\nLicense Key:\n' + license.license_key);
            loadLicenses();
        } else {
            alert('创建失败');
        }
    }

    async function renewLicense(id) {
        const res = await fetch('/api/v1/admin/licenses/' + id + '/renew?token=' + getToken(), {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ days: 30 })
        });
        if (res.ok) {
            alert('续期成功');
            loadLicenses();
        }
    }

    async function deleteLicense(id) {
        if (!confirm('确定要删除这个授权吗？')) return;
        await fetch('/api/v1/admin/licenses/' + id + '?token=' + getToken(), { method: 'DELETE' });
        loadLicenses();
    }
    </script>
</body>
</html>
`
