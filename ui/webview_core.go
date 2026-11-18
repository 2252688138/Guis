// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 内嵌浏览器（WebView）核心层：一套 API + 各端后端。
//
// 平台差异全部收敛到 webViewBackend 接口，各端只实现一份薄后端：
//   - webview_windows.go：系统 WebView2（Edge 内核，purego 绑定）
//   - webview_wasm.go   ：<iframe> 叠加（浏览器端）
//   - webview_stub.go   ：Linux / macOS / Android / iOS 降级后端（外部浏览器 + 占位卡片）
//
// 本文件负责与平台无关的一切：统一的 WebViewOptions / WebView 类型、
// 全部公开方法、页面内查找、缩放档位、静音、深色、注入 CSS、
// JS ↔ Go 消息桥、导航白名单、历史栈、外部浏览器打开，
// 以及节点构建器、占位卡片绘制、每帧位置同步与实例登记。
//
// 能力降级约定：后端没有的能力（例如 stub 端根本没有页面）不报错，
// 而是返回 false / 只记录状态位；调用方用 WebViewAvailable() 与
// Platform() 判断当前强度，用 OpenExternal() 兜底。

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// ---- 创建参数 ----

// WebViewOptions 创建内嵌浏览器参数。
//
// 旧字段（URL / HTML / UserDataDir / Zoom / Transparent / OnChange）语义不变；
// 其余字段全部可选，零值 = 保持原有行为，后端不支持时自动忽略。
type WebViewOptions struct {
	// URL 初始地址（http/https/file 均可）。
	URL string
	// HTML 直接加载 HTML（优先于 URL，适合离线页面 / 自绘模板）。
	HTML string
	// UserDataDir 用户数据目录（cookie / 缓存），空 = 平台默认目录。
	UserDataDir string
	// Zoom 初始缩放（1 = 100%）。
	Zoom float64
	// Transparent 背景透明（网页自身透明处透出下层自绘内容）。
	Transparent bool
	// OnChange 页面状态变化（加载开始/完成、地址、标题、历史）时回调。
	OnChange func()

	// BrowserPath 系统浏览器可执行文件路径（仅 CDP 后端使用，空 = 自动探测）。
	BrowserPath string
	// UserAgent 自定义 UA（后端不支持时忽略）。
	UserAgent string
	// Muted 初始静音。
	Muted bool
	// DarkMode 初始深色模式（注入 CSS 滤镜，非原生暗色主题）。
	DarkMode bool
	// Radius 初始网页区域圆角（逻辑像素）。
	Radius float32
	// BGColor 网页底色 "#rrggbb" 或 "transparent"（空 = 后端默认）。
	BGColor string
	// HomeURL GoHome() 的目标（空 = about:blank）。
	HomeURL string
	// InjectCSS 文档创建后注入的 CSS（可重复调用 InjectCSS 追加）。
	InjectCSS string
	// AllowNavigation 导航白名单：返回 false 取消本次导航（nil = 全部放行）。
	AllowNavigation func(url string) bool
	// OnNavStart 导航开始（地址变化时触发）。
	OnNavStart func(url string)
	// OnNavDone 导航完成（加载结束时触发，ok = 无错误）。
	OnNavDone func(url string, ok bool)
	// OnTitle 页面标题变化。
	OnTitle func(title string)
	// OnNewWindow 页面请求新窗口（window.open / target=_blank）。
	// 返回 true 表示宿主已自行处理（例如在标签页里打开）；返回 false 则拦截。
	OnNewWindow func(url string) bool
	// OnMessage 页面侧调用 guisSend(data) 时回调（JS → Go）。
	OnMessage func(data string)
	// OnError 后端错误（创建失败之外的运行时错误）。
	OnError func(err error)
}

// ---- 后端契约 ----

// webViewBackend 各端内嵌浏览器后端（不导出，差异只允许出现在这里）。
//
// 所有方法都可能在 UI 线程被高频调用（每帧 Place / Hide），实现里不要做重活。
type webViewBackend interface {
	// Ready 内核是否就绪（网页可以交互）。
	Ready() bool
	// Err 后端错误（nil = 正常）。
	Err() error
	// Platform 后端标识，如 "windows-webview2" / "browser-iframe" / "stub"。
	Platform() string

	// Navigate 打开地址；LoadHTML 直接加载一段 HTML（优先）。
	Navigate(url string)
	LoadHTML(html string)

	Back()
	Forward()
	Reload()
	Stop()
	Focus()
	Close()

	// SetRadius 网页区域圆角（逻辑像素）。
	SetRadius(r float32)
	// SetZoom 缩放（1 = 100%；不支持的后端自行降级为忽略）。
	SetZoom(z float64)
	// SetMuted 静音页面媒体。
	SetMuted(m bool)
	// SetUserAgent 设置 UA（只影响后续导航）。
	SetUserAgent(ua string)

	// Exec 执行 JS（丢弃返回值）。
	Exec(js string)
	// ExecAsync 执行 JS 并取回结果（不支持的后端直接回调空串）。
	ExecAsync(js string, cb func(result string))

	URL() string
	Title() string
	Loading() bool
	CanGoBack() bool
	CanGoForward() bool

	// Place 把网页放到逻辑矩形 r（scale = 逻辑像素 → 物理像素的比例）。
	Place(r geom.Rect, scale float32)
	// Hide 隐藏网页（滚出视口 / 切页）。
	Hide()
}

// ---- 错误与常量 ----

// errNoWebView 当前平台没有可用的内嵌浏览器后端。
var errNoWebView = errors.New("ui: 当前平台未接入内嵌浏览器（可用 ui.WebViewAvailable() 判断）")

// errNoWindow 没有活动窗口。
var errNoWindow = errors.New("ui: 没有活动窗口，无法创建 WebView")

// webViewHistoryMax 历史栈上限（超出丢弃最早的记录）。
const webViewHistoryMax = 100

// webViewBridgeInject 页面侧 JS ↔ Go 消息桥（可用 WebViewBridgeJS 让页面自己初始化）。
//
// 页面侧 API：
//
//	guisSend(data)          向 Go 发送消息（字符串或可 JSON 序列化的对象）
//	guisOnMessage(fn)       注册 Go → 页面的消息回调
//	window.dispatchEvent(new CustomEvent("guis:message", {detail}))
const WebViewBridgeJS = `(function () {
  if (window.__guisBridgeReady) { return; }
  window.__guisBridgeReady = true;
  window.__guisQueue = window.__guisQueue || [];
  window.__guisRecv = function (data) {
    var ev = null;
    try { ev = new CustomEvent("guis:message", { detail: data }); } catch (e) { ev = null; }
    if (ev) { try { window.dispatchEvent(ev); } catch (e) {} }
    for (var i = 0; i < window.__guisQueue.length; i++) {
      try { window.__guisQueue[i](data); } catch (e) {}
    }
  };
  window.guisOnMessage = function (fn) {
    if (typeof fn === "function") { window.__guisQueue.push(fn); }
  };
  window.guisSend = function (data) {
    var s = (typeof data === "string") ? data : JSON.stringify(data);
    if (window.__guisHostSend) { try { window.__guisHostSend(s); return; } catch (e) {} }
    window.__guisOutbox = window.__guisOutbox || [];
    window.__guisOutbox.push(s);
  };
  window.__guisDrain = function () {
    var o = window.__guisOutbox || [];
    window.__guisOutbox = [];
    return o.join("\u0001");
  };
})();`

// webViewDarkCSS 深色模式注入的 CSS（滤镜近似，图片/视频回正）。
const webViewDarkCSS = `html{background:#14171c !important;filter:invert(.92) hue-rotate(180deg);}` +
	`img,video,canvas,svg,iframe,picture{filter:invert(1) hue-rotate(180deg);}`

// ---- WebView ----

// WebView 内嵌浏览器节点（跨端同一类型，后端按平台选择）。
//
// 桌面端（Windows / WebView2）为 HWND 托管：网页由 Chromium 的 GPU 进程渲染到
// 自己的子窗口上，位置 = 节点矩形（按 DPI 换算物理像素），因此网页像素不经过
// 框架的 OpenGL 批次，无法被自绘浮层裁剪 / 圆角 / 覆盖（圆角用窗口区域真裁剪）。
// 浏览器端为 <iframe> 叠加。其余平台走降级后端（占位卡片 + 外部浏览器打开）。
type WebView struct {
	be  webViewBackend
	app *App

	opts WebViewOptions

	// 上一帧的逻辑矩形与可见状态（调试 / 命中测试用）
	rect    geom.Rect
	visible bool
	// radius 网页区域圆角（逻辑像素）
	radius float32
	// zoom 当前缩放（1 = 100%）
	zoom  float64
	muted bool
	dark  bool
	ua    string

	// 注入 CSS（编号 → 内容，ClearInjectedCSS 时按编号移除）
	injectSeq int
	injectCSS map[int]string
	// findText 首页查找关键字
	findText string

	// 历史栈（Go 侧维护，跨端一致）
	hist  []string
	histI int

	// 事件 diff 基线
	lastURL     string
	lastTitle   string
	lastLoading bool
	lastErr     error
	msgTick     int

	closed bool
}

// NewWebView 在当前窗口创建内嵌浏览器（须在 UI 线程调用，如按钮回调里）。
func NewWebView(opts WebViewOptions) (*WebView, error) {
	return NewWebViewIn(currentApp.Load(), opts)
}

// NewWebViewIn 在指定窗口创建内嵌浏览器（多窗口场景）。
func NewWebViewIn(a *App, opts WebViewOptions) (*WebView, error) {
	if a == nil {
		return nil, errNoWindow
	}
	be, err := newWebViewBackend(a, opts)
	if err != nil {
		return nil, err
	}
	wv := &WebView{
		be:      be,
		app:     a,
		opts:    opts,
		zoom:    opts.Zoom,
		radius:  opts.Radius,
		muted:   opts.Muted,
		dark:    opts.DarkMode,
		ua:      opts.UserAgent,
		histI:   -1,
		lastURL: be.URL(),
	}
	if wv.zoom <= 0 {
		wv.zoom = 1
	}
	if wv.radius > 0 {
		be.SetRadius(wv.radius)
	}
	if wv.ua != "" {
		be.SetUserAgent(wv.ua)
	}
	if wv.muted {
		be.SetMuted(true)
	}
	if bg := strings.TrimSpace(opts.BGColor); bg != "" {
		wv.InjectCSS("html,body{background:" + bg + " !important;}")
	}
	if strings.TrimSpace(opts.InjectCSS) != "" {
		wv.InjectCSS(opts.InjectCSS)
	}
	if wv.dark {
		wv.SetDarkMode(true)
	}
	if u := normalizeURL(be.URL()); u != "" && u != "about:blank" {
		wv.pushHistory(u)
	}
	a.registerWebView(wv)
	return wv, nil
}

// WebViewAvailable 当前平台 + 运行时是否可用（false 时组件自动降级为占位卡片）。
func WebViewAvailable() bool { return webViewCapable() }

// Platform 后端标识："windows-webview2" / "browser-iframe" / "stub"。
func (wv *WebView) Platform() string {
	if wv == nil || wv.be == nil {
		return "none"
	}
	return wv.be.Platform()
}

// Ready 内核是否就绪。
func (wv *WebView) Ready() bool { return wv != nil && wv.be != nil && wv.be.Ready() }

// Err 后端错误（nil = 正常；降级后端返回 errNoWebView 之外的说明）。
func (wv *WebView) Err() error {
	if wv == nil || wv.be == nil {
		return errNoWebView
	}
	return wv.be.Err()
}

// Navigate 打开地址（受 AllowNavigation 白名单约束）。
func (wv *WebView) Navigate(url string) {
	if wv == nil || wv.be == nil || wv.closed {
		return
	}
	u := normalizeURL(url)
	if u == "" {
		return
	}
	if wv.opts.AllowNavigation != nil && !wv.opts.AllowNavigation(u) {
		return
	}
	wv.be.Navigate(u)
}

// LoadHTML 直接加载一段 HTML（离线页面 / 自绘模板）。
func (wv *WebView) LoadHTML(html string) {
	if wv == nil || wv.be == nil || wv.closed {
		return
	}
	wv.be.LoadHTML(html)
}

// Back 后退。
func (wv *WebView) Back() {
	if wv != nil && wv.be != nil && wv.be.CanGoBack() {
		wv.be.Back()
	}
}

// Forward 前进。
func (wv *WebView) Forward() {
	if wv != nil && wv.be != nil && wv.be.CanGoForward() {
		wv.be.Forward()
	}
}

// Reload 刷新。
func (wv *WebView) Reload() {
	if wv != nil && wv.be != nil {
		wv.be.Reload()
	}
}

// Stop 停止加载。
func (wv *WebView) Stop() {
	if wv != nil && wv.be != nil {
		wv.be.Stop()
	}
}

// Focus 把焦点交给网页。
func (wv *WebView) Focus() {
	if wv != nil && wv.be != nil {
		wv.be.Focus()
	}
}

// SetRadius 网页区域圆角（逻辑像素；桌面端用窗口区域真裁剪）。
func (wv *WebView) SetRadius(r float32) {
	if wv == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	wv.radius = r
	if wv.be != nil {
		wv.be.SetRadius(r)
	}
	RequestRedraw()
}

// SetZoom 缩放（1 = 100%，夹紧在 0.25 ~ 5 之间）。
func (wv *WebView) SetZoom(z float64) {
	if wv == nil {
		return
	}
	wv.zoom = clampWebViewZoom(z)
	if wv.be != nil {
		wv.be.SetZoom(wv.zoom)
	}
}

// ZoomFactor 当前缩放。
func (wv *WebView) ZoomFactor() float64 {
	if wv == nil || wv.zoom <= 0 {
		return 1
	}
	return wv.zoom
}

// ZoomIn 放大一档（×1.1）。
func (wv *WebView) ZoomIn() { wv.SetZoom(wv.ZoomFactor() * 1.1) }

// ZoomOut 缩小一档（÷1.1）。
func (wv *WebView) ZoomOut() { wv.SetZoom(wv.ZoomFactor() / 1.1) }

// ZoomReset 恢复 100%。
func (wv *WebView) ZoomReset() { wv.SetZoom(1) }

// ExecuteScript 在页面执行 JS（丢弃返回值）。
func (wv *WebView) ExecuteScript(js string) {
	if wv != nil && wv.be != nil {
		wv.be.Exec(js)
	}
}

// ExecuteScriptAsync 在页面执行 JS 并取回结果（不支持的后端回调空串）。
func (wv *WebView) ExecuteScriptAsync(js string, cb func(result string)) {
	if wv != nil && wv.be != nil {
		wv.be.ExecAsync(js, cb)
		return
	}
	if cb != nil {
		cb("")
	}
}

// URL 当前地址。
func (wv *WebView) URL() string {
	if wv == nil || wv.be == nil {
		return ""
	}
	return wv.be.URL()
}

// Title 当前页面标题。
func (wv *WebView) Title() string {
	if wv == nil || wv.be == nil {
		return ""
	}
	return wv.be.Title()
}

// Loading 是否正在加载。
func (wv *WebView) Loading() bool { return wv != nil && wv.be != nil && wv.be.Loading() }

// CanGoBack 是否可后退。
func (wv *WebView) CanGoBack() bool { return wv != nil && wv.be != nil && wv.be.CanGoBack() }

// CanGoForward 是否可前进。
func (wv *WebView) CanGoForward() bool { return wv != nil && wv.be != nil && wv.be.CanGoForward() }

// SetMuted 静音页面媒体（记录状态，后端不支持时仅影响后续导航）。
func (wv *WebView) SetMuted(m bool) {
	if wv == nil {
		return
	}
	wv.muted = m
	if wv.be != nil {
		wv.be.SetMuted(m)
	}
}

// Muted 当前是否静音。
func (wv *WebView) Muted() bool { return wv != nil && wv.muted }

// SetUserAgent 设置 UA（只影响后续导航）。
func (wv *WebView) SetUserAgent(ua string) {
	if wv == nil {
		return
	}
	wv.ua = ua
	if wv.be != nil {
		wv.be.SetUserAgent(ua)
	}
}

// SetDarkMode 深色模式（注入 CSS 滤镜近似，图片与视频回正）。
func (wv *WebView) SetDarkMode(on bool) {
	if wv == nil {
		return
	}
	wv.dark = on
	if wv.be == nil {
		return
	}
	if !on {
		wv.be.Exec(`(function(){var e=document.getElementById("guis-dark");if(e){e.remove();}})()`)
		return
	}
	wv.be.Exec(fmt.Sprintf(`(function(){var e=document.getElementById("guis-dark");`+
		`if(!e){e=document.createElement("style");e.id="guis-dark";(document.head||document.documentElement).appendChild(e);}`+
		`e.textContent=%q;})()`, webViewDarkCSS))
}

// DarkMode 当前是否深色。
func (wv *WebView) DarkMode() bool { return wv != nil && wv.dark }

// InjectCSS 追加一段注入 CSS（下次导航后自动重放，可用 ClearInjectedCSS 移除）。
func (wv *WebView) InjectCSS(css string) {
	if wv == nil || strings.TrimSpace(css) == "" {
		return
	}
	wv.injectSeq++
	if wv.injectCSS == nil {
		wv.injectCSS = map[int]string{}
	}
	wv.injectCSS[wv.injectSeq] = css
	if wv.be == nil {
		return
	}
	wv.be.Exec(injectCSSScript(wv.injectSeq, css))
}

// ClearInjectedCSS 移除所有 InjectCSS 注入的样式。
func (wv *WebView) ClearInjectedCSS() {
	if wv == nil || len(wv.injectCSS) == 0 {
		return
	}
	ids := make([]int, 0, len(wv.injectCSS))
	for id := range wv.injectCSS {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	wv.injectCSS = nil
	if wv.be == nil {
		return
	}
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		quoted = append(quoted, fmt.Sprintf("%q", fmt.Sprintf("guis-inject-%d", id)))
	}
	wv.be.Exec(`(function(){var ids=[` + strings.Join(quoted, ",") + `];` +
		`for(var i=0;i<ids.length;i++){var e=document.getElementById(ids[i]);if(e){e.remove();}}})()`)
}

// Post 向页面发送消息：调用页面里的 window.__guisRecv(json) 并派发 guis:message 事件。
func (wv *WebView) Post(data string) {
	if wv == nil || wv.be == nil {
		return
	}
	wv.be.Exec(fmt.Sprintf(`(function(){var d=%q;if(window.__guisRecv){window.__guisRecv(d);}})()`, data))
}

// PostJSON 把任意值序列化成 JSON 后发送给页面。
func (wv *WebView) PostJSON(v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	wv.Post(string(buf))
	return nil
}

// OnMessage 设置「页面 → Go」的消息回调（页面侧用 guisSend 发送）。
func (wv *WebView) OnMessage(fn func(data string)) {
	if wv != nil {
		wv.opts.OnMessage = fn
	}
}

// Find 页面内查找（返回 false = 不支持或关键字为空）。
func (wv *WebView) Find(text string) bool {
	if wv == nil || wv.be == nil {
		return false
	}
	text = strings.TrimSpace(text)
	wv.findText = text
	if text == "" {
		return false
	}
	wv.be.Exec(fmt.Sprintf(`(function(){try{return window.find(%q,false,false,true);}catch(e){return false;}})()`, text))
	return true
}

// FindNext 下一个匹配。
func (wv *WebView) FindNext() {
	if wv == nil || wv.be == nil || wv.findText == "" {
		return
	}
	wv.be.Exec(fmt.Sprintf(`(function(){try{return window.find(%q,false,false,true);}catch(e){return false;}})()`, wv.findText))
}

// FindPrev 上一个匹配。
func (wv *WebView) FindPrev() {
	if wv == nil || wv.be == nil || wv.findText == "" {
		return
	}
	wv.be.Exec(fmt.Sprintf(`(function(){try{return window.find(%q,false,true,true);}catch(e){return false;}})()`, wv.findText))
}

// ClearFind 清除查找高亮。
func (wv *WebView) ClearFind() {
	wv.findText = ""
	if wv != nil && wv.be != nil {
		wv.be.Exec(`(function(){try{window.getSelection().removeAllRanges();}catch(e){}})()`)
	}
}

// FindText 当前查找关键字。
func (wv *WebView) FindText() string {
	if wv == nil {
		return ""
	}
	return wv.findText
}

// GoHome 回到 HomeURL（未配置时回 about:blank）。
func (wv *WebView) GoHome() {
	if wv == nil {
		return
	}
	if wv.opts.HomeURL != "" {
		wv.Navigate(wv.opts.HomeURL)
		return
	}
	if wv.be != nil {
		wv.be.Navigate("about:blank")
	}
}

// History 访问过的地址（最早的在前，最多 100 条）。
func (wv *WebView) History() []string {
	if wv == nil {
		return nil
	}
	out := make([]string, len(wv.hist))
	copy(out, wv.hist)
	return out
}

// HistoryIndex 当前地址在 History 里的下标（-1 = 空）。
func (wv *WebView) HistoryIndex() int {
	if wv == nil {
		return -1
	}
	return wv.histI
}

// GoHistory 跳到历史栈的第 i 条（越界忽略）。
func (wv *WebView) GoHistory(i int) {
	if wv == nil || wv.be == nil || i < 0 || i >= len(wv.hist) {
		return
	}
	wv.histI = i
	wv.be.Navigate(wv.hist[i])
}

// SetAllowNavigation 运行时更新导航白名单（nil = 全部放行）。
func (wv *WebView) SetAllowNavigation(fn func(url string) bool) {
	if wv != nil {
		wv.opts.AllowNavigation = fn
	}
}

// OpenExternal 用系统浏览器打开当前地址（无内嵌浏览器平台的主力降级路径）。
func (wv *WebView) OpenExternal() {
	if wv == nil {
		return
	}
	openExternalURL(wv.URL())
}

// Print 打印当前页面。
func (wv *WebView) Print() {
	if wv != nil && wv.be != nil {
		wv.be.Exec(`(function(){try{window.print();}catch(e){}})()`)
	}
}

// Close 关闭内嵌浏览器并注销登记（可重复调用）。
func (wv *WebView) Close() {
	if wv == nil || wv.closed {
		return
	}
	wv.closed = true
	if wv.app != nil {
		wv.app.unregisterWebView(wv)
	}
	if wv.be != nil {
		wv.be.Close()
		wv.be = nil
	}
}

// ---- 内部：每帧事件分发 ----

// poll 每帧对比后端状态并派发事件（导航开始 / 完成、标题、消息桥）。
func (wv *WebView) poll() {
	if wv == nil || wv.be == nil || wv.closed {
		return
	}
	if tk, ok := wv.be.(webViewTicker); ok {
		tk.Tick()
	}
	url := wv.be.URL()
	title := wv.be.Title()
	loading := wv.be.Loading()

	if url != wv.lastURL {
		wv.lastURL = url
		if u := normalizeURL(url); u != "" && u != "about:blank" {
			wv.pushHistory(u)
			if wv.opts.OnNavStart != nil {
				wv.opts.OnNavStart(u)
			}
		}
		// 新文档：重放注入项与消息桥
		wv.reapplyInjected()
	}
	if title != wv.lastTitle {
		wv.lastTitle = title
		if wv.opts.OnTitle != nil {
			wv.opts.OnTitle(title)
		}
	}
	if err := wv.be.Err(); err != wv.lastErr {
		wv.lastErr = err
		if err != nil && wv.opts.OnError != nil {
			wv.opts.OnError(err)
		}
	}
	if wv.lastLoading && !loading {
		if wv.opts.OnNavDone != nil {
			wv.opts.OnNavDone(url, wv.be.Err() == nil)
		}
	}
	wv.lastLoading = loading

	if wv.opts.OnMessage != nil {
		wv.msgTick++
		if wv.msgTick >= 12 {
			wv.msgTick = 0
			wv.be.ExecAsync(`(function(){try{return window.__guisDrain?window.__guisDrain():"";}catch(e){return "";}})()`,
				func(res string) {
					if res == "" || wv.opts.OnMessage == nil {
						return
					}
					for _, part := range strings.Split(res, "\u0001") {
						if part != "" {
							wv.opts.OnMessage(part)
						}
					}
				})
		}
	}
}

// reapplyInjected 页面切换后重放深色 / 注入 CSS / 消息桥。
func (wv *WebView) reapplyInjected() {
	if wv == nil || wv.be == nil {
		return
	}
	if wv.opts.OnMessage != nil {
		wv.be.Exec(WebViewBridgeJS)
	}
	if wv.dark {
		dark := wv.dark
		wv.dark = false
		wv.SetDarkMode(dark)
	}
	if len(wv.injectCSS) > 0 {
		ids := make([]int, 0, len(wv.injectCSS))
		for id := range wv.injectCSS {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			wv.be.Exec(injectCSSScript(id, wv.injectCSS[id]))
		}
	}
}

// pushHistory 记录一条历史（重复地址与跳转后的分支会先裁掉）。
func (wv *WebView) pushHistory(url string) {
	if wv == nil || url == "" {
		return
	}
	if n := len(wv.hist); n > 0 && wv.histI >= 0 && wv.histI < n && wv.hist[wv.histI] == url {
		return
	}
	if wv.histI >= 0 && wv.histI < len(wv.hist)-1 {
		wv.hist = wv.hist[:wv.histI+1]
	}
	wv.hist = append(wv.hist, url)
	if len(wv.hist) > webViewHistoryMax {
		wv.hist = append([]string(nil), wv.hist[len(wv.hist)-webViewHistoryMax:]...)
	}
	wv.histI = len(wv.hist) - 1
}

// injectCSSScript 生成注入一段 CSS 的脚本（带编号 id，便于移除）。
func injectCSSScript(id int, css string) string {
	return fmt.Sprintf(`(function(){var e=document.getElementById(%q);`+
		`if(!e){e=document.createElement("style");e.id=%q;(document.head||document.documentElement).appendChild(e);}`+
		`e.textContent=%q;})()`, fmt.Sprintf("guis-inject-%d", id), fmt.Sprintf("guis-inject-%d", id), css)
}

// clampWebViewZoom 缩放夹紧。
func clampWebViewZoom(z float64) float64 {
	if math.IsNaN(z) || z <= 0 {
		return 1
	}
	if z < 0.25 {
		return 0.25
	}
	if z > 5 {
		return 5
	}
	return z
}

// webViewTicker 可选后端能力：每帧在 UI 线程被回调一次（把后台线程的结果搬回 UI 线程）。
type webViewTicker interface {
	Tick()
}

// webViewInput 可选后端能力：把自绘网页区域收到的鼠标 / 键盘事件转发给网页。
//
// 实现它的后端（目前是系统浏览器 CDP 后端）会让 WebViewNode 自动挂上交互回调，
// 网页里的滚动、拖选、输入框都能直接用。
type webViewInput interface {
	PointerMove(x, y float32)
	PointerDown()
	PointerDrag(x, y float32)
	PointerUp()
	Wheel(dy float32)
	Key(code uint32)
	Char(r rune)
}

// backendPref 后端偏好（环境变量 GUIS_WEBVIEW_BACKEND = webview2 / cdp / stub）。
func backendPref() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("GUIS_WEBVIEW_BACKEND")))
}

// webViewPainter 可选后端能力：在绘制阶段记录 DOM 叠加层位置（浏览器端 iframe）。
//
// 位置必须随绘制阶段记录，Swap 时才统一写入 DOM，避免与叠加层清理错帧。
type webViewPainter interface {
	Paint(r geom.Rect)
}

// ---- 实例登记与每帧位置同步 ----

// registerWebView 登记实例（窗口销毁时统一关闭）。
func (a *App) registerWebView(wv *WebView) {
	if a == nil || wv == nil {
		return
	}
	if a.webviews == nil {
		a.webviews = map[*WebView]bool{}
	}
	a.webviews[wv] = true
}

// unregisterWebView 注销实例。
func (a *App) unregisterWebView(wv *WebView) {
	if a == nil || wv == nil || a.webviews == nil {
		return
	}
	delete(a.webviews, wv)
	delete(a.wvRects, wv)
	delete(a.wvShown, wv)
}

// syncWebViews 把本帧绘制到的 WebView 矩形交给后端摆放，
// 本帧没绘制的（被滚出视口 / 切页 / 隐藏）一律隐藏；同时推进事件分发。
func (a *App) syncWebViews() {
	if a == nil {
		return
	}
	scale := a.effScale()
	for wv := range a.webviews {
		if wv == nil || wv.be == nil {
			continue
		}
		wv.poll()
		r, ok := a.wvRects[wv]
		if !ok || r.W < 1 || r.H < 1 {
			if a.wvShown[wv] {
				wv.be.Hide()
				wv.visible = false
				delete(a.wvShown, wv)
			}
			continue
		}
		wv.be.Place(r, scale)
		wv.visible = true
		if a.wvShown == nil {
			a.wvShown = map[*WebView]bool{}
		}
		a.wvShown[wv] = true
	}
}

// closeWebViews 关闭本窗口的所有浏览器实例（dispose 时调用）。
func (a *App) closeWebViews() {
	if a == nil {
		return
	}
	for wv := range a.webviews {
		if wv == nil {
			continue
		}
		wv.closed = true
		if wv.be != nil {
			wv.be.Close()
			wv.be = nil
		}
		delete(a.webviews, wv)
	}
	a.webviews = nil
	clear(a.wvRects)
	clear(a.wvShown)
}

// ---- 共用工具 / 节点构建器 / 占位绘制 ----

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	low := strings.ToLower(s)
	for _, p := range []string{"http://", "https://", "file:", "about:", "data:", "edge:"} {
		if strings.HasPrefix(low, p) {
			return s
		}
	}
	if strings.Contains(s, "://") {
		return s
	}
	return "https://" + s
}

// Browser WebView 节点（网页区域的占位节点；真实像素由浏览器子窗口绘制）。
func Browser(wv *WebView) *Node {
	return WebViewNode(wv)
}

// WebViewNode WebView 节点，用法同 ui.Video(p)：尺寸由样式决定，
// 通常 `.St(ui.St().GrowTo(1))` 或 `.St(ui.St().Wp(100).H(520))`。
func WebViewNode(wv *WebView) *Node {
	n := &Node{Kind: KWebView, WebView: wv, Style: Style{Opacity: 1}}
	// 纹理型后端（系统浏览器 CDP）没有原生子窗口，交互必须由自绘节点转发。
	if wv == nil || wv.be == nil {
		return n
	}
	in, ok := wv.be.(webViewInput)
	if !ok {
		return n
	}
	n.Key = fmt.Sprintf("webview-%p", wv)
	n.Style.Focusable = true
	n.OnHoverAt = func(x, y float32) { in.PointerMove(x, y) }
	n.OnPress = func() { in.PointerDown() }
	n.OnDrag = func(x, y float32) { in.PointerDrag(x, y) }
	n.OnDragEnd = func() { in.PointerUp() }
	n.OnScroll = func(dy float32) { in.Wheel(dy) }
	n.OnKey = func(code uint32) { in.Key(code) }
	n.OnChar = func(r rune) { in.Char(r) }
	return n
}

// ---- 自绘浏览器工具条 ----

// WebViewBarOptions 工具条参数。
type WebViewBarOptions struct {
	// Address 地址栏当前文本（由调用方持有）。
	Address string
	// Key 地址输入框的 key（同一界面内唯一）。
	Key string
	// OnAddress 地址文本变化。
	OnAddress func(string)
	// OnGo 回车或点击「打开」时触发（通常调 wv.Navigate(addr)）。
	OnGo func()
	// ShowTitle 在地址栏右侧显示页面标题。
	ShowTitle bool

	// AutoCompact 宽度不足时自动紧凑（隐藏文字按钮、只留字形与地址栏）。
	AutoCompact bool
	// ShowFind 显示「页面内查找」入口（配合 WebViewFindBar / BrowserPanel）。
	ShowFind bool
	// ShowZoom 显示缩放控件（− / 百分比 / +）。
	ShowZoom bool
}

// WebViewBar 自绘浏览器工具条：← → ↻ + 地址栏 + 打开 + 加载指示。
// 放在 WebViewNode 正上方即可组成完整浏览器界面。
func WebViewBar(wv *WebView, o WebViewBarOptions) *Node {
	if o.AutoCompact || o.ShowFind || o.ShowZoom {
		return webViewBarAuto(wv, o)
	}
	key := o.Key
	if key == "" {
		key = "wv-addr"
	}
	canBack, canFwd, loading := false, false, false
	if wv != nil {
		canBack, canFwd, loading = wv.CanGoBack(), wv.CanGoForward(), wv.Loading()
	}

	navBtn := func(glyph string, enabled bool, fn func()) *Node {
		st := St().W(32).H(32).MinWidth(32).Text(13.5, 600, ColTextDim).R(9).
			BgC(ColBgSoft).Stroke(1, ColStroke)
		if !enabled {
			st = St().W(32).H(32).MinWidth(32).Text(13.5, 600, "#3a4358").R(9).
				BgC(ColInput).Stroke(1, ColStroke)
		}
		n := Button(glyph, fn).St(st)
		if !enabled {
			n.Disabled = true
		}
		return n
	}

	left := navBtn("‹", canBack && wv != nil, func() {
		if wv != nil {
			wv.Back()
		}
	})
	right := navBtn("›", canFwd && wv != nil, func() {
		if wv != nil {
			wv.Forward()
		}
	})
	reloadGlyph := "⟳"
	if loading {
		reloadGlyph = "×"
	}
	reload := navBtn(reloadGlyph, wv != nil, func() {
		if wv == nil {
			return
		}
		if wv.Loading() {
			wv.Stop()
		} else {
			wv.Reload()
		}
	})

	addr := Box(InputSubmit(key, o.Address, "输入网址，回车打开", func(v string) {
		if o.OnAddress != nil {
			o.OnAddress(v)
		}
	}, func() {
		if o.OnGo != nil {
			o.OnGo()
		}
	}).St(St().H(34).R(9))).Grow(1)

	cells := []*Node{left, right, reload, addr}
	if loading {
		cells = append(cells, Spinner(16, ColPrimary))
	}
	if o.ShowTitle && wv != nil && wv.Title() != "" {
		cells = append(cells, Text(ellipsize(wv.Title(), 34)).St(St().Text(11.5, 500, ColTextMuted)))
	}
	cells = append(cells, Btn("打开", BtnPrimary, func() {
		if o.OnGo != nil {
			o.OnGo()
		}
	}))

	return Row(cells...).St(St().GapOf(8).Padding(8).R(12).
		BgC(ColBgSoft).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// WebViewStatus 页面状态行：地址 / 标题 / 加载中提示。
func WebViewStatus(wv *WebView) *Node {
	if wv == nil {
		return nil
	}
	label := wv.Title()
	if label == "" {
		label = wv.URL()
	}
	if label == "" {
		label = "空白页"
	}
	dot := ColSuccess
	switch {
	case wv.Err() != nil:
		dot = ColDanger
		label = "WebView2 不可用：" + wv.Err().Error()
	case wv.Loading():
		dot = ColWarn
	case !wv.Ready():
		dot = ColWarn
		label = "正在启动浏览器内核…"
	}
	return Row(
		Dot(dot, 7),
		Text(ellipsize(label, 76)).St(St().Text(11.5, 500, ColTextMuted)),
		Spacer(),
		Text(ellipsize(wv.URL(), 52)).St(St().Text(11, 400, ColPlaceholder)),
	).St(St().GapOf(8).PadXY(12, 6).R(9).BgC(ColBgDeep).Stroke(1, ColStroke)).
		AlignTo(AlignCenter)
}

func ellipsize(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// ---- 绘制层 ----

// paintWebView 绘制网页节点：记录本帧矩形（供 frame 同步子窗口），
// 并在内核未就绪 / 失败时画占位内容（网页窗口盖上去之后就看不见了）。
func (a *App) paintWebView(n *Node, r geom.Rect, st *Style) {
	wv := n.WebView
	if wv == nil || wv.be == nil {
		a.paintWebViewCard(r, st, "WebView 未创建", ColTextMuted, false)
		return
	}
	// 裁剪到祖先裁剪区（滚动视口 / Clipped 容器）：HWND 本身无法裁剪，
	// 这里按交集缩小窗口，粗略还原「被裁掉」的效果。
	vr := r
	if a.clipN > 0 {
		vr = intersectRect(vr, a.clipStack[a.clipN-1])
	}
	if vr.W < 1 || vr.H < 1 {
		return
	}
	if a.wvRects == nil {
		a.wvRects = map[*WebView]geom.Rect{}
	}
	a.wvRects[wv] = vr
	wv.rect = vr
	// 浏览器端：iframe 位置在绘制阶段记录（Swap 时统一写 DOM）
	if pb, ok := wv.be.(webViewPainter); ok {
		pb.Paint(vr)
	}

	if err := wv.be.Err(); err != nil {
		a.paintWebViewCard(r, st, "浏览器内核不可用\n"+err.Error(), ColDanger, false)
		return
	}
	if !wv.be.Ready() {
		// 内核启动中：暗底 + 呼吸提示，逐帧重绘推进动画
		a.paintWebViewCard(r, st, "正在启动浏览器内核…", ColTextMuted, true)
		a.animating = true
	}
}

// paintWebViewCard 网页区域占位卡片（暗底 + 圆角 + 居中提示）。
func (a *App) paintWebViewCard(r geom.Rect, st *Style, label string, color string, pulse bool) {
	alpha := float32(0.04)
	if pulse {
		alpha = 0.03 + 0.02*(1+float32(math.Sin(float64(AnimTime())*2.4)))/2
	}
	a.r.Rect(gpu.Shape{
		Rect:   r,
		Radius: st.Radius,
		Fill:   paintOf(ColText, alpha),
	})
	if label == "" {
		return
	}
	tmp := &Node{
		Kind: KText,
		Text: label,
		Style: Style{
			Opacity: 1, FontSize: 12, FontWeight: 500, Color: C(color),
			CenterY: true, TextAlign: AlignCenter,
		},
	}
	a.paintText(tmp, r, &tmp.Style)
}

// intersectRect 求两个矩形交集（无交集时宽高为 0）。
func intersectRect(a, b geom.Rect) geom.Rect {
	x0 := maxF(a.X, b.X)
	y0 := maxF(a.Y, b.Y)
	x1 := minF(a.Right(), b.Right())
	y1 := minF(a.Bottom(), b.Bottom())
	if x1 <= x0 || y1 <= y0 {
		return geom.Rect{}
	}
	return geom.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// ---- 窗口同步 ----

// registerWebView 登记实例（窗口销毁时统一关闭）。

// ---- 共用脚本片段 ----

// mediaMuteScript 生成静音 / 恢复媒体元素的脚本。
func mediaMuteScript(m bool) string {
	flag := "false"
	if m {
		flag = "true"
	}
	return `(function(){var m=` + flag + `;` +
		`var els=document.querySelectorAll("video,audio");` +
		`for(var i=0;i<els.length;i++){els[i].muted=m;}` +
		`if(!window.__guisMuteHook){window.__guisMuteHook=true;` +
		`document.addEventListener("play",function(e){var t=e.target;` +
		`if(t&&(t.tagName==="VIDEO"||t.tagName==="AUDIO")){t.muted=!!window.__guisMuted;}},true);}` +
		`window.__guisMuted=m;})()`
}

// unquoteJSON 去掉 WebView2 返回值的 JSON 外层引号（"abc" → abc，其余原样）。
func unquoteJSON(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var out strings.Builder
		for i := 1; i < len(s)-1; i++ {
			if s[i] == '\\' && i+1 < len(s)-1 {
				i++
				switch s[i] {
				case 'n':
					out.WriteByte('\n')
				case 'r':
					out.WriteByte('\r')
				case 't':
					out.WriteByte('\t')
				case 'u':
					if i+4 < len(s) {
						var code rune
						ok := true
						for k := 1; k <= 4; k++ {
							c := s[i+k]
							var v rune
							switch {
							case c >= '0' && c <= '9':
								v = rune(c - '0')
							case c >= 'a' && c <= 'f':
								v = rune(c-'a') + 10
							case c >= 'A' && c <= 'F':
								v = rune(c-'A') + 10
							default:
								ok = false
							}
							code = code*16 + v
						}
						if ok {
							out.WriteRune(code)
							i += 4
							continue
						}
					}
					out.WriteByte(s[i])
				default:
					out.WriteByte(s[i])
				}
				continue
			}
			out.WriteByte(s[i])
		}
		return out.String()
	}
	return s
}
