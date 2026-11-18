package httpc

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

// TestRetryThenSuccess 验证 5xx 重试后成功。
func TestRetryThenSuccess(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(500)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c := testClient(Config{Retry: RetryConfig{Max: 2, Backoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond}})
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if resp.Text() != "ok" || resp.Attempts != 3 {
		t.Fatalf("body=%q attempts=%d", resp.Text(), resp.Attempts)
	}
}

// TestRetryExhausted 验证重试耗尽后返回状态错误。
func TestRetryExhausted(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(503)
	}))
	defer srv.Close()

	c := testClient(Config{Retry: RetryConfig{Max: 2, Backoff: time.Millisecond}})
	_, err := c.Get(srv.URL)
	if !IsStatus(err, 503) {
		t.Fatalf("应返回 503 错误，实际 %v", err)
	}
	if attempts != 3 {
		t.Fatalf("尝试次数 = %d，期望 3", attempts)
	}
	if e := ErrorOf(err); e == nil || e.Attempts != 3 {
		t.Fatalf("错误中尝试次数 = %+v", e)
	}
}

// TestRetrySkipOn4xx 验证 4xx 不重试。
func TestRetrySkipOn4xx(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(400)
	}))
	defer srv.Close()

	c := testClient(Config{Retry: RetryConfig{Max: 3, Backoff: time.Millisecond}})
	if _, err := c.Get(srv.URL); !IsStatus(err, 400) {
		t.Fatalf("应返回 400，实际 %v", err)
	}
	if attempts != 1 {
		t.Fatalf("400 不应重试，实际尝试 %d 次", attempts)
	}
}

// TestRetryOnCustom 验证自定义重试判定。
func TestRetryOnCustom(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := testClient(Config{Retry: RetryConfig{
		Max:     3,
		Backoff: time.Millisecond,
		RetryOn: func(resp *Response, err error) bool { return false },
	}})
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("应返回错误")
	}
	if attempts != 1 {
		t.Fatalf("自定义 RetryOn=false 不应重试，实际 %d 次", attempts)
	}
}

// TestRetryNoRetryOption 验证 WithNoRetry。
func TestRetryNoRetryOption(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(502)
			return
		}
	}))
	defer srv.Close()

	c := testClient(Config{Retry: RetryConfig{Max: 5, Backoff: time.Millisecond}})
	if _, err := c.Get(srv.URL, WithNoRetry()); !IsStatus(err, 502) {
		t.Fatalf("应返回 502，实际 %v", err)
	}
	if attempts != 1 {
		t.Fatalf("WithNoRetry 未生效，尝试 %d 次", attempts)
	}
}

// TestRetryDelayUnits 验证退避与 Retry-After 计算。
func TestRetryDelayUnits(t *testing.T) {
	cfg := RetryConfig{Backoff: 100 * time.Millisecond, MaxBackoff: time.Second, Jitter: -1}
	if d := retryDelay(cfg, 1, nil); d != 100*time.Millisecond {
		t.Fatalf("第 1 次退避 = %v", d)
	}
	if d := retryDelay(cfg, 3, nil); d != 400*time.Millisecond {
		t.Fatalf("第 3 次退避 = %v", d)
	}
	resp := &Response{Header: http.Header{"Retry-After": {"2"}}}
	if d := retryDelay(cfg, 1, resp); d != 2*time.Second {
		t.Fatalf("Retry-After 未生效: %v", d)
	}
}

// TestGzipResponse 验证 gzip 自动解压。
func TestGzipResponse(t *testing.T) {
	payload := strings.Repeat("guis-", 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Errorf("缺少 Accept-Encoding: %q", r.Header.Get("Accept-Encoding"))
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, payload)
		_ = gz.Close()
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if resp.Text() != payload {
		t.Fatalf("解压结果长度 = %d，期望 %d", len(resp.Text()), len(payload))
	}
	if resp.Size != int64(len(payload)) {
		t.Fatalf("Size = %d，期望 %d", resp.Size, len(payload))
	}
}

// TestBrotliResponse 验证 br 自动解压。
func TestBrotliResponse(t *testing.T) {
	payload := strings.Repeat("brotli-", 200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		bw := brotli.NewWriter(w)
		_, _ = io.WriteString(bw, payload)
		_ = bw.Close()
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if resp.Text() != payload {
		t.Fatalf("br 解压失败，长度 = %d", len(resp.Text()))
	}
}

// TestCookieJar 验证 Cookie 跨请求保持。
func TestCookieJar(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("sid"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "abc", Path: "/"})
			return
		}
		got = "abc"
	}))
	defer srv.Close()

	c := testClient(Config{Cookies: true})
	if _, err := c.Get(srv.URL); err != nil {
		t.Fatalf("首次请求失败: %v", err)
	}
	if _, err := c.Get(srv.URL); err != nil {
		t.Fatalf("第二次请求失败: %v", err)
	}
	if got != "abc" {
		t.Fatal("Cookie 未跨请求保持")
	}
}

// TestRedirect 验证重定向策略。
func TestRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/b", http.StatusFound)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "final")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := testClient().Get(srv.URL + "/a")
	if err != nil {
		t.Fatalf("默认应跟随重定向: %v", err)
	}
	if resp.Text() != "final" {
		t.Fatalf("body = %q", resp.Text())
	}

	resp, err = testClient().Get(srv.URL+"/a", WithMaxRedirects(-1))
	if err != nil {
		t.Fatalf("禁止重定向时不应报错: %v", err)
	}
	if resp.Status != http.StatusFound {
		t.Fatalf("status = %d，期望 302", resp.Status)
	}
}

// TestRedirectStripsAuth 验证跨域重定向剔除敏感头。
func TestRedirectStripsAuth(t *testing.T) {
	var seenAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
	}))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/x", http.StatusFound)
	}))
	defer src.Close()

	c := testClient()
	if _, err := c.Get(src.URL, WithBearer("secret")); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if seenAuth != "" {
		t.Fatalf("跨域重定向应剔除 Authorization，实际 = %q", seenAuth)
	}
}

// TestMaxContentLength 验证响应体限长。
func TestMaxContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 4096))
	}))
	defer srv.Close()

	_, err := testClient().Get(srv.URL, WithMaxContentLength(100))
	if err == nil || KindOf(err) != KindTooLarge {
		t.Fatalf("应返回 KindTooLarge，实际 %v", err)
	}
	trunced, err := testClient().Get(srv.URL, WithMaxContentLength(8192))
	if err != nil {
		t.Fatalf("限长内应成功: %v", err)
	}
	if len(trunced.Data) != 4096 {
		t.Fatalf("长度 = %d", len(trunced.Data))
	}
}

// TestConcurrencyLimit 验证并发限流。
func TestConcurrencyLimit(t *testing.T) {
	var cur, peak int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&cur, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if n <= old || atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&cur, -1)
	}))
	defer srv.Close()

	c := testClient(Config{MaxConcurrency: 2})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Get(srv.URL); err != nil {
				t.Errorf("Get 失败: %v", err)
			}
		}()
	}
	wg.Wait()
	if peak > 2 {
		t.Fatalf("并发峰值 = %d，超过限制 2", peak)
	}
}

// TestProgressDownload 验证下载进度回调。
func TestProgressDownload(t *testing.T) {
	payload := bytes.Repeat([]byte("z"), 256<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	var mu sync.Mutex
	var last Progress
	var calls int
	_, err := testClient().Get(srv.URL, WithProgress(func(p Progress) {
		mu.Lock()
		last = p
		calls++
		mu.Unlock()
	}), WithProgressInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls == 0 {
		t.Fatal("进度回调未被调用")
	}
	if !last.Done || last.Loaded != int64(len(payload)) {
		t.Fatalf("最后一条进度 = %+v", last)
	}
	if last.Phase != "download" {
		t.Fatalf("phase = %q", last.Phase)
	}
	if last.Percent < 99.9 {
		t.Fatalf("percent = %v", last.Percent)
	}
}

// TestProgressUpload 验证上传进度回调。
func TestProgressUpload(t *testing.T) {
	payload := bytes.Repeat([]byte("u"), 128<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()

	var sawUpload bool
	_, err := testClient().Post(srv.URL, payload, WithProgress(func(p Progress) {
		if p.Phase == "upload" {
			sawUpload = true
		}
	}))
	if err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if !sawUpload {
		t.Fatal("未收到上传进度回调")
	}
}

// TestMultipartUpload 验证分段上传字段与文件。
func TestMultipartUpload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("file-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotField, gotFileName, gotContent, gotInline string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Errorf("ParseMultipartForm 失败: %v", err)
			return
		}
		gotField = r.FormValue("name")
		f, hdr, err := r.FormFile("upload")
		if err == nil {
			b, _ := io.ReadAll(f)
			gotFileName = hdr.Filename
			gotContent = string(b)
		}
		if f2, h2, err2 := r.FormFile("inline"); err2 == nil {
			b, _ := io.ReadAll(f2)
			gotInline = h2.Filename + ":" + string(b)
		}
	}))
	defer srv.Close()

	mp := NewMultipart().
		Field("name", "guis").
		AddFile("upload", path)
	mp.Files = append(mp.Files, FilePart{Field: "inline", FileName: "m.txt", Reader: strings.NewReader("mem")})

	if _, err := testClient().Post(srv.URL, Request{Multipart: mp}); err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if gotField != "guis" || gotFileName != "a.txt" || gotContent != "file-content" {
		t.Fatalf("field=%q file=%q content=%q", gotField, gotFileName, gotContent)
	}
	if gotInline != "m.txt:mem" {
		t.Fatalf("inline = %q", gotInline)
	}
}

// TestMultipartContentType 验证分隔符一致。
func TestMultipartContentType(t *testing.T) {
	mp := NewMultipart().Field("a", "1")
	if _, err := mp.body(); err != nil {
		t.Fatalf("body 失败: %v", err)
	}
	src, err := mp.body()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(src.contentType, "multipart/form-data; boundary=") {
		t.Fatalf("contentType = %q", src.contentType)
	}
	rc, _, err := src.open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("读取 multipart 失败: %v", err)
	}
	if !strings.Contains(string(data), `name="a"`) || !strings.Contains(string(data), "\r\n\r\n1\r\n") {
		t.Fatalf("multipart 内容异常: %q", string(data))
	}
}

// TestDownloadFile 验证下载落盘与进度。
func TestDownloadFile(t *testing.T) {
	payload := bytes.Repeat([]byte("d"), 64<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	var done bool
	if _, err := testClient().Download(srv.URL, dst, WithProgress(func(p Progress) {
		if p.Done {
			done = true
		}
	})); err != nil {
		t.Fatalf("Download 失败: %v", err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if !bytes.Equal(b, payload) {
		t.Fatalf("下载内容不一致，长度 = %d", len(b))
	}
	if !done {
		t.Fatal("未收到完成进度回调")
	}
	entries, _ := os.ReadDir(filepath.Dir(dst))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".httpc-") {
			t.Fatalf("残留临时文件: %s", e.Name())
		}
	}
}

// TestUploadHelper 验证 Upload 便捷方法。
func TestUploadHelper(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		f, _, err := r.FormFile("f")
		if err != nil {
			return
		}
		b, _ := io.ReadAll(f)
		got = string(b)
	}))
	defer srv.Close()

	if _, err := testClient().Upload(srv.URL, []FilePart{{Field: "f", FileName: "x.txt", Reader: strings.NewReader("hi")}}); err != nil {
		t.Fatalf("Upload 失败: %v", err)
	}
	if got != "hi" {
		t.Fatalf("上传内容 = %q", got)
	}
}

// TestTLSSkipVerify 验证自定义 TLS 配置。
func TestTLSSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "tls-ok")
	}))
	defer srv.Close()

	_, err := testClient(Config{TLS: &TLSConfig{InsecureSkipVerify: true}}).Get(srv.URL)
	if err != nil {
		t.Fatalf("跳过校验应成功: %v", err)
	}
	_, err = testClient().Get(srv.URL)
	if err == nil {
		t.Fatal("自签证书应校验失败")
	}
}

// TestSocks5AddrEncoding 验证 SOCKS5 地址编码。
func TestSocks5AddrEncoding(t *testing.T) {
	b, err := socks5DestAddr("example.com:80")
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if b[0] != socks5AtypName || b[1] != 11 {
		t.Fatalf("域名编码 = %v", b)
	}
	if _, err := socks5DestAddr("not-an-address"); err == nil {
		t.Fatal("非法地址应报错")
	}
	if b4, err := socks5DestAddr("127.0.0.1:8080"); err != nil || b4[0] != socks5AtypIPv4 {
		t.Fatalf("IPv4 编码 = %v err=%v", b4, err)
	}
	if socks5ReplyText(0x05) != "连接被拒绝" {
		t.Fatalf("错误码文本 = %q", socks5ReplyText(0x05))
	}
}

// TestProxyParseError 验证非法代理报错。
func TestProxyParseError(t *testing.T) {
	_, err := testClient(Config{Proxy: "ftp://x"}).Get("http://example.com")
	if err == nil {
		t.Fatal("不支持的代理协议应报错")
	}
	if KindOf(err) != KindConfig {
		t.Fatalf("Kind = %q", KindOf(err))
	}
}

// TestResponseSaveTo 非流式响应落盘。
func TestResponseSaveTo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "save-me")
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "x.txt")
	if err := resp.SaveTo(dst); err != nil {
		t.Fatalf("SaveTo 失败: %v", err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "save-me" {
		t.Fatalf("内容 = %q", b)
	}
}
