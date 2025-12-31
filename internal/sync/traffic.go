package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		// 1. 收集当前节点的在线用户 (3 分钟内有活动的算在线)
		onlinePayload := make(map[string][]int64)

		var rules []models.ForwardingRule
		database.DB.Where("entry_node_id = ?", entry.ID).Find(&rules)

		onlineCount := 0
		for _, rule := range rules {
			uid := rule.V2boardUID
			if uid == 0 {
				continue
			}

			// 获取该用户的流量增量
			var u, d int64
			if val, ok := userTrafficMap.Load(uid); ok {
				traffic := val.(*[2]int64)
				u = atomic.SwapInt64(&traffic[0], 0) // 读取并重置，确保不重复计算
				d = atomic.SwapInt64(&traffic[1], 0)
			}

			// 判断是否在线
			isOnline := false
			if lastSeen, ok := activeUsers.Load(uid); ok {
				if now.Sub(lastSeen.(time.Time)) < 3*time.Minute {
					isOnline = true
				} else {
					activeUsers.Delete(uid) // 太久没见，清理掉
				}
			}

			// V2Board 的 push 接口逻辑：
			// 如果 data 为空，在线人数为 0
			// 我们把所有在线用户都塞进 data，即使流量为 0，也能统计人数
			if isOnline || u > 0 || d > 0 {
				onlinePayload[fmt.Sprintf("%d", uid)] = []int64{u, d}
				onlineCount++
			}
		}

		// 2. 发送上报 (心跳模式：即便 onlineCount 为 0 也要发，告诉面板节点还活着且 0 人在线)
		err := reportToV2BoardAPI(entry, onlinePayload)
		if err != nil {
			log.Printf("[Traffic] V2Board 同步失败 (Entry #%d): %v", entry.ID, err)
		}
	}
}

func reportToV2BoardAPI(entry models.EntryNode, importData map[string][]int64) error {
	apiURL := entry.V2boardURL
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}

	// node_type 兼容性处理
	nodeType := entry.V2boardType
	if nodeType == "" {
		nodeType = "v2ray"
	}

	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/push?token=%s&node_id=%d&node_type=%s",
		apiURL, entry.V2boardKey, entry.V2boardNodeID, nodeType)

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
