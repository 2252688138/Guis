//go:build !js

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 系统浏览器后端（CDP）：用本机的 Chrome / Chromium / Edge 真正加载网页，
// 通过 DevTools 协议取帧画进 GL 纹理，并把鼠标 / 键盘事件转发回网页。
//
// 与 Windows WebView2 后端的区别：
//   - 网页像素是普通纹理，能被自绘裁剪、圆角、被浮层盖住（WebView2 是子窗口，做不到）；
//   - 帧率受截图带宽限制（默认约 15fps），输入有几十毫秒延迟；
//   - 需要本机装有 Chromium 系浏览器（找不到时自动降级 stub 后端）。
//
// 所有 CDP 调用都在后台 goroutine 里做，UI 线程只负责「拿最新一帧纹理绘制」
// 与「把回调搬回 UI 线程」（见 Tick）。

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"encoding/json"
	"image"
	"image/draw"
	"image/jpeg"
	"sync"
	"sync/atomic"
	"time"

	"guis/internal/geom"
	"guis/internal/gpu"
	"guis/internal/webcdp"
)

// cdpCaptureInterval 取帧间隔（约 15fps：再高会明显吃 CPU）。
const cdpCaptureInterval = 66 * time.Millisecond

// cdpWebView CDP 后端实例。
type cdpWebView struct {
	app  *App
	sess *webcdp.Session
	page *webcdp.Page

	// 帧缓冲（后台线程写，UI 线程读）
	mu    sync.Mutex
	frame []byte
	pix   []byte
	fw    int
	fh    int
	gen   uint64
	seen  uint64

	// 状态（多数由后台线程更新，用原子或锁保护）
	ready    atomic.Bool
	visible  atomic.Bool
	closed   atomic.Bool
	loading  atomic.Bool
	metaBusy atomic.Bool

	stMu  sync.Mutex
	url   string
	title string
	err   error
	canBack  bool
	canFwd   bool

	// 视图参数（UI 线程写，后台读）
	vpMu  sync.Mutex
	wantW int
	wantH int
	wantS float64
	curW  int
	curH  int
	curS  float64
	zoom  float64
	rad   float32

	// 输入（UI 线程入队，单 worker 顺序派发）
	q       chan func()
	lastX   float32
	lastY   float32
	down    bool
	muted   bool
	ua      string
	keyword string

	// UI 回调队列（后台线程投递，Tick 在 UI 线程执行）
	uiMu    sync.Mutex
	uiQueue []func()

	tex   *gpu.Texture
	texW  int
	texH  int
	texGen uint64

	rect geom.Rect
}

// newCDPWebView 启动系统浏览器并连接 CDP。
func newCDPWebView(a *App, opts WebViewOptions) (*cdpWebView, error) {
	w, h := 1024, 768
	if opts.Zoom <= 0 {
		opts.Zoom = 1
	}
	sess, err := webcdp.Launch(webcdp.Config{
		BrowserPath: browserPathOf(opts),
		UserDataDir: opts.UserDataDir,
		Width:       w,
		Height:      h,
		Zoom:        opts.Zoom,
		URL:         "about:blank",
		Headless:    true,
		Muted:       opts.Muted,
		UserAgent:   opts.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	b := &cdpWebView{
		app:   a,
		sess:  sess,
		page:  sess.Page(),
		q:     make(chan func(), 128),
		zoom:  opts.Zoom,
		rad:   opts.Radius,
		muted: opts.Muted,
		ua:    opts.UserAgent,
	}
	b.ready.Store(true)

	// 页面事件：加载完成 / 导航后刷新状态
	sess.On("Page.loadEventFired", func(json.RawMessage) {
		b.loading.Store(false)
		// 读线程不能阻塞：CDP 命令一律丢到后台
		go b.refreshMeta()
		RequestRedraw()
	})
	sess.On("Page.frameNavigated", func(json.RawMessage) {
		go b.refreshMeta()
	})
	// 浏览器推帧：解码进帧缓冲，UI 线程下一帧把它画成纹理
	sess.On("Page.screencastFrame", func(params json.RawMessage) {
		var f struct {
			Data      string `json:"data"`
			SessionID int64  `json:"sessionId"`
		}
		if json.Unmarshal(params, &f) != nil || f.Data == "" {
			return
		}
		// 必须回执，否则浏览器不再推后续帧
		go func(id int64) {
			_, _ = b.page.S.CallTimeout("Page.screencastFrameAck", map[string]any{"sessionId": id}, 3*time.Second)
		}(f.SessionID)
		raw, err := base64.StdEncoding.DecodeString(f.Data)
		if err != nil {
			return
		}
		img, err := decodeJPEG(raw)
		if err != nil || img == nil {
			return
		}
		b.mu.Lock()
		b.pix, b.fw, b.fh = img.Pix, img.Bounds().Dx(), img.Bounds().Dy()
		b.gen++
		b.mu.Unlock()
		b.stMu.Lock()
		b.err = nil
		b.stMu.Unlock()
		RequestRedraw()
	})

	go b.worker()
	go b.captureLoop()

	switch {
	case opts.HTML != "":
		b.LoadHTML(opts.HTML)
	case opts.URL != "":
		b.Navigate(normalizeURL(opts.URL))
	}
	if b.rad > 0 {
		b.SetRadius(b.rad)
	}
	if b.zoom != 1 {
		b.SetZoom(b.zoom)
	}
	return b, nil
}

// browserPathOf 取调用方指定的浏览器路径（环境变量兜底，空 = 自动探测）。
func browserPathOf(opts WebViewOptions) string {
	return opts.BrowserPath
}

// refreshMeta 后台线程刷新地址 / 标题 / 历史（失败静默）。
func (b *cdpWebView) refreshMeta() {
	if b == nil || !b.metaBusy.CompareAndSwap(false, true) {
		return
	}
	defer b.metaBusy.Store(false)
	url, err := b.page.Evaluate("location.href")
	if err == nil && url != "" {
		b.stMu.Lock()
		if b.url != url {
			b.url = url
			b.title = "" // 换页后标题等下次读
		}
		b.stMu.Unlock()
	}
	if title, err := b.page.Evaluate("document.title"); err == nil {
		b.stMu.Lock()
		b.title = title
		b.stMu.Unlock()
	}
	if entries, idx, err := b.page.History(); err == nil {
		b.stMu.Lock()
		b.canBack = idx > 0
		b.canFwd = idx >= 0 && idx < len(entries)-1
		b.stMu.Unlock()
	}
}

// captureLoop 维持推帧：尺寸 / 像素比变化时重启 screencast。
//
// 这里不用「反复 Page.captureScreenshot 轮询」——headless 下截图命令偶发不返回，
// 一旦卡住会把整条 CDP 命令通道堵死（后续 getFrameTree 之类全部超时）。
// screencast 由浏览器主动推帧（页面没变化就不推），既省 CPU 又不会阻塞命令通道。
func (b *cdpWebView) captureLoop() {
	for !b.closed.Load() {
		if b.visible.Load() {
			b.applyViewport()
		}
		time.Sleep(80 * time.Millisecond)
	}
}

// startScreencast 按当前视口与像素比开始推帧。
func (b *cdpWebView) startScreencast() {
	b.enqueue(func() {
		b.vpMu.Lock()
		w, h := b.wantW, b.wantH
		b.vpMu.Unlock()
		if w < 1 || h < 1 {
			return
		}
		// maxWidth/maxHeight 只是上限：给足空间，HiDPI / 缩放的像素比由
		// Emulation.setDeviceMetricsOverride 决定，改尺寸时不用重启推帧
		_, _ = b.page.S.Call("Page.stopScreencast", nil)
		_, err := b.page.S.Call("Page.startScreencast", map[string]any{
			"format":        "jpeg",
			"quality":       62,
			"maxWidth":      4096,
			"maxHeight":     4096,
			"everyNthFrame": 1,
		})
		b.stMu.Lock()
		b.err = err
		b.stMu.Unlock()
		RequestRedraw()
	})
}

// stopScreencast 停止推帧（窗口隐藏时省 CPU）。
func (b *cdpWebView) stopScreencast() {
	b.enqueue(func() { _, _ = b.page.S.Call("Page.stopScreencast", nil) })
}

// decodeJPEG 解码为 RGBA（预乘，与纹理上传一致）。
func decodeJPEG(data []byte) (*image.RGBA, error) {
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst, nil
}

// applyViewport 尺寸 / 像素比变化时同步给浏览器（后台线程调用）。
func (b *cdpWebView) applyViewport() {
	b.vpMu.Lock()
	w, h, s := b.wantW, b.wantH, b.wantS
	if w == b.curW && h == b.curH && s == b.curS {
		b.vpMu.Unlock()
		return
	}
	b.curW, b.curH, b.curS = w, h, s
	b.vpMu.Unlock()
	_ = b.page.SetViewport(w, h, s)
	// 改尺寸后重启推帧：startScreencast 会立刻补一帧新尺寸的画面，
	// 否则页面没有可见变化时浏览器不会主动推帧，纹理就会停在旧尺寸上。
	if b.visible.Load() {
		b.startScreencast()
	}
}

// worker 顺序派发输入事件（避免 UI 线程等待 CDP）。
func (b *cdpWebView) worker() {
	for fn := range b.q {
		if b.closed.Load() {
			return
		}
		fn()
	}
}

// enqueue 投递一个后台任务（队列满则丢弃，保证 UI 不卡）。
func (b *cdpWebView) enqueue(fn func()) {
	if b == nil || b.closed.Load() {
		return
	}
	select {
	case b.q <- fn:
	default:
	}
}

// postUI 把回调搬回 UI 线程（由 Tick 执行）。
func (b *cdpWebView) postUI(fn func()) {
	if b == nil || fn == nil {
		return
	}
	b.uiMu.Lock()
	b.uiQueue = append(b.uiQueue, fn)
	b.uiMu.Unlock()
	RequestRedraw()
}

// ---- webViewBackend ----

func (b *cdpWebView) Ready() bool { return b != nil && b.ready.Load() }

func (b *cdpWebView) Err() error {
	if b == nil {
		return errNoWebView
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.err
}

func (b *cdpWebView) Platform() string { return "system-browser-cdp" }

func (b *cdpWebView) Navigate(url string) {
	b.loading.Store(true)
	b.enqueue(func() {
		if err := b.page.Navigate(url); err != nil {
			b.stMu.Lock()
			b.err = err
			b.stMu.Unlock()
		}
	})
}

func (b *cdpWebView) LoadHTML(html string) {
	b.loading.Store(true)
	b.enqueue(func() {
		if err := b.page.LoadHTML(html); err != nil {
			b.stMu.Lock()
			b.err = err
			b.stMu.Unlock()
		}
	})
}

func (b *cdpWebView) Back() {
	b.enqueue(func() { _, _ = b.page.Evaluate("history.back()") })
}

func (b *cdpWebView) Forward() {
	b.enqueue(func() { _, _ = b.page.Evaluate("history.forward()") })
}

func (b *cdpWebView) Reload() {
	b.loading.Store(true)
	b.enqueue(func() { _ = b.page.Reload() })
}

func (b *cdpWebView) Stop() {
	b.loading.Store(false)
	b.enqueue(func() { _ = b.page.Stop() })
}

func (b *cdpWebView) Focus() {
	b.enqueue(func() { _, _ = b.page.Evaluate("window.focus()") })
}

func (b *cdpWebView) Close() {
	if b == nil || !b.closed.CompareAndSwap(false, true) {
		return
	}
	close(b.q)
	go b.sess.Close()
	b.mu.Lock()
	tex := b.tex
	b.tex = nil
	b.texW, b.texH = 0, 0
	b.mu.Unlock()
	if tex != nil && b.app != nil && b.app.r != nil {
		b.app.r.DeleteTexture(tex)
	}
}

func (b *cdpWebView) SetRadius(r float32) {
	if b == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	b.rad = r
	RequestRedraw()
}

// SetZoom 页面缩放：用 CSS zoom 缩放网页内容（取帧尺寸不变，纹理不重建）。
func (b *cdpWebView) SetZoom(z float64) {
	if b == nil {
		return
	}
	b.zoom = clampWebViewZoom(z)
	script := fmt.Sprintf("document.documentElement.style.zoom='%g'", b.zoom)
	b.enqueue(func() { _, _ = b.page.Evaluate(script) })
	RequestRedraw()
}

func (b *cdpWebView) SetMuted(m bool) {
	if b == nil {
		return
	}
	b.muted = m
	script := mediaMuteScript(m)
	b.enqueue(func() { _, _ = b.page.Evaluate(script) })
}

func (b *cdpWebView) SetUserAgent(ua string) {
	if b == nil {
		return
	}
	b.ua = ua
	b.enqueue(func() { _ = b.page.SetUserAgent(ua) })
}

func (b *cdpWebView) Exec(js string) {
	if b == nil || js == "" {
		return
	}
	b.enqueue(func() { _, _ = b.page.Evaluate(js) })
}

func (b *cdpWebView) ExecAsync(js string, cb func(result string)) {
	if b == nil {
		if cb != nil {
			cb("")
		}
		return
	}
	b.enqueue(func() {
		res, err := b.page.Evaluate(js)
		if err != nil {
			res = ""
		}
		if cb != nil {
			b.postUI(func() { cb(res) })
		}
	})
}

func (b *cdpWebView) URL() string {
	if b == nil {
		return ""
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.url
}

func (b *cdpWebView) Title() string {
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

func (b *cdpWebView) Loading() bool { return b != nil && b.loading.Load() }

func (b *cdpWebView) CanGoBack() bool {
	if b == nil {
		return false
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.canBack
}

func (b *cdpWebView) CanGoForward() bool {
	if b == nil {
		return false
	}
	b.stMu.Lock()
	defer b.stMu.Unlock()
	return b.canFwd
}

// Place 记录节点矩形并同步视口尺寸（UI 线程，不阻塞）。
func (b *cdpWebView) Place(r geom.Rect, scale float32) {
	if b == nil || b.closed.Load() {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	b.rect = r
	if b.visible.CompareAndSwap(false, true) {
		b.startScreencast()
	}

	w := int(r.W + 0.5)
	h := int(r.H + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	b.vpMu.Lock()
	// 像素比固定 1：Chrome 的 screencast 只能降采样不能放大，
	// 想要更大尺寸只能把「视图区」设大（那会改变网页布局），所以这里按 CSS 像素取帧，
	// 绘制时再由 GPU 缩放到节点矩形（HiDPI 下文字略软，需要锐利请用 WebView2 后端）。
	b.wantW, b.wantH, b.wantS = w, h, 1
	b.vpMu.Unlock()
}

// Hide 隐藏（后台停止抓帧）。
func (b *cdpWebView) Hide() {
	if b != nil && b.visible.CompareAndSwap(true, false) {
		b.stopScreencast()
	}
}

// ---- webViewPainter（绘制阶段画纹理） ----

func (b *cdpWebView) Paint(r geom.Rect) {
	if b == nil || b.app == nil || b.app.r == nil {
		return
	}
	rad := [4]float32{b.rad, b.rad, b.rad, b.rad}
	b.mu.Lock()
	pix, w, h, gen := b.pix, b.fw, b.fh, b.gen
	b.mu.Unlock()

	if pix == nil || w < 1 || h < 1 {
		b.app.r.Rect(gpu.Shape{Rect: r, Radius: rad, Fill: paintOf(ColText, 0.06)})
		if err := b.Err(); err != nil {
			b.app.paintWebViewCard(r, &Style{Radius: rad}, "系统浏览器连接失败\n"+cdpShortErr(err.Error()), ColDanger, false)
		}
		return
	}
	b.mu.Lock()
	if b.tex == nil || b.texW != w || b.texH != h {
		if b.tex != nil {
			b.app.r.DeleteTexture(b.tex)
		}
		b.tex = b.app.r.NewTextureRGBA(w, h, pix)
		b.texW, b.texH, b.texGen = w, h, gen
	} else if b.texGen != gen {
		b.app.r.UpdateTextureRGBA(b.tex, 0, 0, w, h, pix)
		b.texGen = gen
	}
	tex := b.tex
	b.mu.Unlock()

	b.app.r.DrawImage(gpu.Image{
		Rect:   r,
		Tex:    tex,
		Tint:   gpu.Paint{R: 1, G: 1, B: 1, A: 1},
		Radius: rad,
	})
}

// ---- webViewTicker（UI 线程每帧回调） ----

func (b *cdpWebView) Tick() {
	if b == nil {
		return
	}
	b.uiMu.Lock()
	q := b.uiQueue
	b.uiQueue = nil
	b.uiMu.Unlock()
	for _, fn := range q {
		fn()
	}
}

// ---- webViewInput（把自绘控件的事件转发进网页） ----

func (b *cdpWebView) PointerMove(x, y float32) {
	if b == nil {
		return
	}
	b.lastX, b.lastY = x, y
	lx, ly := x, y
	b.enqueue(func() {
		if b.down {
			_, _ = b.page.S.Call("Input.dispatchMouseEvent", map[string]any{
				"type":    "mouseMoved",
				"x":       lx,
				"y":       ly,
				"button":  "left",
				"buttons": 1,
			})
			return
		}
		_ = b.page.MouseMove(float64(lx), float64(ly))
	})
}

func (b *cdpWebView) PointerDown() {
	if b == nil {
		return
	}
	b.down = true
	lx, ly := b.lastX, b.lastY
	b.enqueue(func() { _ = b.page.MouseDown(float64(lx), float64(ly), "left") })
}

func (b *cdpWebView) PointerDrag(x, y float32) {
	if b == nil {
		return
	}
	if !b.down {
		b.PointerMove(x, y)
		b.PointerDown()
		return
	}
	b.lastX, b.lastY = x, y
	lx, ly := x, y
	b.enqueue(func() {
		_, _ = b.page.S.Call("Input.dispatchMouseEvent", map[string]any{
			"type":    "mouseMoved",
			"x":       lx,
			"y":       ly,
			"button":  "left",
			"buttons": 1,
		})
	})
}

func (b *cdpWebView) PointerUp() {
	if b == nil {
		return
	}
	b.down = false
	b.enqueue(func() { _ = b.page.MouseUp(float64(b.lastX), float64(b.lastY), "left") })
}

func (b *cdpWebView) Wheel(dy float32) {
	if b == nil {
		return
	}
	x, y := b.lastX, b.lastY
	b.enqueue(func() { _ = b.page.Wheel(float64(x), float64(y), 0, float64(dy)) })
}

// Key 具名按键转发（Node.OnKey 的键码 → CDP 按键名）。
func (b *cdpWebView) Key(code uint32) {
	name := cdpKeyName(code)
	if name == "" {
		return
	}
	b.enqueue(func() { _ = b.page.NamedKey(name) })
}

// Char 字符输入转发（中文等走 insertText）。
func (b *cdpWebView) Char(r rune) {
	if b == nil || r == 0 {
		return
	}
	text := string(r)
	b.enqueue(func() { _ = b.page.InsertText(text) })
}

// cdpKeyName 自绘按键码 → CDP 具名按键。
func cdpKeyName(code uint32) string {
	switch code {
	case 8:
		return "backspace"
	case 9:
		return "tab"
	case 13:
		return "enter"
	case 27:
		return "escape"
	case 32:
		return "space"
	case 33:
		return "pageup"
	case 34:
		return "pagedown"
	case 35:
		return "end"
	case 36:
		return "home"
	case 37:
		return "left"
	case 38:
		return "up"
	case 39:
		return "right"
	case 40:
		return "down"
	case 46:
		return "delete"
	}
	return ""
}

// cdpShortErr 错误信息截断（占位卡片用）。
func cdpShortErr(s string) string {
	s = string([]rune(s))
	if len([]rune(s)) > 90 {
		return string([]rune(s)[:89]) + "…"
	}
	return s
}


