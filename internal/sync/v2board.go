package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nasstoki/stealthforward/internal/database"
	"github.com/nasstoki/stealthforward/internal/models"
)

// V2boardUser 对应 UniProxy 接口返回的用户结构
type V2boardUser struct {
	ID    uint   `json:"id"`
	UUID  string `json:"uuid"`
	Email string `json:"email"`
}

type V2boardResponse struct {
	Data []V2boardUser `json:"data"`
}

// StartV2boardSync 启动一个后台任务，定时同步用户列表
func StartV2boardSync() {
	ticker := time.NewTicker(2 * time.Minute) // 每2分钟同步一次
	go func() {
		for range ticker.C {
			syncAllNodes()
		}
	}()
	// 启动时先同步一次
	go syncAllNodes()
}

func syncAllNodes() {
	var entries []models.EntryNode
	// 只要配置了 URL 和 KEY 的节点才需要同步
	database.DB.Where("v2board_url <> '' AND v2board_key <> ''").Find(&entries)

	for _, entry := range entries {
		users, err := fetchUsersFromV2Board(entry.V2boardURL, entry.V2boardKey, entry.V2boardNodeID)
		if err != nil {
			log.Printf("同步失败 (Entry #%d): %v", entry.ID, err)
			continue
		}

		// 将获取到的用户写入 ForwardingRule 表
		updateRulesForEntry(entry, users)
	}
}

func fetchUsersFromV2Board(apiURL, key string, nodeID int) ([]V2boardUser, error) {
	// 使用 UniProxy 标准接口
	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/user?node_id=%d&token=%s", apiURL, nodeID, key)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var v2resp V2boardResponse
	if err := json.Unmarshal(body, &v2resp); err != nil {
		return nil, err
	}

	return v2resp.Data, nil
}

func updateRulesForEntry(entry models.EntryNode, users []V2boardUser) {
	log.Printf("Entry #%d [%s]: 开始处理同步, 抓取到用户数: %d", entry.ID, entry.Name, len(users))

	targetExit := entry.TargetExitID

	for _, user := range users {
		var rule models.ForwardingRule
		err := database.DB.Where("user_email = ? AND entry_node_id = ?", user.Email, entry.ID).First(&rule).Error

		if err != nil {
			newRule := models.ForwardingRule{
				EntryNodeID: entry.ID,
				ExitNodeID:  targetExit,
				UserEmail:   user.Email,
				UserID:      user.UUID,
				Enabled:     targetExit != 0,
			}
			database.DB.Create(&newRule)
		} else {
			if rule.UserID != user.UUID || rule.ExitNodeID != targetExit {
				rule.UserID = user.UUID
				rule.ExitNodeID = targetExit
				rule.Enabled = targetExit != 0
				database.DB.Save(&rule)
			}
		}
	}
}

// GlobalSyncNow 提供给 API 调用的立即同步接口
func GlobalSyncNow() {
	go syncAllNodes()
}
