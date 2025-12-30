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
	"strings"

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
	configPath := filepath.Join(a.cfg.LocalConfigDir, "config.json")

	// 如果配置没变，跳过
	if configStr == a.lastConfig {
		return nil
	}

	// 1. 验证 JSON 合法性
	var configObj map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &configObj); err != nil {
		return fmt.Errorf("invalid json config: %v", err)
	}

	// 2. 写入文件
	err := os.WriteFile(configPath, []byte(configStr), 0644)
	if err != nil {
		return err
	}

	a.lastConfig = configStr
	log.Printf("New config applied to %s", configPath)

	// 3. 提取域名并配置 Nginx Sniproxy (仅 Linux)
	if runtime.GOOS == "linux" {
		domain := a.extractDomain(configObj)
		if domain != "" {
			a.SetupSniproxy(domain)
		}
	}

	// 4. 重启 Sing-box 服务
	return a.RestartSingBox()
}

func (a *Agent) extractDomain(config map[string]interface{}) string {
	inbounds, ok := config["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		return ""
	}
	for _, in := range inbounds {
		inMap, ok := in.(map[string]interface{})
		if !ok {
			continue
		}
		if inMap["type"] == "vless" {
			tls, ok := inMap["tls"].(map[string]interface{})
			if ok {
				if sn, ok := tls["server_name"].(string); ok {
					return sn
				}
			}
		}
	}
	return ""
}

func (a *Agent) SetupSniproxy(domain string) {
	log.Printf("Setting up Nginx Sniproxy for domain: %s", domain)

	// Nginx Stream 配置块
	streamConf := fmt.Sprintf(`
stream {
    map $ssl_preread_server_name $backend_name {
        %s  singbox_backend;
        default      fallback_backend;
    }

    upstream singbox_backend {
        server 127.0.0.1:8443;
    }

    upstream fallback_backend {
        server 127.0.0.1:8080;
    }

    server {
        listen 443;
        listen [::]:443;
        proxy_pass $backend_name;
        ssl_preread on;
    }
}
`, domain)

	const stealthStreamPath = "/etc/nginx/stealth_stream.conf"
	os.MkdirAll("/etc/nginx", 0755)
	os.WriteFile(stealthStreamPath, []byte(streamConf), 0644)

	// 尝试在主配置中注入 include (如果还没注入)
	nginxMainConf := "/etc/nginx/nginx.conf"
	content, err := os.ReadFile(nginxMainConf)
	if err == nil {
		if !strings.Contains(string(content), "include "+stealthStreamPath) {
			f, err := os.OpenFile(nginxMainConf, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString("\ninclude " + stealthStreamPath + ";\n")
				f.Close()
			}
		}
	}

	exec.Command("systemctl", "reload", "nginx").Run()
}

func (a *Agent) RestartSingBox() error {
	if runtime.GOOS == "windows" {
		log.Println("Windows detected, skipping service restart logic.")
		return nil
	}

	cmd := exec.Command("systemctl", "restart", "sing-box")
	if err := cmd.Run(); err != nil {
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
