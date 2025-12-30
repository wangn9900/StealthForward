package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

// V2boardUser 对应 UniProxy 接口返回的用户结�?
type V2boardUser struct {
	ID   uint   `json:"id"`
	UUID string `json:"uuid"`
}

type V2boardResponse struct {
	Data  []V2boardUser `json:"data"`
	Users []V2boardUser `json:"users"` // 适配 V2board 源码中的 users �?
}

// StartV2boardSync 启动一个后台任务，定时同步用户列表
func StartV2boardSync() {
	ticker := time.NewTicker(2 * time.Minute) // �?分钟同步一�?
	go func() {
		for range ticker.C {
			syncAllNodes()
		}
	}()
	// 启动时先同步一�?
	go syncAllNodes()
}

func syncAllNodes() {
	var entries []models.EntryNode
	database.DB.Where("v2board_url <> '' AND v2board_key <> ''").Find(&entries)

	for _, entry := range entries {
		// 如果没填类型，默认为 v2ray
		nodeType := entry.V2boardType
		if nodeType == "" {
			nodeType = "v2ray"
		}
		users, err := fetchUsersFromV2Board(entry.V2boardURL, entry.V2boardKey, entry.V2boardNodeID, nodeType)
		if err != nil {
			log.Printf("同步失败 (Entry #%d): %v", entry.ID, err)
			continue
		}
		updateRulesForEntry(entry, users)
	}
}

func fetchUsersFromV2Board(apiURL, key string, nodeID int, nodeType string) ([]V2boardUser, error) {
	// 如果 URL �?/ 结尾，去掉它
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}

	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/user?node_id=%d&token=%s&node_type=%s", apiURL, nodeID, key, nodeType)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// 打印�?Body，这就是我们要找的“病根�?
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var v2resp V2boardResponse
	if err := json.Unmarshal(body, &v2resp); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	// 兼容不同�?V2Board 版本
	allUsers := append(v2resp.Data, v2resp.Users...)
	return allUsers, nil
}

func updateRulesForEntry(entry models.EntryNode, users []V2boardUser) {
	log.Printf("Entry #%d [%s]: 开始处理同�? 抓取到用户数: %d", entry.ID, entry.Name, len(users))

	targetExit := entry.TargetExitID

	for _, user := range users {
		var rule models.ForwardingRule
		// 改用 UserID (UUID) 作为查重索引
		err := database.DB.Where("user_id = ? AND entry_node_id = ?", user.UUID, entry.ID).First(&rule).Error

		if err != nil {
			newRule := models.ForwardingRule{
				EntryNodeID: entry.ID,
				ExitNodeID:  targetExit,
				UserEmail:   fmt.Sprintf("v2b-%s", user.UUID[:8]), // 源码不返�?Email，用 UUID 前缀占位
				UserID:      user.UUID,
				Enabled:     targetExit != 0,
			}
			database.DB.Create(&newRule)
		} else {
			if rule.ExitNodeID != targetExit {
				rule.ExitNodeID = targetExit
				rule.Enabled = targetExit != 0
				database.DB.Save(&rule)
			}
		}
	}
}

// GlobalSyncNow 提供�?API 调用的立即同步接�?
func GlobalSyncNow() {
	go syncAllNodes()
}
