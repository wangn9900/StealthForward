package api

import (
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nasstoki/stealthforward/internal/database"
	"github.com/nasstoki/stealthforward/internal/generator"
	"github.com/nasstoki/stealthforward/internal/models"
	"github.com/nasstoki/stealthforward/internal/sync"
)

// GetConfigHandler 为指定的入口节点生成 Sing-box 配置
func GetConfigHandler(c *gin.Context) {
	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node id"})
		return
	}

	// 1. 获取入口节点信息
	var entry models.EntryNode
	if err := database.DB.First(&entry, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry node not found"})
		return
	}

	// 2. 获取该节点下的所有有效转发规则
	var rules []models.ForwardingRule
	database.DB.Where("entry_node_id = ? AND enabled = ?", nodeID, true).Find(&rules)

	// 3. 获取所有涉及的落地节点
	exitIDs := []uint{}
	if entry.TargetExitID != 0 {
		exitIDs = append(exitIDs, entry.TargetExitID)
	}
	for _, r := range rules {
		exitIDs = append(exitIDs, r.ExitNodeID)
	}

	var exits []models.ExitNode
	if len(exitIDs) > 0 {
		database.DB.Where("id IN ?", exitIDs).Find(&exits)
	}

	// 4. 生成配置
	config, err := generator.GenerateEntryConfig(&entry, rules, exits)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate config"})
		return
	}

	c.String(http.StatusOK, config)
}

func RegisterNodeHandler(c *gin.Context) {
	var entry models.EntryNode
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&entry)
	// 保存成功后立即尝试拉取一次 V2Board 数据
	sync.GlobalSyncNow()
	c.JSON(http.StatusOK, entry)
}

// ExitNode 管理接口
func CreateExitNodeHandler(c *gin.Context) {
	var exit models.ExitNode
	if err := c.ShouldBindJSON(&exit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&exit)
	c.JSON(http.StatusOK, exit)
}

func ListExitNodesHandler(c *gin.Context) {
	var exits []models.ExitNode
	database.DB.Find(&exits)
	c.JSON(http.StatusOK, exits)
}

// ForwardingRule 管理接口 (分流核心)
func CreateForwardingRuleHandler(c *gin.Context) {
	var rule models.ForwardingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 默认设为启用
	rule.Enabled = true
	database.DB.Create(&rule)
	c.JSON(http.StatusOK, rule)
}

func ListForwardingRulesHandler(c *gin.Context) {
	var rules []models.ForwardingRule
	database.DB.Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func ListEntryNodesHandler(c *gin.Context) {
	var entries []models.EntryNode
	database.DB.Find(&entries)
	c.JSON(http.StatusOK, entries)
}

func DeleteEntryNodeHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.EntryNode{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func DeleteExitNodeHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.ExitNode{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func DeleteForwardingRuleHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.ForwardingRule{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func TriggerSyncHandler(c *gin.Context) {
	sync.GlobalSyncNow()
	c.JSON(http.StatusOK, gin.H{"status": "sync triggered"})
}

// IssueCertHandler 使用 acme.sh 为指定域名签发证书
func IssueCertHandler(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	// 定义证书存放路径
	certDir := "/etc/stealthforward/certs/" + req.Domain
	exec.Command("mkdir", "-p", certDir).Run()

	// 执行 acme.sh 签发命令 (使用 standalone 模式，需要 80 端口空闲)
	// 如果服务器没有 acme.sh，脚本安装时应该已经处理或提示
	home, _ := os.UserHomeDir()
	acmePath := home + "/.acme.sh/acme.sh"

	// 1. 尝试签发
	output, err := exec.Command(acmePath, "--issue", "-d", req.Domain, "--standalone", "--force").CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "申请失败，请确保 80 端口未被占用且域名解析正确",
			"detail": string(output),
		})
		return
	}

	// 2. 安装证书到指定目录
	certFile := certDir + "/cert.crt"
	keyFile := certDir + "/cert.key"
	_, err = exec.Command(acmePath, "--install-cert", "-d", req.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile).CombinedOutput()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "证书下载/安装失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "证书申请并安装成功",
		"cert":    certFile,
		"key":     keyFile,
	})
}
