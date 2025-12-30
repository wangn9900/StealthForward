package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nasstoki/stealthforward/internal/api"
	"github.com/nasstoki/stealthforward/internal/database"
)

func main() {
	// 1. 初始化数据库
	database.InitDB()

	// 2. 设置 Gin 路由
	r := gin.Default()

	// 静态文件目录 (用于面板)
	r.Static("/static", "./web/static")
	r.StaticFile("/dashboard", "./web/index.html")
	r.StaticFile("/", "./web/index.html")

	// API 分组
	v1 := r.Group("/api/v1")
	{
		// 节点管理 (Entry)
		v1.GET("/entries", api.ListEntryNodesHandler)
		v1.POST("/entries", api.RegisterNodeHandler)

		// 落地管理 (Exit)
		v1.GET("/exits", api.ListExitNodesHandler)
		v1.POST("/exits", api.CreateExitNodeHandler)

		// 转发链路管理 (Rules)
		v1.GET("/rules", api.ListForwardingRulesHandler)
		v1.POST("/rules", api.CreateForwardingRuleHandler)

		// Agent 获取配置的核心接口
		// 例如: GET /api/v1/node/1/config
		v1.GET("/node/:id/config", api.GetConfigHandler)
	}

	log.Println("StealthForward Controller is running on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
