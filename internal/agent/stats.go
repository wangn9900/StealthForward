package agent

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
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
	// log.Printf("[Debug] Hook TCP for User: %s", m.User)

	val, _ := h.counter.LoadOrStore(m.User, &TrafficStorage{})
	storage := val.(*TrafficStorage)

	// 使用标准 Conn 包装，不透传 SyscallConn，强制禁用 Splice 以捕获在用户态的流量
	return &ConnCounter{
		Conn:    conn,
		storage: storage,
	}
}

func (h *HookServer) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, m adapter.InboundContext, rule adapter.Rule, outbound adapter.Outbound) N.PacketConn {
	if m.User == "" {
		return conn
	}
	// log.Printf("[Debug] Hook UDP for User: %s", m.User)

	val, _ := h.counter.LoadOrStore(m.User, &TrafficStorage{})
	storage := val.(*TrafficStorage)

	return &PacketConnCounter{
		PacketConn: conn,
		storage:    storage,
	}
}

// ConnCounter 包装 net.Conn 以统计流量 (TCP)
// 显式实现 net.Conn 而不是嵌入，以隐藏 ReaderFrom/WriterTo/SyscallConn 接口
// 这会强制 Go 使用标准的 Read/Write 循环，从而确保流量被统计到
type ConnCounter struct {
	net.Conn
	storage *TrafficStorage
}

func (c *ConnCounter) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if n > 0 {
		c.storage.UpCounter.Add(int64(n))
		// log.Printf("TCP Read %d", n)
	}
	return
}

func (c *ConnCounter) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if n > 0 {
		c.storage.DownCounter.Add(int64(n))
		// log.Printf("TCP Write %d", n)
	}
	return
}

// 即使没有嵌入接口，也要重写 ReaderFrom 以防万一 (虽然不嵌入就不会被类型断言成功)
// 但为了保险起见，显式拦截是个好习惯
func (c *ConnCounter) ReadFrom(r io.Reader) (n int64, err error) {
	// 强制降级到 copy loop
	return io.Copy(struct{ io.Writer }{c}, r)
}

// PacketConnCounter 包装 N.PacketConn 以统计流量 (UDP/QUIC)
type PacketConnCounter struct {
	N.PacketConn
	storage *TrafficStorage
}

// ReadPacket captures UDP Upload traffic
func (c *PacketConnCounter) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		c.storage.UpCounter.Add(int64(buffer.Len()))
	}
	return
}

// WritePacket captures UDP Download traffic
func (c *PacketConnCounter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	l := int64(buffer.Len())
	err := c.PacketConn.WritePacket(buffer, destination)
	if err == nil {
		c.storage.DownCounter.Add(l)
		// log.Printf("UDP WritePacket (Down) %d", l)
	}
	return err
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
