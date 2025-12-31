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
	cfg        Config
	lastConfig string
	box        *box.Box
	hs         *HookServer
	client     *http.Client
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
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
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
			dir := filepath.Dir(path)
			os.MkdirAll(dir, 0755)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				log.Printf("Failed to provision file %s: %v", path, err)
			} else {
				log.Printf("Synthesized missing file from controller: %s", path)
			}
		}
	}

	configPath := filepath.Join(a.cfg.LocalConfigDir, "config.json")

	// 2. 写入文件
	err := os.WriteFile(configPath, []byte(configStr), 0644)
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
	cmd := exec.Command("systemctl", "restart", "sing-box")
	if err := cmd.Run(); err != nil {
		log.Printf("Systemd restart failed, trying direct reload: %v", err)
		return exec.Command(a.cfg.SingBoxPath, "check", "-c", filepath.Join(a.cfg.LocalConfigDir, "config.json")).Run()
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
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		if a.hs == nil {
			continue
		}

		stats := a.hs.GetStats()
		if len(stats) == 0 {
			continue
		}

		userTraffic := make([]models.UserTraffic, 0, len(stats))
		for email, traffic := range stats {
			userTraffic = append(userTraffic, models.UserTraffic{
				UserEmail: email,
				Upload:    traffic[0],
				Download:  traffic[1],
			})
		}

		report := models.NodeTrafficReport{
			NodeID:  uint(a.cfg.NodeID),
			Traffic: userTraffic,
		}

		jsonData, _ := json.Marshal(report)
		url := fmt.Sprintf("%s/api/v1/node/%d/traffic", a.cfg.ControllerAddr, a.cfg.NodeID)

		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if a.cfg.AdminToken != "" {
			req.Header.Set("Authorization", a.cfg.AdminToken)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			log.Printf("Traffic report error: %v", err)
			continue
		}
		resp.Body.Close()
	}
}

func (a *Agent) RunOnce() {
	log.Println("Syncing config from controller...")

	// 1. 确保伪装站已生成
	a.EnsureMasquerade()

	// 2. 尝试从本地加载旧配置（仅在第一次运行且内存中无配置时）
	if a.lastConfig == "" {
		configPath := filepath.Join(a.cfg.LocalConfigDir, "config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			log.Println("Detected local config backup, performing offline bootstrap...")
			a.lastConfig = string(data)
			if err := a.RestartSingBox(); err != nil {
				log.Printf("Offline bootstrap failed: %v", err)
				a.lastConfig = "" // 失败了清空，等待从控制端拉取
			} else {
				log.Println("Offline bootstrap success! Service restored using local cache.")
			}
		}
	}

	// 3. 获取来自控制端的最新配置
	config, err := a.FetchConfig()
	if err != nil {
		log.Printf("Fetch error (Controller may be down): %v", err)
		// 如果获取失败且我们已经有了本地配置在跑，那就继续跑，不报错
		return
	}

	if err := a.ApplyConfig(config); err != nil {
		log.Printf("Apply error: %v", err)
	}
}

// EnsureMasquerade 检查并生成唯一的伪装页面
func (a *Agent) EnsureMasquerade() {
	indexFile := filepath.Join(a.cfg.MasqueradeDir, "index.html")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
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
