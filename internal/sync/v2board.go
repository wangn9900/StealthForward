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

// V2boardUser 对应 UniProxy 接口返回的用户结构
type V2boardUser struct {
	ID   uint   `json:"id"`
	UUID string `json:"uuid"`
}

type V2boardResponse struct {
	Data  []V2boardUser `json:"data"`
	Users []V2boardUser `json:"users"` // 适配 V2board 源码中的 users 键
}

// StartV2boardSync 启动一个后台任务，定时同步用户列表
func StartV2boardSync() {
	ticker := time.NewTicker(2 * time.Minute) // 每 2 分钟同步一次
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
	database.DB.Where("v2board_url <> '' AND v2board_key <> ''").Find(&entries)

	for _, entry := range entries {
		processedUUIDs := make(map[string]bool) // 记录已由 Mapping 处理过的用户，防止被默认规则覆盖

		// 1. 先同步 Mapping 规则 (最高优先级，按 ID 降序排列，让新节点/手动节点优先夺取用户)
		var mappings []models.NodeMapping
		database.DB.Where("entry_node_id = ?", entry.ID).Order("id DESC").Find(&mappings)

		var activeUUIDs []string
		for _, m := range mappings {
			log.Printf(">>>> [D-Sync] 优先分流同步: V2B节点#%d -> 落地ID#%d", m.V2boardNodeID, m.TargetExitID)
			uuids := syncSingleTarget(entry, m.V2boardNodeID, m.V2boardType, m.TargetExitID, processedUUIDs)
			activeUUIDs = append(activeUUIDs, uuids...)
		}

		// 2. 再同步 EntryNode 自身的默认规则 (避开已被映射的用户)
		if entry.V2boardNodeID != 0 {
			nodeType := entry.V2boardType
			if nodeType == "" {
				nodeType = "v2ray"
			}
			uuids := syncSingleTarget(entry, entry.V2boardNodeID, nodeType, entry.TargetExitID, processedUUIDs)
			activeUUIDs = append(activeUUIDs, uuids...)
		}

		// 3. 清理已失效/过期用户
		if len(activeUUIDs) > 0 {
			database.DB.Where("entry_node_id = ? AND user_id NOT IN ?", entry.ID, activeUUIDs).Delete(&models.ForwardingRule{})
		} else {
			database.DB.Where("entry_node_id = ?", entry.ID).Delete(&models.ForwardingRule{})
		}
	}
}

func syncSingleTarget(entry models.EntryNode, v2bNodeID int, v2bType string, targetExitID uint, processed map[string]bool) []string {
	if v2bNodeID <= 0 {
		return nil
	}
	users, err := fetchUsers(entry, v2bNodeID, v2bType)
	if err != nil {
		log.Printf("!!!! [D-Sync] 同步故障 (NodeID %d): %v", v2bNodeID, err)
		return nil
	}

	var filtered []V2boardUser
	for _, u := range users {
		if !processed[u.UUID] {
			filtered = append(filtered, u)
			processed[u.UUID] = true
		}
	}

	if len(filtered) > 0 {
		// 关键修正：传递 v2bNodeID 用于生成统一的识别标签
		updateRulesForEntry(entry.ID, entry.Name, targetExitID, v2bNodeID, filtered)
	}

	uuids := make([]string, len(filtered))
	for i, u := range filtered {
		uuids[i] = u.UUID
	}
	return uuids
}

func fetchUsers(entry models.EntryNode, nodeID int, nodeType string) ([]V2boardUser, error) {
	apiURL := entry.V2boardURL
	key := entry.V2boardKey

	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}
	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/user?node_id=%d&token=%s&node_type=%s", apiURL, nodeID, key, nodeType)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("V2Board 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var v2resp V2boardResponse
	if err := json.Unmarshal(body, &v2resp); err != nil {
		var directUsers []V2boardUser
		if err2 := json.Unmarshal(body, &directUsers); err2 == nil {
			return directUsers, nil
		}
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	return append(v2resp.Data, v2resp.Users...), nil
}

func updateRulesForEntry(entryID uint, entryName string, targetExitID uint, v2bNodeID int, users []V2boardUser) {
	for _, user := range users {
		var rule models.ForwardingRule
		err := database.DB.Where("user_id = ? AND entry_node_id = ?", user.UUID, entryID).First(&rule).Error

		// 核心修正：使用 v2bNodeID 作为标签前缀，与中转机内核上报的标签完全对齐
		identityTag := fmt.Sprintf("n%d-%s", v2bNodeID, user.UUID[:8])

		if err != nil {
			newRule := models.ForwardingRule{
				EntryNodeID: entryID,
				ExitNodeID:  targetExitID,
				UserID:      user.UUID,
				V2boardUID:  user.ID,
				UserEmail:   identityTag,
				Enabled:     true, // 始终启用：只要 V2Board 有此用户，就允许入站。转发去向由 Routing 兜底。
			}
			database.DB.Create(&newRule)
		} else {
			updated := false
			if rule.V2boardUID != user.ID {
				rule.V2boardUID = user.ID
				updated = true
			}
			if rule.ExitNodeID != targetExitID {
				rule.ExitNodeID = targetExitID
				updated = true
			}
			if !rule.Enabled {
				rule.Enabled = true
				updated = true
			}
			if rule.UserEmail != identityTag {
				rule.UserEmail = identityTag
				updated = true
			}
			if updated {
				database.DB.Save(&rule)
			}
		}
	}
}

// GlobalSyncNow 提供给 API 调用的立即同步接口
func GlobalSyncNow() {
	go syncAllNodes()
}
