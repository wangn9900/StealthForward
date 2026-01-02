package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/api"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/sync"
)

func main() {
	// 0. 解析参数
	listenAddr := flag.String("addr", ":8080", "Listen address (e.g. :8080 or 127.0.0.1:8080)")
	flag.Parse()

	// 1. 初始化数据库
	database.InitDB()

	// 2. 启动 V2Board 自动同步任务与流量上报任务
	sync.StartV2boardSync()
	sync.StartTrafficReporting()

	// 2. 设置 Gin 路由
	r := gin.Default()

	// --- 鉴权中间件 ---
	adminToken := os.Getenv("STEALTH_ADMIN_TOKEN")
	authMiddleware := func(c *gin.Context) {
		if adminToken != "" {
			token := c.GetHeader("Authorization")
			if token == "" {
				token = c.Query("token")
			}
			if token != adminToken {
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
				return
			}
		}
		c.Next()
	}

	// 存活检查
	r.GET("/api/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	// 静态文件目录 (极致鲁棒探测)
	cwd, _ := os.Getwd()
	searchPaths := []string{
		"./web",
		filepath.Join(cwd, "web"),
		"/usr/local/share/stealthforward/web",
	}

	finalWebRoot := ""
	for _, p := range searchPaths {
		if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
			finalWebRoot, _ = filepath.Abs(p)
			break
		}
	}

	if finalWebRoot != "" {
		log.Printf("成功定位 Web 目录: %s", finalWebRoot)
		r.Static("/static", filepath.Join(finalWebRoot, "static"))
		r.Static("/assets", filepath.Join(finalWebRoot, "assets"))

		// 安全加载安装脚本，防止因文件缺失导致进程崩溃 (502)
		if _, err := os.Stat("./scripts/install.sh"); err == nil {
			r.StaticFile("/install.sh", "./scripts/install.sh")
			r.StaticFile("/static/install.sh", "./scripts/install.sh")
		} else {
			log.Printf("警告: 未找到 ./scripts/install.sh，相关下载链接将不可用")
		}

		r.StaticFile("/dashboard", filepath.Join(finalWebRoot, "index.html"))
		r.StaticFile("/", filepath.Join(finalWebRoot, "index.html"))

		r.NoRoute(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.File(filepath.Join(finalWebRoot, "index.html"))
				return
			}
			c.Status(404)
		})
	} else {
		log.Printf("严重警告: 无法定位 web/index.html，请确保 web 文件夹在运行目录下。")
	}

	// 公开 API
	r.POST("/api/v1/auth/login", api.LoginHandler)

	// API 分组 (Protected)
	v1 := r.Group("/api/v1")
	v1.Use(authMiddleware)
	{
		// 系统设置
		v1.GET("/system/config", api.GetSystemConfigHandler)
		v1.POST("/system/config", api.UpdateSystemConfigHandler)

		// 节点管理 (Entry)
		v1.GET("/entries", api.ListEntryNodesHandler)
		v1.POST("/entries", api.RegisterNodeHandler)
		v1.DELETE("/entries/:id", api.DeleteEntryNodeHandler)
		v1.POST("/entries/issue-cert", api.IssueCertHandler)
		v1.POST("/entries/upload-cert", api.UploadCertHandler) // Agent 申请成功后回传

		// 落地管理 (Exit)
		v1.GET("/exits", api.ListExitNodesHandler)
		v1.POST("/exits", api.CreateExitNodeHandler)
		v1.DELETE("/exits/:id", api.DeleteExitNodeHandler)

		// 转发链路管理 (Rules)
		v1.GET("/rules", api.ListForwardingRulesHandler)
		v1.POST("/rules", api.CreateForwardingRuleHandler)
		v1.DELETE("/rules/:id", api.DeleteForwardingRuleHandler)

		// Agent 获取配置的核心接口
		v1.GET("/node/:id/config", api.GetConfigHandler)
		// Agent 上报流量的接口
		v1.POST("/node/:id/traffic", api.ReportTrafficHandler)

		// Agent 一键换 IP 接口 (AWS Only)
		// --- Cloud Instance Provisioning & Keys ---
		v1.POST("/cloud/instances", api.ProvisionInstanceHandler)
		v1.POST("/cloud/instances/terminate", api.TerminateInstanceHandler)
		v1.GET("/cloud/keys", api.ListKeysHandler)
		v1.GET("/cloud/keys/:name", api.DownloadKeyHandler) // 云平台辅助
		v1.GET("/cloud/regions", api.ListRegionsHandler)
		v1.GET("/cloud/images", api.ListImagesHandler)
		v1.GET("/cloud/instances", api.ListCloudInstancesHandler)
		v1.GET("/cloud/auto-detect", api.AutoDetectInstanceHandler)
		v1.POST("/cloud/rotate-ip", api.RotateIPHandler) // 通用入口

		// --- Lightsail ---
		v1.GET("/cloud/lightsail/regions", api.ListLightsailRegionsHandler)
		v1.GET("/cloud/lightsail/bundles", api.ListLightsailBundlesHandler)
		v1.GET("/cloud/lightsail/blueprints", api.ListLightsailBlueprintsHandler)
		v1.POST("/cloud/lightsail/instances", api.ProvisionLightsailHandler)
		v1.POST("/cloud/lightsail/terminate", api.TerminateLightsailHandler)
		v1.POST("/cloud/lightsail/rotate-ip", api.RotateLightsailIPHandler)

		// --- Traffic Stats ---
		v1.GET("/traffic", api.GetTrafficStatsHandler)

		// 分流映射管理 (NodeMappings)
		v1.GET("/mappings", api.ListNodeMappingsHandler)
		v1.POST("/mappings", api.CreateNodeMappingHandler)
		v1.PUT("/mappings/:id", api.UpdateNodeMappingHandler)
		v1.DELETE("/mappings/:id", api.DeleteNodeMappingHandler)

		// 触发 V2Board 同步
		v1.POST("/sync", api.TriggerSyncHandler)

		// 系统备份与恢复
		v1.GET("/system/backup", api.ExportConfigHandler)
		v1.POST("/system/restore", api.ImportConfigHandler)
	}

	log.Printf("StealthForward Controller is running on %s", *listenAddr)
	if err := r.Run(*listenAddr); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
