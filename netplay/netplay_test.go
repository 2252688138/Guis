// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package netplay_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"guis/netplay"
)

func TestProtocolRoundTrip(t *testing.T) {
	types := []netplay.MsgType{
		netplay.MsgHandshake, netplay.MsgHandshakeAck, netplay.MsgSnapshot,
		netplay.MsgInput, netplay.MsgChat, netplay.MsgPing, netplay.MsgPong,
		netplay.MsgJoin, netplay.MsgLeave,
	}
	for _, ty := range types {
		frame, err := netplay.EncodeMessage(netplay.Message{Type: ty, Seq: 42, Payload: []byte("hello")})
		if err != nil {
			t.Fatalf("编码 %d 失败: %v", ty, err)
		}
		msg, err := netplay.DecodeMessage(frame)
		if err != nil {
			t.Fatalf("解码 %d 失败: %v", ty, err)
		}
		if msg.Type != ty || msg.Seq != 42 || string(msg.Payload) != "hello" {
			t.Fatalf("往返不一致: %+v", msg)
		}
	}
}

func TestProtocolRejectsBadFrames(t *testing.T) {
	// 超长载荷编码失败
	big := make([]byte, netplay.MaxFrameSize+1)
	if _, err := netplay.EncodeMessage(netplay.Message{Type: netplay.MsgChat, Payload: big}); err == nil {
		t.Fatal("超长载荷应编码失败")
	}
	// 太短
	if _, err := netplay.DecodeMessage([]byte{1, 2, 3}); err == nil {
		t.Fatal("短帧应解码失败")
	}
	// 魔数错 / 版本错 / 长度不符
	good, _ := netplay.EncodeMessage(netplay.Message{Type: netplay.MsgChat, Payload: []byte("x")})
	badMagic := append([]byte(nil), good...)
	badMagic[0] ^= 0xFF
	if _, err := netplay.DecodeMessage(badMagic); err == nil {
		t.Fatal("错魔数应解码失败")
	}
	badVer := append([]byte(nil), good...)
	badVer[2] = 99
	if _, err := netplay.DecodeMessage(badVer); err == nil {
		t.Fatal("错版本应解码失败")
	}
	badLen := append([]byte(nil), good...)
	badLen[11] = 8
	if _, err := netplay.DecodeMessage(badLen); err == nil {
		t.Fatal("长度不符应解码失败")
	}
}

func TestTCPTransportLargePayload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type res struct {
		tr  netplay.Transport
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			ch <- res{nil, err}
			return
		}
		ch <- res{netplay.NewTCPTransport(c), nil}
	}()

	cli, err := netplay.DialTCP(ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	srvRes := <-ch
	if srvRes.err != nil {
		t.Fatal(srvRes.err)
	}
	srv := srvRes.tr
	defer srv.Close()

	payload := bytes.Repeat([]byte("0123456789abcdef"), 64*1024) // 1 MiB
	if len(payload) != netplay.MaxFrameSize {
		t.Fatalf("测试载荷长度 = %d", len(payload))
	}
	errCh := make(chan error, 1)
	go func() { errCh <- cli.SendMessage(payload) }()
	got, err := srv.RecvMessage()
	if err != nil {
		t.Fatalf("接收失败: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("数据不一致: 收到 %d 字节", len(got))
	}
}

func waitPeers(t *testing.T, srv *netplay.Server, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.Peers()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待 %d 个玩家超时（当前 %d）", n, len(srv.Peers()))
}

func TestServerClientBroadcast(t *testing.T) {
	srv, err := netplay.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	c1, err := netplay.Dial(srv.Addr(), "p1", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	c2, err := netplay.Dial(srv.Addr(), "p2", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if c1.ID() == c2.ID() || c1.ID() == 0 {
		t.Fatalf("ID 分配异常: %d / %d", c1.ID(), c2.ID())
	}
	waitPeers(t, srv, 2)

	got1 := make(chan string, 4)
	got2 := make(chan string, 4)
	c1.Peer().On(netplay.MsgChat, func(_ *netplay.Peer, m netplay.Message) { got1 <- string(m.Payload) })
	c2.Peer().On(netplay.MsgChat, func(_ *netplay.Peer, m netplay.Message) { got2 <- string(m.Payload) })

	if n := srv.Broadcast(netplay.MsgChat, []byte("hi all")); n != 2 {
		t.Fatalf("广播成功数 = %d，期望 2", n)
	}
	for i, ch := range []chan string{got1, got2} {
		select {
		case s := <-ch:
			if s != "hi all" {
				t.Fatalf("客户端 %d 收到 %q", i+1, s)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("客户端 %d 超时未收到广播", i+1)
		}
	}
}

func TestPeerTimeoutDisconnect(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	p := netplay.NewPeer(7, netplay.NewTCPTransport(a))
	p.Timeout = 120 * time.Millisecond
	done := make(chan error, 1)
	p.OnDisconnect = func(_ *netplay.Peer, err error) { done <- err }
	p.Start()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("超时断开应带错误")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("超时未触发断开回调")
	}
}

func TestInterpolatorSample(t *testing.T) {
	ip := netplay.NewInterpolator()
	if _, _, _, ok := ip.Sample(0); ok {
		t.Fatal("空缓冲应返回 ok=false")
	}
	ip.Push(netplay.Snapshot{Tick: 1, Time: 0, Data: []byte("a")})
	ip.Push(netplay.Snapshot{Tick: 2, Time: 1, Data: []byte("b")})
	prev, next, alpha, ok := ip.Sample(0.5)
	if !ok || prev.Tick != 1 || next.Tick != 2 {
		t.Fatalf("采样帧不对: prev=%d next=%d ok=%v", prev.Tick, next.Tick, ok)
	}
	if alpha < 0.49 || alpha > 0.51 {
		t.Fatalf("alpha = %v，期望 0.5", alpha)
	}
	// 重复 Tick 去重
	ip.Push(netplay.Snapshot{Tick: 2, Time: 1, Data: []byte("c")})
	if ip.Len() != 2 {
		t.Fatalf("去重后长度 = %d，期望 2", ip.Len())
	}
	// 超出范围夹持
	if _, _, a, _ := ip.Sample(99); a != 0 {
		t.Fatalf("超时时间应夹持到最新帧，alpha=%v", a)
	}
}

func TestLockstep(t *testing.T) {
	ls := netplay.NewLockstep(2)
	ls.Submit(1, []byte("up"))
	if ls.Pending() != 1 {
		t.Fatalf("Pending = %d，期望 1", ls.Pending())
	}
	if _, _, ok := ls.Advance(); ok {
		t.Fatal("输入不齐不应放行")
	}
	ls.Submit(2, []byte("down"))
	tick, inputs, ok := ls.Advance()
	if !ok || tick != 1 {
		t.Fatalf("放行失败 tick=%d ok=%v", tick, ok)
	}
	if len(inputs) != 2 || string(inputs[1]) != "up" || string(inputs[2]) != "down" {
		t.Fatalf("输入归并不对: %+v", inputs)
	}
	if ls.Pending() != 2 {
		t.Fatalf("放行后应清空缓冲，Pending=%d", ls.Pending())
	}
}

// TestLANPeerSync 局域网双人同步 demo：服务端广播快照，客户端插值取帧。
func TestLANPeerSync(t *testing.T) {
	srv, err := netplay.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	cli, err := netplay.Dial(srv.Addr(), "player", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	waitPeers(t, srv, 1)

	ip := netplay.NewInterpolator()
	cli.Peer().On(netplay.MsgSnapshot, func(_ *netplay.Peer, m netplay.Message) {
		var s netplay.Snapshot
		if err := netplay.DecodeJSON(m.Payload, &s); err == nil {
			ip.Push(s)
		}
	})

	type state struct {
		X float64 `json:"x"`
	}
	for tick := uint32(1); tick <= 3; tick++ {
		payload, err := netplay.EncodeJSON(netplay.Snapshot{Tick: tick, Time: float64(tick) / 10, Data: mustState(t, state{X: float64(tick)})})
		if err != nil {
			t.Fatal(err)
		}
		if n := srv.Broadcast(netplay.MsgSnapshot, payload); n != 1 {
			t.Fatalf("广播成功数 = %d", n)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ip.Len() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	if ip.Len() != 3 {
		t.Fatalf("客户端快照缓冲 = %d，期望 3", ip.Len())
	}
	prev, next, alpha, ok := ip.Sample(0.15)
	if !ok || prev.Tick != 1 || next.Tick != 2 || alpha < 0.49 || alpha > 0.51 {
		t.Fatalf("插值结果不对: prev=%d next=%d alpha=%v", prev.Tick, next.Tick, alpha)
	}
}

func mustState(t *testing.T, v any) []byte {
	t.Helper()
	b, err := netplay.EncodeJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
