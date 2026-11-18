//go:build !js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 降级后端（兜底）：既没有系统内嵌浏览器能力、也没有可用的 Chromium 系浏览器时使用。
//
// 它不出画面，只维护状态（地址 / 标题 / 缩放 / 静音），并且提供
// OpenExternal() 让调用方一键交给系统浏览器打开 —— 保证同一套 UI 代码
// 在任何平台都能编译、能跑、有明确提示。

import (
	"strings"

	"guis/internal/geom"
)

// stubWebView 降级后端：记录状态、不出画面。
type stubWebView struct {
	url     string
	title   string
	rad     float32
	zoom    float64
	muted   bool
	ua      string
	loading bool
}

// newStubWebView 创建降级后端（不返回错误：让同一套 UI 代码处处能跑）。
func newStubWebView(a *App, opts WebViewOptions) (webViewBackend, error) {
	b := &stubWebView{
		rad:   opts.Radius,
		zoom:  opts.Zoom,
		muted: opts.Muted,
		ua:    strings.TrimSpace(opts.UserAgent),
	}
	if b.zoom <= 0 {
		b.zoom = 1
	}
	switch {
	case strings.TrimSpace(opts.HTML) != "":
		b.url = "about:blank"
		b.title = "本地 HTML 页面"
	case strings.TrimSpace(opts.URL) != "":
		b.url = normalizeURL(opts.URL)
		b.title = b.url
	}
	return b, nil
}

func (b *stubWebView) Ready() bool      { return false }
func (b *stubWebView) Err() error       { return errNoWebView }
func (b *stubWebView) Platform() string { return "stub" }

func (b *stubWebView) Navigate(url string) {
	if b == nil {
		return
	}
	b.url = normalizeURL(url)
	b.title = b.url
}

func (b *stubWebView) LoadHTML(html string) {
	if b == nil {
		return
	}
	b.url = "about:blank"
	b.title = "本地 HTML 页面"
}

func (b *stubWebView) Back()    {}
func (b *stubWebView) Forward() {}
func (b *stubWebView) Focus()   {}

func (b *stubWebView) Reload() {
	if b != nil {
		b.loading = true
		b.loading = false
	}
}

func (b *stubWebView) Stop() {
	if b != nil {
		b.loading = false
	}
}

func (b *stubWebView) Close() {}

func (b *stubWebView) SetRadius(r float32) {
	if b != nil {
		b.rad = r
	}
}

func (b *stubWebView) SetZoom(z float64) {
	if b != nil {
		b.zoom = clampWebViewZoom(z)
	}
}

func (b *stubWebView) SetMuted(m bool) {
	if b != nil {
		b.muted = m
	}
}

func (b *stubWebView) SetUserAgent(ua string) {
	if b != nil {
		b.ua = strings.TrimSpace(ua)
	}
}

func (b *stubWebView) Exec(js string) {}

func (b *stubWebView) ExecAsync(js string, cb func(result string)) {
	if cb != nil {
		cb("")
	}
}

func (b *stubWebView) URL() string {
	if b == nil {
		return ""
	}
	return b.url
}

func (b *stubWebView) Title() string {
	if b == nil {
		return ""
	}
	return b.title
}

func (b *stubWebView) Loading() bool { return b != nil && b.loading }

func (b *stubWebView) CanGoBack() bool { return false }

func (b *stubWebView) CanGoForward() bool { return false }

func (b *stubWebView) Place(r geom.Rect, scale float32) {}

func (b *stubWebView) Hide() {}
