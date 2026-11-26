// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package netplay 提供轻量联机框架：TCP/UDP 传输、长度前缀分帧帧头、
// 握手 / 心跳、房间广播、快照插值与帧步输入同步。
//
// 设计边界（重要）：
//   - 只面向局域网 / 直连，不含中继、匹配、账号与反作弊；
//   - 不引入任何第三方依赖（只用标准库）；
//   - 状态序列化交给上层（默认提供 JSON 辅助函数，也可自己写二进制）。
//
// 典型用法：服务端 Listen(addr)，客户端 Dial(addr)，握手后双方用
// Peer.Send / Peer.On 收发消息；同步用 Snapshot + Interpolator（状态同步）
// 或 Lockstep（确定性帧同步）。示例见 example_test.go。
package netplay

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Transport 一条已完成分帧的消息通道（上层只看到完整消息）。
type Transport interface {
	io.Closer
	// SendMessage 发送一条消息（内部加锁，可并发调用）。
	SendMessage(b []byte) error
	// RecvMessage 接收一条消息（并发调用不安全，只给 Peer 的读循环用）。
	RecvMessage() ([]byte, error)
	// RemoteAddr 对端地址（日志用）。
	RemoteAddr() string
}

// MaxFrameSize 单条消息上限（默认 1 MiB）：防止对端用一个超大长度字段
// 把本端内存拉爆。
const MaxFrameSize = 1 << 20

var (
	// ErrFrameTooLarge 帧长度超过 MaxFrameSize。
	ErrFrameTooLarge = errors.New("netplay: 帧超过大小上限")
	// ErrClosed 传输已关闭。
	ErrClosed = errors.New("netplay: 连接已关闭")
)

// framedConn TCP 分帧传输：4 字节大端长度 + 消息体。
type framedConn struct {
	c      net.Conn
	wmu    sync.Mutex
	max    int
	closed bool
	mu     sync.Mutex
}

// NewTCPTransport 把 net.Conn 包装成 TCP 分帧传输。
func NewTCPTransport(c net.Conn) Transport {
	return &framedConn{c: c, max: MaxFrameSize}
}

func (t *framedConn) SendMessage(b []byte) error {
	if len(b) > t.max {
		return ErrFrameTooLarge
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := t.c.Write(hdr[:]); err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	_, err := t.c.Write(b)
	return err
}

func (t *framedConn) RecvMessage() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(t.c, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if int(n) > t.max {
		return nil, ErrFrameTooLarge
	}
	if n == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(t.c, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (t *framedConn) RemoteAddr() string { return t.c.RemoteAddr().String() }

func (t *framedConn) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	return t.c.Close()
}

// udpTransport UDP 传输（不可靠、不保序、不重传）：
// 一个 UDP 包就是一条消息，适合帧同步里「丢了就丢」的输入广播。
type udpTransport struct {
	c      *net.UDPConn
	peer   *net.UDPAddr
	max    int
	closed bool
	mu     sync.Mutex
}

// NewUDPTransport 把已连接的 UDPConn 包装成消息传输。
func NewUDPTransport(c *net.UDPConn, max int) Transport {
	if max <= 0 || max > MaxFrameSize {
		max = MaxFrameSize
	}
	return &udpTransport{c: c, max: max}
}

func (t *udpTransport) SendMessage(b []byte) error {
	if len(b) > t.max {
		return ErrFrameTooLarge
	}
	_, err := t.c.Write(b)
	return err
}

func (t *udpTransport) RecvMessage() ([]byte, error) {
	buf := make([]byte, t.max)
	n, err := t.c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (t *udpTransport) RemoteAddr() string {
	if a := t.c.RemoteAddr(); a != nil {
		return a.String()
	}
	return ""
}

func (t *udpTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.mu.Unlock()
	return t.c.Close()
}

// DialTCP 连接地址并返回分帧传输（timeout ≤ 0 时用 5 秒）。
func DialTCP(addr string, timeout time.Duration) (Transport, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return NewTCPTransport(c), nil
}
