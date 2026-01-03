package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
	"github.com/wangn9900/StealthForward/internal/remote"
	"github.com/wangn9900/StealthForward/internal/tunnel"
	"gorm.io/gorm"
)

// ... existing handlers ...

// DeployUltraNode 触发 SSH 自动部署逻辑 (彻底隔离 VLESS)
func DeployUltraNode(c *gin.Context) {
	id := c.Param("id")
	var node models.UltraTunnelNode
	if err := database.DB.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	// 1. 获取该机器承担的所有任务 (中转 + 落地)
	var transitRules []models.UltraTunnelRule
	var exitRules []models.UltraTunnelRule
	database.DB.Where("node_id = ?", node.ID).Find(&transitRules)
	database.DB.Where("exit_node_id = ?", node.ID).Find(&exitRules)

	// 2. 构造独立的隧道配置文件
	agentCfg := tunnel.TunnelConfig{Tasks: []tunnel.Task{}}

	// 处理中转任务
	for _, r := range transitRules {
		// 查找对应的落地节点获取 IP
		var exitNode models.UltraTunnelNode
		database.DB.First(&exitNode, r.ExitNodeID)

		agentCfg.Tasks = append(agentCfg.Tasks, tunnel.Task{
			ID:         r.ID,
			Mode:       "transit",
			ListenAddr: fmt.Sprintf("0.0.0.0:%d", r.ListenPort),
			TargetAddr: fmt.Sprintf("%s:%d", exitNode.PublicAddr, r.TunnelPort),
			Key:        r.Key,
		})
	}

	// 处理落地任务
	for _, r := range exitRules {
		agentCfg.Tasks = append(agentCfg.Tasks, tunnel.Task{
			ID:         r.ID,
			Mode:       "exit",
			ListenAddr: fmt.Sprintf("0.0.0.0:%d", r.TunnelPort),
			TargetAddr: r.LocalDest,
			Key:        r.Key,
		})
	}

	// 3. 异步 SSH 部署
	database.DB.Model(&node).Update("status", "deploying")

	go func() {
		client := remote.NewSSHClient(node.SSHHost, node.SSHPort, node.SSHUser, node.SSHPass)

		// 转换配置为 JSON
		cfgJSON, _ := json.MarshalIndent(agentCfg, "", "  ")

		// 通过 SSH 推送配置并重启独立服务
		// 注意：这里用 cat 注入，绝不干扰 /etc/stealthforward (VLESS 目录)
		deployCmd := fmt.Sprintf(`
mkdir -p /etc/stealth-pass
cat << 'EOF' > /etc/stealth-pass/config.json
%s
EOF

# 检查二进制，没有就下载最新的
if [ ! -f /usr/local/bin/stealth-agent ]; then
  curl -L -o /usr/local/bin/stealth-agent http://%s/static/install.sh # 暂时复用，未来改为独立 agent 下载
fi

# 写入独立 Systemd (完全隔离 vless 的服务)
cat << 'EOF' > /etc/systemd/system/stealth-pass.service
[Unit]
Description=StealthPass Tunnel Agent (Isolated)
After=network.target

[Service]
ExecStart=/usr/local/bin/stealth-agent -tunnel /etc/stealth-pass/config.json
Restart=always
WorkingDirectory=/etc/stealth-pass

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable stealth-pass
systemctl restart stealth-pass
`, string(cfgJSON), c.Request.Host)

		output, err := client.Run(deployCmd)
		if err != nil {
			database.DB.Model(&node).Update("status", "error: "+err.Error())
			return
		}

		log.Printf("SSH Isolated Deploy Output for %s: %s", node.Name, output)
		database.DB.Model(&node).Update("status", "online")
	}()

	c.JSON(http.StatusOK, gin.H{"message": "deployment started in background"})
}

// GetUltraLines 获取所有线路
func GetUltraLines(c *gin.Context) {
	var lines []models.UltraTunnelLine
	database.DB.Find(&lines)
	c.JSON(http.StatusOK, lines)
}

// AddUltraLine 添加或更新线路
func AddUltraLine(c *gin.Context) {
	var line models.UltraTunnelLine
	if err := c.ShouldBindJSON(&line); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&line)
	c.JSON(http.StatusOK, line)
}

// GetUltraTunnels 获取所有高级隧道规则
func GetUltraTunnels(c *gin.Context) {
	var rules []models.UltraTunnelRule
	database.DB.Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// GetUltraNodes 获取所有入口机节点
func GetUltraNodes(c *gin.Context) {
	var nodes []models.UltraTunnelNode
	database.DB.Find(&nodes)
	c.JSON(http.StatusOK, nodes)
}

// AddUltraNode 添加中转节点
func AddUltraNode(c *gin.Context) {
	var node models.UltraTunnelNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	node.Status = "offline"
	database.DB.Create(&node)
	c.JSON(http.StatusOK, node)
}

// AddUltraRule 添加隧道规则
func AddUltraRule(c *gin.Context) {
	var rule models.UltraTunnelRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Create(&rule)
	c.JSON(http.StatusOK, rule)
}

// DeleteUltraRule 删除规则
func DeleteUltraRule(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.UltraTunnelRule{}, id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ReportUltraTraffic 接收来自中转机的流量统计 (带倍率计费)
func ReportUltraTraffic(c *gin.Context) {
	var report map[uint]struct {
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for ruleID, t := range report {
		var rule models.UltraTunnelRule
		if err := database.DB.Preload("Line").First(&rule, ruleID).Error; err != nil {
			// 如果规则不存在或被删，直接跳过
			continue
		}

		// 1. 更新规则原始流量 (用于显示在表格里)
		database.DB.Model(&rule).Updates(map[string]interface{}{
			"upload":   gorm.Expr("upload + ?", t.Upload),
			"download": gorm.Expr("download + ?", t.Download),
		})

		// 2. 根据倍率扣除用户额度
		totalDelta := t.Upload + t.Download
		multiplier := 1.0

		// 尝试获取线路倍率
		var line models.UltraTunnelLine
		if database.DB.First(&line, rule.LineID).Error == nil {
			multiplier = line.Price
		}

		multipliedDelta := int64(float64(totalDelta) * multiplier)

		// 3. 更新用户账户
		database.DB.Model(&models.User{}).Where("id = ?", rule.UserID).
			UpdateColumn("used_traffic", gorm.Expr("used_traffic + ?", multipliedDelta))
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
