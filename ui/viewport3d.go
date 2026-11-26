// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// ViewportSource 3D 视口像素来源。
//
// RenderViewport 在 UI 线程上每帧调用一次：w/h 是控件逻辑像素尺寸（用于保持宽高比），
// dt 是距上一帧的秒数（≤ 0 时按 60fps 计）。返回 RGBA 直通 alpha 像素（长度 tw*th*4）。
//
// game.Engine 已实现该接口（无需任何适配）。
type ViewportSource interface {
	RenderViewport(w, h int, dt float32) (pix []byte, tw, th int)
}

// NativeViewportSource 可选接口：支持零回读呈现的来源。
//
// 实现它之后，视口不再走「CPU 渲染 → pixel 上传 → 采样绘制」的老链路，而是
// 直接把 3D 引擎 FBO 的颜色纹理句柄交给 GUI 合成（同一 GL 上下文，零拷贝）。
// 返回 ok=false 时控件自动回退到 ViewportSource 像素路径。
//
// game.Engine 已实现该接口（配合 Engine.AttachGPU）。
type NativeViewportSource interface {
	RenderViewportNative(w, h int, dt float32) (texID uint32, tw, th int, ok bool)
}

// Viewport3DOpts 3D 视口参数。
type Viewport3DOpts struct {
	// Src 像素来源（必填，通常是 *game.Engine）。
	Src ViewportSource
	// Key 控件 Key（多视口时用于区分状态与纹理缓存，默认 "viewport3d"）。
	Key string
	// Resolution 控件未显式指定尺寸时的默认尺寸（默认 640×360）。
	Resolution [2]int
	// AutoFocus 按下时自动取得键盘焦点（默认 true）。
	AutoFocus bool
	// OnKey 键盘事件（key 为小写键名，如 "w" / "arrowup" / "shift"）。
	OnKey func(key string, down bool)
	// OnMouse 鼠标事件（button：0 左 / 1 右 / 2 中，-1 仅移动；坐标相对控件左上角）。
	OnMouse func(x, y float32, button int, down bool)
	// OnWheel 滚轮事件（dy > 0 向下）。
	OnWheel func(dy float32)
	// OnTouch 触摸事件（phase：0 按下 / 1 移动 / 2 抬起；坐标相对控件左上角）。
	OnTouch func(x, y float32, phase int, id int)
	// MouseLock 指针锁定：开启后视口接管鼠标相对位移（Event.DX/DY），
	// 适合 FPS 转视角；光标不再用于命中判定，建议配合全屏使用。
	MouseLock bool
}

// Viewport3DState 3D 视口控件状态（由 Viewport3D 创建，内部使用）。
type Viewport3DState struct {
	opts    *Viewport3DOpts
	tex     *gpu.Texture
	tw, th  int
	last    time.Time
	key     string
	w, h    float32
	down    bool
	mouseX  float32
	mouseY  float32
	pressed bool
}

// MouseLocked 该视口是否处于指针锁定模式。
func (v *Viewport3DState) MouseLocked() bool {
	return v != nil && v.opts != nil && v.opts.MouseLock
}

// MouseDelta 把鼠标相对位移交给游戏（锁定模式下由 App 派发）。
func (v *Viewport3DState) MouseDelta(dx, dy float32) {
	if v == nil || v.opts == nil {
		return
	}
	if v.opts.OnMouse != nil {
		// 锁定时光标坐标固定在控件中心，避免绝对位置乱跳影响命中
		v.opts.OnMouse(v.w/2, v.h/2, -1, false)
	}
	if m, ok := v.opts.Src.(interface{ HandleMouseMove(dx, dy float32) }); ok {
		m.HandleMouseMove(dx, dy)
	}
}

// lockedViewport 返回第一个开启指针锁定的视口（无则 nil）。
func lockedViewport(n *Node) *Viewport3DState {
	if n == nil || n.Disabled {
		return nil
	}
	if n.Viewport != nil && n.Viewport.MouseLocked() {
		return n.Viewport
	}
	for _, c := range n.Children {
		if v := lockedViewport(c); v != nil {
			return v
		}
	}
	return nil
}

// vpRegistry 按 Key 复用视口状态（纹理与帧间隔跨帧保持；声明式视图每帧重建节点）。
var (
	vpMu     sync.Mutex
	vpStates = map[string]*Viewport3DState{}
)

// viewportState 取得（或创建）指定 Key 的视口状态，并刷新本帧参数。
func viewportState(key string, o *Viewport3DOpts) *Viewport3DState {
	vpMu.Lock()
	defer vpMu.Unlock()
	if st, ok := vpStates[key]; ok {
		st.opts = o
		return st
	}
	st := &Viewport3DState{opts: o, key: key}
	vpStates[key] = st
	return st
}

// ResetViewport3D 丢弃指定 Key 的视口状态与纹理（窗口重开或切换引擎时调用）。
func ResetViewport3D(key string) {
	if key == "" {
		key = "viewport3d"
	}
	vpMu.Lock()
	defer vpMu.Unlock()
	delete(vpStates, key)
}

// Viewport3D 创建 3D 游戏视口控件。
//
// 控件状态（纹理、帧间隔）按 Key 跨帧复用：单视口用默认 Key，多视口请显式指定 Key；
// 切换引擎或重开窗口时调用 ResetViewport3D 释放纹理。
//
// 典型用法：
//
//	eng := game.NewEngine(640, 360)
//	ui.Viewport3D(&ui.Viewport3DOpts{Src: eng, AutoFocus: true,
//		OnKey: eng.HandleKey, OnWheel: eng.HandleWheel,
//		OnMouse: eng.HandleMouse,
//	})
func Viewport3D(o *Viewport3DOpts) *Node {
	if o == nil {
		o = &Viewport3DOpts{}
	}
	key := o.Key
	if key == "" {
		key = "viewport3d"
	}
	vp := viewportState(key, o)
	n := &Node{Kind: KViewport3D, Key: key, Viewport: vp, Style: Style{Opacity: 1}}
	n.Style.Focusable = true
	if o.Resolution[0] > 0 {
		n.Style.Width = float32(o.Resolution[0])
	}
	if o.Resolution[1] > 0 {
		n.Style.Height = float32(o.Resolution[1])
	}

	n.OnHoverAt = func(x, y float32) {
		vp.mouseX, vp.mouseY = x, y
		vp.emit(x, y, -1, false)
	}
	n.OnPress = func() {
		vp.down = true
		if o.AutoFocus {
			SetFocus(vp.key)
		}
		vp.emit(vp.mouseX, vp.mouseY, 0, true)
	}
	n.OnDrag = func(x, y float32) {
		vp.mouseX, vp.mouseY = x, y
		vp.emit(x, y, 0, vp.down)
	}
	n.OnDragEnd = func() {
		if vp.down {
			vp.down = false
			vp.emit(vp.mouseX, vp.mouseY, 0, false)
		}
	}
	n.OnRightClickAt = func(x, y float32) {
		vp.emit(x, y, 1, true)
		vp.emit(x, y, 1, false)
	}
	n.OnScroll = func(dy float32) {
		if o.OnWheel != nil {
			o.OnWheel(dy)
		}
	}
	n.OnKey = func(vk uint32) {
		if o.OnKey != nil {
			o.OnKey(GameKeyName(vk), true)
		}
	}
	n.OnKeyUp = func(vk uint32) {
		if o.OnKey != nil {
			o.OnKey(GameKeyName(vk), false)
		}
	}
	return n
}

// emit 派发鼠标事件（供回调使用）。
func (v *Viewport3DState) emit(x, y float32, button int, down bool) {
	if v == nil || v.opts == nil || v.opts.OnMouse == nil {
		return
	}
	v.opts.OnMouse(x, y, button, down)
}

// touch 处理一次触摸（坐标相对控件左上角），并同步模拟鼠标回调以保持兼容。
func (v *Viewport3DState) touch(x, y float32, phase int, id int) {
	if v == nil || v.opts == nil {
		return
	}
	v.mouseX, v.mouseY = x, y
	switch phase {
	case 0:
		v.down = true
		if v.opts.AutoFocus {
			SetFocus(v.key)
		}
		v.emit(x, y, 0, true)
	case 1:
		v.emit(x, y, 0, v.down)
	case 2:
		if v.down {
			v.down = false
			v.emit(x, y, 0, false)
		}
	}
	if v.opts.OnTouch != nil {
		v.opts.OnTouch(x, y, phase, id)
	}
}

// dispatchTouch 把触摸事件派发给命中的视口（App 收到 EvTouch* 时调用），返回是否已消费。
func (a *App) dispatchTouch(x, y float32, phase int, id int) bool {
	if a == nil || a.root == nil {
		return false
	}
	n := hitTest(a.root, x, y, 0, a)
	for c := n; c != nil; c = c.Parent {
		if c.Viewport != nil {
			c.Viewport.touch(x-c.Frame.X, y-c.Frame.Y, phase, id)
			return true
		}
		// 自定义控件（虚拟摇杆 / 触屏按钮）：命中即消费，不再下滑给视口
		if c.OnTouch != nil && !c.Disabled {
			c.OnTouch(x-c.Frame.X, y-c.Frame.Y, phase, id)
			return true
		}
	}
	return false
}

// paintViewport3D 绘制 3D 视口：取一帧像素 → 上传纹理 → 绘制。
func (a *App) paintViewport3D(n *Node, r geom.Rect, st *Style) {
	vp := n.Viewport
	dbg := os.Getenv("GUIS_VP_LOG") != ""
	logf := func(format string, args ...any) {
		if !dbg {
			return
		}
		f, err := os.OpenFile(os.Getenv("GUIS_VP_LOG"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		fmt.Fprintf(f, format+"\n", args...)
	}
	if vp == nil || vp.opts == nil || vp.opts.Src == nil || a.r == nil {
		logf("[vp] 跳过绘制: vp=%v opts=%v src=%v r=%v", vp != nil, vp != nil && vp.opts != nil, vp != nil && vp.opts != nil && vp.opts.Src != nil, a.r)
		return
	}
	if r.W < 4 || r.H < 4 {
		logf("[vp] 跳过绘制: rect=%vx%v", r.W, r.H)
		return
	}
	vp.w, vp.h = r.W, r.H

	now := time.Now()
	dt := float32(1.0 / 60.0)
	if !vp.last.IsZero() {
		dt = float32(now.Sub(vp.last).Seconds())
		if dt <= 0 {
			dt = 1.0 / 60.0
		}
		if dt > 0.25 {
			dt = 0.25
		}
	}
	vp.last = now

	// 硬件零回读路径：来源直接把 FBO 颜色纹理交过来，不再上传像素。
	if ns, ok := vp.opts.Src.(NativeViewportSource); ok {
		if texID, tw, th, ok2 := ns.RenderViewportNative(int(r.W), int(r.H), dt); ok2 && texID != 0 {
			if vp.tex == nil || !vp.tex.External || vp.tex.ID != texID || vp.tw != tw || vp.th != th {
				if vp.tex != nil {
					a.r.DeleteTexture(vp.tex)
				}
				vp.tex = a.r.WrapTexture(texID, tw, th)
				vp.tw, vp.th = tw, th
			}
			if vp.tex != nil {
				a.r.DrawImage(gpu.Image{
					Rect:   r,
					Tex:    vp.tex,
					Tint:   gpu.Paint{R: 1, G: 1, B: 1, A: 1},
					Radius: st.Radius,
					FlipY:  true,
				})
				logf("[vp] native tex=%d %dx%d", texID, tw, th)
				markAnim()
				return
			}
		}
	}

	pix, tw, th := vp.opts.Src.RenderViewport(int(r.W), int(r.H), dt)
	if tw <= 0 || th <= 0 || len(pix) < tw*th*4 {
		logf("[vp] 源返回无效: tw=%d th=%d len=%d need=%d", tw, th, len(pix), tw*th*4)
		return
	}
	logf("[vp] rect=(%.0f,%.0f %.0fx%.0f) tex=%dx%d len=%d newTex=%v", r.X, r.Y, r.W, r.H, tw, th, len(pix), vp.tex == nil)
	if vp.tex == nil || vp.tw != tw || vp.th != th {
		if vp.tex != nil {
			a.r.DeleteTexture(vp.tex)
		}
		vp.tex = a.r.NewTextureRGBA(tw, th, pix)
		vp.tw, vp.th = tw, th
	} else {
		a.r.UpdateTextureRGBA(vp.tex, 0, 0, tw, th, pix)
	}
	a.r.DrawImage(gpu.Image{
		Rect:   r,
		Tex:    vp.tex,
		Tint:   gpu.Paint{R: 1, G: 1, B: 1, A: 1},
		Radius: st.Radius,
	})
	// 游戏视口是持续动画的：标记本帧有动画，主循环才会继续重绘下一帧。
	markAnim()
}

// releaseViewport 回收视口纹理（控件被移除时由调用方触发）。
func (a *App) releaseViewport(vp *Viewport3DState) {
	if a == nil || a.r == nil || vp == nil || vp.tex == nil {
		return
	}
	a.r.DeleteTexture(vp.tex)
	vp.tex = nil
	vp.tw, vp.th = 0, 0
}

// GameKeyName 把虚拟键码转成小写键名（如 "w" / "arrowup" / "shift"），
// 供 Viewport3DOpts.OnKey 与 game.Input 使用（与 ui.KeyName 的展示用名称区分）。
func GameKeyName(vk uint32) string {
	switch {
	case vk >= 0x41 && vk <= 0x5A: // A-Z
		return string(rune('a' + (vk - 0x41)))
	case vk >= 0x30 && vk <= 0x39: // 0-9
		return string(rune('0' + (vk - 0x30)))
	case vk >= 0x70 && vk <= 0x7B: // F1-F12
		return fmt.Sprintf("f%d", vk-0x6F)
	}
	switch vk {
	case 0x08:
		return "backspace"
	case 0x09:
		return "tab"
	case 0x0D:
		return "enter"
	case 0x10:
		return "shift"
	case 0x11:
		return "ctrl"
	case 0x12:
		return "alt"
	case 0x13:
		return "pause"
	case 0x1B:
		return "escape"
	case 0x20:
		return "space"
	case 0x21:
		return "pageup"
	case 0x22:
		return "pagedown"
	case 0x23:
		return "end"
	case 0x24:
		return "home"
	case 0x25:
		return "arrowleft"
	case 0x26:
		return "arrowup"
	case 0x27:
		return "arrowright"
	case 0x28:
		return "arrowdown"
	case 0x2D:
		return "insert"
	case 0x2E:
		return "delete"
	case 0xBA:
		return ";"
	case 0xBB:
		return "="
	case 0xBC:
		return ","
	case 0xBD:
		return "-"
	case 0xBE:
		return "."
	case 0xBF:
		return "/"
	case 0xC0:
		return "`"
	case 0xDB:
		return "["
	case 0xDC:
		return "\\"
	case 0xDD:
		return "]"
	case 0xDE:
		return "'"
	}
	if vk >= 0x60 && vk <= 0x69 { // 小键盘
		return fmt.Sprintf("num%d", vk-0x60)
	}
	return fmt.Sprintf("vk%#x", vk)
}
