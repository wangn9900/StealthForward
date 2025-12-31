package agent

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

// TrafficStorage 存储单个用户的流量
type TrafficStorage struct {
	UpCounter   atomic.Int64
	DownCounter atomic.Int64
}

// HookServer 实现 sing-box 的 ConnectionTracker 接口
type HookServer struct {
	counter sync.Map // map[string]*TrafficStorage
}

func (h *HookServer) ModeList() []string {
	return nil
}

func (h *HookServer) RoutedConnection(ctx context.Context, conn net.Conn, m adapter.InboundContext, rule adapter.Rule, outbound adapter.Outbound) net.Conn {
	if m.User == "" {
		return conn
	}

	val, _ := h.counter.LoadOrStore(m.User, &TrafficStorage{})
	storage := val.(*TrafficStorage)

	return &ConnCounter{
		ExtendedConn: bufio.NewExtendedConn(conn),
		storage:      storage,
	}
}

func (h *HookServer) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, m adapter.InboundContext, rule adapter.Rule, outbound adapter.Outbound) N.PacketConn {
	// UDP 流量统计类似，暂时简化
	return conn
}

// ConnCounter 包装 net.Conn 以统计流量
type ConnCounter struct {
	N.ExtendedConn
	storage *TrafficStorage
}

func (c *ConnCounter) Read(b []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(b)
	c.storage.UpCounter.Add(int64(n)) // 这里的 Up/Down 定义需要跟 V2Board 对齐
	return
}

func (c *ConnCounter) Write(b []byte) (n int, err error) {
	n, err = c.ExtendedConn.Write(b)
	c.storage.DownCounter.Add(int64(n))
	return
}

// GetStats 获取并重置流量统计
func (h *HookServer) GetStats() map[string][2]int64 {
	stats := make(map[string][2]int64)
	h.counter.Range(func(key, value interface{}) bool {
		user := key.(string)
		storage := value.(*TrafficStorage)
		up := storage.UpCounter.Swap(0)
		down := storage.DownCounter.Swap(0)
		if up > 0 || down > 0 {
			stats[user] = [2]int64{up, down}
		}
		return true
	})
	return stats
}
