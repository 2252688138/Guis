// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package netplay

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Server 局域网房间服务端：监听、握手分配 ID、广播。
type Server struct {
	// OnJoin 新玩家加入（ID 已分配、握手已应答）。
	OnJoin func(*Peer)
	// OnLeave 玩家断开。
	OnLeave func(*Peer)

	ln     net.Listener
	mu     sync.RWMutex
	peers  map[uint64]*Peer
	nextID atomic.Uint64
	closed atomic.Bool
	done   chan struct{}
	wg     sync.WaitGroup
}

// Listen 开始监听（会自动接受连接）。
func Listen(addr string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, peers: make(map[uint64]*Peer), done: make(chan struct{})}
	s.wg.Add(1)
	go s.acceptLoop()
	return s, nil
}

// Addr 监听地址（端口为 0 时用来取实际端口）。
func (s *Server) Addr() string {
	if s == nil || s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(c)
		}()
	}
}

// handleConn 握手：第一条消息必须是版本匹配的 MsgHandshake。
func (s *Server) handleConn(c net.Conn) {
	tr := NewTCPTransport(c)
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	b, err := tr.RecvMessage()
	if err != nil {
		tr.Close()
		return
	}
	msg, err := DecodeMessage(b)
	if err != nil || msg.Type != MsgHandshake {
		tr.Close()
		return
	}
	var hs Handshake
	if err := DecodeJSON(msg.Payload, &hs); err != nil || hs.Version != Version {
		frame, _ := EncodeMessage(Message{Type: MsgHandshakeAck, Payload: mustJSON(HandshakeAck{Version: Version, Message: "协议版本不匹配"})})
		_ = tr.SendMessage(frame)
		tr.Close()
		return
	}
	// 握手完成后清掉读超时（MAC 上 accept 回来的 conn 共用 deadline）
	_ = c.SetReadDeadline(time.Time{})

	id := s.nextID.Add(1)
	peer := NewPeer(id, tr)
	peer.OnDisconnect = func(p *Peer, err error) {
		s.mu.Lock()
		delete(s.peers, p.ID)
		s.mu.Unlock()
		if s.OnLeave != nil {
			s.OnLeave(p)
		}
	}

	ack, _ := EncodeMessage(Message{Type: MsgHandshakeAck, Payload: mustJSON(HandshakeAck{Version: Version, ID: id})})
	if err := tr.SendMessage(ack); err != nil {
		tr.Close()
		return
	}

	s.mu.Lock()
	s.peers[id] = peer
	s.mu.Unlock()

	peer.Start()
	if s.OnJoin != nil {
		s.OnJoin(peer)
	}
	s.Broadcast(MsgJoin, mustJSON(HandshakeAck{Version: Version, ID: id, Message: hs.Name}))
}

func mustJSON(v any) []byte {
	b, err := EncodeJSON(v)
	if err != nil {
		return nil
	}
	return b
}

// Peers 当前在线玩家（快照）。
func (s *Server) Peers() []*Peer {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Peer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, p)
	}
	return out
}

// Broadcast 向所有玩家广播（返回成功发送的数量）。
func (s *Server) Broadcast(t MsgType, payload []byte) int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	peers := make([]*Peer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	s.mu.RUnlock()
	n := 0
	for _, p := range peers {
		if err := p.Send(t, payload); err == nil {
			n++
		}
	}
	return n
}

// BroadcastExcept 向除指定玩家外的所有人广播。
func (s *Server) BroadcastExcept(except uint64, t MsgType, payload []byte) int {
	if s == nil {
		return 0
	}
	n := 0
	for _, p := range s.Peers() {
		if p.ID == except {
			continue
		}
		if err := p.Send(t, payload); err == nil {
			n++
		}
	}
	return n
}

// Close 关闭监听并断开所有玩家。
func (s *Server) Close() error {
	if s == nil || s.closed.Swap(true) {
		return nil
	}
	close(s.done)
	err := s.ln.Close()
	for _, p := range s.Peers() {
		p.Close()
	}
	s.wg.Wait()
	return err
}

// Client 客户端连接（Dial 完成握手后才返回）。
type Client struct {
	peer *Peer
	ack  HandshakeAck
}

// Dial 连接服务端并完成握手（timeout ≤ 0 时用 5 秒）。
func Dial(addr, name string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	tr, err := DialTCP(addr, timeout)
	if err != nil {
		return nil, err
	}
	hs, _ := EncodeMessage(Message{Type: MsgHandshake, Payload: mustJSON(Handshake{Version: Version, Name: name})})
	if err := tr.SendMessage(hs); err != nil {
		tr.Close()
		return nil, err
	}
	b, err := tr.RecvMessage()
	if err != nil {
		tr.Close()
		return nil, err
	}
	msg, err := DecodeMessage(b)
	if err != nil {
		tr.Close()
		return nil, err
	}
	if msg.Type != MsgHandshakeAck {
		tr.Close()
		return nil, fmt.Errorf("netplay: 期望握手应答，收到消息类型 %d", msg.Type)
	}
	var ack HandshakeAck
	if err := DecodeJSON(msg.Payload, &ack); err != nil {
		tr.Close()
		return nil, err
	}
	if ack.Version != Version || ack.ID == 0 {
		tr.Close()
		return nil, fmt.Errorf("netplay: 握手被拒：%s", ack.Message)
	}
	peer := NewPeer(ack.ID, tr)
	peer.Start()
	return &Client{peer: peer, ack: ack}, nil
}

// Peer 客户端连接。
func (c *Client) Peer() *Peer {
	if c == nil {
		return nil
	}
	return c.peer
}

// ID 自己的玩家 ID。
func (c *Client) ID() uint64 {
	if c == nil {
		return 0
	}
	return c.ack.ID
}

// Close 断开连接。
func (c *Client) Close() error {
	if c == nil || c.peer == nil {
		return nil
	}
	return c.peer.Close()
}
