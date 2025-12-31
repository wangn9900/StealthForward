package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/wangn9900/StealthForward/internal/database"
	"github.com/wangn9900/StealthForward/internal/models"
)

var (
	// userTrafficMap stores UID -> [Upload, Download]
	userTrafficMap sync.Map
)

// CollectTraffic 接收来自 Agent 的流量快照（通常是 Delta 增量）
func CollectTraffic(report models.NodeTrafficReport) {
	for _, t := range report.Traffic {
		// 根据 Email 查找对应的 V2Board UID
		var rule models.ForwardingRule
		err := database.DB.Where("user_email = ?", t.UserEmail).First(&rule).Error
		if err != nil {
			log.Printf("流量上报失败: 未识别的用户 Email %s", t.UserEmail)
			continue
		}

		if rule.V2boardUID == 0 {
			log.Printf("流量上报跳过: 用户 %s 尚未绑定 V2Board UID", t.UserEmail)
			continue
		}

		// 存入 Map 进行累加
		val, _ := userTrafficMap.LoadOrStore(rule.V2boardUID, &[2]int64{0, 0})
		traffic := val.(*[2]int64)
		traffic[0] += t.Upload
		traffic[1] += t.Download
	}
}

// StartTrafficReporting 启动定时任务，将汇总后的流量提交给 V2Board
func StartTrafficReporting() {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			pushTrafficToV2Board()
		}
	}()
}

func pushTrafficToV2Board() {
	// 获取所有有流量的用户
	count := 0

	// 我们按 EntryNode 进行分组汇报，因为不同入口可能对应不同 V2Board 配置
	var entries []models.EntryNode
	database.DB.Where("v2board_url <> '' AND v2board_key <> ''").Find(&entries)

	for _, entry := range entries {
		// 收集该入口下的流量
		payload := make(map[uint][2]int64)

		// 查找该入口节点关联的规则，从而确定哪些用户属于这个 V2Board 实例
		var rules []models.ForwardingRule
		database.DB.Where("entry_node_id = ?", entry.ID).Find(&rules)

		for _, rule := range rules {
			if val, ok := userTrafficMap.Load(rule.V2boardUID); ok {
				traffic := val.(*[2]int64)
				if traffic[0] > 0 || traffic[1] > 0 {
					payload[rule.V2boardUID] = *traffic
					count++
				}
			}
		}

		if len(payload) > 0 {
			err := reportToV2BoardAPI(entry, payload)
			if err != nil {
				log.Printf("V2Board 流量同步失败 (Entry #%d): %v", entry.ID, err)
			} else {
				// 只有成功了，才从 map 扣除掉已经报上去的量
				for uid := range payload {
					userTrafficMap.Delete(uid)
				}
			}
		}
	}

	if count > 0 {
		log.Printf("已将 %d 个用户的流量数据推送到 V2Board", count)
	}
}

func reportToV2BoardAPI(entry models.EntryNode, payload map[uint][2]int64) error {
	importData := make(map[string][]int64)
	for uid, traffic := range payload {
		importData[fmt.Sprintf("%d", uid)] = []int64{traffic[0], traffic[1]}
	}

	apiURL := entry.V2boardURL
	if len(apiURL) > 0 && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}
	fullURL := fmt.Sprintf("%s/api/v1/server/UniProxy/push?token=%s&node_id=%d&node_type=%s",
		apiURL, entry.V2boardKey, entry.V2boardNodeID, entry.V2boardType)

	jsonData, err := json.Marshal(importData)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
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
