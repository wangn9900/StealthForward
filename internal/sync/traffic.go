package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

var (
	// userTrafficMap stores UID -> [Upload, Download]
	userTrafficMap sync.Map
	// activeUsers stores UID -> LastSeenTime
	activeUsers sync.Map
)

// CollectTraffic 接收来自 Agent 的流量快照
func CollectTraffic(report models.NodeTrafficReport) {
	for _, t := range report.Traffic {
		lookupEmail := t.UserEmail
		parts := strings.Split(lookupEmail, "-")
		if len(parts) > 1 {
			lookupEmail = parts[len(parts)-1]
		}

		var rule models.ForwardingRule
		err := database.DB.Where("user_email LIKE ?", "%"+lookupEmail+"%").First(&rule).Error
		if err != nil {
			log.Printf("[Traffic] 未识别用户标签 %s", t.UserEmail)
			continue
		}

		if rule.V2boardUID == 0 {
			continue
		}

		// 记录在线状态
		activeUsers.Store(rule.V2boardUID, time.Now())

		// 累加流量 (增量)
		if t.Upload > 0 || t.Download > 0 {
			val, _ := userTrafficMap.LoadOrStore(rule.V2boardUID, &[2]int64{0, 0})
			traffic := val.(*[2]int64)
			atomic.AddInt64(&traffic[0], t.Upload)
			atomic.AddInt64(&traffic[1], t.Download)
		}
	}
}

// StartTrafficReporting 启动心跳和上报任务
func StartTrafficReporting() {
	// 流量与人数合一上报，每 1 分钟执行一次 (配合 V2Board 默认缓存时间)
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			pushTrafficAndOnlineToV2Board()
		}
	}()
}

func pushTrafficAndOnlineToV2Board() {
	var entries []models.EntryNode
	database.DB.Where("v2board_url <> '' AND v2board_key <> ''").Find(&entries)

	now := time.Now()
	for _, entry := range entries {
		// 按 V2Board Node ID 分组的 Payloads
		nodePayloads := make(map[int]map[string][]int64)

		var rules []models.ForwardingRule
		database.DB.Where("entry_node_id = ?", entry.ID).Find(&rules)

		for _, rule := range rules {
			uid := rule.V2boardUID
			if uid == 0 {
				continue
			}

			// 从 UserEmail (标签) 中提取真正的 V2Board 节点 ID
			// 格式: n20-ed296cba
			reportingNodeID := entry.V2boardNodeID // 默认值
			if strings.HasPrefix(rule.UserEmail, "n") && strings.Contains(rule.UserEmail, "-") {
				idPart := strings.Split(rule.UserEmail, "-")[0][1:] // 拿到 "20"
				if id, err := strconv.Atoi(idPart); err == nil {
					reportingNodeID = id
				}
			}

			// 初始化该节点的 PayloadMap
			if _, ok := nodePayloads[reportingNodeID]; !ok {
				nodePayloads[reportingNodeID] = make(map[string][]int64)
			}

			// 获取流量增量
			var u, d int64
			if val, ok := userTrafficMap.Load(uid); ok {
				traffic := val.(*[2]int64)
				u = atomic.SwapInt64(&traffic[0], 0)
				d = atomic.SwapInt64(&traffic[1], 0)
			}

			// 判断是否在线
			isOnline := false
			if lastSeen, ok := activeUsers.Load(uid); ok {
				if now.Sub(lastSeen.(time.Time)) < 3*time.Minute {
					isOnline = true
				} else {
					activeUsers.Delete(uid)
				}
			}

			if isOnline || u > 0 || d > 0 {
				nodePayloads[reportingNodeID][fmt.Sprintf("%d", uid)] = []int64{u, d}
			}
		}

		// 确保 Entry 默认节点和 Mapping 节点都有心跳
		var mappings []models.NodeMapping
		database.DB.Where("entry_node_id = ?", entry.ID).Find(&mappings)

		allTargetNodeIDs := make(map[int]bool)
		if entry.V2boardNodeID != 0 {
			allTargetNodeIDs[entry.V2boardNodeID] = true
		}
		for _, m := range mappings {
			allTargetNodeIDs[m.V2boardNodeID] = true
		}

		for nodeID := range allTargetNodeIDs {
			payload := nodePayloads[nodeID]
			if payload == nil {
				payload = make(map[string][]int64)
			}

			nodeType := entry.V2boardType
			for _, m := range mappings {
				if m.V2boardNodeID == nodeID && m.V2boardType != "" {
					nodeType = m.V2boardType
					break
				}
			}
			if nodeType == "" {
				nodeType = "v2ray"
			}

			err := reportToV2BoardAPIWithID(entry, nodeID, nodeType, payload)
			if err != nil {
				log.Printf("[Traffic] V2Board 同步失败 (Entry #%d, Node #%d): %v", entry.ID, nodeID, err)
			}
		}
	}
}

func reportToV2BoardAPIWithID(entry models.EntryNode, nodeID int, nodeType string, importData map[string][]int64) error {
	apiURL := entry.V2boardURL
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}

	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/push?token=%s&node_id=%d&node_type=%s",
		apiURL, entry.V2boardKey, nodeID, nodeType)

	jsonData, _ := json.Marshal(importData)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(fullURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
