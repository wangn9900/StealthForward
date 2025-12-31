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
		// 1. 首先尝试同步 EntryNode 自身的默认同步字段 (确保主节点用户不丢失)
		if entry.V2boardNodeID != 0 {
			nodeType := entry.V2boardType
			if nodeType == "" {
				nodeType = "v2ray"
			}
			syncSingleTarget(entry, entry.V2boardNodeID, nodeType, entry.TargetExitID)
		}

		// 2. 查找并处理针对该入口的所有特定额外绑定 (NodeMapping)
		var mappings []models.NodeMapping
		database.DB.Where("entry_node_id = ?", entry.ID).Find(&mappings)

		for _, m := range mappings {
			// 如果 Mapping 的节点 ID 和默认 ID 重复，syncSingleTarget 内部会处理覆盖，是安全的
			syncSingleTarget(entry, m.V2boardNodeID, m.V2boardType, m.TargetExitID)
		}
	}
}

// syncSingleTarget 负责执行具体的拉取和更新动作
func syncSingleTarget(entry models.EntryNode, v2bNodeID int, v2bType string, targetExitID uint) {
	log.Printf(">>>> [D-Sync] 正在从 V2Board 拉取: NodeID=%d, Type=%s, URL=%s", v2bNodeID, v2bType, entry.V2boardURL)
	users, err := fetchUsersFromV2Board(entry.V2boardURL, entry.V2boardKey, v2bNodeID, v2bType)
	if err != nil {
		log.Printf("!!!! [D-Sync] 同步失败 (Entry #%d): %v", entry.ID, err)
		return
	}
	log.Printf(">>>> [D-Sync] 成功获取 %d 个用户，准备更新本地规则...", len(users))
	updateRulesForEntry(entry.ID, entry.Name, targetExitID, users)
}

func fetchUsersFromV2Board(apiURL, key string, nodeID int, nodeType string) ([]V2boardUser, error) {
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}
	// 强制要求 nodeType 参与请求
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
		// 尝试兼容某些直接返回数组的旧版本或特殊魔改版
		var directUsers []V2boardUser
		if err2 := json.Unmarshal(body, &directUsers); err2 == nil {
			return directUsers, nil
		}
		return nil, fmt.Errorf("JSON 解析失败 (可能是 API 结构不匹配): %v | 原始数据: %s", err, string(body))
	}

	allUsers := append(v2resp.Data, v2resp.Users...)
	return allUsers, nil
}

func updateRulesForEntry(entryID uint, entryName string, targetExitID uint, users []V2boardUser) {
	log.Printf("Entry #%d [%s]: 处理同步, 节点对应落地ID: %d, 用户数: %d", entryID, entryName, targetExitID, len(users))

	for _, user := range users {
		var rule models.ForwardingRule
		err := database.DB.Where("user_id = ? AND entry_node_id = ?", user.UUID, entryID).First(&rule).Error

		if err != nil {
			newRule := models.ForwardingRule{
				EntryNodeID: entryID,
				ExitNodeID:  targetExitID,
				UserID:      user.UUID,
				V2boardUID:  user.ID,
				UserEmail:   fmt.Sprintf("v2b-%s", user.UUID[:8]),
				Enabled:     targetExitID != 0,
			}
			database.DB.Create(&newRule)
		} else {
			// 更新已有规则，确保 UID 和落地节点同步
			updated := false
			if rule.V2boardUID != user.ID {
				rule.V2boardUID = user.ID
				updated = true
			}
			if rule.ExitNodeID != targetExitID {
				rule.ExitNodeID = targetExitID
				rule.Enabled = targetExitID != 0
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
