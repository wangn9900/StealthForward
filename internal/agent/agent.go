package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/wangn9900/StealthForward/internal/generator"
	"github.com/wangn9900/StealthForward/internal/models"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"
)

type Config struct {
	ControllerAddr string
	NodeID         int
	LocalConfigDir string
	MasqueradeDir  string
	SingBoxPath    string
	AdminToken     string
	UseInternal    bool // 是否使用内置内核 (支持精准流量统计)
}

type Agent struct {
	cfg             Config
	lastConfig      string
	box             *box.Box
	hs              *HookServer
	client          *http.Client
	externalTraffic map[uint][2]int64
	trafficMu       sync.Mutex
}

func NewAgent(cfg Config) *Agent {
	// 确保目录存在
	dirs := []string{cfg.LocalConfigDir, cfg.MasqueradeDir}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			os.MkdirAll(d, 0755)
		}
	}
	a := &Agent{
		cfg:             cfg,
		client:          &http.Client{Timeout: 10 * time.Second},
		externalTraffic: make(map[uint][2]int64),
	}
	// 启动时确保伪装页存在
	a.EnsureMasquerade()
	log.Printf("Masquerade directory: %s", cfg.MasqueradeDir)

	// 启动定时上报任务
	go a.reportTrafficLoop()
	return a
}

// FetchConfig 从 Controller 获取最新的 Sing-box 配置
func (a *Agent) FetchConfig() (string, error) {
	url := fmt.Sprintf("%s/api/v1/node/%d/config", a.cfg.ControllerAddr, a.cfg.NodeID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if a.cfg.AdminToken != "" {
		req.Header.Set("Authorization", a.cfg.AdminToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("controller returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ApplyConfig 将配置保存到本地并尝试重启 Sing-box
func (a *Agent) ApplyConfig(configStr string) error {
	// 如果配置没变，跳过
	if configStr == a.lastConfig {
		return nil
	}

	// 解析配置以处理 Provision 文件下发
	var fullConfig struct {
		Provision map[string]string `json:"provision"`
	}
	if err := json.Unmarshal([]byte(configStr), &fullConfig); err == nil {
		for path, content := range fullConfig.Provision {
			if path == "" {
				continue
			}
			dir := filepath.Dir(path)
			os.MkdirAll(dir, 0755)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				log.Printf("Failed to provision file %s: %v", path, err)
			} else {
				log.Printf("Synthesized missing file from controller: %s", path)
			}
		}
	}

	// 2. 移除 root 级的 provision 字段后再写入文件，防止内核解码失败
	var configMap map[string]interface{}
	finalConfigStr := configStr
	if err := json.Unmarshal([]byte(configStr), &configMap); err == nil {
		delete(configMap, "provision")
		if bytes, err := json.MarshalIndent(configMap, "", "  "); err == nil {
			finalConfigStr = string(bytes)
		}
	}

	configPath := filepath.Join(a.cfg.LocalConfigDir, "config.json")

	// 3. 写入文件
	err := os.WriteFile(configPath, []byte(finalConfigStr), 0644)
	if err != nil {
		return err
	}

	a.lastConfig = configStr
	log.Printf("New config applied to %s", configPath)

	// 3. 重启 Sing-box 服务
	return a.RestartSingBox()
}

func (a *Agent) RestartSingBox() error {
	if a.cfg.UseInternal {
		return a.UpdateInternalCore(a.lastConfig)
	}

	if runtime.GOOS == "windows" {
		log.Println("Windows detected, skipping service restart logic.")
		return nil
	}
	// ... (原逻辑) ...
	// 尝试重启我们的隔离服务
	cmd := exec.Command("systemctl", "restart", "stealth-core")
	if err := cmd.Run(); err != nil {
		log.Printf("Stealth-core restart failed, trying standard sing-box: %v", err)
		// 备选方案：尝试重启标准的 sing-box 服务
		if err := exec.Command("systemctl", "restart", "sing-box").Run(); err != nil {
			log.Printf("Standard sing-box restart also failed, trying direct reload.")
			return exec.Command(a.cfg.SingBoxPath, "check", "-c", filepath.Join(a.cfg.LocalConfigDir, "config.json")).Run()
		}
	}

	log.Println("Sing-box service restarted successfully.")
	return nil
}

func (a *Agent) UpdateInternalCore(configStr string) error {
	ctx := context.Background()
	ctx = box.Context(ctx, include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), include.DNSTransportRegistry(), include.ServiceRegistry())

	options, err := sjson.UnmarshalExtendedContext[option.Options](ctx, []byte(configStr))
	if err != nil {
		return fmt.Errorf("unmarshal config error: %s", err)
	}

	b, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		return fmt.Errorf("create sing-box error: %s", err)
	}

	// 注入我们的统计钩子
	hs := &HookServer{
		counter: sync.Map{},
	}
	b.Router().AppendTracker(hs)

	if a.box != nil {
		a.box.Close()
	}

	err = b.Start()
	if err != nil {
		return fmt.Errorf("start sing-box error: %s", err)
	}

	a.box = b
	a.hs = hs
	log.Println("Internal Sing-box core updated and started with traffic tracking.")
	return nil
}

func (a *Agent) reportTrafficLoop() {
	ticker := time.NewTicker(20 * time.Second) // 加快频率
	// pendingStats: [Email] -> [Up, Down]
	pendingUserStats := make(map[string][2]int64)

	for range ticker.C {
		userTraffic := []models.UserTraffic{}

		// 1. 尝试从内置核心获取用户级流量
		if a.hs != nil {
			newStats := a.hs.GetStats()
			for email, traffic := range newStats {
				val := pendingUserStats[email]
				val[0] += traffic[0]
				val[1] += traffic[1]
				pendingUserStats[email] = val
			}
			for email, traffic := range pendingUserStats {
				userTraffic = append(userTraffic, models.UserTraffic{
					UserEmail: email,
					Upload:    traffic[0],
					Download:  traffic[1],
				})
			}
		}

		// 2. 尝试获取节点级汇总流量 (支持外部魔改内核)
		var nodeUp, nodeDown int64
		// 这里未来可以扩展：通过 ss -ti 实时扫描并将数据存入 a.externalTraffic
		// 暂时先上报已发现的部分
		a.trafficMu.Lock()
		nodeUp = a.externalTraffic[uint(a.cfg.NodeID)][0]
		nodeDown = a.externalTraffic[uint(a.cfg.NodeID)][1]
		// 上报后清空，实现增量上报
		a.externalTraffic[uint(a.cfg.NodeID)] = [2]int64{0, 0}
		a.trafficMu.Unlock()

		if len(userTraffic) == 0 && nodeUp == 0 && nodeDown == 0 {
			continue
		}

		report := models.NodeTrafficReport{
			NodeID:        uint(a.cfg.NodeID),
			Traffic:       userTraffic,
			TotalUpload:   nodeUp,
			TotalDownload: nodeDown,
			Stats:         GetSystemStats(), // 获取并附加系统状态
		}

		jsonData, _ := json.Marshal(report)
		url := fmt.Sprintf("%s/api/v1/node/%d/traffic", a.cfg.ControllerAddr, a.cfg.NodeID)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if a.cfg.AdminToken != "" {
			req.Header.Set("Authorization", a.cfg.AdminToken)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				pendingUserStats = make(map[string][2]int64) // 只有成功才清空
			}
			resp.Body.Close()
		}
	}
}

func (a *Agent) RunOnce() {
	log.Println("Syncing state from controller...")

	// 1. 获取来自控制端的最新数据 (JSON 格式)
	url := fmt.Sprintf("%s/api/v1/node/%d/config", a.cfg.ControllerAddr, a.cfg.NodeID)
	req, _ := http.NewRequest("GET", url, nil)
	if a.cfg.AdminToken != "" {
		req.Header.Set("Authorization", a.cfg.AdminToken)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("Fetch error: %v", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Config   string `json:"config"`
		CertTask bool   `json:"cert_task"`
		Domain   string `json:"domain"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode sync response: %v", err)
		return
	}

	// 2. 检查是否有证书申请任务
	if result.CertTask {
		log.Printf("Received certificate issuance task for domain: %s", result.Domain)
		go a.IssueCertLocally(result.Domain)
	}

	// 3. 应用配置
	if result.Config != "" {
		if err := a.ApplyConfig(result.Config); err != nil {
			log.Printf("Apply error: %v", err)
		}
	}
}

func (a *Agent) IssueCertLocally(domain string) {
	log.Printf("Starting local ACME issuance for %s...", domain)
	home, _ := os.UserHomeDir()
	acmePath := home + "/.acme.sh/acme.sh"

	// 尝试自动匹配宝塔之类的 webroot
	btPath := "/www/wwwroot/" + domain
	webroot := "/var/www/html"
	if _, err := os.Stat(btPath); err == nil {
		webroot = btPath
	}

	// 申请证书：强制指定使用 letsencrypt，避免 ZeroSSL 的 retryafter 86400 坑
	cmd := exec.Command(acmePath, "--issue", "--server", "letsencrypt", "-d", domain, "-w", webroot, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Local cert issuance failed: %v, Output: %s", err, string(output))
		// 如果 letsencrypt 也失败，尝试设置全局默认 CA 再试一次（可选）
		exec.Command(acmePath, "--set-default-ca", "--server", "letsencrypt").Run()
		return
	}

	// 安装证书到本地指定目录
	certDir := "/etc/stealthforward/certs/" + domain
	os.MkdirAll(certDir, 0755)
	certFile := certDir + "/cert.crt"
	keyFile := certDir + "/cert.key"

	cmd = exec.Command(acmePath, "--install-cert", "-d", domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile)
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to install cert: %v", err)
		return
	}

	// 回传给 Controller 备份
	cb, _ := os.ReadFile(certFile)
	kb, _ := os.ReadFile(keyFile)

	uploadURL := fmt.Sprintf("%s/api/v1/entries/upload-cert", a.cfg.ControllerAddr)
	payload := map[string]string{
		"domain":    domain,
		"cert_body": string(cb),
		"key_body":  string(kb),
	}
	jsonPayload, _ := json.Marshal(payload)

	postReq, _ := http.NewRequest("POST", uploadURL, bytes.NewBuffer(jsonPayload))
	if a.cfg.AdminToken != "" {
		postReq.Header.Set("Authorization", a.cfg.AdminToken)
	}
	postReq.Header.Set("Content-Type", "application/json")

	respUpload, err := a.client.Do(postReq)
	if err == nil && respUpload.StatusCode == http.StatusOK {
		log.Printf("Certificate issued and backed up to controller for %s", domain)
	} else {
		log.Printf("Failed to backup certificate to controller")
	}
}

// EnsureMasquerade 检查并生成唯一的伪装页面
func (a *Agent) EnsureMasquerade() {
	indexFile := filepath.Join(a.cfg.MasqueradeDir, "index.html")
	info, err := os.Stat(indexFile)
	if os.IsNotExist(err) || (err == nil && info.Size() < 500) {
		log.Println("Generating unique masquerade site...")
		html := generator.GenerateMasqueradeHTML()
		os.WriteFile(indexFile, []byte(html), 0644)
	}
}

// StartMasqueradeServer 在后台启动一个轻量级的 HTTP 服务器用于回落
func (a *Agent) StartMasqueradeServer(port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("Starting masquerade server on %s", addr)
	fs := http.FileServer(http.Dir(a.cfg.MasqueradeDir))
	http.Handle("/", fs)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Masquerade server error: %v", err)
		}
	}()
}
