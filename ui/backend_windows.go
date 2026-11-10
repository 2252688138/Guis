//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"

	"guis/internal/display"
	"guis/internal/win32"
)

// platformWin 桌面端（Windows）窗口后端。
//
// 它把 display.Window（原生窗口 + GL 上下文）适配成 ui.Backend，并额外承担
// Windows 独有的 DirectComposition 合成（网页与自绘内容同层）。
// 其余平台（backend_wasm.go / backend_x11.go / backend_cocoa.go / backend_android.go）
// 提供完全相同的方法集，因此 app.go 一行都不用改。
type platformWin struct {
	display.Window
	comp    *win32.Compositor
	handler func(Event)
}

// newBackend 创建 Win32 窗口（含图标与 WGL 上下文），返回平台窗口。
func newBackend(opts Options) (*platformWin, error) {
	w, err := display.New(display.Config{
		Title:        opts.Title,
		Width:        opts.Width,
		Height:       opts.Height,
		Resizable:    true,
		Decorated:    !opts.NoTitleBar,
		Transparent:  false,
		CornerRadius: opts.CornerRadius,
	})
	if err != nil {
		return nil, err
	}
	ico := opts.Icon
	if len(ico) == 0 {
		ico = defaultIcon
	}
	_ = w.SetIcon(ico)

	p := &platformWin{Window: w}
	w.SetEventHandler(p.onDisplayEvent)
	return p, nil
}

// pollMessages 处理一轮窗口消息；返回 false 表示所有窗口都已关闭。
func pollMessages() bool { return display.Poll() }

// SetEventHandler 注册事件回调（App.handleEvent）。
func (p *platformWin) SetEventHandler(fn func(Event)) { p.handler = fn }

// Visible 窗口当前是否可见（关闭时收进托盘后为 false，渲染循环据此跳过绘制）。
func (p *platformWin) Visible() bool {
	if v, ok := p.Window.(interface{ Visible() bool }); ok {
		return v.Visible()
	}
	return true
}

// BlockClose 放弃本次 WM_CLOSE：窗口只隐藏、不销毁（关闭时收进托盘）。
func (p *platformWin) BlockClose() {
	if b, ok := p.Window.(interface{ BlockClose() }); ok {
		b.BlockClose()
	}
}

// onDisplayEvent 把平台无关事件转成 ui.Event 后交给 App。
func (p *platformWin) onDisplayEvent(ev display.Event) {
	if p.handler == nil {
		return
	}
	p.handler(Event{
		Type:     EventType(ev.Type),
		X:        ev.X,
		Y:        ev.Y,
		DX:       ev.DX,
		DY:       ev.DY,
		Button:   ev.Button,
		Pointer:  ev.Pointer,
		Key:      ev.Key,
		Char:     ev.Char,
		Runes:    ev.Runes,
		Width:    ev.Width,
		Height:   ev.Height,
		Scale:    ev.Scale,
		Text:     ev.Text,
		Caret:    ev.Caret,
		Paths:    ev.Paths,
		HotkeyID: ev.HotkeyID,
	})
}

// AttachCompose 尝试启用 DirectComposition 合成（失败则回退子窗口叠加）。
func (p *platformWin) AttachCompose(pw, ph int32) {
	if !win32.ComposeSupported() {
		return
	}
	comp, err := win32.NewCompositor(p.NativeHandle(), pw, ph)
	if err != nil {
		win32.Debugf("DComp 合成不可用，回退 A 阶段: %v", err)
		return
	}
	p.comp = comp
}

// ComposeBegin 开始一帧合成，返回离屏目标 FBO（0 = 不用合成）。
func (p *platformWin) ComposeBegin() uint32 {
	if p.comp == nil {
		return 0
	}
	f, _, _, err := p.comp.BeginFrame()
	if err != nil {
		return 0
	}
	return f
}

// HasCompose 是否启用了 DirectComposition 合成。
func (p *platformWin) HasCompose() bool { return p.comp != nil }

// Compositor 返回 DirectComposition 合成器（未启用时为 nil）。
// 供 webview.go 把网页做成合成图层（B 阶段）。
func (p *platformWin) Compositor() any {
	if p.comp == nil {
		return nil
	}
	return p.comp
}

// ResizeCompose 尺寸变化时重建合成表面（无合成器时为空操作）。
func (p *platformWin) ResizeCompose(pw, ph int32) {
	if p.comp != nil {
		_ = p.comp.Resize(pw, ph)
	}
}

// ComposeEnd 提交本帧合成。
func (p *platformWin) ComposeEnd() {
	if p.comp != nil {
		p.comp.EndFrame()
	}
}

// Destroy 释放窗口（先关合成器）。
func (p *platformWin) Destroy() {
	if p.comp != nil {
		p.comp.Close()
		p.comp = nil
	}
	if p.Window != nil {
		p.Window.Destroy()
	}
}

// Bounds 读取窗口在屏幕上的位置与尺寸（物理像素），供热重载重启时保留现场。
// 非 Windows 后端没有实现该可选接口，热重载时只丢失窗口几何恢复能力。
func (p *platformWin) Bounds() (x, y, w, h int, maximized bool) {
	if p.Window == nil || p.NativeHandle() == 0 || win32.GetWindowRect == nil {
		return 0, 0, 0, 0, false
	}
	var r win32.RECT
	if win32.GetWindowRect(p.NativeHandle(), &r) == 0 {
		return 0, 0, 0, 0, false
	}
	return int(r.Left), int(r.Top), int(r.Right - r.Left), int(r.Bottom - r.Top), p.IsMaximized()
}

// SetBounds 把窗口移到指定位置与尺寸（物理像素）。
// 窗口首帧之前调用不会把它提前显示出来（SetWindowPos 不改可见性）。
func (p *platformWin) SetBounds(x, y, w, h int) {
	if p.Window == nil || p.NativeHandle() == 0 || win32.SetWindowPos == nil || w <= 0 || h <= 0 {
		return
	}
	win32.SetWindowPos(p.NativeHandle(), 0, int32(x), int32(y), int32(w), int32(h),
		win32.SWP_NOZORDER|win32.SWP_NOACTIVATE)
}

// ---- 窗口特效（ui.windowFXBackend）----
//
// p.Window 的类型是 display.Window 接口，其中不含特效方法；display 的
// winWindow 上额外实现了下面这组方法。这里用可选接口断言取得，断言失败
// （换平台）就是空操作。

// winFXer display 层窗口的特效方法集（只有 Windows 后端实现）。
type winFXer interface {
	SetWindowOpacity(f float32)
	WindowOpacity() float32
	SetClickThrough(on bool)
	ClickThrough() bool
	SetKeepOnBottom(on bool)
	SetSkipTaskbar(on bool)
	SkipTaskbar() bool
	SetCaptureExcluded(on bool)
	SetDarkTitleBar(on bool)
	SetBackdrop(kind int)
	SetMinSize(w, h int)
	SetMaxSize(w, h int)
	Activate()
	FlashTaskbar(count int, untilFocused bool)
}

// fx 取特效接口（不支持时 nil）。
func (p *platformWin) fx() winFXer {
	if p == nil || p.Window == nil {
		return nil
	}
	fx, _ := p.Window.(winFXer)
	return fx
}

// SetWindowOpacity 设置整窗不透明度。
func (p *platformWin) SetWindowOpacity(f float32) {
	if x := p.fx(); x != nil {
		x.SetWindowOpacity(f)
	}
}

// WindowOpacity 当前不透明度。
func (p *platformWin) WindowOpacity() float32 {
	if x := p.fx(); x != nil {
		return x.WindowOpacity()
	}
	return 1
}

// SetClickThrough 鼠标穿透。
func (p *platformWin) SetClickThrough(on bool) {
	if x := p.fx(); x != nil {
		x.SetClickThrough(on)
	}
}

// ClickThrough 当前是否鼠标穿透。
func (p *platformWin) ClickThrough() bool {
	if x := p.fx(); x != nil {
		return x.ClickThrough()
	}
	return false
}

// SetKeepOnBottom 窗口置底。
func (p *platformWin) SetKeepOnBottom(on bool) {
	if x := p.fx(); x != nil {
		x.SetKeepOnBottom(on)
	}
}

// SetSkipTaskbar 隐藏 / 显示任务栏按钮。
func (p *platformWin) SetSkipTaskbar(on bool) {
	if x := p.fx(); x != nil {
		x.SetSkipTaskbar(on)
	}
}

// SkipTaskbar 当前是否隐藏任务栏按钮。
func (p *platformWin) SkipTaskbar() bool {
	if x := p.fx(); x != nil {
		return x.SkipTaskbar()
	}
	return false
}

// SetCaptureExcluded 禁止截屏 / 录屏捕获。
func (p *platformWin) SetCaptureExcluded(on bool) {
	if x := p.fx(); x != nil {
		x.SetCaptureExcluded(on)
	}
}

// SetDarkTitleBar 系统标题栏深色。
func (p *platformWin) SetDarkTitleBar(on bool) {
	if x := p.fx(); x != nil {
		x.SetDarkTitleBar(on)
	}
}

// SetBackdrop 设置 Win11 背景材质。
func (p *platformWin) SetBackdrop(kind BackdropKind) {
	if x := p.fx(); x != nil {
		x.SetBackdrop(int(kind))
	}
}

// SetMinSize 最小窗口尺寸（逻辑像素）。
func (p *platformWin) SetMinSize(w, h int) {
	if x := p.fx(); x != nil {
		x.SetMinSize(w, h)
	}
}

// SetMaxSize 最大窗口尺寸（逻辑像素）。
func (p *platformWin) SetMaxSize(w, h int) {
	if x := p.fx(); x != nil {
		x.SetMaxSize(w, h)
	}
}

// Activate 拉到前台并抢焦点。
func (p *platformWin) Activate() {
	if x := p.fx(); x != nil {
		x.Activate()
	}
}

// ---- 任务栏（ui.taskbarBackend）----

// SetTaskbarProgress 任务栏进度条（total<=0 为不确定进度）。
func (p *platformWin) SetTaskbarProgress(done, total int64) {
	win32.SetTaskbarProgress(p.NativeHandle(), done, total)
}

// ClearTaskbarProgress 清除任务栏进度条。
func (p *platformWin) ClearTaskbarProgress() {
	win32.ClearTaskbarProgress(p.NativeHandle())
}

// SetTaskbarBadge 任务栏未读角标（0 = 清除）。
func (p *platformWin) SetTaskbarBadge(n int) {
	desc := fmt.Sprintf("%d 条未读", n)
	if n <= 0 {
		desc = ""
	}
	win32.SetTaskbarBadge(p.NativeHandle(), n, desc)
}

// FlashTaskbar 闪烁任务栏图标。
func (p *platformWin) FlashTaskbar(count int, untilFocused bool) {
	if x := p.fx(); x != nil {
		x.FlashTaskbar(count, untilFocused)
	}
}
