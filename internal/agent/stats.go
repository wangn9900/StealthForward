package agent

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
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

	val, _ := h.counter.LoadOrStore(m.User, &TrafficStorage{})
	storage := val.(*TrafficStorage)

	return &ConnCounter{
		ExtendedConn: bufio.NewExtendedConn(conn),
		storage:      storage,
	}
}

func (h *HookServer) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, m adapter.InboundContext, rule adapter.Rule, outbound adapter.Outbound) N.PacketConn {
	if m.User == "" {
		return conn
	}

	val, _ := h.counter.LoadOrStore(m.User, &TrafficStorage{})
	storage := val.(*TrafficStorage)

	return &PacketConnCounter{
		PacketConn: conn,
		storage:    storage,
	}
}

// ConnCounter 包装 net.Conn 以统计流量 (TCP)
type ConnCounter struct {
	N.ExtendedConn
	storage *TrafficStorage
}

func (c *ConnCounter) Read(b []byte) (n int, err error) {
	n, err = c.ExtendedConn.Read(b)
	if n > 0 {
		c.storage.UpCounter.Add(int64(n))
	}
	return
}

func (c *ConnCounter) Write(b []byte) (n int, err error) {
	n, err = c.ExtendedConn.Write(b)
	if n > 0 {
		c.storage.DownCounter.Add(int64(n))
	}
	return
}

// Override ReadFrom to capture Slice/Sendfile traffic (Download)
func (c *ConnCounter) ReadFrom(r io.Reader) (n int64, err error) {
	if rf, ok := c.ExtendedConn.(io.ReaderFrom); ok {
		n, err = rf.ReadFrom(r)
		if n > 0 {
			c.storage.DownCounter.Add(n)
		}
		return
	}
	return io.Copy(struct{ io.Writer }{c.ExtendedConn}, r)
}

// Override WriteTo to capture Slice/Sendfile traffic (Upload)
func (c *ConnCounter) WriteTo(w io.Writer) (n int64, err error) {
	if wt, ok := c.ExtendedConn.(io.WriterTo); ok {
		n, err = wt.WriteTo(w)
		if n > 0 {
			c.storage.UpCounter.Add(n)
		}
		return
	}
	return io.Copy(w, struct{ io.Reader }{c.ExtendedConn})
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
