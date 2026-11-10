//go:build js && wasm

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"fmt"
	"syscall/js"

	"guis/internal/gpu"
)

// 浏览器端窗口层：<canvas> + WebGL 2 + DOM 事件 + requestAnimationFrame。
//
// 与桌面端 win_windows.go 提供完全相同的 platformWin 方法集，因此 app.go 里的
// 布局、绘制、命中、滚动与事件分发一行都不用改——同一套 UI 代码两端通用。

var (
	frameCh = make(chan int, 1)
	winOnce *platformWin
	// frames 已渲染帧数（写到 window.__guisFrames，浏览器冒烟测试用）
	frames int64
)

// signalFrame 非阻塞放行一帧（rAF 回调里调用）。
func signalFrame() {
	select {
	case frameCh <- 1:
	default:
	}
}

type platformWin struct {
	canvas  js.Value
	handler func(Event)

	pw, ph int32   // 画布像素尺寸
	present *gpu.PresentTarget // 离屏呈现目标（整帧合成后一次性 blit 上屏）
	dpr    float32 // 设备像素比
	uiPx   float32 // 界面缩放（浏览器端固定 1）

	visible    bool
	minimized  bool
	fullscreen bool
	title      string
	closed     bool

	// 指针位置：wheel / mouseup 等事件不带坐标时复用
	lastX, lastY float32

	// 触摸合成（手机 / 平板）：浏览器没有滚轮，手指拖动要合成为滚轮事件，
	// 否则组件库的 Scroll 在移动端完全滚不动。
	touching bool
	// touchX0, touchY0 手势起点（逻辑像素），用于区分「点击」与「滑动」。
	touchX0, touchY0 float32
	// touchMoved 本次手势已超过滑动阈值：松手不再触发点击。
	touchMoved bool
	// touchDir 手势方向已锁定（避免斜着拖时页面跟着横向抖动乱滚）。
	touchDir bool
	// touchHoriz 本次手势判为横向：只发 TouchMove，不合成滚轮。
	touchHoriz bool
	// touchUpSent 滑动开始后已经补发过 EvMouseUp（结束按下态），
	// 松手时不再发第二条，避免一次手势被当成一次点击。
	touchUpSent bool
	// dragAcc 手指纵向累计位移（逻辑像素，未发出的事件量）。
	dragAcc float32
	// rectX, rectY 画布在视口中的左上角（touchstart 时缓存一次）。
	// Touch 对象没有 offsetX/offsetY，只能用 clientX/clientY 减去画布原点。
	rectX, rectY float32
	// 惯性采样：velY 为平滑后的手指纵向速度（逻辑像素/秒，正 = 手指向下），
	// lastMoveAt / sampleY 为上一次采样时间与位置。松手时据此算出甩动速度。
	velY       float32
	lastMoveAt float64
	sampleY    float32
	// lastTouchY 上一个触摸点的 y（滚动位移按「事件间增量」发，1:1 跟手且不丢阈值前的位移）。
	lastTouchY float32

	// rafAt 最近一次 rAF 回调的时间（ms），供 pumpFrames 判断 rAF 是否被节流。
	rafAt float64
	// rafWarned 已经标记过「rAF 被暂停」的诊断位（只写一次 window 全局）。
	rafWarned bool

	raf js.Func
}

// newBackend 创建浏览器端窗口（canvas 已存在则复用，否则自动插入）。
func newBackend(opts Options) (*platformWin, error) {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", "screen")
	if canvas.IsNull() || canvas.IsUndefined() {
		canvas = doc.Call("createElement", "canvas")
		canvas.Set("id", "screen")
		doc.Get("body").Call("appendChild", canvas)
	}
	if err := gpu.InitWasm(canvas); err != nil {
		return nil, err
	}

	p := &platformWin{canvas: canvas, visible: true, title: opts.Title, uiPx: 1}
	p.dpr = float32(js.Global().Get("devicePixelRatio").Float())
	if p.dpr <= 0 {
		p.dpr = 1
	}
	// 浏览器端渲染分辨率上限：手机普遍是 2.75~3.5x 设备像素比，按原值渲染
	// canvas 就是 300 万+ 像素（每帧一次整屏混合 + 一次离屏 blit），中端机 GPU
	// 直接跑满、滚动停帧。限到 2x 后像素数减一半以上，文字仍按 2x 光栅化，
	// 肉眼几乎看不出差别。?dpr=1.5 / ?dpr=3 可手动覆盖（调试对比用）。
	if v := dprOverride(); v > 0 {
		p.dpr = v
	} else if p.dpr > maxWebDPR {
		p.dpr = maxWebDPR
	}
	p.resize()
	p.bindEvents()
	winOnce = p
	return p, nil
}

// pollMessages 等待下一帧（rAF 节拍），让共享的 runLoop 在浏览器里同样按帧推进。
func pollMessages() bool {
	if winOnce == nil || winOnce.closed {
		return false
	}
	<-frameCh
	return true
}

func (p *platformWin) resize() {
	w := float32(js.Global().Get("innerWidth").Float())
	h := float32(js.Global().Get("innerHeight").Float())
	if w < 16 {
		w = 1024
	}
	if h < 16 {
		h = 700
	}
	p.pw = int32(w * p.dpr)
	p.ph = int32(h * p.dpr)
	p.canvas.Set("width", int(p.pw))
	p.canvas.Set("height", int(p.ph))
	style := p.canvas.Get("style")
	style.Set("width", fmt.Sprintf("%.0fpx", w))
	style.Set("height", fmt.Sprintf("%.0fpx", h))
	if p.handler != nil {
		p.handler(Event{Type: EvResize, Width: w, Height: h, Scale: p.dpr})
	}
}

func (p *platformWin) bindEvents() {
	move := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		p.emit(Event{Type: EvMouseMove, X: float32(e.Get("offsetX").Float()), Y: float32(e.Get("offsetY").Float())})
		return nil
	})
	down := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		p.canvas.Call("focus")
		p.emit(Event{Type: EvMouseDown, X: float32(e.Get("offsetX").Float()), Y: float32(e.Get("offsetY").Float())})
		return nil
	})
	up := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.emit(Event{Type: EvMouseUp, X: p.lastX, Y: p.lastY})
		return nil
	})
	leave := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.emit(Event{Type: EvMouseLeave})
		return nil
	})
	dropOver := js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		return nil
	})
	// 拖入文件：浏览器只暴露文件名，这里把文件名当路径交给同一套 OnDrop 逻辑
	drop := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		var paths []string
		files := e.Get("dataTransfer").Get("files")
		for i := 0; i < files.Length(); i++ {
			paths = append(paths, files.Index(i).Get("name").String())
		}
		if len(paths) == 0 {
			return nil
		}
		p.emit(Event{Type: EvDropFiles, Paths: paths, X: p.lastX, Y: p.lastY})
		return nil
	})
	wheel := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		p.emit(Event{Type: EvMouseWheel, X: p.lastX, Y: p.lastY, DY: wheelClicks(e)})
		return nil
	})
	key := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		k := e.Get("key").String()
		vk := keyCode(k)
		if vk != 0 {
			e.Call("preventDefault")
			p.emit(Event{Type: EvKeyDown, Key: vk})
			return nil
		}
		if len([]rune(k)) == 1 {
			e.Call("preventDefault")
			for _, r := range k {
				p.emit(Event{Type: EvChar, Char: r})
			}
		}
		return nil
	})
	compStart := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.emit(Event{Type: EvCompositionStart})
		return nil
	})
	compUpdate := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.emit(Event{Type: EvComposition, Text: args[0].Get("data").String()})
		return nil
	})
	compEnd := js.FuncOf(func(this js.Value, args []js.Value) any {
		txt := args[0].Get("data").String()
		p.emit(Event{Type: EvCompositionEnd})
		for _, r := range txt {
			p.emit(Event{Type: EvChar, Char: r})
		}
		return nil
	})
	// 触摸：手机 / 平板没有鼠标和滚轮，这里把手指事件合成为鼠标事件 + 滚轮事件。
	// 触摸按下 → EvMouseDown、移动 → EvMouseMove、抬起 → EvMouseUp；
	// 同时额外发 EvTouch*，业务需要区分手指 / 鼠标时看 Pointer 字段。
	touchStart := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		// 阻止浏览器原生手势（页面回弹 / 下拉刷新 / 双击放大）。
		// 触摸设备上 preventDefault 也会抑制 touchend 后的「兼容鼠标事件」，
		// 否则一次点击会被我们和浏览器各处理一遍（点一下跳两次）。
		e.Call("preventDefault")
		p.canvas.Call("focus")
		rect := p.canvas.Call("getBoundingClientRect")
		p.rectX = float32(rect.Get("left").Float())
		p.rectY = float32(rect.Get("top").Float())
		x, y := p.touchAt(e)
		p.touching, p.touchMoved, p.touchDir, p.touchHoriz = true, false, false, false
		p.touchUpSent, p.dragAcc = false, 0
		p.touchX0, p.touchY0 = x, y
		// 惯性采样起点
		p.velY, p.sampleY, p.lastMoveAt = 0, y, nowMs()
		p.lastTouchY = y
		p.emit(Event{Type: EvMouseMove, X: x, Y: y, Pointer: 1})
		p.emit(Event{Type: EvMouseDown, X: x, Y: y, Pointer: 1})
		p.emit(Event{Type: EvTouchDown, X: x, Y: y, Pointer: 1})
		return nil
	})
	touchMove := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		if !p.touching {
			return nil
		}
		x, y := p.touchAt(e)
		now := nowMs()
		// 速度采样：瞬时速度 + 指数平滑。间隔过大（手指停住 / 主线程卡了一帧）
		// 就当作减速——否则会得到一个假的“甩动”，松手后页面自己飞出去。
		if dt := now - p.lastMoveAt; dt > 0 && dt < 120 {
			inst := (y - p.sampleY) / float32(dt) * 1000
			p.velY = p.velY*0.6 + inst*0.4
		} else {
			p.velY *= 0.5
		}
		p.sampleY, p.lastMoveAt = y, now

		dx, dyTotal := x-p.touchX0, y-p.touchY0
		// 方向锁：第一次超过阈值时确定本次手势是横还是竖。
		// 以前只看纵向位移，斜着拖（如从屏幕边缘往内划）时页面会跟着跳；
		// 横向比纵向大 1.2 倍以上就判为横滑，本次手势不再纵向滚。
		if !p.touchDir && (abs32(dx) > touchDirSlop || abs32(dyTotal) > touchDirSlop) {
			p.touchDir = true
			p.touchHoriz = abs32(dx) > abs32(dyTotal)*1.2
		}
		if !p.touchMoved && abs32(dyTotal) > touchSlop {
			p.touchMoved = true
		}
		// 确认在滑页面后立刻结束按下态（补发一条 Cancel 的 EvMouseUp）：
		// 否则整个滑动期间 pressed 一直为真，frame() 每帧都要 hitInteractive，
		// 按下的卡片还会跟着手指一路亮下去（看上去像界面在抖）。
		if p.touchMoved && !p.touchUpSent {
			p.touchUpSent = true
			p.emit(Event{Type: EvMouseUp, X: x, Y: y, Pointer: 1, Cancel: true})
			// 手指一旦在滑页面，悬停态就没意义了：清掉，避免滚动时旧卡片一路亮着
			p.emit(Event{Type: EvMouseLeave})
		}
		if p.touchMoved || p.touchDir {
			// 滑动中不再发 EvMouseMove：悬停判定对滚动毫无作用，
			// 每次 touchmove 多跑一次全树命中只是在给主线程加负担。
			p.emit(Event{Type: EvTouchMove, X: x, Y: y, Pointer: 1})
		} else {
			p.emit(Event{Type: EvMouseMove, X: x, Y: y, Pointer: 1})
			p.emit(Event{Type: EvTouchMove, X: x, Y: y, Pointer: 1})
		}
		if p.touchHoriz || !p.touchMoved {
			return nil
		}
		// 位移用「上一个触摸点」算：1:1 跟手，不会因为阈值丢掉开头几像素
		dy := y - p.lastTouchY
		p.lastTouchY = y
		p.synthWheel(x, y, dy)
		return nil
	})
	touchEnd := js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		e.Call("preventDefault")
		if !p.touching {
			return nil
		}
		x, y := p.lastX, p.lastY
		if ct := e.Get("changedTouches"); ct.Truthy() && ct.Length() > 0 {
			t := ct.Index(0)
			x = float32(t.Get("clientX").Float()) - p.rectX
			y = float32(t.Get("clientY").Float()) - p.rectY
		}
		moved, horiz, upSent := p.touchMoved, p.touchHoriz, p.touchUpSent
		vel := p.velY
		lastMove := p.lastMoveAt
		p.touching, p.touchMoved, p.touchDir, p.touchHoriz = false, false, false, false
		p.touchUpSent = false
		p.dragAcc = 0
		// 滑动后松手不进点击链路：手指停下时落点上的卡片 / 按钮不该被误触。
		// touchmove 阶段已经补发过 Cancel，这里不再重复（否则会多跑一次按下/抬起状态机）。
		if !upSent {
			p.emit(Event{Type: EvMouseUp, X: x, Y: y, Pointer: 1, Cancel: moved})
		}
		p.emit(Event{Type: EvTouchUp, X: x, Y: y, Pointer: 1})
		// 惯性滑动：松手前还在动（<90ms）且本次是纵向滚动才甩。
		// 手指停下再松开（“拖到位置后停一下”）应当就停在原地，不要自己滑走。
		if moved && !horiz && nowMs()-lastMove < 90 {
			if a := currentApp.Load(); a != nil {
				// velY 是手指速度（向下为正），内容偏移方向相反，所以取负号
				a.startFling(x, y, -vel*0.95)
			}
		}
		// 触摸设备没有「悬停」：松手就清掉悬停态，否则点过的卡片会一直亮着。
		p.emit(Event{Type: EvMouseLeave})
		return nil
	})
	touchCancel := js.FuncOf(func(this js.Value, args []js.Value) any {
		if !p.touching {
			return nil
		}
		p.touching, p.touchMoved, p.touchDir, p.touchHoriz = false, false, false, false
		p.touchUpSent = false
		p.dragAcc = 0
		p.emit(Event{Type: EvMouseUp, X: p.lastX, Y: p.lastY, Pointer: 1, Cancel: true})
		p.emit(Event{Type: EvTouchUp, X: p.lastX, Y: p.lastY, Pointer: 1})
		p.emit(Event{Type: EvMouseLeave})
		return nil
	})
	resize := js.FuncOf(func(this js.Value, args []js.Value) any { p.resize(); return nil })

	p.canvas.Call("addEventListener", "mousemove", move)
	p.canvas.Call("addEventListener", "mousedown", down)
	js.Global().Call("addEventListener", "mouseup", up)
	p.canvas.Call("addEventListener", "mouseleave", leave)
	p.canvas.Call("addEventListener", "wheel", wheel, map[string]any{"passive": false})
	// 触摸监听必须 passive:false 才能 preventDefault（否则浏览器忽略并告警）。
	p.canvas.Call("addEventListener", "touchstart", touchStart, map[string]any{"passive": false})
	p.canvas.Call("addEventListener", "touchmove", touchMove, map[string]any{"passive": false})
	p.canvas.Call("addEventListener", "touchend", touchEnd, map[string]any{"passive": false})
	p.canvas.Call("addEventListener", "touchcancel", touchCancel, map[string]any{"passive": false})
	p.canvas.Call("addEventListener", "keydown", key)
	p.canvas.Call("addEventListener", "dragover", dropOver)
	p.canvas.Call("addEventListener", "drop", drop)
	p.canvas.Call("addEventListener", "compositionstart", compStart)
	p.canvas.Call("addEventListener", "compositionupdate", compUpdate)
	p.canvas.Call("addEventListener", "compositionend", compEnd)
	js.Global().Call("addEventListener", "resize", resize)
	p.canvas.Set("tabIndex", 0)

	// 帧节拍：rAF 只做「放行一帧」的信号，真正绘制仍在共享 runLoop 里
	p.raf = js.FuncOf(func(this js.Value, args []js.Value) any {
		p.rafAt = nowMs()
		signalFrame()
		frames++
		js.Global().Set("__guisFrames", frames)
		js.Global().Set("__guisRaf", true)
		js.Global().Call("requestAnimationFrame", p.raf)
		return nil
	})
	js.Global().Call("requestAnimationFrame", p.raf)
	// rAF 可能被浏览器完全暂停（窗口被遮挡 / 后台标签 / 远程桌面 / 嵌入式 WebView），
	// 没有兜底节拍的话整个界面一帧都不会画，启动页就永远停在那里。
	p.pumpFrames()
}

// pumpFrames rAF 兜底节拍：rAF 超过 250ms 没回调就自己放行一帧。
//
// 浏览器在「窗口被遮挡 / 后台标签页 / 远程桌面 / 某些嵌入式 WebView」里会暂停
// requestAnimationFrame，而 runLoop 是 <-frameCh 等帧的——现象就是页面加载完了
// 却一直卡在启动页，连第一帧都出不来。这里用 setTimeout 每 32ms 检查一次：
// rAF 正常时什么都不做（rAF 恢复后自动让位），被节流时按约 30fps 自己驱动。
func (p *platformWin) pumpFrames() {
	var tick js.Func
	tick = js.FuncOf(func(this js.Value, args []js.Value) any {
		if p.closed {
			return nil
		}
		if now := nowMs(); now-p.rafAt > 250 {
			p.rafAt = now
			if !p.rafWarned {
				p.rafWarned = true
				js.Global().Set("__guisRafThrottled", true)
			}
			signalFrame()
			frames++
			js.Global().Set("__guisFrames", frames)
		}
		js.Global().Call("setTimeout", tick, 32)
		return nil
	})
	js.Global().Call("setTimeout", tick, 250)
}

// nowMs 当前时间（毫秒）。优先 performance.now（不受系统时钟调整影响），
// 老浏览器没有时退回 Date.now。
func nowMs() float64 {
	if perf := js.Global().Get("performance"); perf.Truthy() {
		if fn := perf.Get("now"); fn.Type() == js.TypeFunction {
			return perf.Call("now").Float()
		}
	}
	return js.Global().Get("Date").Call("now").Float()
}

// touchSlop 判定「这次手势是滑动而不是点击」的位移阈值（逻辑像素）。
//
// 太小：点击时手指的轻微桁动会被当成滑动，连点击一起吞掉；
// 太大：开始滑的前十几像素不跟手，手感发黏。
const touchSlop = 8

// touchDirSlop 方向锁阈值（逻辑像素）：超过后才判定本次手势是横还是竖。
const touchDirSlop = 8

// maxWebDPR 浏览器端渲染分辨率上限。
//
// 手机普遍是 2.75~3.5x 设备像素比，按原值渲染 canvas 就是 300 万+ 像素，
// 每帧一次整屏混合 + 一次离屏 blit，中端机 GPU 直接跑满、滚动停帧。
// 限到 2x 后像素数减一半以上，字仍按 2x 光栅化，肉眼几乎看不出差别。
const maxWebDPR = 2

// abs32 float32 绝对值（wasm 端不为这点小事引 math）。
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// dprOverride 读取 URL 参数 ?dpr=1.5 手动指定渲染分辨率（调试 / A/B 对比用）。
// 超出 1~4 的非法值返回 0（走默认上限）。
func dprOverride() float32 {
	loc := js.Global().Get("location")
	u := js.Global().Get("URLSearchParams")
	if !loc.Truthy() || !u.Truthy() {
		return 0
	}
	v := u.New(loc.Get("search")).Call("get", "dpr")
	if !v.Truthy() {
		return 0
	}
	f := v.Float()
	if f < 1 || f > 4 {
		return 0
	}
	return float32(f)
}

// touchAt 取第一个触点在画布内的坐标（逻辑像素）。
func (p *platformWin) touchAt(e js.Value) (float32, float32) {
	touches := e.Get("touches")
	if !touches.Truthy() || touches.Length() == 0 {
		return p.lastX, p.lastY
	}
	t := touches.Index(0)
	return float32(t.Get("clientX").Float()) - p.rectX,
		float32(t.Get("clientY").Float()) - p.rectY
}

// synthWheel 把手指纵向拖动合成成滚轮事件（与 Android 后端同一套约定）。
//
// 组件库的 Scroll 只认 EvMouseWheel（桌面端由滚轮驱动），移动端没有滚轮，
// 不合成的话页面上根本滚不动——这就是「手机上滑不动网页」的根因。
//
// EvMouseWheel.DY 的单位是「滚轮格」（1 格 = wheelStepSize 逻辑像素），
// Pointer=1 让 App 走 1:1 跟手定位，而不是滚轮那种缓动。
// 换算规则见 touchDragClicks。
func (p *platformWin) synthWheel(x, y, dy float32) {
	if !p.touching || dy == 0 {
		return
	}
	p.dragAcc += dy
	// 每帧都发事件会让长页面高频重建 + 重绘；累计到阈值才发一次。
	clicks := touchDragClicks(p.dragAcc)
	if clicks == 0 {
		return
	}
	p.emit(Event{Type: EvMouseWheel, X: x, Y: y, DY: clicks, Pointer: 1})
	p.dragAcc = 0
}

// wheelClicks 把 DOM wheel 事件的 deltaY 折算成框架约定的「滚轮格数」（上正下负），
// 具体换算与夹取规则见 wheelClicksFrom。
func wheelClicks(e js.Value) float32 {
	return wheelClicksFrom(e.Get("deltaY").Float(), e.Get("deltaMode").Int())
}

func (p *platformWin) emit(ev Event) {
	if ev.Type == EvMouseMove || ev.Type == EvMouseDown || ev.Type == EvMouseUp {
		p.lastX, p.lastY = ev.X, ev.Y
	}
	if p.handler != nil {
		p.handler(ev)
	}
}

// keyCode 把 DOM key 映射成 Windows VK 码（框架里只用到少数几个功能键）。
func keyCode(k string) uint32 {
	switch k {
	case "Escape":
		return VK_ESCAPE
	case "Enter":
		return VK_RETURN
	case "Tab":
		return VK_TAB
	case "Backspace":
		return VK_BACK
	case "Delete":
		return VK_DELETE
	case "ArrowLeft":
		return VK_LEFT
	case "ArrowRight":
		return VK_RIGHT
	case "ArrowUp":
		return VK_UP
	case "ArrowDown":
		return VK_DOWN
	case " ":
		return VK_SPACE
	}
	return 0
}

// NativeHandle 浏览器端无窗口句柄，返回 0（保持 Backend 接口一致）。
func (p *platformWin) NativeHandle() uintptr { return 0 }

// Compositor 浏览器端无 DirectComposition 合成器。
func (p *platformWin) Compositor() any { return nil }

// ---- platformWin 方法集（与桌面端一一对应）----

func (p *platformWin) SetEventHandler(fn func(Event)) { p.handler = fn }
func (p *platformWin) MakeCurrent()                   {}
func (p *platformWin) Swap()                          { syncOverlays() }
func (p *platformWin) PixelSize() (int32, int32)      { return p.pw, p.ph }
func (p *platformWin) Scale() float32                 { return p.uiPx * p.dpr }
func (p *platformWin) IsMinimized() bool              { return p.minimized }
func (p *platformWin) SetTitle(s string)              { p.title = s }
func (p *platformWin) SetIcon([]byte) bool            { return true }
func (p *platformWin) SetVisible(v bool)              { p.visible = v }
func (p *platformWin) Minimize()                      { p.minimized = true }
func (p *platformWin) Restore()                       { p.minimized = false }
func (p *platformWin) Center()                        {}
func (p *platformWin) BeginDragMove()                 {}

func (p *platformWin) SetAlwaysOnTop(bool)                             {}
func (p *platformWin) AlwaysOnTop() bool                               { return false }
func (p *platformWin) SetAcceptDrops(bool)                             {}
func (p *platformWin) AcceptDrops() bool                               { return true }
func (p *platformWin) RegisterHotKey(mods, vk uint32, fn func()) int32 { return 0 }
func (p *platformWin) UnregisterHotkey(id int32)                       {}
func (p *platformWin) SetCornerRadius(float32)                         {}
func (p *platformWin) CornerRadius() float32                           { return 0 }

func (p *platformWin) ToggleMaximize()   { p.SetFullscreen(!p.fullscreen) }
func (p *platformWin) IsMaximized() bool { return p.fullscreen }

func (p *platformWin) SetFullscreen(v bool) {
	p.fullscreen = v
	doc := js.Global().Get("document")
	if v {
		doc.Get("documentElement").Call("requestFullscreen")
	} else {
		doc.Call("exitFullscreen")
	}
	p.resize()
}

func (p *platformWin) IsFullscreen() bool        { return p.fullscreen }
func (p *platformWin) SetImePosition(x, y int32) {}

func (p *platformWin) Close() {
	if p.closed {
		return
	}
	p.closed = true
	if p.handler != nil {
		p.handler(Event{Type: EvClose})
	}
}

func (p *platformWin) Destroy() { p.closed = true }

// 合成与硬件句柄：浏览器端不存在，统一空实现。
func (p *platformWin) AttachCompose(pw, ph int32) {}
func (p *platformWin) HasCompose() bool           { return false }
func (p *platformWin) ResizeCompose(pw, ph int32) {}
// ComposeBegin 返回离屏呈现目标：整帧先画进 FBO，由 ComposeEnd 一次性上屏。
//
// 直接画默认帧缓冲时，每帧「先清屏再重画」的中间态可能被浏览器合成器抓到，
// 鼠标移动触发的高频重绘就会表现为界面闪烁；走离屏 + blit 后每次上屏都是完整帧。
func (p *platformWin) ComposeBegin() uint32 {
	if p.pw <= 0 || p.ph <= 0 {
		return 0
	}
	if p.present == nil {
		p.present = gpu.NewPresentTarget()
	}
	return p.present.Ensure(p.pw, p.ph)
}

func (p *platformWin) ComposeEnd() {
	if p.present != nil {
		p.present.Blit()
	}
}
