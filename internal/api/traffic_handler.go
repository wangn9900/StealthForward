package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/sync"
)

// GetTrafficStatsHandler 返回当前所有节点的流量统计
func GetTrafficStatsHandler(c *gin.Context) {
	stats := sync.GetTrafficStats()
	c.JSON(http.StatusOK, stats)
}
