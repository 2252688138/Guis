//go:build !js

package webcdp

// 真浏览器冒烟测试：需要本机装有 Chromium 系浏览器，否则跳过。
// 验证「启动 → 导航 → 求值 → 截图 → 改视图 → 输入 → 历史」整条链路。

import (
	"bytes"
	"image/jpeg"
	"testing"
	"time"
)

func TestCDPSessionSmoke(t *testing.T) {
	if LocateBrowser() == "" {
		t.Skip("本机没有 Chromium 系浏览器，跳过")
	}
	s, err := Launch(Config{URL: "about:blank", Width: 640, Height: 480, Headless: true})
	if err != nil {
		t.Fatalf("启动浏览器失败：%v", err)
	}
	defer s.Close()

	p := s.Page()
	page := "data:text/html,<h1 id=t>hello-cdp</h1><input id=i><script>document.title='T-CDP'</script>"
	if err := p.Navigate(page); err != nil {
		t.Fatalf("导航失败：%v", err)
	}
	if err := waitReady(p, 10*time.Second); err != nil {
		t.Fatalf("等页面就绪失败：%v", err)
	}
	if got, _ := p.Evaluate("document.getElementById('t').textContent"); got != "hello-cdp" {
		t.Fatalf("求值结果 = %q, 期望 hello-cdp", got)
	}

	shot, err := p.ScreenshotJPEG(60)
	if err != nil {
		t.Fatalf("截图失败：%v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(shot))
	if err != nil {
		t.Fatalf("截图不是合法 JPEG：%v", err)
	}
	if b := img.Bounds(); b.Dx() != 640 || b.Dy() != 480 {
		t.Fatalf("截图尺寸 = %dx%d, 期望 640x480", b.Dx(), b.Dy())
	}

	// 视图区 + 像素比（内嵌 DPI 缩放靠它）
	if err := p.SetViewport(320, 200, 2); err != nil {
		t.Fatalf("设置视图区失败：%v", err)
	}
	shot2, err := p.ScreenshotJPEG(60)
	if err != nil {
		t.Fatalf("第二次截图失败：%v", err)
	}
	img2, err := jpeg.Decode(bytes.NewReader(shot2))
	if err != nil {
		t.Fatalf("第二次截图非法：%v", err)
	}
	if b := img2.Bounds(); b.Dx() != 640 || b.Dy() != 400 {
		t.Fatalf("像素比 2 的截图 = %dx%d, 期望 640x400", b.Dx(), b.Dy())
	}

	// 输入：聚焦输入框后插字
	if _, err := p.Evaluate("document.getElementById('i').focus()"); err != nil {
		t.Fatalf("聚焦失败：%v", err)
	}
	if err := p.InsertText("abc"); err != nil {
		t.Fatalf("输入失败：%v", err)
	}
	if got, _ := p.Evaluate("document.getElementById('i').value"); got != "abc" {
		t.Fatalf("输入框内容 = %q, 期望 abc", got)
	}

	// 历史
	entries, idx, err := p.History()
	if err != nil {
		t.Fatalf("读历史失败：%v", err)
	}
	if len(entries) == 0 || idx < 0 {
		t.Fatalf("历史异常：%d 条, index=%d", len(entries), idx)
	}

	// 离线 HTML
	if err := p.LoadHTML("<p id=o>offline-ok</p>"); err != nil {
		t.Fatalf("加载 HTML 失败：%v", err)
	}
	if err := waitReady(p, 10*time.Second); err != nil {
		t.Fatalf("等离线页面就绪失败：%v", err)
	}
	if got, _ := p.Evaluate("document.getElementById('o').textContent"); got != "offline-ok" {
		t.Fatalf("离线页面内容 = %q", got)
	}
}

// waitReady 等文档加载完成。
func waitReady(p *Page, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v, err := p.Evaluate("document.readyState"); err == nil && v == "complete" {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return errTimeout
}

type timeoutError struct{}

func (timeoutError) Error() string { return "等待页面就绪超时" }

var errTimeout = timeoutError{}
