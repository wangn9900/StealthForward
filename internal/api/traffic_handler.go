package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/sync"
)

// GetTrafficStatsHandler 返回按入口节点聚合的流量统计
func GetTrafficStatsHandler(c *gin.Context) {
	stats := sync.GetTrafficStatsByEntry()
	c.JSON(http.StatusOK, stats)
}

// ClearEntryTrafficHandler 清除指定入口节点的流量统计
func ClearEntryTrafficHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entry ID"})
		return
	}

	if err := sync.ClearEntryTraffic(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Traffic cleared for entry node"})
}

// ClearExitTrafficHandler 清除指定落地节点的流量统计
func ClearExitTrafficHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid exit ID"})
		return
	}

	if err := sync.ClearExitTraffic(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Traffic cleared for exit node"})
}

// ClearAllTrafficHandler 清除所有流量统计
func ClearAllTrafficHandler(c *gin.Context) {
	if err := sync.ClearAllTraffic(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All traffic cleared"})
}
