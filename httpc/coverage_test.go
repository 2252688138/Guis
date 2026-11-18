package httpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestValueStrings 验证查询参数值展开。
func TestValueStrings(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		in   any
		want []string
	}{
		{nil, nil},
		{"s", []string{"s"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]byte("b"), []string{"b"}},
		{true, []string{"true"}},
		{int(1), []string{"1"}},
		{int8(2), []string{"2"}},
		{int16(3), []string{"3"}},
		{int32(4), []string{"4"}},
		{int64(5), []string{"5"}},
		{uint(6), []string{"6"}},
		{uint8(7), []string{"7"}},
		{uint16(8), []string{"8"}},
		{uint32(9), []string{"9"}},
		{uint64(10), []string{"10"}},
		{float32(1.5), []string{"1.5"}},
		{float64(2.25), []string{"2.25"}},
		{now, []string{now.Format(time.RFC3339)}},
		{[]any{"x", 1}, []string{"x", "1"}},
		{url.Values{"k": {"v"}}, []string{"v"}},
		{errors.New("err"), []string{"err"}},
	}
	for _, tc := range cases {
		got := valueStrings(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("valueStrings(%#v) = %v，期望 %v", tc.in, got, tc.want)
		}
	}
}

// TestMergeParams 验证参数合并与 nil 删除。
func TestMergeParams(t *testing.T) {
	if mergeParams(nil, nil) != nil {
		t.Fatal("空参数应返回 nil")
	}
	got := mergeParams(Params{"a": 1, "b": 2}, Params{"b": nil, "c": 3})
	if _, ok := got["b"]; ok {
		t.Fatal("nil 应删除同名项")
	}
	if got["a"] != 1 || got["c"] != 3 {
		t.Fatalf("合并结果 = %v", got)
	}
}

// TestCustomParamsEncoder 验证自定义编码器与已有 query 保留。
func TestCustomParamsEncoder(t *testing.T) {
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
	}))
	defer srv.Close()

	enc := func(p Params, rawQuery string) string {
		out := rawQuery
		if out != "" {
			out += "&"
		}
		return out + "custom=1"
	}
	if _, err := testClient().Get(srv.URL+"/x?pre=kept", WithParam("a", 1), WithParamsEncoder(enc)); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if raw != "pre=kept&custom=1" {
		t.Fatalf("rawQuery = %q", raw)
	}
}

// TestOptionsCoverage 验证常用选项。
func TestOptionsCoverage(t *testing.T) {
	var got http.Header
	var raw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		raw = r.URL.RawQuery
	}))
	defer srv.Close()

	opts := []Option{
		WithUserAgent("ua-test"),
		WithContentType("application/x-test"),
		WithParams(Params{"p": "1"}),
		WithParam("q", "2"),
		WithMaxRedirects(3),
		WithMaxContentLength(1024),
		WithProgressInterval(time.Millisecond),
		WithCookies(&http.Cookie{Name: "c", Value: "v"}),
		WithHeaders(http.Header{"X-Multi": {"1", "2"}}),
		WithBody("ignored"),
	}
	if _, err := testClient().Post(srv.URL, nil, opts...); err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if got.Get("User-Agent") != "ua-test" || got.Get("X-Multi") == "" {
		t.Fatalf("头部 = %v", got)
	}
	if !strings.Contains(raw, "p=1") || !strings.Contains(raw, "q=2") {
		t.Fatalf("rawQuery = %q", raw)
	}
	if !strings.Contains(got.Get("Cookie"), "c=v") {
		t.Fatalf("Cookie = %q", got.Get("Cookie"))
	}
}

// TestHooksAndDebug 验证 OnRequest/OnResponse/Debug 钩子。
func TestHooksAndDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	var onReq, onResp int
	var debug []string
	c := testClient(Config{
		Retry: RetryConfig{Max: 1, Backoff: time.Millisecond},
		OnRequest: func(r *http.Request) {
			onReq++
		},
		OnResponse: func(r *Response) {
			onResp++
		},
		Debug: func(format string, args ...any) {
			debug = append(debug, format)
		},
	})
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("应失败")
	}
	if onReq != 2 {
		t.Fatalf("OnRequest 调用 = %d，期望 2", onReq)
	}
	if onResp != 0 {
		t.Fatalf("失败时 OnResponse 不应调用，实际 %d", onResp)
	}
	if len(debug) == 0 {
		t.Fatal("Debug 未收到重试日志")
	}
}

// TestCustomCheckRedirect 验证自定义重定向判定。
func TestCustomCheckRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/b", http.StatusFound)
	}))
	defer srv.Close()

	deny := errors.New("不跳转")
	c := testClient(Config{CheckRedirect: func(req *http.Request, via []*http.Request) error { return deny }})
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("自定义重定向错误应导致请求失败")
	}
}

// TestRedirectLimitExceeded 验证重定向次数超限。
func TestRedirectLimitExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()

	_, err := testClient().Get(srv.URL+"/a", WithMaxRedirects(2))
	if err == nil {
		t.Fatal("超过重定向次数应报错")
	}
	if !strings.Contains(err.Error(), "重定向次数") {
		t.Fatalf("错误文本 = %q", err.Error())
	}
}

// TestDisableCompression 验证关闭自动解压。
func TestDisableCompression(t *testing.T) {
	var ae string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae = r.Header.Get("Accept-Encoding")
		_, _ = io.WriteString(w, "plain")
	}))
	defer srv.Close()

	if _, err := testClient(Config{DisableCompression: true}).Get(srv.URL); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if ae != "" {
		t.Fatalf("关闭压缩后不应自动加 Accept-Encoding，实际 %q", ae)
	}
}

// TestDownloadErrors 验证下载参数校验与目录创建。
func TestDownloadErrors(t *testing.T) {
	if _, err := testClient().Download("http://127.0.0.1:1/x", ""); err == nil {
		t.Fatal("空路径应报错")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	base := filepath.Join(t.TempDir(), "nested", "deep")
	dst := filepath.Join(base, "f.txt")
	if _, err := testClient().Download(srv.URL, dst); err != nil {
		t.Fatalf("Download 失败: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("文件不存在: %v", err)
	}
}

// TestUploadFields 验证携带表单字段的上传。
func TestUploadFields(t *testing.T) {
	var field string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		field = r.FormValue("tag")
	}))
	defer srv.Close()

	files := []FilePart{{Field: "f", FileName: "a.txt", Reader: strings.NewReader("x")}}
	if _, err := testClient().UploadFields(srv.URL, Params{"tag": "v1"}, files); err != nil {
		t.Fatalf("UploadFields 失败: %v", err)
	}
	if field != "v1" {
		t.Fatalf("字段 = %q", field)
	}
}

// TestMultipartMissingSource 验证文件来源缺失时报错。
func TestMultipartMissingSource(t *testing.T) {
	mp := &Multipart{Files: []FilePart{{Field: "f"}}}
	if _, err := mp.body(); err != nil {
		t.Fatalf("body 不应立即报错: %v", err)
	}
	var buf bytes.Buffer
	if err := mp.write(&buf, "b"); err == nil {
		t.Fatal("缺少 Path/Reader 应报错")
	}
	mp2 := &Multipart{Files: []FilePart{{Path: filepath.Join(t.TempDir(), "nope")}}}
	if _, err := mp2.body(); err != nil {
		t.Fatalf("不存在的文件不应在构建阶段报错: %v", err)
	}
}

// TestStreamSaveTo 验证流式响应落盘。
func TestStreamSaveTo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "stream-data")
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL, WithResponseType(ResponseStream))
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "s.txt")
	if err := resp.SaveTo(dst); err != nil {
		t.Fatalf("SaveTo 失败: %v", err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "stream-data" {
		t.Fatalf("内容 = %q", b)
	}
	if err := resp.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
}

// TestBasicAuthEncoding 验证 Basic 编码与标准实现一致。
func TestBasicAuthEncoding(t *testing.T) {
	for _, tc := range [][2]string{{"", ""}, {"a", ""}, {"ab", "c"}, {"用户", "密码"}} {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(tc[0]+":"+tc[1]))
		if got := basicAuth(tc[0], tc[1]); got != want {
			t.Errorf("basicAuth(%q,%q) = %q，期望 %q", tc[0], tc[1], got, want)
		}
	}
}

// TestSocks5HandshakeNoAuth 验证 SOCKS5 无认证握手。
func TestSocks5HandshakeNoAuth(t *testing.T) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		buf := make([]byte, 2)
		_, _ = io.ReadFull(server, buf)
		_, _ = io.ReadFull(server, make([]byte, int(buf[1])))
		_, _ = server.Write([]byte{socks5Version, socks5NoAuth})
		hdr := make([]byte, 4)
		_, _ = io.ReadFull(server, hdr)
		if hdr[3] == socks5AtypName {
			lb := make([]byte, 1)
			_, _ = io.ReadFull(server, lb)
			_, _ = io.ReadFull(server, make([]byte, int(lb[0])+2))
		}
		_, _ = server.Write([]byte{socks5Version, 0x00, 0x00, socks5AtypIPv4, 127, 0, 0, 1, 0, 80})
	}()
	if err := socks5Handshake(client, "", "", "example.com:443"); err != nil {
		t.Fatalf("握手失败: %v", err)
	}
}

// TestSocks5HandshakeUserPass 验证 SOCKS5 用户名密码握手成功与失败。
func TestSocks5HandshakeUserPass(t *testing.T) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		buf := make([]byte, 2)
		_, _ = io.ReadFull(server, buf)
		_, _ = io.ReadFull(server, make([]byte, int(buf[1])))
		_, _ = server.Write([]byte{socks5Version, socks5UserPass})
		auth := make([]byte, 2)
		_, _ = io.ReadFull(server, auth)
		_, _ = io.ReadFull(server, make([]byte, int(auth[1])))
		pl := make([]byte, 1)
		_, _ = io.ReadFull(server, pl)
		_, _ = io.ReadFull(server, make([]byte, int(pl[0])))
		_, _ = server.Write([]byte{0x01, 0x00})
		hdr := make([]byte, 4)
		_, _ = io.ReadFull(server, hdr)
		_, _ = io.ReadFull(server, make([]byte, 4+2))
		_, _ = server.Write([]byte{socks5Version, 0x00, 0x00, socks5AtypIPv4, 127, 0, 0, 1, 0, 80})
	}()
	if err := socks5Handshake(client, "user", "pass", "1.2.3.4:80"); err != nil {
		t.Fatalf("握手失败: %v", err)
	}

	c2, s2 := net.Pipe()
	go func() {
		defer s2.Close()
		buf := make([]byte, 2)
		_, _ = io.ReadFull(s2, buf)
		_, _ = io.ReadFull(s2, make([]byte, int(buf[1])))
		_, _ = s2.Write([]byte{socks5Version, socks5UserPass})
		auth := make([]byte, 2)
		_, _ = io.ReadFull(s2, auth)
		_, _ = io.ReadFull(s2, make([]byte, int(auth[1])))
		pl := make([]byte, 1)
		_, _ = io.ReadFull(s2, pl)
		_, _ = io.ReadFull(s2, make([]byte, int(pl[0])))
		_, _ = s2.Write([]byte{0x01, 0x01})
	}()
	if err := socks5Handshake(c2, "user", "bad", "1.2.3.4:80"); err == nil {
		t.Fatal("密码错误应报错")
	}
}

// TestSocks5HandshakeRejected 验证代理拒绝与不支持认证。
func TestSocks5HandshakeRejected(t *testing.T) {
	for _, method := range []byte{socks5NoAccept, 0x02} {
		c, s := net.Pipe()
		go func(m byte) {
			defer s.Close()
			buf := make([]byte, 2)
			_, _ = io.ReadFull(s, buf)
			_, _ = io.ReadFull(s, make([]byte, int(buf[1])))
			_, _ = s.Write([]byte{socks5Version, m})
		}(method)
		if err := socks5Handshake(c, "", "", "h:80"); err == nil {
			t.Fatalf("method=%d 应报错", method)
		}
	}
}

// TestSocks5HandshakeConnectError 验证 CONNECT 失败分支。
func TestSocks5HandshakeConnectError(t *testing.T) {
	c, s := net.Pipe()
	go func() {
		defer s.Close()
		buf := make([]byte, 2)
		_, _ = io.ReadFull(s, buf)
		_, _ = io.ReadFull(s, make([]byte, int(buf[1])))
		_, _ = s.Write([]byte{socks5Version, socks5NoAuth})
		hdr := make([]byte, 4)
		_, _ = io.ReadFull(s, hdr)
		_, _ = io.ReadFull(s, make([]byte, 4+2))
		_, _ = s.Write([]byte{socks5Version, 0x05, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0, 0})
	}()
	if err := socks5Handshake(c, "", "", "1.2.3.4:80"); err == nil || !strings.Contains(err.Error(), "连接被拒绝") {
		t.Fatalf("错误 = %v", err)
	}
}

// TestSocks5DialContext 验证通过假 SOCKS5 代理完成请求。
func TestSocks5DialContext(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "via-socks")
	}))
	defer target.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 2)
				if _, err := io.ReadFull(conn, buf); err != nil {
					return
				}
				if _, err := io.ReadFull(conn, make([]byte, int(buf[1]))); err != nil {
					return
				}
				if _, err := conn.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
					return
				}
				head := make([]byte, 4)
				if _, err := io.ReadFull(conn, head); err != nil {
					return
				}
				switch head[3] {
				case socks5AtypIPv4:
					_, _ = io.ReadFull(conn, make([]byte, 4+2))
				case socks5AtypName:
					lb := make([]byte, 1)
					_, _ = io.ReadFull(conn, lb)
					_, _ = io.ReadFull(conn, make([]byte, int(lb[0])+2))
				default:
					_, _ = io.ReadFull(conn, make([]byte, 16+2))
				}
				if _, err := conn.Write([]byte{socks5Version, 0x00, 0x00, socks5AtypIPv4, 127, 0, 0, 1, 0, 80}); err != nil {
					return
				}
				// 管道转发到目标 httptest 服务器
				up, err := net.Dial("tcp", strings.TrimPrefix(target.URL, "http://"))
				if err != nil {
					return
				}
				defer up.Close()
				go func() { _, _ = io.Copy(up, conn) }()
				_, _ = io.Copy(conn, up)
			}(conn)
		}
	}()

	proxyAddr := "socks5://" + ln.Addr().String()
	c := testClient(Config{Proxy: proxyAddr})
	resp, err := c.Get(target.URL)
	if err != nil {
		t.Fatalf("经 socks5 代理请求失败: %v", err)
	}
	if resp.Text() != "via-socks" {
		t.Fatalf("body = %q", resp.Text())
	}
}

// TestProgressReaderClose 验证进度包装器透传 Close。
func TestProgressReaderClose(t *testing.T) {
	rc := io.NopCloser(strings.NewReader("abc"))
	pr := newProgressReader(rc, 3, func(p Progress) {}, "download", time.Millisecond)
	closer, ok := pr.(io.Closer)
	if !ok {
		t.Fatal("包装后应实现 io.Closer")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	plain := strings.NewReader("x")
	if got := newProgressReader(plain, 0, nil, "download", 0); got != io.Reader(plain) {
		t.Fatal("无回调时应原样返回")
	}
}

// TestSelectLastResponseOnError 验证状态错误时返回响应。
func TestSelectLastResponseOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(418)
		_, _ = io.WriteString(w, "teapot")
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err == nil || resp == nil {
		t.Fatalf("err=%v resp=%v", err, resp)
	}
	if resp.Text() != "teapot" {
		t.Fatalf("业务错误体 = %q", resp.Text())
	}
}

// TestSleepCtxCancel 验证退避可被取消。
func TestSleepCtxCancel(t *testing.T) {
	if err := sleepCtx(context.Background(), 0); err != nil {
		t.Fatalf("0 延迟应返回 nil: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := sleepCtx(ctx, time.Second); err == nil {
		t.Fatal("应被 context 中断")
	}
}
