package httpc

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestBuildTransportNone 验证 "none" 代理与非法代理。
func TestBuildTransportNone(t *testing.T) {
	cfg := Config{Proxy: "none"}.normalized()
	tr, err := buildTransport(cfg)
	if err != nil {
		t.Fatalf("buildTransport 失败: %v", err)
	}
	if tr.Proxy != nil {
		t.Fatal("none 应强制直连")
	}
	if _, err := buildTransport(Config{Proxy: "socks5://"}.normalized()); err == nil {
		// socks5 没有 host 时应报错
		t.Fatal("socks5 缺少 host 应报错")
	}
	if _, err := buildTransport(Config{Proxy: "http://127.0.0.1:1"}.normalized()); err != nil {
		t.Fatalf("http 代理不应报错: %v", err)
	}
}

// TestBuildTLSConfig 验证 TLS 快捷配置。
func TestBuildTLSConfig(t *testing.T) {
	if c, err := buildTLSConfig(nil); c != nil || err != nil {
		t.Fatalf("nil 应返回 nil,nil，实际 %v %v", c, err)
	}
	if _, err := buildTLSConfig(&TLSConfig{RootCAs: []byte("not-a-cert")}); err == nil {
		t.Fatal("非法 RootCAs 应报错")
	}
	if _, err := buildTLSConfig(&TLSConfig{ClientCertFile: "nope.pem", ClientKeyFile: "nope.key"}); err == nil {
		t.Fatal("不存在的客户端证书应报错")
	}
}

// TestHasPort 验证 SOCKS5 代理端口推断。
func TestHasPort(t *testing.T) {
	if hasPort("127.0.0.1:1080") != true {
		t.Fatal("应识别出端口")
	}
	if hasPort("127.0.0.1") != false {
		t.Fatal("无端口时应返回 false")
	}
}

// TestSocks5HandshakeErrors 验证握手读失败分支。
func TestSocks5HandshakeErrors(t *testing.T) {
	c, s := net.Pipe()
	s.Close()
	if err := socks5Handshake(c, "", "", "h:80"); err == nil {
		t.Fatal("连接已关闭应报错")
	}
	c2, s2 := net.Pipe()
	go func() {
		io.ReadFull(s2, make([]byte, 3))
		s2.Write([]byte{0x04, 0x00})
	}()
	if err := socks5Handshake(c2, "", "", "h:80"); err == nil {
		t.Fatal("版本不匹配应报错")
	}
	if _, err := socks5DestAddr("h:notaport"); err == nil {
		t.Fatal("非法端口应报错")
	}
}

// TestSocks5DialContextUnsupportedNetwork 验证非 TCP 网络报错。
func TestSocks5DialContextUnsupportedNetwork(t *testing.T) {
	u, _ := url.Parse("socks5://127.0.0.1:1080")
	dial := socks5DialContext(u, &net.Dialer{Timeout: time.Second})
	if _, err := dial(context.Background(), "udp", "1.2.3.4:53"); err == nil {
		t.Fatal("udp 应被拒绝")
	}
	if _, err := dial(context.Background(), "tcp", "1.2.3.4:80"); err == nil {
		t.Fatal("代理不可达应报错")
	}
}

// TestClientSetAndRoundTripper 验证配置热更新与自定义传输层。
func TestClientSetAndRoundTripper(t *testing.T) {
	c := New(Config{Proxy: "none", BaseURL: "http://a"})
	if _, err := c.roundTripper(c.config()); err != nil {
		t.Fatalf("roundTripper 失败: %v", err)
	}
	custom := &http.Transport{}
	c.Set(Config{Transport: custom})
	if rt, err := c.roundTripper(c.config()); err != nil || rt != custom {
		t.Fatalf("自定义 Transport 未生效: %v %v", rt, err)
	}
	if c.UseRequest(nil) != c || c.UseResponse(nil) != c || c.UseError(nil) != c {
		t.Fatal("注册 nil 拦截器应原样返回")
	}
	if c.Use(nil, nil) != c {
		t.Fatal("Use 应返回自身")
	}
}

// TestSemaphore 验证信号量懒创建与不限并发。
func TestSemaphore(t *testing.T) {
	c := New()
	if sem := c.semaphore(Config{}); sem != nil {
		t.Fatal("未设置并发上限时应返回 nil")
	}
	sem := c.semaphore(Config{MaxConcurrency: 2})
	if cap(sem) != 2 {
		t.Fatalf("容量 = %d", cap(sem))
	}
	sem2 := c.semaphore(Config{MaxConcurrency: 3})
	if cap(sem2) != 3 {
		t.Fatalf("更改上限后应重建，容量 = %d", cap(sem2))
	}
}
