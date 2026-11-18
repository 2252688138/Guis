// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 内嵌浏览器核心层的平台无关单测：用假后端断言「代码生成了什么」，
// 因此三端都能跑（真实后端行为由各平台手工验证）。

import (
	"fmt"
	"strings"
	"testing"

	"guis/internal/geom"
)

// fakeBackend 记录调用的假后端。
type fakeBackend struct {
	execs []string
	navs  []string

	url     string
	title   string
	loading bool
	zoom    float64
	muted   bool
	ua      string
	err     error

	placed int
	hidden int
	closed bool
}

func (f *fakeBackend) Ready() bool      { return true }
func (f *fakeBackend) Err() error       { return f.err }
func (f *fakeBackend) Platform() string { return "fake" }

func (f *fakeBackend) Navigate(url string) {
	f.navs = append(f.navs, url)
	f.url = url
}
func (f *fakeBackend) LoadHTML(html string) { f.url = "about:srcdoc" }
func (f *fakeBackend) Back()                {}
func (f *fakeBackend) Forward()             {}
func (f *fakeBackend) Reload()              {}
func (f *fakeBackend) Stop()                {}
func (f *fakeBackend) Focus()               {}
func (f *fakeBackend) Close()               { f.closed = true }

func (f *fakeBackend) SetRadius(r float32)    {}
func (f *fakeBackend) SetZoom(z float64)      { f.zoom = z }
func (f *fakeBackend) SetMuted(m bool)        { f.muted = m }
func (f *fakeBackend) SetUserAgent(ua string) { f.ua = ua }

func (f *fakeBackend) Exec(js string) { f.execs = append(f.execs, js) }
func (f *fakeBackend) ExecAsync(js string, cb func(result string)) {
	if cb != nil {
		cb("")
	}
}

func (f *fakeBackend) URL() string        { return f.url }
func (f *fakeBackend) Title() string      { return f.title }
func (f *fakeBackend) Loading() bool      { return f.loading }
func (f *fakeBackend) CanGoBack() bool    { return false }
func (f *fakeBackend) CanGoForward() bool { return false }

func (f *fakeBackend) Place(r geom.Rect, scale float32) { f.placed++ }
func (f *fakeBackend) Hide()                            { f.hidden++ }

func (f *fakeBackend) lastExec() string {
	if len(f.execs) == 0 {
		return ""
	}
	return f.execs[len(f.execs)-1]
}

// newFake 造一个只带假后端的 WebView（不依赖窗口）。
func newFake() (*WebView, *fakeBackend) {
	fb := &fakeBackend{}
	wv := &WebView{be: fb, zoom: 1, histI: -1}
	return wv, fb
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"go.dev", "https://go.dev"},
		{"https://go.dev", "https://go.dev"},
		{"HTTP://Go.Dev/Path", "HTTP://Go.Dev/Path"},
		{"about:blank", "about:blank"},
		{"file:///tmp/a.html", "file:///tmp/a.html"},
		{"  example.com/a?b=1  ", "https://example.com/a?b=1"},
		{"ftp://x/y", "ftp://x/y"},
	}
	for _, c := range cases {
		if got := normalizeURL(c.in); got != c.want {
			t.Fatalf("normalizeURL(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

func TestWebViewHistoryCap(t *testing.T) {
	wv, _ := newFake()
	for i := 0; i < 120; i++ {
		wv.Navigate(fmt.Sprintf("https://example.com/%d", i))
		wv.poll()
	}
	hist := wv.History()
	if len(hist) != webViewHistoryMax {
		t.Fatalf("历史长度 = %d, 期望 %d", len(hist), webViewHistoryMax)
	}
	if wv.HistoryIndex() != len(hist)-1 {
		t.Fatalf("HistoryIndex = %d, 期望 %d", wv.HistoryIndex(), len(hist)-1)
	}
	if hist[len(hist)-1] != "https://example.com/119" {
		t.Fatalf("最后一条 = %q", hist[len(hist)-1])
	}

	wv.GoHistory(0)
	if wv.HistoryIndex() != 0 {
		t.Fatalf("GoHistory(0) 后 HistoryIndex = %d, 期望 0", wv.HistoryIndex())
	}
	if wv.URL() != hist[0] {
		t.Fatalf("GoHistory(0) 后 URL = %q, 期望 %q", wv.URL(), hist[0])
	}
}

func TestWebViewAllowNavigation(t *testing.T) {
	wv, fb := newFake()
	wv.SetAllowNavigation(func(url string) bool { return !strings.Contains(url, "blocked") })

	wv.Navigate("https://blocked.example.com")
	if len(fb.navs) != 0 {
		t.Fatalf("白名单拦截失败，navs = %v", fb.navs)
	}
	wv.Navigate("example.com/ok")
	if len(fb.navs) != 1 || fb.navs[0] != "https://example.com/ok" {
		t.Fatalf("放行失败，navs = %v", fb.navs)
	}
}

func TestWebViewInjectCSS(t *testing.T) {
	wv, fb := newFake()
	wv.InjectCSS("body{color:red}")
	wv.InjectCSS("a{color:blue}")
	if len(fb.execs) != 2 {
		t.Fatalf("注入脚本数量 = %d, 期望 2", len(fb.execs))
	}
	if !strings.Contains(fb.execs[0], "guis-inject-1") || !strings.Contains(fb.execs[1], "guis-inject-2") {
		t.Fatalf("注入 id 不符合预期：%v", fb.execs)
	}
	wv.ClearInjectedCSS()
	last := fb.lastExec()
	if !strings.Contains(last, "guis-inject-1") || !strings.Contains(last, "guis-inject-2") {
		t.Fatalf("移除脚本缺少 id：%q", last)
	}
	if len(wv.injectCSS) != 0 {
		t.Fatalf("注入表未清空：%v", wv.injectCSS)
	}
}

func TestWebViewZoom(t *testing.T) {
	wv, fb := newFake()
	wv.SetZoom(9)
	if wv.ZoomFactor() != 5 {
		t.Fatalf("上限夹紧失败：%v", wv.ZoomFactor())
	}
	wv.SetZoom(0.01)
	if wv.ZoomFactor() != 0.25 {
		t.Fatalf("下限夹紧失败：%v", wv.ZoomFactor())
	}
	wv.ZoomReset()
	if wv.ZoomFactor() != 1 || fb.zoom != 1 {
		t.Fatalf("重置失败：%v / %v", wv.ZoomFactor(), fb.zoom)
	}
	wv.ZoomIn()
	if wv.ZoomFactor() <= 1 {
		t.Fatalf("ZoomIn 未生效：%v", wv.ZoomFactor())
	}
	wv.ZoomOut()
	if v := wv.ZoomFactor(); v < 0.99 || v > 1.01 {
		t.Fatalf("ZoomIn + ZoomOut 未回到 1：%v", v)
	}
}

func TestWebViewPost(t *testing.T) {
	wv, fb := newFake()
	payload := "he said \"hi\"\n中文"
	wv.Post(payload)
	last := fb.lastExec()
	if !strings.Contains(last, "__guisRecv") {
		t.Fatalf("Post 未调用 __guisRecv：%q", last)
	}
	if !strings.Contains(last, "\\\"hi\\\"") || !strings.Contains(last, "中文") {
		t.Fatalf("Post 负载转义不符合预期：%q", last)
	}

	if err := wv.PostJSON(map[string]any{"a": 1}); err != nil {
		t.Fatalf("PostJSON 意外失败：%v", err)
	}
	if !strings.Contains(fb.lastExec(), "{\\\"a\\\":1}") {
		t.Fatalf("PostJSON 负载不对：%q", fb.lastExec())
	}

	if err := wv.PostJSON(func() {}); err == nil {
		t.Fatalf("PostJSON 对不可序列化值应返回错误")
	}
}

func TestWebViewFind(t *testing.T) {
	wv, fb := newFake()
	if wv.Find("   ") {
		t.Fatalf("空关键字应返回 false")
	}
	if len(fb.execs) != 0 {
		t.Fatalf("空关键字不应下发脚本：%v", fb.execs)
	}
	if !wv.Find("abc") {
		t.Fatalf("Find 应返回 true")
	}
	if wv.FindText() != "abc" {
		t.Fatalf("FindText = %q", wv.FindText())
	}
	if !strings.Contains(fb.lastExec(), "window.find") {
		t.Fatalf("Find 脚本不对：%q", fb.lastExec())
	}
	wv.FindNext()
	wv.FindPrev()
	if n := len(fb.execs); n != 3 {
		t.Fatalf("脚本数量 = %d, 期望 3", n)
	}
	wv.ClearFind()
	if wv.FindText() != "" {
		t.Fatalf("ClearFind 后 FindText 应为空")
	}
}

func TestWebViewBridgeJS(t *testing.T) {
	for _, want := range []string{"guisSend", "__guisRecv", "guis:message", "guisOnMessage", "__guisDrain"} {
		if !strings.Contains(WebViewBridgeJS, want) {
			t.Fatalf("WebViewBridgeJS 缺少 %q", want)
		}
	}
}

func TestWebViewPollEvents(t *testing.T) {
	wv, fb := newFake()
	var (
		starts  []string
		titles  []string
		doneURL string
		doneOK  bool
	)
	wv.opts.OnNavStart = func(u string) { starts = append(starts, u) }
	wv.opts.OnTitle = func(s string) { titles = append(titles, s) }
	wv.opts.OnNavDone = func(u string, ok bool) { doneURL, doneOK = u, ok }

	fb.loading = true
	wv.Navigate("https://example.com/a")
	wv.poll()
	fb.title = "示例"
	wv.poll()
	fb.loading = false
	wv.poll()

	if len(starts) != 1 || starts[0] != "https://example.com/a" {
		t.Fatalf("OnNavStart = %v", starts)
	}
	if len(titles) != 1 || titles[0] != "示例" {
		t.Fatalf("OnTitle = %v", titles)
	}
	if doneURL != "https://example.com/a" || !doneOK {
		t.Fatalf("OnNavDone = %q / %v", doneURL, doneOK)
	}
}

func TestWebViewStubState(t *testing.T) {
	wv, fb := newFake()
	wv.SetMuted(true)
	if !wv.Muted() || !fb.muted {
		t.Fatalf("静音状态未同步")
	}
	wv.SetDarkMode(true)
	if !wv.DarkMode() || !strings.Contains(fb.lastExec(), "guis-dark") {
		t.Fatalf("深色模式注入失败：%q", fb.lastExec())
	}
	wv.SetDarkMode(false)
	if wv.DarkMode() {
		t.Fatalf("关闭深色后状态不对")
	}
	if got := wv.Platform(); got != "fake" {
		t.Fatalf("Platform = %q", got)
	}
	wv.Close()
	if !fb.closed {
		t.Fatalf("Close 未透传到后端")
	}
}
