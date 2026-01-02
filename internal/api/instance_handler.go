package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/cloud"
)

// ProvisionInstanceHandler 处理创建 AWS 实例请求
func ProvisionInstanceHandler(c *gin.Context) {
	var req cloud.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 基础校验
	if req.Region == "" {
		req.Region = "ap-northeast-1" // Default
	}
	if req.InstanceType == "" {
		req.InstanceType = "t3.micro"
	}

	// 调用云端逻辑
	// 这是一个耗时操作，建议异步。但在本阶段为了简单直接同步等待
	res, err := cloud.ProvisionInstance(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to provision instance: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
