//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Windows 内嵌浏览器后端：系统 WebView2（Edge 内核，purego 绑定）。
//
// 渲染方式（A 阶段 · HWND 托管）：网页由 Chromium 的 GPU 进程渲染到自己的
// 子窗口上，子窗口位置 = 节点矩形（按 DPI 换算成物理像素），因此：
//   - 帧率与交互是浏览器原生的，滚动、输入法、视频、canvas 全部真实可用；
//   - 但网页像素不经过 GUIS 的 OpenGL 批次，**无法**被自绘内容裁剪 / 圆角 / 覆盖。
//
// 圆角用 SetWindowRgn 做真裁剪（见 SetRadius）；想让它像普通控件一样被裁剪、
// 被菜单盖住，需要升级到 DirectComposition 合成（B 阶段，见 COMPONENTS.md）。

import (
	"math"
	"strings"

	"guis/internal/geom"
	"guis/internal/win32"
)

// webViewWindows WebView2 后端实现。
type webViewWindows struct {
	core *win32.WebView2
	app  *App

	rad      float32 // 期望圆角（逻辑像素）
	applied  int32   // 已应用的圆角半径（物理像素）
	ua       string
	muted    bool
	lastPhys [4]int32
	placed   bool
}

// newWebView2Backend 创建 Windows WebView2 后端。
func newWebView2Backend(a *App, opts WebViewOptions) (webViewBackend, error) {
	core, err := win32.NewWebView2(win32.WebView2Options{
		Parent:      a.HWND(),
		URL:         opts.URL,
		HTML:        opts.HTML,
		UserDataDir: opts.UserDataDir,
		Zoom:        opts.Zoom,
		Transparent: opts.Transparent,
		OnChange: func() {
			a.Invalidate()
			if opts.OnChange != nil {
				opts.OnChange()
			}
		},
	})
	if err != nil {
		a.Invalidate()
		return nil, err
	}
	return &webViewWindows{
		core:  core,
		app:   a,
		rad:   opts.Radius,
		ua:    opts.UserAgent,
		muted: opts.Muted,
	}, nil
}


func (b *webViewWindows) Ready() bool { return b != nil && b.core != nil && b.core.Ready() }
func (b *webViewWindows) Err() error {
	if b == nil || b.core == nil {
		return errNoWebView
	}
	return b.core.Err()
}
func (b *webViewWindows) Platform() string { return "windows-webview2" }

func (b *webViewWindows) Navigate(url string) {
	if b != nil && b.core != nil {
		b.core.Navigate(url)
	}
}

func (b *webViewWindows) LoadHTML(html string) {
	if b != nil && b.core != nil {
		b.core.LoadHTML(html)
	}
}

func (b *webViewWindows) Back() {
	if b != nil && b.core != nil {
		b.core.Back()
	}
}

func (b *webViewWindows) Forward() {
	if b != nil && b.core != nil {
		b.core.Forward()
	}
}

func (b *webViewWindows) Reload() {
	if b != nil && b.core != nil {
		b.core.Reload()
	}
}

func (b *webViewWindows) Stop() {
	if b != nil && b.core != nil {
		b.core.StopLoad()
	}
}

func (b *webViewWindows) Focus() {
	if b != nil && b.core != nil {
		b.core.Focus()
	}
}

func (b *webViewWindows) Close() {
	if b != nil && b.core != nil {
		b.core.Close()
	}
}

// SetRadius 圆角：SetWindowRgn 真裁剪，半径按物理像素换算（0 = 恢复矩形）。
func (b *webViewWindows) SetRadius(r float32) {
	if b == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	b.rad = r
	b.applyRadius()
}

// applyRadius 把期望圆角应用到窗口区域。
func (b *webViewWindows) applyRadius() {
	if b == nil || b.core == nil {
		return
	}
	scale := float32(1)
	if b.app != nil {
		scale = b.app.effScale()
	}
	want := int32(math.Round(float64(b.rad * scale)))
	if want == b.applied {
		return
	}
	b.applied = want
	b.core.SetRoundedRegion(want)
}

func (b *webViewWindows) SetZoom(z float64) {
	if b != nil && b.core != nil {
		b.core.SetZoom(z)
	}
}

// SetMuted 静音页面内所有媒体元素（含之后新插入的）。
func (b *webViewWindows) SetMuted(m bool) {
	if b == nil || b.core == nil {
		return
	}
	b.muted = m
	b.core.ExecuteScript(mediaMuteScript(m))
}

// SetUserAgent UA 目前只记录（WebView2 的 settings.put_UserAgent 走 double/字符串
// 槽位绑定尚未接入），下次导航生效的能力留待 B 阶段补充。
func (b *webViewWindows) SetUserAgent(ua string) {
	if b != nil {
		b.ua = strings.TrimSpace(ua)
	}
}

func (b *webViewWindows) Exec(js string) {
	if b != nil && b.core != nil {
		b.core.ExecuteScript(js)
	}
}

func (b *webViewWindows) ExecAsync(js string, cb func(result string)) {
	if b == nil || b.core == nil {
		if cb != nil {
			cb("")
		}
		return
	}
	b.core.ExecuteScriptAsync(js, func(result string) {
		if b.app != nil {
			b.app.Invalidate()
		}
		if cb != nil {
			cb(unquoteJSON(result))
		}
	})
}

func (b *webViewWindows) URL() string {
	if b == nil || b.core == nil {
		return ""
	}
	return b.core.URL()
}

func (b *webViewWindows) Title() string {
	if b == nil || b.core == nil {
		return ""
	}
	return b.core.Title()
}

func (b *webViewWindows) Loading() bool   { return b != nil && b.core != nil && b.core.Loading() }
func (b *webViewWindows) CanGoBack() bool { return b != nil && b.core != nil && b.core.CanGoBack() }
func (b *webViewWindows) CanGoForward() bool {
	return b != nil && b.core != nil && b.core.CanGoForward()
}

// Place 把网页子窗口放到逻辑矩形 r（换算成物理像素），并推进内核刷新。
func (b *webViewWindows) Place(r geom.Rect, scale float32) {
	if b == nil || b.core == nil {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	b.applyRadius()
	b.core.Refresh()
	x := int32(math.Round(float64(r.X * scale)))
	y := int32(math.Round(float64(r.Y * scale)))
	w := int32(math.Round(float64(r.W * scale)))
	h := int32(math.Round(float64(r.H * scale)))
	if b.placed && x == b.lastPhys[0] && y == b.lastPhys[1] && w == b.lastPhys[2] && h == b.lastPhys[3] {
		return
	}
	b.lastPhys = [4]int32{x, y, w, h}
	b.placed = true
	b.core.SetBounds(x, y, w, h)
	b.core.SetVisible(true)
}

// Hide 隐藏网页子窗口。
func (b *webViewWindows) Hide() {
	if b != nil && b.core != nil {
		b.core.SetVisible(false)
	}
}
