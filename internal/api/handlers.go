package api

import (
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/generator"
	"github.com/wangn9900/StealthForward/internal/models"
	"github.com/wangn9900/StealthForward/internal/sync"
	"gorm.io/gorm"
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

	// 查找域名对应的节点信息，以便后续保存证书内容
	var entry models.EntryNode
	if err := database.DB.Where("domain = ?", req.Domain).First(&entry).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到绑定该域名的节点"})
		return
	}

	// 检查 80 端口是否被占用 (通常是 Nginx 或宝塔)
	ln, lerr := net.Listen("tcp", ":80")
	if lerr != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "端口 80 已被占用（通常是 Nginx 或宝塔）。请先停止占用该端口的服务，或者在宝塔面板手动申请证书并填写路径。",
		})
		return
	}
	ln.Close()

	// 定义证书存放路径
	certDir := "/etc/stealthforward/certs/" + req.Domain
	exec.Command("mkdir", "-p", certDir).Run()

	home, _ := os.UserHomeDir()
	acmePath := home + "/.acme.sh/acme.sh"

	// 方案 A 增强：全自动探测模式
	// 常见的 Webroot 路径（按优先级排列）
	commonWebroots := []string{"/var/www/html", "/usr/share/nginx/html", "/var/www/v2board/public"}
	var finalWebroot string
	for _, path := range commonWebroots {
		if _, err := os.Stat(path); err == nil {
			finalWebroot = path
			break
		}
	}

	var output []byte
	var err error

	// 检查 80 端口是否被占用 (简易检测)
	portInUse := false
	checkCmd := exec.Command("sh", "-c", "lsof -i :80 | grep LISTEN")
	if errCheck := checkCmd.Run(); errCheck == nil {
		portInUse = true
	}

	if portInUse && finalWebroot != "" {
		// 1. 如果 80 占用且有 webroot，走 webroot 模式 (无感)
		output, err = exec.Command(acmePath, "--issue", "-d", req.Domain, "-w", finalWebroot, "--force").CombinedOutput()
	} else if portInUse && finalWebroot == "" {
		// 2. 如果 80 占用但找不到路径，尝试停掉 nginx (后备方案)
		exec.Command("systemctl", "stop", "nginx").Run()
		output, err = exec.Command(acmePath, "--issue", "-d", req.Domain, "--standalone", "--force").CombinedOutput()
		exec.Command("systemctl", "start", "nginx").Run()
	} else {
		// 3. 80 没被占用，直接走 standalone
		output, err = exec.Command(acmePath, "--issue", "-d", req.Domain, "--standalone", "--force").CombinedOutput()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "申请失败，请确保域名解析正确且 80 端口可联通",
			"detail": string(output),
		})
		return
	}

	// 安装证书到指定目录
	certFile := certDir + "/cert.crt"
	keyFile := certDir + "/cert.key"
	_, err = exec.Command(acmePath, "--install-cert", "-d", req.Domain,
		"--fullchain-file", certFile,
		"--key-file", keyFile).CombinedOutput()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "证书下载/安装失败"})
		return
	}

	// 读取物理文件内容并备份到数据库，实现换机无感恢复
	cb, _ := os.ReadFile(certFile)
	kb, _ := os.ReadFile(keyFile)
	entry.CertBody = string(cb)
	entry.KeyBody = string(kb)
	entry.Certificate = certFile
	entry.Key = keyFile
	database.DB.Save(&entry)

	c.JSON(http.StatusOK, gin.H{
		"message": "证书申请成功并已备份到大脑数据库",
		"cert":    certFile,
		"key":     keyFile,
	})
}

// NodeMapping 管理接口
func ListNodeMappingsHandler(c *gin.Context) {
	var mappings []models.NodeMapping
	database.DB.Find(&mappings)
	c.JSON(http.StatusOK, mappings)
}

func CreateNodeMappingHandler(c *gin.Context) {
	var mapping models.NodeMapping
	if err := c.ShouldBindJSON(&mapping); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&mapping)
	// 创建映射后立即尝试同步该节点数据
	sync.GlobalSyncNow()
	c.JSON(http.StatusOK, mapping)
}

func DeleteNodeMappingHandler(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.NodeMapping{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ReportTrafficHandler 接收 Agent 上报的流量数据
func ReportTrafficHandler(c *gin.Context) {
	var report models.NodeTrafficReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将流量数据存入同步模块进行汇总
	sync.CollectTraffic(report)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ExportConfigHandler 导出系统核心配置（备份用）
func ExportConfigHandler(c *gin.Context) {
	var backup struct {
		Entries  []models.EntryNode   `json:"entries"`
		Exits    []models.ExitNode    `json:"exits"`
		Mappings []models.NodeMapping `json:"mappings"`
	}

	database.DB.Find(&backup.Entries)
	database.DB.Find(&backup.Exits)
	database.DB.Find(&backup.Mappings)

	c.JSON(http.StatusOK, backup)
}

// ImportConfigHandler 导入系统核心配置（恢复用）
func ImportConfigHandler(c *gin.Context) {
	var backup struct {
		Entries  []models.EntryNode   `json:"entries"`
		Exits    []models.ExitNode    `json:"exits"`
		Mappings []models.NodeMapping `json:"mappings"`
	}

	if err := c.ShouldBindJSON(&backup); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份文件格式"})
		return
	}

	// 使用事务确保操作安全
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 清空旧数据 (按需)
		tx.Exec("DELETE FROM entry_nodes")
		tx.Exec("DELETE FROM exit_nodes")
		tx.Exec("DELETE FROM node_mappings")
		tx.Exec("DELETE FROM forwarding_rules") // 清空规则，等待下次同步重建

		// 2. 写入新数据
		if len(backup.Entries) > 0 {
			if err := tx.Create(&backup.Entries).Error; err != nil {
				return err
			}
		}
		if len(backup.Exits) > 0 {
			if err := tx.Create(&backup.Exits).Error; err != nil {
				return err
			}
		}
		if len(backup.Mappings) > 0 {
			if err := tx.Create(&backup.Mappings).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复失败: " + err.Error()})
		return
	}

	// 触发一次全量同步
	sync.GlobalSyncNow()

	c.JSON(http.StatusOK, gin.H{"message": "配置恢复成功，已触发全量同步"})
}
