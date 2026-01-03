package tunnel

import (
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	MaxPayloadSize = 16383
	TagSize        = 16
	NonceSize      = 12
	// 初始分片混淆的最大长度
	MaxInitialJitter = 128
)

// SecureConn 包装 net.Conn，处理全加密流
type SecureConn struct {
	net.Conn
	enc      cipher.AEAD
	dec      cipher.AEAD
	encNonce [NonceSize]byte
	decNonce [NonceSize]byte
	muEnc    sync.Mutex
	muDec    sync.Mutex
	readBuf  []byte
}

func NewSecureConn(conn net.Conn, keyStr string, isServer bool) (*SecureConn, error) {
	h := sha256.New()
	h.Write([]byte(keyStr))
	key := h.Sum(nil)

	enc, _ := chacha20poly1305.New(key)
	dec, _ := chacha20poly1305.New(key)

	sc := &SecureConn{
		Conn:    conn,
		enc:     enc,
		dec:     dec,
		readBuf: make([]byte, 0),
	}

	// 核心隐身逻辑：初始长度扰动
	// Transit (Client) 发送 jitter，Exit (Server) 接收并丢弃
	if !isServer {
		// 发送 0 - MaxInitialJitter 字节的随机垃圾数据
		jitterLen := rand.Intn(MaxInitialJitter)
		jitter := make([]byte, jitterLen)
		rand.Read(jitter)

		// 注意：这里的 jitter 本身不加密，它就是为了干扰墙对开头字节的关注
		// 但为了更像“正常加密流”，我们用 AEAD 封一个空包
		if _, err := sc.Write(nil); err != nil {
			return nil, err
		}
	} else {
		// 接收第一个空包并丢弃
		dummy := make([]byte, 1)
		_, err := sc.Read(dummy)
		if err != nil && err != io.EOF {
			// 如果握手包解不开，说明密钥错或者是探测包，直接断开
			return nil, err
		}
	}

	return sc, nil
}

func (s *SecureConn) incNonce(n *[NonceSize]byte) {
	for i := 0; i < NonceSize; i++ {
		n[i]++
		if n[i] != 0 {
			break
		}
	}
}

func (s *SecureConn) Write(p []byte) (n int, err error) {
	s.muEnc.Lock()
	defer s.muEnc.Unlock()

	// 处理空包的情况 (用于握手或 Keepalive)
	if len(p) == 0 {
		lenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBuf, 0)
		lenNonce := s.encNonce
		s.incNonce(&s.encNonce)
		header := s.enc.Seal(nil, lenNonce[:], lenBuf, nil)
		_, err := s.Conn.Write(header)
		return 0, err
	}

	totalSent := 0
	for totalSent < len(p) {
		chunkSize := len(p) - totalSent
		if chunkSize > MaxPayloadSize {
			chunkSize = MaxPayloadSize
		}

		payload := p[totalSent : totalSent+chunkSize]

		lenBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(lenBuf, uint16(chunkSize))

		lenNonce := s.encNonce
		s.incNonce(&s.encNonce)
		header := s.enc.Seal(nil, lenNonce[:], lenBuf, nil)
		if _, err := s.Conn.Write(header); err != nil {
			return totalSent, err
		}

		nonce := s.encNonce
		s.incNonce(&s.encNonce)
		body := s.enc.Seal(nil, nonce[:], payload, nil)
		if _, err := s.Conn.Write(body); err != nil {
			return totalSent, err
		}

		totalSent += chunkSize
	}

	return totalSent, nil
}

func (s *SecureConn) Read(p []byte) (n int, err error) {
	s.muDec.Lock()
	defer s.muDec.Unlock()

	for {
		if len(s.readBuf) > 0 {
			n = copy(p, s.readBuf)
			s.readBuf = s.readBuf[n:]
			return n, nil
		}

		headerBuf := make([]byte, 2+TagSize)
		if _, err := io.ReadFull(s.Conn, headerBuf); err != nil {
			return 0, err
		}

		lenNonce := s.decNonce
		s.incNonce(&s.decNonce)
		lenPlain, err := s.dec.Open(nil, lenNonce[:], headerBuf, nil)
		if err != nil {
			return 0, fmt.Errorf("decrypt header failed (suspicious probing): %v", err)
		}

		chunkLen := binary.BigEndian.Uint16(lenPlain)
		if chunkLen == 0 {
			// 跳过空包
			continue
		}

		bodyBuf := make([]byte, int(chunkLen)+TagSize)
		if _, err := io.ReadFull(s.Conn, bodyBuf); err != nil {
			return 0, err
		}

		nonce := s.decNonce
		s.incNonce(&s.decNonce)
		payload, err := s.dec.Open(nil, nonce[:], bodyBuf, nil)
		if err != nil {
			return 0, fmt.Errorf("decrypt body failed: %v", err)
		}

		n = copy(p, payload)
		if n < len(payload) {
			s.readBuf = append(s.readBuf, payload[n:]...)
		}
		return n, nil
	}
}
