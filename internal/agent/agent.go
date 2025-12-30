package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/wangn9900/StealthForward/internal/generator"
)

type Config struct {
	ControllerAddr string
	NodeID         int
	LocalConfigDir string
	MasqueradeDir  string
	SingBoxPath    string
	AdminToken     string
}

type Agent struct {
	cfg        Config
	lastConfig string
}

func NewAgent(cfg Config) *Agent {
	// 确保目录存在
	dirs := []string{cfg.LocalConfigDir, cfg.MasqueradeDir}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			os.MkdirAll(d, 0755)
		}
	}
	return &Agent{cfg: cfg}
}

// FetchConfig �?Controller 获取最新的 Sing-box 配置
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

// ApplyConfig 将配置保存到本地并尝试重�?Sing-box
func (a *Agent) ApplyConfig(configStr string) error {
	configPath := filepath.Join(a.cfg.LocalConfigDir, "config.json")

	// 如果配置没变，跳�?
	if configStr == a.lastConfig {
		return nil
	}

	// 1. 验证 JSON 合法�?
	var js json.RawMessage
	if err := json.Unmarshal([]byte(configStr), &js); err != nil {
		return fmt.Errorf("invalid json config: %v", err)
	}

	// 2. 写入文件
	err := os.WriteFile(configPath, []byte(configStr), 0644)
	if err != nil {
		return err
	}

	a.lastConfig = configStr
	log.Printf("New config applied to %s", configPath)

	// 3. 重启 Sing-box 服务 (根据平台不同处理方式不同)
	return a.RestartSingBox()
}

func (a *Agent) RestartSingBox() error {
	if runtime.GOOS == "windows" {
		log.Println("Windows detected, skipping service restart logic.")
		return nil
	}

	// �?Linux 下，我们通常通过 systemd 管理
	// 假设我们的服务名�?stealthforward-singbox
	cmd := exec.Command("systemctl", "restart", "sing-box")
	if err := cmd.Run(); err != nil {
		// 如果没有 systemd，尝试直接重启进程或�?reload
		log.Printf("Systemd restart failed, trying direct reload: %v", err)
		return exec.Command(a.cfg.SingBoxPath, "check", "-c", filepath.Join(a.cfg.LocalConfigDir, "config.json")).Run()
	}

	log.Println("Sing-box service restarted successfully.")
	return nil
}

func (a *Agent) RunOnce() {
	log.Println("Syncing config from controller...")

	// 1. 确保伪装站已生成
	a.EnsureMasquerade()

	// 2. 获取配置
	config, err := a.FetchConfig()
	if err != nil {
		log.Printf("Fetch error: %v", err)
		return
	}

	if err := a.ApplyConfig(config); err != nil {
		log.Printf("Apply error: %v", err)
	}
}

// EnsureMasquerade 检查并生成唯一的伪装页�?
func (a *Agent) EnsureMasquerade() {
	indexFile := filepath.Join(a.cfg.MasqueradeDir, "index.html")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		log.Println("Generating unique masquerade site...")
		html := generator.GenerateMasqueradeHTML()
		os.WriteFile(indexFile, []byte(html), 0644)
	}
}

// StartMasqueradeServer 在后台启动一个轻量级�?HTTP 服务器用于回�?
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
