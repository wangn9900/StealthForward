package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	ServerURL     string `json:"server_url"`
}

// ========== 全局变量 ==========

var db *gorm.DB
var signSecret = "your-secret-key-change-in-production"

const SmartKeySecret = "StealthForward_Smart_License_Key_2025_Secret"

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

	// 生成 Smart Key (如果传入了 Server URL)
	smartKey := ""
	if req.ServerURL != "" {
		smartKey = generateSmartKey(key, req.ServerURL)
	}

	// 手动构造响应 Map，以便添加 smart_key 字段
	resp := gin.H{
		"id":             license.ID,
		"license_key":    license.LicenseKey,
		"level":          license.Level,
		"customer_name":  license.CustomerName,
		"customer_email": license.CustomerEmail,
		"is_active":      license.IsActive,
		"created_at":     license.CreatedAt,
		"expires_at":     license.ExpiresAt,
		"smart_key":      smartKey,
	}

	c.JSON(http.StatusOK, resp)
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

func generateSmartKey(key, url string) string {
	payload := map[string]string{
		"k": key,
		"u": url,
	}
	jsonBytes, _ := json.Marshal(payload)
	encrypted := encryptAES(jsonBytes)
	return "STEALTH-" + base64.StdEncoding.EncodeToString(encrypted)
}

func encryptAES(data []byte) []byte {
	keyHash := sha256.Sum256([]byte(SmartKeySecret))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return data
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return data
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return data
	}

	return gcm.Seal(nonce, nonce, data, nil)
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
        h1 { margin-bottom: 2rem; color: #7c3aed; display: flex; align-items: center; gap: 10px; }
        .card {
            background: rgba(255,255,255,0.05);
            border-radius: 16px;
            padding: 1.5rem;
            margin-bottom: 1rem;
            border: 1px solid rgba(255,255,255,0.1);
            backdrop-filter: blur(10px);
        }
        input, select, button {
            padding: 0.75rem 1rem;
            border-radius: 8px;
            border: 1px solid rgba(255,255,255,0.2);
            background: rgba(255,255,255,0.1);
            color: #fff;
            margin-right: 0.5rem;
            margin-bottom: 0.5rem;
            font-size: 14px;
        }
        input:focus, select:focus {
            outline: none;
            border-color: #7c3aed;
            background: rgba(255,255,255,0.15);
        }
        button {
            background: #7c3aed;
            cursor: pointer;
            border: none;
            font-weight: bold;
            transition: all 0.2s;
        }
        button:hover { background: #6d28d9; transform: translateY(-1px); }
        button:active { transform: translateY(0); }
        .btn-danger { background: #ef4444; }
        .btn-danger:hover { background: #dc2626; }
        .btn-success { background: #10b981; }
        .btn-success:hover { background: #059669; }
        
        table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
        th, td { 
            padding: 1rem; 
            text-align: left; 
            border-bottom: 1px solid rgba(255,255,255,0.1);
        }
        th { color: #9ca3af; font-size: 0.875rem; text-transform: uppercase; letter-spacing: 0.05em; }
        tr:hover td { background: rgba(255,255,255,0.02); }
        
        .badge {
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: bold;
            display: inline-block;
        }
        .badge-basic { background: rgba(59, 130, 246, 0.2); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.4); }
        .badge-pro { background: rgba(124, 58, 237, 0.2); color: #a78bfa; border: 1px solid rgba(124, 58, 237, 0.4); }
        .badge-admin { background: rgba(239, 68, 68, 0.2); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.4); }
        .badge-active { background: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.4); }
        .badge-expired { background: rgba(107, 114, 128, 0.2); color: #9ca3af; border: 1px solid rgba(107, 114, 128, 0.4); }
        
        .code { 
            font-family: 'JetBrains Mono', monospace; 
            background: rgba(0,0,0,0.3);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            color: #e5e7eb;
            font-size: 0.875rem;
            user-select: all;
        }
        .hidden { display: none; }
        .toast {
            position: fixed;
            bottom: 20px;
            right: 20px;
            padding: 1rem 1.5rem;
            background: #1f2937;
            border: 1px solid rgba(255,255,255,0.1);
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
            transform: translateY(100px);
            transition: transform 0.3s;
            z-index: 100;
        }
        .toast.show { transform: translateY(0); }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 StealthForward License Server</h1>
        
        <!-- 登录面板 -->
        <div class="card" id="login-app">
            <h3 style="margin-bottom:1rem">管理员登录</h3>
            <div style="display:flex; gap:10px;">
                <input type="password" id="token" placeholder="请输入管理员Token" style="width:300px" onkeypress="handleEnter(event)">
                <button onclick="login()">登录</button>
            </div>
            <p style="margin-top:1rem; color:#9ca3af; font-size:0.875rem">Token 在安装完成后会显示在终端中。</p>
        </div>

        <!-- 主界面 (默认隐藏) -->
        <div id="main-app" class="hidden">
            <div class="card">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem">
                    <h3>创建新授权</h3>
                    <button onclick="logout()" style="background:rgba(255,255,255,0.1); font-size:0.875rem">退出登录</button>
                </div>
                <div style="display:flex; flex-wrap:wrap; gap:10px; align-items:center;">
                    <select id="new-level">
                        <option value="basic">Basic (基础版)</option>
                        <option value="pro">Pro (专业版)</option>
                        <option value="admin">Admin (管理员)</option>
                    </select>
                    <input type="text" id="new-name" placeholder="客户名称">
                    <input type="email" id="new-email" placeholder="客户邮箱">
                    <div style="display:flex; align-items:center; background:rgba(255,255,255,0.1); border-radius:8px; border:1px solid rgba(255,255,255,0.2); padding-right:10px">
                        <input type="number" id="new-days" placeholder="30" value="30" style="width:70px; border:none; margin:0; background:transparent">
                        <span style="color:#9ca3af">天</span>
                    </div>
                    <button class="btn-success" onclick="createLicense()">生成授权</button>
                </div>
                <p style="margin-top:0.5rem;color:#6b7280;font-size:12px">
                    * 生成的智能Key已自动内置当前服务器地址 (<span id="server-url-display"></span>)
                </p>
            </div>

            <div class="card">
                <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem">
                    <h3>授权列表</h3>
                    <button onclick="loadLicenses()" style="background:transparent; border:1px solid rgba(255,255,255,0.2)">刷新</button>
                </div>
                <div style="overflow-x:auto">
                    <table>
                        <thead>
                            <tr>
                                <th>License Key</th>
                                <th>等级</th>
                                <th>客户信息</th>
                                <th>状态</th>
                                <th>有效期</th>
                                <th>绑定IP</th>
                                <th style="text-align:right">操作</th>
                            </tr>
                        </thead>
                        <tbody id="license-list"></tbody>
                    </table>
                </div>
                <p id="empty-hint" style="text-align:center; color:#6b7280; padding:2rem; display:none">暂无授权数据</p>
            </div>
        </div>
    </div>

    <div id="toast" class="toast"></div>

    <script>
    let currentToken = '';

    // 初始化：检查本地存储
    window.onload = function() {
        const storedToken = localStorage.getItem('sf_admin_token');
        if (storedToken) {
            document.getElementById('token').value = storedToken;
            login(); // 尝试自动登录
        }
        const display = document.getElementById('server-url-display');
        if(display) display.innerText = window.location.origin + "/api/v1";
    }

    function handleEnter(e) {
        if (e.key === 'Enter') login();
    }

    function showToast(msg, type = 'info') {
        const toast = document.getElementById('toast');
        toast.textContent = msg;
        toast.style.borderColor = type === 'error' ? '#ef4444' : '#10b981';
        toast.style.color = type === 'error' ? '#ef4444' : '#fff';
        toast.classList.add('show');
        setTimeout(() => toast.classList.remove('show'), 3000);
    }

    async function login() {
        const token = document.getElementById('token').value.trim();
        if (!token) return showToast('请输入 Token', 'error');

        try {
            const res = await fetch('/api/v1/admin/licenses?token=' + token);
            if (!res.ok) throw new Error('Token 无效或认证失败');
            const data = await res.json();
            
            // 认证成功
            currentToken = token;
            localStorage.setItem('sf_admin_token', token);
            
            document.getElementById('login-app').classList.add('hidden');
            document.getElementById('main-app').classList.remove('hidden');
            renderLicenses(data);
            showToast('登录成功');
        } catch (e) {
            showToast(e.message, 'error');
            localStorage.removeItem('sf_admin_token');
        }
    }

    function logout() {
        currentToken = '';
        localStorage.removeItem('sf_admin_token');
        document.getElementById('main-app').classList.add('hidden');
        document.getElementById('login-app').classList.remove('hidden');
        document.getElementById('token').value = '';
    }

    async function loadLicenses() {
        if (!currentToken) return;
        try {
            const res = await fetch('/api/v1/admin/licenses?token=' + currentToken);
            if (res.status === 401) { logout(); return; }
            if (!res.ok) throw new Error('加载失败');
            const data = await res.json();
            renderLicenses(data);
        } catch (e) {
            showToast(e.message, 'error');
        }
    }

    function renderLicenses(licenses) {
        const tbody = document.getElementById('license-list');
        const emptyHint = document.getElementById('empty-hint');
        
        if (!licenses || licenses.length === 0) {
            tbody.innerHTML = '';
            emptyHint.style.display = 'block';
            return;
        }
        emptyHint.style.display = 'none';

        tbody.innerHTML = licenses.map(l => {
            const isExpired = new Date(l.expires_at) < new Date();
            const statusBadge = isExpired 
                ? '<span class="badge badge-expired">已过期</span>'
                : (l.is_active ? '<span class="badge badge-active">有效</span>' : '<span class="badge badge-expired">已禁用</span>');
            
            const customerInfo = (l.customer_name || '-') + (l.customer_email ? '<br><span style="font-size:12px;color:#9ca3af">' + l.customer_email + '</span>' : '');

            return '<tr>' +
                '<td><span class="code">' + l.license_key + '</span></td>' +
                '<td><span class="badge badge-' + l.level + '">' + l.level.toUpperCase() + '</span></td>' +
                '<td>' + customerInfo + '</td>' +
                '<td>' + statusBadge + '</td>' +
                '<td>' + new Date(l.expires_at).toLocaleDateString() + '</td>' +
                '<td><span class="code" style="font-size:12px">' + (l.bound_ip || '未绑定') + '</span></td>' +
                '<td style="text-align:right">' +
                    '<button class="btn-success" onclick="renewLicense(' + l.id + ')" title="续期30天" style="padding:0.4rem 0.8rem;margin-right:0.5rem">续期</button>' +
                    '<button class="btn-danger" onclick="deleteLicense(' + l.id + ')" title="删除" style="padding:0.4rem 0.8rem">删除</button>' +
                '</td>' +
            '</tr>';
        }).join('');
    }

    async function createLicense() {
        const serverUrl = window.location.origin + "/api/v1";
        const data = {
            level: document.getElementById('new-level').value,
            customer_name: document.getElementById('new-name').value,
            customer_email: document.getElementById('new-email').value,
            duration_days: parseInt(document.getElementById('new-days').value) || 30,
            server_url: serverUrl
        };
        
        try {
            const res = await fetch('/api/v1/admin/licenses?token=' + currentToken, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
            
            if (res.ok) {
                const license = await res.json();
                showToast('创建成功');
                
                const displayKey = license.smart_key || license.license_key;
                const msg = license.smart_key 
                    ? '🔥 创建成功！\n请复制 key 发给客户 (内置服务器地址):\n\n' + displayKey 
                    : '🔥 创建成功！\nLicense Key:\n\n' + displayKey;

                alert(msg);
                loadLicenses();
                // 清空表单
                document.getElementById('new-name').value = '';
                document.getElementById('new-email').value = '';
            } else {
                throw new Error('创建失败');
            }
        } catch (e) {
            showToast(e.message, 'error');
        }
    }

    async function renewLicense(id) {
        try {
            const res = await fetch('/api/v1/admin/licenses/' + id + '/renew?token=' + currentToken, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ days: 30 })
            });
            if (res.ok) {
                showToast('已续期 30 天');
                loadLicenses();
            } else {
                throw new Error('续期失败');
            }
        } catch(e) {
            showToast(e.message, 'error');
        }
    }

    async function deleteLicense(id) {
        if (!confirm('⚠️ 确定要永久删除这个授权吗？此操作不可恢复！')) return;
        try {
            await fetch('/api/v1/admin/licenses/' + id + '?token=' + currentToken, { method: 'DELETE' });
            showToast('已删除');
            loadLicenses();
        } catch(e) {
            showToast('删除失败', 'error');
        }
    }
    </script>
</body>
</html>
`
