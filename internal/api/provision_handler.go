package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
	"github.com/wangn9900/StealthForward/internal/remote"
)

// ReprovisionNodeHandler 触发远程节点的初始化流程 (BBR + 对接)
func ReprovisionNodeHandler(c *gin.Context) {
	id := c.Param("id")
	var entry models.EntryNode
	if err := database.DB.First(&entry, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	if entry.IP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node has no IP, please provision it first"})
		return
	}

	// 1. 获取 SSH 密钥
	var sshKey models.SSHKey
	if err := database.DB.First(&sshKey).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No SSH Provisioning Key found. Please add one in Settings."})
		return
	}

	// 2. 构造对接指令 (由主控地址和 Token 组成)
	// 假设主控公网地址可以通过配置获取，或者从请求头推断 (这里暂时推断)
	host := c.Request.Host
	protocol := "http"
	if c.Request.TLS != nil {
		protocol = "https"
	}
	controllerURL := fmt.Sprintf("%s://%s", protocol, host)

	// 鉴权 Token
	adminToken := "" // 如果有环境变量则使用
	// TODO: 更好地获取 admin token

	// 获取版本号（假设最新）
	version := "v3.3.6"

	// 构造一键安装 & 对接脚本
	// 注意：这里直接下载二进制并运行，不再依赖 install.sh 以提高成功率
	installCmd := fmt.Sprintf(
		"curl -L https://github.com/wangn9900/StealthForward/releases/download/%s/stealth-agent-amd64 -o /usr/local/bin/stealth-agent && "+
			"chmod +x /usr/local/bin/stealth-agent && "+
			"/usr/local/bin/stealth-agent -id %d -controller %s -token %s >> /var/log/stealth-init.log 2>&1 &",
		version, entry.ID, controllerURL, adminToken,
	)

	// 如果用户有特殊的 install.sh 逻辑，也可以考虑用它
	// 但为了 BBR 和 RLimit，我们已经在 internal/remote 里写好了

	// 3. 异步执行
	go func() {
		cfg := remote.ProvisionConfig{
			Host:       entry.IP,
			Port:       22,
			User:       sshKey.User,
			PrivateKey: sshKey.KeyContent,
			AgentCmd:   installCmd,
		}

		if err := remote.RunProvisioning(cfg); err != nil {
			fmt.Printf("[Provision] Failed for node %d: %v\n", entry.ID, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "自动化初始化任务已启动，请等待约 1-2 分钟。可在中转机 /var/log/stealth-init.log 查看进度。",
	})
}
