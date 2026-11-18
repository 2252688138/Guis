//go:build (linux || freebsd) && !android

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 真窗口嵌入后端（Linux / X11）：性能路线。
//
// 做法：用系统浏览器带窗口启动（--app=URL），再用 XReparentWindow 把它的顶层窗口
// 挂到 GUIS 窗口里，随着节点矩形移动 / 缩放；窗口隐藏时 Unmap。
//
// 优点：网页由浏览器自己合成到窗口里，帧率与浏览器一致（无截图开销），
// 输入 / 中文输入法 / 视频播放全部原生。
// 代价：X11 子窗口永远画在 GL 画面之上，所以这块区域无法被自绘浮层覆盖、
// 也不支持圆角裁剪；任何一步失败（找不到窗口、重挂载失败）都会自动回退到
// CDP 纹理后端（newCDPWebView），保证功能不丢失。
//
// 想强制走哪条路：GUIS_WEBVIEW_BACKEND=embed / cdp / stub。

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"guis/internal/geom"
	"guis/internal/webcdp"
	"guis/internal/x11"
)

// embedWebView X11 真窗口嵌入后端。
type embedWebView struct {
	app    *App
	sess   *webcdp.Session
	page   *webcdp.Page
	dpy    *x11.Display
	parent x11.Window

	mu      sync.Mutex
	child   x11.Window
	mapped  bool
	closed  bool
	lastX   int
	lastY   int
	lastW   int
	lastH   int
	visible bool

	stMu    sync.Mutex
	url     string
	title   string
	loading bool
	err     error

	zoom  float64
	muted bool
	ua    string
}

// embedBrowserAvailable 本平台能否真窗口嵌入（X11 + 本机有 Chromium 系浏览器）。
func embedBrowserAvailable() bool {
	return x11Disp != nil && webcdp.LocateBrowser() != ""
}

// newEmbeddedBrowserBackend 创建真窗口嵌入后端（不可用时返回 false）。
func newEmbeddedBrowserBackend(a *App, opts WebViewOptions) (webViewBackend, bool) {
	if a == nil || a.win == nil || x11Disp == nil {
		return nil, false
	}
	pw, ok := a.win.(*platformWin)
	if !ok || pw.win == 0 {
		return nil, false
	}
	if webcdp.LocateBrowser() == "" {
		return nil, false
	}
	zoom := opts.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	w, h := int(pw.pw), int(pw.ph)
	if w < 64 || h < 64 {
		w, h = 1024, 720
	}
	sess, err := webcdp.Launch(webcdp.Config{
		BrowserPath: opts.BrowserPath,
		UserDataDir: opts.UserDataDir,
		Width:       w,
		Height:      h,
		ShowWindow:  true,
		Muted:       opts.Muted,
		UserAgent:   opts.UserAgent,
		URL:         "about:blank",
	})
	if err != nil {
		return nil, false
	}
	b := &embedWebView{
		app:    a,
		sess:   sess,
		page:   sess.Page(),
		dpy:    x11Disp,
		parent: pw.win,
		zoom:   zoom,
		muted:  opts.Muted,
		ua:     opts.UserAgent,
	}

	// 浏览器窗口是异步建出来的：按 _NET_WM_PID 轮询查找
	pid := 0
	if br := sess.Browser(); br != nil {
		pid = br.Pid()
	}
	var child x11.Window
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && child == 0 {
		child = b.dpy.FindWindowByPID(pid)
		if child == 0 {
			time.Sleep(120 * time.Millisecond)
		}
	}
	if child == 0 {
		sess.Close()
		return nil, false
	}
	// 先藏起来再挂载，避免浏览器窗口在屏幕上闪一下
	b.dpy.Unmap(child)
	if err := b.dpy.Reparent(child, pw.win, 0, 0); err != nil {
		sess.Close()
		return nil, false
	}
	b.dpy.Flush()
	b.mu.Lock()
	b.child = child
	b.mu.Unlock()

	// 页面事件：刷新地址 / 标题 / 加载状态
	sess.On("Page.loadEventFired", func(json.RawMessage) {
		b.stMu.Lock()
		b.loading = false
		b.stMu.Unlock()
		go b.refreshMeta()
		RequestRedraw()
	})
	sess.On("Page.frameNavigated", func(json.RawMessage) {
		go b.refreshMeta()
	})

	switch {
	case strings.TrimSpace(opts.HTML) != "":
		b.LoadHTML(opts.HTML)
	case strings.TrimSpace(opts.URL) != "":
		b.Navigate(normalizeURL(opts.URL))
	}
	if zoom != 1 {
		b.SetZoom(zoom)
	}
	return b, true
}

// refreshMeta 后台刷新地址与标题（不能在事件回调里同步发命令，会自己等自己）。
func (b *embedWebView) refreshMeta() {
	if b == nil || b.page == nil {
		return
	}
	if url, err := b.page.Evaluate("location.href"); err == nil && url != "" {
		b.stMu.Lock()
		b.url = url
		b.stMu.Unlock()
	}
	if title, err := b.page.Evaluate("document.title"); err == nil {
		b.stMu.Lock()
		b.title = title
		b.stMu.Unlock()
	}
}

// ---- webViewBackend ----

func (b *embedWebView) Ready() bool      { return b != nil }
func (b *embedWebView) Platform() string { return "system-browser-x11" }

func (b *embedWebView) Err() error {
	if b == nil {
		return errNoWebView
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.err
}

func (b *embedWebView) Navigate(url string) {
	if b == nil {
		return
	}
	b.stMu.Lock()
	b.loading = true
	b.url = url
	b.stMu.Unlock()
	go func() {
		if err := b.page.Navigate(url); err != nil {
			b.stMu.Lock()
			b.err = err
			b.stMu.Unlock()
		}
	}()
}

func (b *embedWebView) LoadHTML(html string) {
	if b == nil {
		return
	}
	b.stMu.Lock()
	b.loading = true
	b.stMu.Unlock()
	go func() { _ = b.page.LoadHTML(html) }()
}

func (b *embedWebView) Back()    { b.exec("history.back()") }
func (b *embedWebView) Forward() { b.exec("history.forward()") }
func (b *embedWebView) Focus()   { b.focusChild() }

func (b *embedWebView) Reload() {
	if b == nil {
		return
	}
	b.stMu.Lock()
	b.loading = true
	b.stMu.Unlock()
	go func() { _ = b.page.Reload() }()
}

func (b *embedWebView) Stop() {
	if b == nil {
		return
	}
	b.stMu.Lock()
	b.loading = false
	b.stMu.Unlock()
	go func() { _ = b.page.Stop() }()
}

// Close 结束浏览器进程（子窗口随进程消失）。
func (b *embedWebView) Close() {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	child := b.child
	b.child = 0
	b.mu.Unlock()
	if child != 0 && b.dpy != nil {
		b.dpy.Unmap(child)
		b.dpy.Flush()
	}
	go b.sess.Close()
}

func (b *embedWebView) SetRadius(r float32) {
	// X11 子窗口是矩形，圆角只能靠浏览器页面自身（CSS）实现
	_ = r
}

func (b *embedWebView) SetZoom(z float64) {
	if b == nil {
		return
	}
	b.zoom = clampWebViewZoom(z)
	b.exec("document.documentElement.style.zoom='" + strconv.FormatFloat(b.zoom, 'g', -1, 64) + "'")
}

func (b *embedWebView) SetMuted(m bool) {
	if b == nil {
		return
	}
	b.muted = m
	b.exec(mediaMuteScript(m))
}

func (b *embedWebView) SetUserAgent(ua string) {
	if b == nil {
		return
	}
	b.ua = ua
	go func() { _ = b.page.SetUserAgent(ua) }()
}

func (b *embedWebView) Exec(js string) { b.exec(js) }

func (b *embedWebView) ExecAsync(js string, cb func(result string)) {
	if b == nil {
		if cb != nil {
			cb("")
		}
		return
	}
	go func() {
		res, err := b.page.Evaluate(js)
		if err != nil {
			res = ""
		}
		if cb != nil {
			cb(res)
		}
	}()
}

func (b *embedWebView) URL() string {
	if b == nil {
		return ""
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.url
}

func (b *embedWebView) Title() string {
	if b == nil {
		return ""
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	if b.title != "" {
		return b.title
	}
	return b.url
}

func (b *embedWebView) Loading() bool {
	if b == nil {
		return false
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.loading
}

func (b *embedWebView) CanGoBack() bool    { return true }
func (b *embedWebView) CanGoForward() bool { return true }

// exec 后台执行一段 JS。
func (b *embedWebView) exec(js string) {
	if b == nil || js == "" {
		return
	}
	go func() { _, _ = b.page.Evaluate(js) }()
}


// Place 把浏览器子窗口摆到节点矩形上（逻辑像素 → 物理像素）。
func (b *embedWebView) Place(r geom.Rect, scale float32) {
	if b == nil {
		return
	}
	b.mu.Lock()
	child, closed := b.child, b.closed
	b.visible = true
	b.mu.Unlock()
	if closed || child == 0 {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	x := int(r.X*scale + 0.5)
	y := int(r.Y*scale + 0.5)
	w := int(r.W*scale + 0.5)
	h := int(r.H*scale + 0.5)
	// 超出窗口客户区就裁掉（X11 子窗口不会自己裁）
	maxW, maxH := 0, 0
	if b.app != nil && b.app.win != nil {
		pw, ph := b.app.win.PixelSize()
		maxW, maxH = int(pw), int(ph)
	}
	if maxW > 0 && maxH > 0 {
		if x < 0 {
			w += x
			x = 0
		}
		if y < 0 {
			h += y
			y = 0
		}
		if x+w > maxW {
			w = maxW - x
		}
		if y+h > maxH {
			h = maxH - y
		}
	}
	if w <= 0 || h <= 0 {
		b.Hide()
		return
	}
	b.mu.Lock()
	changed := x != b.lastX || y != b.lastY || w != b.lastW || h != b.lastH
	needMap := !b.mapped
	b.lastX, b.lastY, b.lastW, b.lastH = x, y, w, h
	if needMap {
		b.mapped = true
	}
	b.mu.Unlock()

	if changed {
		b.dpy.MoveResize(child, x, y, w, h)
	}
	if needMap {
		b.dpy.Map(child)
		b.dpy.SetInputFocus(child)
	}
	if changed || needMap {
		b.dpy.Flush()
	}
}

// Hide 隐藏浏览器子窗口（停止占位与遮挡）。
func (b *embedWebView) Hide() {
	if b == nil {
		return
	}
	b.mu.Lock()
	child, mapped := b.child, b.mapped
	b.mapped = false
	b.visible = false
	b.mu.Unlock()
	if child == 0 || !mapped {
		return
	}
	b.dpy.Unmap(child)
	if b.parent != 0 {
		b.dpy.SetInputFocus(b.parent)
	}
	b.dpy.Flush()
}

// focusChild 把键盘焦点交给网页。
func (b *embedWebView) focusChild() {
	if b == nil {
		return
	}
	b.mu.Lock()
	child, mapped := b.child, b.mapped
	b.mu.Unlock()
	if child == 0 || !mapped {
		return
	}
	b.dpy.SetInputFocus(child)
	b.dpy.Flush()
}
