package api

import (
	"encoding/json"
	"net/http"
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
	// 4. 返回 JSON 响应，包含配置和可能的任务
	c.JSON(http.StatusOK, gin.H{
		"config":    config,
		"cert_task": entry.CertTask,
		"domain":    entry.Domain,
	})
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

	// 后端纠偏：如果 port 为 0，尝试从 Config JSON 里找
	for i := range exits {
		if exits[i].Port == 0 && exits[i].Config != "" {
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(exits[i].Config), &cfg); err == nil {
				// 尝试多个可能的端口字段
				if p, ok := cfg["server_port"].(float64); ok {
					exits[i].Port = int(p)
				} else if p, ok := cfg["port"].(float64); ok {
					exits[i].Port = int(p)
				}
			}
		}
	}
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

// IssueCertHandler 不再直接申请，而是下发任务给 Agent 执行
func IssueCertHandler(c *gin.Context) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var entry models.EntryNode
	if err := database.DB.Where("domain = ?", req.Domain).First(&entry).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到绑定该域名的节点"})
		return
	}

	// 标记该节点有待处理的证书任务
	entry.CertTask = true
	database.DB.Save(&entry)

	c.JSON(http.StatusOK, gin.H{
		"message": "申请指令已下发！中转机将在下次同步时（约1分钟内）自动开始申请。申请成功后证书将自动同步回来。",
	})
}

// UploadCertHandler 供 Agent 申请成功后回传证书内容
func UploadCertHandler(c *gin.Context) {
	var req struct {
		Domain   string `json:"domain"`
		CertBody string `json:"cert_body"`
		KeyBody  string `json:"key_body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}

	var entry models.EntryNode
	if err := database.DB.Where("domain = ?", req.Domain).First(&entry).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	entry.CertBody = req.CertBody
	entry.KeyBody = req.KeyBody
	entry.CertTask = false // 任务完成，清除标志

	// 自动更新路径为 Agent 默认安装路径，让用户在 UI 上无感
	entry.Certificate = "/etc/stealthforward/certs/" + req.Domain + "/cert.crt"
	entry.Key = "/etc/stealthforward/certs/" + req.Domain + "/cert.key"

	database.DB.Save(&entry)

	c.JSON(http.StatusOK, gin.H{"message": "证书备份成功"})
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
