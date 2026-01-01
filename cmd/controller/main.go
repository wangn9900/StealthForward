package main

import (
	"flag"
	"log"
	"os"

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
			if token != adminToken {
				c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
				return
			}
		}
		c.Next()
	}
	// ----------------

	// 静态文件目录 (用于面板)
	// 增加文件存在性检查，防止 Panic
	if _, err := os.Stat("./web/index.html"); err == nil {
		r.Static("/static", "./web/static")
		r.StaticFile("/dashboard", "./web/index.html")
		r.StaticFile("/", "./web/index.html")
	} else {
		log.Printf("警告: 未找到 Web 面板文件 (./web/index.html)，控制台将不可用。")
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
		v1.POST("/node/:id/rotate-ip", api.RotateIPHandler)

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
