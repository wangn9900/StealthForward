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
	// userTrafficMap stores UID -> [Upload, Download] (Deltas for V2Board sync)
	userTrafficMap sync.Map
	// totalTrafficMap stores Tag/UserEmail -> [TotalUpload, TotalDownload] (Lifetime stats for UI)
	totalTrafficMap sync.Map
	// activeUsers stores UserEmail (Tag) -> LastSeenTime
	activeUsers sync.Map
)

// CollectTraffic 接收来自 Agent 的流量快照
func CollectTraffic(report models.NodeTrafficReport) {
	for _, t := range report.Traffic {
		// 精确匹配：必须是这个入口下的这个特定标签 (例如 n21-ed296cba)
		var rule models.ForwardingRule
		err := database.DB.Where("user_email = ? AND entry_node_id = ?", t.UserEmail, report.NodeID).First(&rule).Error
		if err != nil {
			// 兜底：如果完全匹配失败，尝试去掉前缀匹配 UUID (兼容旧版或特殊标签)
			lookupUUID := t.UserEmail
			if parts := strings.Split(t.UserEmail, "-"); len(parts) > 1 {
				lookupUUID = parts[len(parts)-1]
			}
			err = database.DB.Where("user_id = ? AND entry_node_id = ?", lookupUUID, report.NodeID).First(&rule).Error
			if err != nil {
				log.Printf("[Traffic] 无法定位用户规则: %s (Entry #%d)", t.UserEmail, report.NodeID)
				continue
			}
		}

		if rule.V2boardUID == 0 {
			continue
		}

		// 记录在线状态 (使用 Tag 而不是 UID，以便区分不同节点的在线状态)
		activeUsers.Store(rule.UserEmail, time.Now())

		// 累加流量 (增量)
		if t.Upload > 0 || t.Download > 0 {
			// 1. 记录增量 (用于 V2Board 同步, 同步后清零)
			val, _ := userTrafficMap.LoadOrStore(t.UserEmail, &[2]int64{0, 0})
			traffic := val.(*[2]int64)
			atomic.AddInt64(&traffic[0], t.Upload)
			atomic.AddInt64(&traffic[1], t.Download)

			// 2. 记录总量 (用于 UI 展示, 不清零)
			totVal, _ := totalTrafficMap.LoadOrStore(t.UserEmail, &[2]int64{0, 0})
			totalTraffic := totVal.(*[2]int64)
			atomic.AddInt64(&totalTraffic[0], t.Upload)
			atomic.AddInt64(&totalTraffic[1], t.Download)
			// log.Printf("[Debug] 收到用户 %s (UID %d) 流量: Up %d, Down %d", t.UserEmail, rule.V2boardUID, t.Upload, t.Download)
		}
	}
	// log.Printf("[Traffic] 收到 Agent 流量汇报: Node %d, 条目数 %d", report.NodeID, len(report.Traffic))
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

			// 获取流量增量 (使用 UserEmail 作为 Key)
			var u, d int64
			if val, ok := userTrafficMap.Load(rule.UserEmail); ok {
				traffic := val.(*[2]int64)
				u = atomic.SwapInt64(&traffic[0], 0)
				d = atomic.SwapInt64(&traffic[1], 0)
			}

			// 判断是否在线
			isOnline := false
			// 使用 rule.UserEmail (Tag) 检查在线状态，实现分节点在线统计
			if lastSeen, ok := activeUsers.Load(rule.UserEmail); ok {
				if now.Sub(lastSeen.(time.Time)) < 3*time.Minute {
					isOnline = true
				} else {
					activeUsers.Delete(rule.UserEmail)
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
			// 兼容性修复: AnyTLS 本质是本地协议，上报 V2Board 时需映射为标准协议 (默认 vless)
			if nodeType == "anytls" {
				nodeType = "vless"
			}

			var totalUp, totalDown int64
			for _, v := range payload {
				totalUp += v[0]
				totalDown += v[1]
			}

			err := reportToV2BoardAPIWithID(entry, nodeID, nodeType, payload)
			if err != nil {
				log.Printf("[Traffic] V2Board 同步失败 (Entry #%d, Node #%d): %v", entry.ID, nodeID, err)
			} else {
				log.Printf("[Traffic] V2Board 同步成功 (Entry #%d, Node #%d): %d 用户, ↑ %s, ↓ %s",
					entry.ID, nodeID, len(payload), formatBytes(totalUp), formatBytes(totalDown))
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

// GetTrafficStats 返回所有标签的流量总计 map[UserEmail]TrafficStats
func GetTrafficStats() map[string]models.TrafficStat {
	stats := make(map[string]models.TrafficStat)
	totalTrafficMap.Range(func(key, value interface{}) bool {
		userEmail := key.(string)
		counters := value.(*[2]int64)
		stats[userEmail] = models.TrafficStat{
			Upload:   atomic.LoadInt64(&counters[0]),
			Download: atomic.LoadInt64(&counters[1]),
		}
		return true
	})
	return stats
}
