package license

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// 授权服务器地址 - 部署后修改
	DefaultLicenseServer = "https://license.stealthforward.com/api/v1"
	HeartbeatInterval    = 6 * time.Hour
	VerifyTimeout        = 30 * time.Second
	KeyFile              = "data/license.key"
)

// 授权等级
const (
	LevelBasic = "basic"
	LevelPro   = "pro"
	LevelAdmin = "admin"
)

// 协议列表
var (
	BasicProtocols = []string{"anytls"}
	ProProtocols   = []string{"anytls", "vless", "vmess", "trojan", "shadowsocks", "hysteria2"}
	AdminProtocols = []string{"*"} // 全部
)

// Limits 授权限制
type Limits struct {
	Protocols    []string `json:"protocols"`
	MaxEntries   int      `json:"max_entries"`
	MaxExits     int      `json:"max_exits"`
	CloudEnabled bool     `json:"cloud_enabled"`
}

// LicenseInfo 授权信息
type LicenseInfo struct {
	Valid     bool      `json:"valid"`
	Level     string    `json:"level"`
	ExpiresAt time.Time `json:"expires_at"`
	Limits    Limits    `json:"limits"`
	Error     string    `json:"error,omitempty"`
	Signature string    `json:"signature,omitempty"`
}

// VerifyRequest 验证请求
type VerifyRequest struct {
	Key         string `json:"key"`
	IP          string `json:"ip"`
	Fingerprint string `json:"fingerprint"`
	Version     string `json:"version"`
}

var (
	currentLicense *LicenseInfo
	licenseMu      sync.RWMutex
	licenseKey     string
	serverURL      string
	stopChan       chan struct{}
)

// Init 初始化授权模块
func Init() {
	licenseKey = os.Getenv("STEALTH_LICENSE_KEY")
	// 如果环境变量没配，尝试从文件加载
	if licenseKey == "" {
		licenseKey = LoadKey()
	}

	serverURL = os.Getenv("STEALTH_LICENSE_SERVER")
	if serverURL == "" {
		serverURL = DefaultLicenseServer
	}
	stopChan = make(chan struct{})
}

// SetKey 设置内存中的 License Key (用于初次激活)
func SetKey(key string) {
	licenseKey = key
}

// SaveKey 持久化保存 Key
func SaveKey(key string) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	// 同时更新内存
	SetKey(key)
	return os.WriteFile(KeyFile, []byte(key), 0644)
}

const CacheFile = "data/license.cache"

// saveCache 保存授权缓存
func saveCache(info *LicenseInfo) {
	data, _ := json.Marshal(info)
	os.MkdirAll("data", 0755)
	os.WriteFile(CacheFile, data, 0644)
}

// loadCache 加载授权缓存
func loadCache() *LicenseInfo {
	data, err := os.ReadFile(CacheFile)
	if err != nil {
		return nil
	}
	var info LicenseInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	// 校验缓存是否过期
	if time.Now().After(info.ExpiresAt) {
		return nil
	}
	return &info
}

// LoadKey 从文件加载 Key
func LoadKey() string {
	data, err := os.ReadFile(KeyFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetKey 获取当前的 License Key
func GetKey() string {
	return licenseKey
}

// Verify 验证授权
func Verify() error {
	Init()

	if licenseKey == "" {
		return fmt.Errorf("未配置授权Key，请设置环境变量 STEALTH_LICENSE_KEY")
	}

	// 获取服务器信息
	ip := getServerIP()
	fingerprint := getMachineFingerprint()

	// 构建请求
	reqBody := VerifyRequest{
		Key:         licenseKey,
		IP:          ip,
		Fingerprint: fingerprint,
		Version:     "3.5.6",
	}

	jsonData, _ := json.Marshal(reqBody)

	// 发送验证请求
	client := &http.Client{Timeout: VerifyTimeout}
	resp, err := client.Post(
		serverURL+"/license/verify",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		// 网络连接失败，尝试读取本地缓存救援
		if cache := loadCache(); cache != nil {
			log.Printf("⚠️ 无法连接授权服务器 (%v)，即使切换至离线缓存模式 (有效期至 %s)", err, cache.ExpiresAt.Format("2006-01-02"))
			licenseMu.Lock()
			currentLicense = cache
			licenseMu.Unlock()
			return nil
		}
		return fmt.Errorf("无法连接授权服务器且无有效缓存: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var info LicenseInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return fmt.Errorf("授权服务器响应异常: %v", err)
	}

	if !info.Valid {
		errMsg := info.Error
		if errMsg == "" {
			errMsg = "授权无效"
		}
		return fmt.Errorf(errMsg)
	}

	// 检查是否过期
	if time.Now().After(info.ExpiresAt) {
		return fmt.Errorf("授权已过期，过期时间: %s", info.ExpiresAt.Format("2006-01-02"))
	}

	// 保存授权信息
	licenseMu.Lock()
	currentLicense = &info
	licenseMu.Unlock()

	// 更新本地缓存
	saveCache(currentLicense)

	return nil
}

// StartHeartbeat 启动心跳检查
func StartHeartbeat() {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := Verify(); err != nil {
				log.Printf("⚠️ 授权验证失败: %v", err)
				// 连续失败可考虑停止服务
			} else {
				log.Printf("✅ 授权心跳验证成功")
			}
		case <-stopChan:
			return
		}
	}
}

// StopHeartbeat 停止心跳
func StopHeartbeat() {
	close(stopChan)
}

// IsValid 检查授权是否有效
func IsValid() bool {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return false
	}
	return currentLicense.Valid && time.Now().Before(currentLicense.ExpiresAt)
}

// GetLevel 获取授权等级
func GetLevel() string {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return ""
	}
	return currentLicense.Level
}

// GetLimits 获取限制配置
func GetLimits() Limits {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return Limits{}
	}
	return currentLicense.Limits
}

// GetInfo 获取完整授权信息
func GetInfo() *LicenseInfo {
	licenseMu.RLock()
	defer licenseMu.RUnlock()
	return currentLicense
}

// IsProtocolAllowed 检查协议是否允许
func IsProtocolAllowed(protocol string) bool {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return false
	}

	protocol = strings.ToLower(protocol)

	for _, p := range currentLicense.Limits.Protocols {
		if p == "*" || strings.ToLower(p) == protocol {
			return true
		}
	}
	return false
}

// IsCloudEnabled 检查云功能是否启用
func IsCloudEnabled() bool {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return false
	}
	return currentLicense.Limits.CloudEnabled
}

// CanAddEntry 检查是否可以添加入口节点
func CanAddEntry(currentCount int) bool {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return false
	}
	// Admin = 无限
	if currentLicense.Level == LevelAdmin {
		return true
	}
	return currentCount < currentLicense.Limits.MaxEntries
}

// CanAddExit 检查是否可以添加落地节点
func CanAddExit(currentCount int) bool {
	licenseMu.RLock()
	defer licenseMu.RUnlock()

	if currentLicense == nil {
		return false
	}
	if currentLicense.Level == LevelAdmin {
		return true
	}
	return currentCount < currentLicense.Limits.MaxExits
}

// GetDefaultLimits 根据等级返回默认限制（用于离线/测试）
func GetDefaultLimits(level string) Limits {
	switch level {
	case LevelBasic:
		return Limits{
			Protocols:    BasicProtocols,
			MaxEntries:   10,
			MaxExits:     100,
			CloudEnabled: false,
		}
	case LevelPro:
		return Limits{
			Protocols:    ProProtocols,
			MaxEntries:   20,
			MaxExits:     200,
			CloudEnabled: true,
		}
	case LevelAdmin:
		return Limits{
			Protocols:    AdminProtocols,
			MaxEntries:   999999,
			MaxExits:     999999,
			CloudEnabled: true,
		}
	default:
		return Limits{}
	}
}

// --- 辅助函数 ---

// getServerIP 获取服务器公网IP
func getServerIP() string {
	// 尝试从公共服务获取
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		// 回退到本地IP
		return getLocalIP()
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(ip))
}

// getLocalIP 获取本地IP
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}

// getMachineFingerprint 获取机器指纹
func getMachineFingerprint() string {
	// 简化版：使用hostname + 网卡MAC
	hostname, _ := os.Hostname()

	macs := []string{}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			macs = append(macs, iface.HardwareAddr.String())
		}
	}

	data := hostname + "|" + strings.Join(macs, ",")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 前16字节
}

// verifySignature 验证服务器响应签名
func verifySignature(data []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
