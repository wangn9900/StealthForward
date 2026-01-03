package tunnel

import (
	"context"
	"io"
	"log"
	"net"
	"sync/atomic"
)

// TrafficCounter 记录单条隧道的流量
type TrafficCounter struct {
	Upload   int64
	Download int64
}

type TransitServer struct {
	RuleID     uint
	ListenAddr string
	TargetAddr string
	Key        string
	Counter    *TrafficCounter
}

func (t *TransitServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("[Transit] Rule #%d 正在监听 %s", t.RuleID, t.ListenAddr)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Printf("[Transit] Accept 错误: %v", err)
				continue
			}
		}
		go t.handle(clientConn)
	}
}

func (t *TransitServer) handle(clientConn net.Conn) {
	defer clientConn.Close()

	remoteConn, err := net.Dial("tcp", t.TargetAddr)
	if err != nil {
		log.Printf("[Transit] 无法连接落地机 %s: %v", t.TargetAddr, err)
		return
	}
	defer remoteConn.Close()

	secureConn, err := NewSecureConn(remoteConn, t.Key, false)
	if err != nil {
		log.Printf("[Transit] 初始化加密隧道失败: %v", err)
		return
	}

	done := make(chan struct{})

	// 上行：Client -> Secure (Transit to Exit)
	go func() {
		n, _ := io.Copy(secureConn, clientConn)
		if t.Counter != nil {
			atomic.AddInt64(&t.Counter.Upload, n)
		}
		close(done)
	}()

	// 下行：Secure -> Client (Exit to Transit)
	go func() {
		n, _ := io.Copy(clientConn, secureConn)
		if t.Counter != nil {
			atomic.AddInt64(&t.Counter.Download, n)
		}
		close(done)
	}()

	<-done
}

type ExitServer struct {
	ListenAddr string
	LocalAddr  string
	Key        string
}

func (e *ExitServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", e.ListenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("[Exit] 正在接收隧道端口 %s", e.ListenAddr)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		tunnelConn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Printf("[Exit] Accept 错误: %v", err)
				continue
			}
		}
		go e.handle(tunnelConn)
	}
}

func (e *ExitServer) handle(tunnelConn net.Conn) {
	defer tunnelConn.Close()

	secureConn, err := NewSecureConn(tunnelConn, e.Key, true)
	if err != nil {
		log.Printf("[Exit] 握手失败: %v", err)
		return
	}

	localConn, err := net.Dial("tcp", e.LocalAddr)
	if err != nil {
		log.Printf("[Exit] 无法连接本地服务 %s: %v", e.LocalAddr, err)
		return
	}
	defer localConn.Close()

	done := make(chan struct{})
	go func() {
		io.Copy(localConn, secureConn)
		close(done)
	}()
	go func() {
		io.Copy(secureConn, localConn)
		close(done)
	}()
	<-done
}
