//go:build js && wasm

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 浏览器端（js/wasm）后端：用 <iframe> 承载网页，叠加在画布上
// （pointer-events:auto，可直接与网页交互）。
//
// 与桌面端（WebView2 子窗口）的差异：
//   - 没有独立进程，网页复用浏览器自身的渲染与网络栈；
//   - 跨域页面读不到 location / title / document，脚本注入会抛 SecurityError，
//     因此 ExecuteScript / 查找 / 注入 CSS 只在同源页面生效（失败返回 false）；
//   - 前进后退走自建地址栈（跨域时读 iframe 历史会抛异常）。

import (
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// iframeBackend 浏览器端内核。
type iframeBackend struct {
	app  *App
	url  string
	html string

	title   string
	loading bool
	visible bool
	zoom    float64
	radius  float32
	muted   bool
	ua      string

	id    string
	el    js.Value
	funcs []js.Func
	stack []string
	idx   int
}

// webViewSeq 浏览器端实例编号（单线程，无需加锁）。
var webViewSeq int

// webViewCapable 浏览器端始终可用。
func webViewCapable() bool { return true }

// newWebViewBackend 创建 iframe 后端。
func newWebViewBackend(a *App, opts WebViewOptions) (webViewBackend, error) {
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return nil, errors.New("ui: 浏览器端没有 document，无法创建内嵌浏览器")
	}
	b := &iframeBackend{
		app:    a,
		radius: opts.Radius,
		zoom:   opts.Zoom,
		muted:  opts.Muted,
		ua:     strings.TrimSpace(opts.UserAgent),
		idx:    -1,
	}
	if b.zoom <= 0 {
		b.zoom = 1
	}
	webViewSeq++
	b.id = fmt.Sprintf("guis-webview-%d", webViewSeq)

	el := doc.Call("createElement", "iframe")
	el.Set("id", b.id)
	el.Set("allow", "fullscreen; autoplay; clipboard-read; clipboard-write")
	st := el.Get("style")
	st.Set("position", "fixed")
	st.Set("left", "0px")
	st.Set("top", "0px")
	st.Set("width", "0px")
	st.Set("height", "0px")
	st.Set("border", "0")
	st.Set("margin", "0")
	st.Set("zIndex", "4")
	st.Set("display", "none")
	st.Set("pointerEvents", "auto")
	st.Set("borderRadius", fmt.Sprintf("%.2fpx", b.radius))
	if opts.Transparent {
		st.Set("background", "transparent")
	} else {
		st.Set("background", "#ffffff")
	}
	if b.ua != "" {
		el.Set("data-guis-ua", b.ua)
	}
	b.el = el

	parent := doc.Get("body")
	if parent.IsUndefined() || parent.IsNull() {
		parent = doc.Get("documentElement")
	}
	if !parent.IsUndefined() && !parent.IsNull() {
		parent.Call("appendChild", el)
	}

	load := js.FuncOf(func(this js.Value, args []js.Value) any {
		b.loading = false
		if s := el.Get("src").String(); s != "" && s != "about:blank" {
			b.url = s
			b.push(s)
		}
		b.syncTitle()
		if b.muted {
			b.SetMuted(true)
		}
		RequestRedraw()
		return nil
	})
	b.funcs = append(b.funcs, load)
	el.Call("addEventListener", "load", load)

	switch {
	case strings.TrimSpace(opts.HTML) != "":
		b.LoadHTML(opts.HTML)
	case strings.TrimSpace(opts.URL) != "":
		b.Navigate(normalizeURL(opts.URL))
	}
	if opts.OnChange != nil {
		opts.OnChange()
	}
	return b, nil
}

// openExternalURL 浏览器端开新标签页打开地址。
func openExternalURL(url string) {
	url = strings.TrimSpace(url)
	if url == "" || url == "about:blank" {
		return
	}
	js.Global().Call("open", url, "_blank")
}

func (b *iframeBackend) Ready() bool {
	return b != nil && !b.el.IsUndefined() && !b.el.IsNull()
}

func (b *iframeBackend) Err() error {
	if b == nil || b.el.IsUndefined() || b.el.IsNull() {
		return errNoWebView
	}
	return nil
}

func (b *iframeBackend) Platform() string { return "browser-iframe" }

// Navigate 打开地址（写入 iframe.src）。
func (b *iframeBackend) Navigate(url string) {
	if b == nil {
		return
	}
	u := normalizeURL(url)
	b.url = u
	b.loading = true
	b.push(u)
	if b.Ready() {
		b.el.Set("srcdoc", "")
		b.el.Set("src", u)
	}
	RequestRedraw()
}

// LoadHTML 加载 HTML 片段（iframe srcdoc）。
func (b *iframeBackend) LoadHTML(html string) {
	if b == nil {
		return
	}
	b.html = html
	b.loading = true
	b.url = "about:srcdoc"
	if b.Ready() {
		b.el.Set("src", "about:blank")
		b.el.Set("srcdoc", html)
	}
	RequestRedraw()
}

// push 把地址压入自建栈（跨域时读不到 iframe 历史，只能自己记）。
func (b *iframeBackend) push(url string) {
	if b == nil || url == "" {
		return
	}
	if b.idx >= 0 && b.idx < len(b.stack) && b.stack[b.idx] == url {
		return
	}
	if b.idx+1 < len(b.stack) {
		b.stack = b.stack[:b.idx+1]
	}
	b.stack = append(b.stack, url)
	b.idx = len(b.stack) - 1
}

// navTo 跳到自建栈里的地址（不再压栈）。
func (b *iframeBackend) navTo(url string) {
	if b == nil || !b.Ready() {
		return
	}
	b.url = url
	b.loading = true
	b.el.Set("srcdoc", "")
	b.el.Set("src", url)
	RequestRedraw()
}

func (b *iframeBackend) Back() {
	if b == nil || b.idx <= 0 {
		return
	}
	b.idx--
	b.navTo(b.stack[b.idx])
}

func (b *iframeBackend) Forward() {
	if b == nil || b.idx+1 >= len(b.stack) {
		return
	}
	b.idx++
	b.navTo(b.stack[b.idx])
}

func (b *iframeBackend) Reload() {
	if b == nil || !b.Ready() {
		return
	}
	b.loading = true
	if b.html != "" {
		b.LoadHTML(b.html)
		return
	}
	if b.url != "" {
		b.el.Set("src", b.url)
	}
	RequestRedraw()
}

func (b *iframeBackend) Stop() {
	if b == nil {
		return
	}
	b.loading = false
	if b.Ready() {
		// 停不下来（iframe 没有 stop），把地址回退到当前 src 即可
		b.el.Set("src", b.el.Get("src").String())
	}
	RequestRedraw()
}

func (b *iframeBackend) Focus() {
	if b == nil || !b.Ready() {
		return
	}
	b.el.Call("focus")
}

func (b *iframeBackend) Close() {
	if b == nil {
		return
	}
	for _, f := range b.funcs {
		f.Release()
	}
	b.funcs = nil
	if !b.el.IsUndefined() && !b.el.IsNull() {
		if p := b.el.Get("parentNode"); !p.IsNull() && !p.IsUndefined() {
			p.Call("removeChild", b.el)
		}
	}
	b.el = js.Undefined()
}

func (b *iframeBackend) SetRadius(r float32) {
	if b == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	b.radius = r
	if b.Ready() {
		b.el.Get("style").Set("borderRadius", fmt.Sprintf("%.2fpx", r))
	}
}

func (b *iframeBackend) SetZoom(z float64) {
	if b == nil {
		return
	}
	b.zoom = clampWebViewZoom(z)
	if b.Ready() {
		b.el.Get("style").Set("zoom", fmt.Sprintf("%g", b.zoom))
	}
}

func (b *iframeBackend) SetMuted(m bool) {
	if b == nil {
		return
	}
	b.muted = m
	b.Exec(mediaMuteScript(m))
}

func (b *iframeBackend) SetUserAgent(ua string) {
	if b == nil {
		return
	}
	b.ua = strings.TrimSpace(ua)
	if b.Ready() && b.ua != "" {
		// iframe 无法改 UA（浏览器安全策略），记录下来供页面自行读取
		b.el.Set("data-guis-ua", b.ua)
	}
}

func (b *iframeBackend) Exec(code string) {
	b.execSafe(code)
}

func (b *iframeBackend) ExecAsync(code string, cb func(result string)) {
	out, ok := b.execSafe(code)
	if !ok {
		out = ""
	}
	if cb != nil {
		cb(out)
	}
}

// execSafe 在 iframe 里执行脚本：跨域 / 语法错误一律当作失败返回。
func (b *iframeBackend) execSafe(code string) (out string, ok bool) {
	if b == nil || code == "" || !b.Ready() {
		return "", false
	}
	defer func() {
		if r := recover(); r != nil {
			out, ok = "", false
		}
	}()
	win := b.el.Get("contentWindow")
	if win.IsUndefined() || win.IsNull() {
		return "", false
	}
	v := win.Call("eval", code)
	return jsToString(v), true
}

// jsToString 把 JS 值转成字符串（undefined/null → 空串）。
func jsToString(v js.Value) string {
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	if v.Type() == js.TypeString {
		return v.String()
	}
	s := js.Global().Get("String").Invoke(v)
	if s.IsUndefined() || s.IsNull() {
		return ""
	}
	return s.String()
}

// syncTitle 同源时读取 document.title（跨域忽略）。
func (b *iframeBackend) syncTitle() {
	if t, ok := b.execSafe("document.title"); ok {
		b.title = strings.TrimSpace(t)
	}
}

func (b *iframeBackend) URL() string {
	if b == nil {
		return ""
	}
	return b.url
}

func (b *iframeBackend) Title() string {
	if b == nil {
		return ""
	}
	if b.title != "" {
		return b.title
	}
	return b.url
}

func (b *iframeBackend) Loading() bool { return b != nil && b.loading }

func (b *iframeBackend) CanGoBack() bool { return b != nil && b.idx > 0 }

func (b *iframeBackend) CanGoForward() bool { return b != nil && b.idx+1 < len(b.stack) }

// Paint 绘制阶段记录 iframe 位置（Swap 时统一写 DOM）。
func (b *iframeBackend) Paint(r geom.Rect) {
	if b == nil || !b.Ready() {
		return
	}
	var clip geom.Rect
	if b.app != nil {
		clip = b.app.clipRect(r)
	}
	b.visible = true
	radius := fmt.Sprintf("%.2fpx", b.radius)
	placeOverlay(b.id, r, clip, func(el js.Value) {
		st := el.Get("style")
		st.Set("pointerEvents", "auto")
		st.Set("borderRadius", radius)
	})
}

// Place DOM 位置已在绘制阶段记录（见 Paint），这里只保证可见。
func (b *iframeBackend) Place(r geom.Rect, scale float32) {
	if b == nil || !b.Ready() {
		return
	}
	b.el.Get("style").Set("display", "block")
}

// Hide 隐藏 iframe（滚出视口 / 切页）。
func (b *iframeBackend) Hide() {
	if b == nil || !b.Ready() {
		return
	}
	b.visible = false
	b.el.Get("style").Set("display", "none")
}

// cssRadius 生成 CSS 圆角值（四角）。
func cssRadius(r [4]float32) string {
	if r[0] == r[1] && r[1] == r[2] && r[2] == r[3] {
		return fmt.Sprintf("%.2fpx", r[0])
	}
	return fmt.Sprintf("%.2fpx %.2fpx %.2fpx %.2fpx", r[0], r[1], r[2], r[3])
}

func (a *App) paintVideo(n *Node, r geom.Rect, st *Style) {
	p := n.Video
	if p == nil {
		a.paintVideoPlaceholder(n, r, st)
		return
	}
	// 记录画面矩形，供控制条自动隐藏与点击判定（与桌面端一致）
	p.rect = rectBox{X: r.X, Y: r.Y, W: r.W, H: r.H}

	src, ok := p.Source.(*MediaSource)
	if !ok || src == nil || src.el.IsUndefined() || src.el.IsNull() {
		a.paintVideoPlaceholder(n, r, st)
		return
	}
	fit := "contain"
	switch p.Fit {
	case FitCover:
		fit = "cover"
	case FitFill:
		fit = "fill"
	}
	rot := p.RotationDeg()
	radius := cssRadius(st.Radius)
	clip := a.clipRect(r)
	placeOverlay(src.id, r, clip, func(el js.Value) {
		stx := el.Get("style")
		stx.Set("objectFit", fit)
		stx.Set("borderRadius", radius)
		if rot != 0 {
			stx.Set("transform", fmt.Sprintf("rotate(%ddeg)", rot))
		} else {
			stx.Set("transform", "none")
		}
	})
}

// paintVideoPlaceholder 无画面时的占位（暗底 + 提示文字）。
func (a *App) paintVideoPlaceholder(n *Node, r geom.Rect, st *Style) {
	a.r.Rect(gpu.Shape{
		Rect:   r,
		Radius: st.Radius,
		Fill:   paintOf(ColText, 0.06),
	})
	label := "无画面"
	if p := n.Video; p != nil {
		switch {
		case p.Err() != nil:
			label = "浏览器无法播放该源"
		case p.Source != nil:
			label = "解码中…"
		}
	}
	tmp := &Node{
		Kind:  KText,
		Text:  label,
		Style: Style{Opacity: 1, FontSize: 12, FontWeight: 500, Color: C("#6b7694"), CenterY: true, TextAlign: AlignCenter},
	}
	a.paintText(tmp, r, &tmp.Style)
}

// videoTexture 浏览器端不使用显存纹理（画面由 DOM 元素直接显示），仅保留类型占位。
type videoTexture struct{}
