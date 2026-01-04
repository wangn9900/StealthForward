package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/cloud"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// ProvisionInstanceHandler 处理创建 AWS 实例请求
func ProvisionInstanceHandler(c *gin.Context) {
	// === 授权检查：云功能 ===
	if !CheckCloudEnabled(c) {
		return
	}

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

// TerminateInstanceHandler 处理销毁实例请求
type TerminateInstanceRequest struct {
	Region     string `json:"region"`
	InstanceID string `json:"instance_id"`
}

func TerminateInstanceHandler(c *gin.Context) {
	// === 授权检查：云功能 ===
	if !CheckCloudEnabled(c) {
		return
	}

	var req TerminateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Region == "" || req.InstanceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Region and InstanceID are required"})
		return
	}

	if err := cloud.TerminateInstance(c.Request.Context(), req.Region, req.InstanceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terminate failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Termination signal sent"})
}

// --- Key Management ---

type KeyFile struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updated_at"`
}

func ListKeysHandler(c *gin.Context) {
	files, err := os.ReadDir("store/keys")
	if err != nil {
		// create dir if not exists
		os.MkdirAll("store/keys", 0700)
		c.JSON(http.StatusOK, []KeyFile{})
		return
	}

	var keys []KeyFile
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".pem") {
			info, _ := f.Info()
			keys = append(keys, KeyFile{
				Name:      f.Name(),
				Size:      info.Size(),
				UpdatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}
	c.JSON(http.StatusOK, keys)
}

func DownloadKeyHandler(c *gin.Context) {
	name := c.Param("name")
	// Basic security check to prevent directory traversal
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}

	target := filepath.Join("store", "keys", name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key file not found"})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.Header("Content-Type", "application/x-pem-file")
	c.Header("Content-Type", "application/x-pem-file")
	c.File(target)
}

func ListRegionsHandler(c *gin.Context) {
	regions, err := cloud.ListRegions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list regions: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, regions)
}

func ListImagesHandler(c *gin.Context) {
	region := c.Query("region")
	if region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "region is required"})
		return
	}

	images, err := cloud.ListFeaturedImages(c.Request.Context(), region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list images: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, images)
}

// --- Lightsail Handlers ---

func ListLightsailRegionsHandler(c *gin.Context) {
	regions, err := cloud.ListLightsailRegions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, regions)
}

func ListLightsailBundlesHandler(c *gin.Context) {
	region := c.Query("region")
	if region == "" {
		region = "us-east-1"
	}
	bundles, err := cloud.ListLightsailBundles(c.Request.Context(), region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundles)
}

func ListLightsailBlueprintsHandler(c *gin.Context) {
	region := c.Query("region")
	if region == "" {
		region = "us-east-1"
	}
	blueprints, err := cloud.ListLightsailBlueprints(c.Request.Context(), region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, blueprints)
}

func ProvisionLightsailHandler(c *gin.Context) {
	// === 授权检查：云功能 ===
	if !CheckCloudEnabled(c) {
		return
	}

	var req cloud.CreateLightsailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := cloud.ProvisionLightsailInstance(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// Reuse existing terminate handler? Or new one?
// Existing handler takes "instance_id". Lightsail uses "instance_name".
// We can overload or make new. Let's make new for clarity.

func TerminateLightsailHandler(c *gin.Context) {
	// === 授权检查：云功能 ===
	if !CheckCloudEnabled(c) {
		return
	}

	var req struct {
		Region       string `json:"region"`
		InstanceName string `json:"instance_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := cloud.TerminateLightsailInstance(c.Request.Context(), req.Region, req.InstanceName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func RotateLightsailIPHandler(c *gin.Context) {
	var req struct {
		Region       string `json:"region"`
		InstanceName string `json:"instance_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newIP, err := cloud.RotateLightsailIP(c.Request.Context(), req.Region, req.InstanceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "new_ip": newIP})
}

// ListCloudInstancesHandler 列出选定区域和供应商的所有实例
func ListCloudInstancesHandler(c *gin.Context) {
	provider := c.Query("provider")
	region := c.Query("region")

	if region == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要选择区域"})
		return
	}

	var instances []cloud.CloudInstance
	var err error

	if provider == "aws_lightsail" {
		instances, err = cloud.ListLightsailInstances(c.Request.Context(), region)
	} else {
		instances, err = cloud.ListEC2Instances(c.Request.Context(), region)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, instances)
}

// AutoDetectInstanceHandler 根据 IP 自动检测绑定的云平台
func AutoDetectInstanceHandler(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供 IP 地址"})
		return
	}

	provider, region, instanceID, err := cloud.AutoDetectCloudInstance(c.Request.Context(), ip)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "识别失败: " + err.Error()})
		return
	}

	// 查找是否已经有绑定的域名？
	var recordName string
	var entry models.EntryNode
	if err := database.DB.Where("ip = ?", ip).First(&entry).Error; err == nil {
		recordName = entry.CloudRecordName
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":    provider,
		"region":      region,
		"instance_id": instanceID,
		"record_name": recordName,
	})
}
