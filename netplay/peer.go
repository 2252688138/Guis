// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package netplay

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// errTimeout 心跳 / 读超时。
var errTimeout = errors.New("netplay: 连接超时")

// Peer 一条已握手连接：负责读循环、消息分发、序号递增与心跳。
type Peer struct {
	// ID 服务端分配的玩家 ID（客户端从 HandshakeAck 取得）。
	ID uint64
	// Timeout 读超时：超过该时长没收到任何消息就断开（0 = 不检测）。
	Timeout time.Duration
	// Heartbeat 心跳间隔（0 = 不主动发 Ping）。
	Heartbeat time.Duration
	// OnDisconnect 连接断开时回调（只会调一次）。
	OnDisconnect func(*Peer, error)

	tr        Transport
	mu        sync.RWMutex
	handlers  map[MsgType][]func(*Peer, Message)
	seq       atomic.Uint32
	lastRecv  atomic.Int64
	closeOnce sync.Once
	done      chan struct{}
}

// NewPeer 包装传输（不启动读循环，需自己调 Start）。
func NewPeer(id uint64, tr Transport) *Peer {
	return &Peer{
		ID:       id,
		tr:       tr,
		handlers: make(map[MsgType][]func(*Peer, Message)),
		done:     make(chan struct{}),
	}
}

// On 注册消息处理器（同一类型可注册多个，按注册顺序调用）。
func (p *Peer) On(t MsgType, h func(*Peer, Message)) {
	if p == nil || h == nil {
		return
	}
	p.mu.Lock()
	p.handlers[t] = append(p.handlers[t], h)
	p.mu.Unlock()
}

// Send 发送一条消息（并发安全，序号自动递增）。
func (p *Peer) Send(t MsgType, payload []byte) error {
	if p == nil || p.tr == nil {
		return ErrClosed
	}
	frame, err := EncodeMessage(Message{Type: t, Seq: p.seq.Add(1), Payload: payload})
	if err != nil {
		return err
	}
	return p.tr.SendMessage(frame)
}

// RemoteAddr 对端地址。
func (p *Peer) RemoteAddr() string {
	if p == nil || p.tr == nil {
		return ""
	}
	return p.tr.RemoteAddr()
}

// Start 启动读循环（只能调一次）与心跳。
func (p *Peer) Start() {
	if p == nil || p.tr == nil {
		return
	}
	p.lastRecv.Store(time.Now().UnixNano())
	go p.readLoop()
	if p.Heartbeat > 0 {
		go p.heartbeatLoop()
	}
	if p.Timeout > 0 {
		go p.watchdogLoop()
	}
}

func (p *Peer) readLoop() {
	for {
		b, err := p.tr.RecvMessage()
		if err != nil {
			p.fail(err)
			return
		}
		msg, err := DecodeMessage(b)
		if err != nil {
			p.fail(err)
			return
		}
		p.lastRecv.Store(time.Now().UnixNano())
		switch msg.Type {
		case MsgPing:
			_ = p.Send(MsgPong, nil)
			continue
		case MsgPong:
			continue
		}
		p.dispatch(msg)
	}
}

func (p *Peer) dispatch(msg Message) {
	p.mu.RLock()
	var hs []func(*Peer, Message)
	hs = append(hs, p.handlers[msg.Type]...)
	p.mu.RUnlock()
	for _, h := range hs {
		h(p, msg)
	}
}

func (p *Peer) heartbeatLoop() {
	t := time.NewTicker(p.Heartbeat)
	defer t.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-t.C:
			if err := p.Send(MsgPing, nil); err != nil {
				p.fail(err)
				return
			}
		}
	}
}

func (p *Peer) watchdogLoop() {
	t := time.NewTicker(p.Timeout / 2)
	defer t.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-t.C:
			last := time.Unix(0, p.lastRecv.Load())
			if time.Since(last) > p.Timeout {
				p.fail(errTimeout)
				return
			}
		}
	}
}

// LastRecv 最近一次收到消息的时间。
func (p *Peer) LastRecv() time.Time { return time.Unix(0, p.lastRecv.Load()) }

func (p *Peer) fail(err error) {
	p.closeOnce.Do(func() {
		close(p.done)
		_ = p.tr.Close()
		if p.OnDisconnect != nil {
			p.OnDisconnect(p, err)
		}
	})
}

// Close 主动断开（不会触发 OnDisconnect 以外的额外行为）。
func (p *Peer) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		close(p.done)
	})
	if p.tr != nil {
		return p.tr.Close()
	}
	return nil
}
