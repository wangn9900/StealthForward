package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/api"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/sync"
)

func main() {
	// 1. 初始化数据库
	database.InitDB()

	// 2. 启动 V2Board 自动同步任务
	sync.StartV2boardSync()

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

	// API 分组
	v1 := r.Group("/api/v1")
	v1.Use(authMiddleware)
	{
		// 节点管理 (Entry)
		v1.GET("/entries", api.ListEntryNodesHandler)
		v1.POST("/entries", api.RegisterNodeHandler)
		v1.DELETE("/entries/:id", api.DeleteEntryNodeHandler)
		v1.POST("/entries/issue-cert", api.IssueCertHandler)

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

		// 分流映射管理 (NodeMappings)
		v1.GET("/mappings", api.ListNodeMappingsHandler)
		v1.POST("/mappings", api.CreateNodeMappingHandler)
		v1.DELETE("/mappings/:id", api.DeleteNodeMappingHandler)

		// 触发 V2Board 同步
		v1.POST("/sync", api.TriggerSyncHandler)
	}

	log.Println("StealthForward Controller is running on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
