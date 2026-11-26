// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"image"
	"sync"
	"time"

	"guis/game"
)

// GameViewOpts GameView 参数（开箱即用的 3D / 2D 游戏视口）。
type GameViewOpts struct {
	// Engine 游戏引擎（必填，*game.Engine 同时支持 3D 场景与 2D 精灵）。
	Engine *game.Engine
	// Key 控件标识（多视口区分状态，默认 "gameview"）。
	Key string
	// Resolution 控件未显式指定尺寸时的默认尺寸（默认 640×360）。
	Resolution [2]int
	// NoAutoFocus 关闭"点击自动取得键盘焦点"（默认开启）。
	NoAutoFocus bool
	// IdleTimeout 空闲秒数：超过这么久没有任何键鼠事件就自动暂停逻辑更新
	// （0 = 不自动暂停，默认 0）。
	IdleTimeout float32
	// MouseLock 指针锁定：开启后鼠标相对位移直接进引擎（FPS 转视角）。
	MouseLock bool
	// ShowFPS 左上角显示帧率（用内置像素字体，零外部资源）。
	ShowFPS bool
	// ShowStats 左上角显示帧率 + 三角形数 + 像素数（更详细的调试信息）。
	ShowStats bool
	// OnFrame 每帧回调（渲染前调用，可用于 HUD / 调试 / 暂停菜单）。
	OnFrame func(eng *game.Engine, dt float32)
	// OnKey / OnMouse / OnWheel / OnTouch 在引擎输入之后追加的自定义处理。
	OnKey   func(key string, down bool)
	OnMouse func(x, y float32, button int, down bool)
	OnWheel func(dy float32)
	OnTouch func(x, y float32, phase int, id int)
}

// gameViewState 按 Key 复用的视口包装（声明式视图每帧重建节点，状态放注册表）。
type gameViewState struct {
	opts      *GameViewOpts
	lastInput time.Time
	fps       float32
}

var (
	gvMu     sync.Mutex
	gvStates = map[string]*gameViewState{}
)

// gameViewStateFor 取得（或创建）指定 Key 的视口状态。
func gameViewStateFor(key string, o *GameViewOpts) *gameViewState {
	gvMu.Lock()
	defer gvMu.Unlock()
	st, ok := gvStates[key]
	if !ok {
		st = &gameViewState{}
		gvStates[key] = st
	}
	st.opts = o
	return st
}

// ResetGameView 重置指定 Key 的视口状态（关卡切换 / 释放资源时调用）。
func ResetGameView(key string) {
	gvMu.Lock()
	defer gvMu.Unlock()
	delete(gvStates, key)
}

// markInput 记录一次输入事件时间（用于空闲自动暂停）。
func (s *gameViewState) markInput() {
	if s == nil {
		return
	}
	s.lastInput = time.Now()
}

// RenderViewport 实现 ViewportSource：暂停判定 → 帧回调 → 引擎渲染 → 调试叠字。
func (s *gameViewState) RenderViewport(w, h int, dt float32) ([]byte, int, int) {
	if s == nil || s.opts == nil || s.opts.Engine == nil {
		return nil, 0, 0
	}
	o := s.opts
	eng := o.Engine
	if dt <= 0 {
		dt = 1.0 / 60
	}
	// 空闲自动暂停（切到别的窗口后不再有输入事件，省 CPU）
	if o.IdleTimeout > 0 && !s.lastInput.IsZero() && time.Since(s.lastInput).Seconds() > float64(o.IdleTimeout) {
		eng.Paused = true
	}
	// 帧率平滑
	inst := 1 / dt
	if s.fps == 0 {
		s.fps = inst
	} else {
		s.fps += (inst - s.fps) * 0.1
	}
	if o.OnFrame != nil {
		o.OnFrame(eng, dt)
	}
	pix, tw, th := eng.RenderViewport(w, h, dt)
	if (o.ShowFPS || o.ShowStats) && pix != nil && tw > 0 && th > 0 && len(pix) >= tw*th*4 {
		img := &image.RGBA{Pix: pix, Stride: tw * 4, Rect: image.Rect(0, 0, tw, th)}
		fps := int(s.fps + 0.5)
		var txt string
		if o.ShowStats {
			st := eng.FrameStats()
			txt = fmt.Sprintf("FPS %d TRI %d PX %d", fps, st.Triangles, st.Pixels)
		} else {
			txt = fmt.Sprintf("FPS %d", fps)
		}
		game.BuiltinFont().DrawTextShadow(img, txt, 4, 4, 1, game.RGB(255, 255, 255), game.RGB(0, 0, 0))
	}
	return pix, tw, th
}

// RenderViewportNative 实现 NativeViewportSource：硬件零回读呈现。
//
// 前置流程（空闲暂停 / 帧率平滑 / OnFrame 回调）与像素路径一致；
// 只是把最后的「渲染 + 回读 + 上传」换成「渲染 + 交出显存纹理句柄」。
// 开了 FPS/统计叠字时回退像素路径（叠字要写进 CPU 缓冲）。
func (s *gameViewState) RenderViewportNative(w, h int, dt float32) (uint32, int, int, bool) {
	if s == nil || s.opts == nil || s.opts.Engine == nil {
		return 0, 0, 0, false
	}
	o := s.opts
	eng := o.Engine
	if o.ShowFPS || o.ShowStats || !eng.ZeroCopy() {
		return 0, 0, 0, false
	}
	if dt <= 0 {
		dt = 1.0 / 60
	}
	if o.IdleTimeout > 0 && !s.lastInput.IsZero() && time.Since(s.lastInput).Seconds() > float64(o.IdleTimeout) {
		eng.Paused = true
	}
	inst := 1 / dt
	if s.fps == 0 {
		s.fps = inst
	} else {
		s.fps += (inst - s.fps) * 0.1
	}
	if o.OnFrame != nil {
		o.OnFrame(eng, dt)
	}
	texID, tw, th, ok := eng.RenderViewportNative(w, h, dt)
	if !ok || texID == 0 || tw <= 0 || th <= 0 {
		return 0, 0, 0, false
	}
	return texID, tw, th, true
}

// GameView 一体化游戏控件：自动接线键鼠 / 触摸到引擎，自带帧率显示与空闲暂停。
//
//	eng := game.NewEngine(640, 360)
//	eng.SetGame(myGame{})
//	ui.Column(ui.GameView(&ui.GameViewOpts{Engine: eng, ShowFPS: true}).Grow(1))
//
// 与手写 Viewport3D 相比少写 4 个事件回调与暂停逻辑，2D / 3D 通用。
func GameView(o *GameViewOpts) *Node {
	if o == nil {
		o = &GameViewOpts{}
	}
	key := o.Key
	if key == "" {
		key = "gameview"
	}
	st := gameViewStateFor(key, o)

	onKey := func(k string, down bool) {
		st.markInput()
		if o.Engine != nil {
			o.Engine.HandleKey(k, down)
			// 按任意键恢复运行（从空闲暂停中回到游戏）
			if down {
				o.Engine.Paused = false
			}
		}
		if o.OnKey != nil {
			o.OnKey(k, down)
		}
	}
	onMouse := func(x, y float32, button int, down bool) {
		st.markInput()
		if o.Engine != nil {
			o.Engine.HandleMouse(x, y, button, down)
			if down {
				o.Engine.Paused = false
				// 指针锁定：按下视口后进入「只看位移」模式，转视角不会因光标撞边而卡住
				if o.MouseLock {
					o.Engine.SetPointerLock(true)
				}
			}
		}
		if o.OnMouse != nil {
			o.OnMouse(x, y, button, down)
		}
	}
	onWheel := func(dy float32) {
		st.markInput()
		if o.Engine != nil {
			o.Engine.HandleWheel(dy)
		}
		if o.OnWheel != nil {
			o.OnWheel(dy)
		}
	}
	onTouch := func(x, y float32, phase int, id int) {
		st.markInput()
		if o.Engine != nil {
			o.Engine.HandleTouch(x, y, phase, id)
			if phase == 0 {
				o.Engine.Paused = false
			}
		}
		if o.OnTouch != nil {
			o.OnTouch(x, y, phase, id)
		}
	}

	return Viewport3D(&Viewport3DOpts{
		Src:        st,
		Key:        key,
		Resolution: o.Resolution,
		AutoFocus:  !o.NoAutoFocus,
		MouseLock:  o.MouseLock,
		OnKey:      onKey,
		OnMouse:    onMouse,
		OnWheel:    onWheel,
		OnTouch:    onTouch,
	})
}
