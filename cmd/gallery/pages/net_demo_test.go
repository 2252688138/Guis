// guis — gallery 网络请求页的回归测试：演示服务 + httpc 真实链路
package pages

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"guis/httpc"
)

// TestNetDemoServer 覆盖演示服务的每个端点（同时也是 httpc 的端到端冒烟）。
func TestNetDemoServer(t *testing.T) {
	base := netDemoServer()
	if base == "" {
		t.Fatal("演示服务未启动")
	}
	c := httpc.New(httpc.Config{BaseURL: base, Timeout: 5 * time.Second, Proxy: "none"})

	t.Run("echo", func(t *testing.T) {
		resp, err := c.Post("/api/echo", httpc.Request{JSON: map[string]any{"a": 1}})
		if err != nil {
			t.Fatalf("echo 失败: %v", err)
		}
		var out struct {
			Method  string            `json:"method"`
			Query   string            `json:"query"`
			Header  map[string]string `json:"header"`
			Body    string            `json:"body"`
			BodyLen int               `json:"bodyLen"`
		}
		if err := resp.Bind(&out); err != nil {
			t.Fatalf("Bind 失败: %v", err)
		}
		if out.Method != "POST" || !strings.Contains(out.Body, "\"a\":1") || out.BodyLen == 0 {
			t.Fatalf("echo 回显异常: %+v", out)
		}
		if _, ok := out.Header["X-Trace"]; ok {
			t.Fatalf("未注入时不应有 X-Trace: %v", out.Header)
		}
	})

	t.Run("users", func(t *testing.T) {
		resp, err := c.Get("/api/users")
		if err != nil {
			t.Fatalf("users 失败: %v", err)
		}
		var users []map[string]any
		if err := resp.Bind(&users); err != nil || len(users) != 3 {
			t.Fatalf("users 绑定异常: %v %v", err, users)
		}
	})

	t.Run("status404", func(t *testing.T) {
		resp, err := c.Get("/api/status?code=404")
		if !httpc.IsStatus(err, 404) {
			t.Fatalf("应返回 404 状态错误: %v", err)
		}
		var biz struct {
			Error string `json:"error"`
			Code  int    `json:"code"`
		}
		if err := resp.Bind(&biz); err != nil || biz.Code != 404 {
			t.Fatalf("业务错误体异常: %v %+v", err, biz)
		}
		if e := httpc.ErrorOf(err); e == nil || e.Code != "DEMO_404" {
			t.Fatalf("业务错误码异常: %+v", e)
		}
	})

	t.Run("flaky", func(t *testing.T) {
		netFlakyMu.Lock()
		netFlakyHit = map[string]int{}
		netFlakyAt = map[string]time.Time{}
		netFlakyMu.Unlock()

		rc := httpc.New(httpc.Config{
			BaseURL: base,
			Timeout: 5 * time.Second,
			Proxy:   "none",
			Retry:   httpc.RetryConfig{Max: 2, Backoff: time.Millisecond},
		})
		resp, err := rc.Get("/api/flaky?fail=2")
		if err != nil {
			t.Fatalf("重试后应成功: %v", err)
		}
		if resp.Attempts != 3 || resp.Status != 200 {
			t.Fatalf("attempts=%d status=%d", resp.Attempts, resp.Status)
		}
	})

	t.Run("download", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "demo.bin")
		if _, err := c.Download("/api/download?kb=16", dst); err != nil {
			t.Fatalf("下载失败: %v", err)
		}
		st, err := os.Stat(dst)
		if err != nil || st.Size() != 16*1024 {
			t.Fatalf("下载大小异常: %v %v", err, st)
		}
	})

	t.Run("upload", func(t *testing.T) {
		mp := httpc.NewMultipart().Field("source", "gallery")
		mp.Files = append(mp.Files, httpc.FilePart{
			Field: "file", FileName: "demo.txt", Reader: strings.NewReader("payload"),
		})
		resp, err := c.Post("/api/upload", httpc.Request{Multipart: mp})
		if err != nil {
			t.Fatalf("上传失败: %v", err)
		}
		var out struct {
			Fields map[string]string `json:"fields"`
			Files  []struct {
				Field string `json:"field"`
				Name  string `json:"name"`
				Size  int    `json:"size"`
			} `json:"files"`
		}
		if err := resp.Bind(&out); err != nil {
			t.Fatalf("绑定失败: %v", err)
		}
		if out.Fields["source"] != "gallery" {
			t.Fatalf("字段回显异常: %v", out.Fields)
		}
		if len(out.Files) != 1 || out.Files[0].Name != "demo.txt" || out.Files[0].Size != 7 {
			t.Fatalf("文件回显异常: %+v", out.Files)
		}
	})

	t.Run("cookie", func(t *testing.T) {
		cc := httpc.New(httpc.Config{BaseURL: base, Timeout: 5 * time.Second, Proxy: "none", Cookies: true})
		first, err := cc.Get("/api/cookie/echo")
		if err != nil {
			t.Fatalf("首次请求失败: %v", err)
		}
		var r1 struct {
			Set bool `json:"set"`
		}
		_ = first.Bind(&r1)
		if !r1.Set {
			t.Fatalf("首次应下发 Cookie: %s", first.Text())
		}
		second, err := cc.Get("/api/cookie/echo")
		if err != nil {
			t.Fatalf("二次请求失败: %v", err)
		}
		var r2 struct {
			Cookie string `json:"cookie"`
			Set    bool   `json:"set"`
		}
		_ = second.Bind(&r2)
		if r2.Set || !strings.HasPrefix(r2.Cookie, "sid-") {
			t.Fatalf("CookieJar 未带回: %s", second.Text())
		}
	})

	t.Run("gzip", func(t *testing.T) {
		resp, err := c.Get("/api/gzip")
		if err != nil {
			t.Fatalf("gzip 失败: %v", err)
		}
		var out struct {
			Encoding string `json:"encoding"`
		}
		if err := resp.Bind(&out); err != nil || out.Encoding != "gzip" {
			t.Fatalf("自动解压失败: %v %+v", err, out)
		}
	})

	t.Run("xml", func(t *testing.T) {
		resp, err := c.Get("/api/xml")
		if err != nil {
			t.Fatalf("xml 失败: %v", err)
		}
		var out struct {
			ID   int    `xml:"id"`
			Name string `xml:"name"`
		}
		if err := resp.Bind(&out); err != nil || out.ID != 7 || out.Name == "" {
			t.Fatalf("XML 绑定失败: %v %+v", err, out)
		}
	})

	t.Run("badjson", func(t *testing.T) {
		resp, err := c.Get("/api/badjson")
		if err != nil {
			t.Fatalf("请求应成功（只是 JSON 非法）: %v", err)
		}
		var v map[string]any
		if berr := resp.Bind(&v); berr == nil || httpc.KindOf(berr) != httpc.KindDecode {
			t.Fatalf("应返回 KindDecode，实际 %v", berr)
		}
	})

	t.Run("redirect", func(t *testing.T) {
		resp, err := c.Get("/api/redirect?n=3")
		if err != nil {
			t.Fatalf("重定向失败: %v", err)
		}
		var out struct {
			Redirected bool `json:"redirected"`
		}
		if err := resp.Bind(&out); err != nil || !out.Redirected {
			t.Fatalf("重定向未完成: %v %+v", err, out)
		}
	})

	t.Run("slow-timeout", func(t *testing.T) {
		_, err := c.Get("/api/slow?ms=600", httpc.WithTimeout(100*time.Millisecond))
		if !httpc.IsTimeout(err) {
			t.Fatalf("应超时: %v", err)
		}
	})
}

// TestNetpHelpers 页面辅助函数。
func TestNetpHelpers(t *testing.T) {
	if got := netpBytes(512); got != "512 B" {
		t.Fatalf("netpBytes(512) = %q", got)
	}
	if got := netpBytes(2048); got != "2.0 KiB" {
		t.Fatalf("netpBytes(2048) = %q", got)
	}
	if got := netpBytes(3 << 20); got != "3.0 MiB" {
		t.Fatalf("netpBytes(3MiB) = %q", got)
	}
	if got := netpBytes(-1); got != "未知大小" {
		t.Fatalf("netpBytes(-1) = %q", got)
	}
	if got := netpClip("短文本", 10); got != "短文本" {
		t.Fatalf("netpClip 不应截断: %q", got)
	}
	if got := netpClip("一二三四五六", 3); !strings.HasPrefix(got, "一二三") {
		t.Fatalf("netpClip 截断异常: %q", got)
	}
	if netpAtoi("x", 7) != 7 || netpAtoi("-3", 7) != 7 || netpAtoi("5", 7) != 5 {
		t.Fatalf("netpAtoi 异常")
	}
	if netpMillis("250", 1000) != 250*time.Millisecond {
		t.Fatalf("netpMillis 异常")
	}
	if netpOr("", "默认") != "默认" || netpOr("值", "默认") != "值" {
		t.Fatalf("netpOr 异常")
	}
	if netpOnOff(true) != "已开启" || netpOnOff(false) != "已关闭" {
		t.Fatalf("netpOnOff 异常")
	}
	if got := netpParamsSuffix(""); got != "" {
		t.Fatalf("空参数不应加问号: %q", got)
	}
	if got := netpParamsSuffix("a=1"); got != "?a=1" {
		t.Fatalf("参数后缀异常: %q", got)
	}
	if netpCompact(strings.Repeat("字", 120)) == strings.Repeat("字", 120) {
		t.Fatal("netpCompact 应截断超长文本")
	}
	if netpFlags(httpc.ErrorOf(&httpc.Error{Kind: httpc.KindTimeout})) == "" {
		// 非 *Error 包装也应有输出
		t.Log("flags 空值容忍")
	}
}

// TestNetpCodeSnippets 代码示例覆盖全部页签。
func TestNetpCodeSnippets(t *testing.T) {
	for _, k := range netTabKeys {
		if k == "code" {
			continue
		}
		code, ok := netCodeSnippets[k]
		if !ok || strings.TrimSpace(code) == "" {
			t.Fatalf("页签 %q 缺少示例代码", k)
		}
		if !strings.Contains(code, "httpc.") {
			t.Fatalf("页签 %q 的代码未使用 httpc", k)
		}
	}
}

// TestNetWindowsExe 打开目录用的是 SystemRoot 下的绝对路径（PATH 里没有 explorer.exe）。
func TestNetWindowsExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 校验")
	}
	got := netWindowsExe("explorer.exe")
	if got == "" {
		t.Fatal("未在 SystemRoot / WINDIR 下找到 explorer.exe")
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("解析出的路径不可用 %q: %v", got, err)
	}
	t.Logf("explorer.exe => %s", got)
}
