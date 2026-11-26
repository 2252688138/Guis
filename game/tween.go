// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// EaseFunc 缓动曲线：输入归一化时间 t∈[0,1]，输出插值系数。
//
// 已有的 EaseLinear / EaseIn / EaseOut / EaseInOut（见 anim3d.go）可直接当
// EaseFunc 使用，本文件另外补齐常用二次/三次/正弦/指数/圆形/回弹/弹跳曲线。
type EaseFunc = func(t float32) float32

// 常用缓动曲线（Back 系列在两端会略微超出 [0,1]，属正常的回拉/过冲）。
var (
	// EaseInQuad 二次加速。
	EaseInQuad = func(t float32) float32 { return t * t }
	// EaseOutQuad 二次减速。
	EaseOutQuad = func(t float32) float32 { return t * (2 - t) }
	// EaseInOutQuad 二次进出。
	EaseInOutQuad = func(t float32) float32 {
		if t < 0.5 {
			return 2 * t * t
		}
		t = 2*t - 2
		return 1 - t*t/2
	}
	// EaseInCubic 三次加速。
	EaseInCubic = func(t float32) float32 { return t * t * t }
	// EaseOutCubic 三次减速（最常用的“舒服”曲线）。
	EaseOutCubic = func(t float32) float32 {
		t--
		return 1 + t*t*t
	}
	// EaseInOutCubic 三次进出。
	EaseInOutCubic = func(t float32) float32 {
		if t < 0.5 {
			return 4 * t * t * t
		}
		t = 2*t - 2
		return 1 + t*t*t/2
	}
	// EaseInSine 正弦加速。
	EaseInSine = func(t float32) float32 { return 1 - float32(math.Cos(float64(t)*math.Pi/2)) }
	// EaseOutSine 正弦减速。
	EaseOutSine = func(t float32) float32 { return float32(math.Sin(float64(t) * math.Pi / 2)) }
	// EaseInOutSine 正弦进出。
	EaseInOutSine = func(t float32) float32 {
		return -(float32(math.Cos(math.Pi*float64(t))) - 1) / 2
	}
	// EaseInExpo 指数加速。
	EaseInExpo = func(t float32) float32 {
		if t <= 0 {
			return 0
		}
		return float32(math.Pow(2, 10*float64(t)-10))
	}
	// EaseOutExpo 指数减速。
	EaseOutExpo = func(t float32) float32 {
		if t >= 1 {
			return 1
		}
		return 1 - float32(math.Pow(2, -10*float64(t)))
	}
	// EaseInCirc 圆形加速。
	EaseInCirc = func(t float32) float32 { return 1 - float32(math.Sqrt(float64(1-t*t))) }
	// EaseOutCirc 圆形减速。
	EaseOutCirc = func(t float32) float32 {
		t--
		return float32(math.Sqrt(float64(1 - t*t)))
	}
	// EaseInBack 起始回拉（蓄力感）。
	EaseInBack = func(t float32) float32 {
		const c1 = 1.70158
		return t * t * ((c1+1)*t - c1)
	}
	// EaseOutBack 收尾过冲（弹出感）。
	EaseOutBack = func(t float32) float32 {
		const c1 = 1.70158
		t--
		return 1 + t*t*((c1+1)*t+c1)
	}
	// EaseOutElastic 收尾弹性震荡。
	EaseOutElastic = func(t float32) float32 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		const c4 = 2 * math.Pi / 3
		return float32(math.Pow(2, -10*float64(t)))*float32(math.Sin((float64(t)*10-0.75)*c4)) + 1
	}
	// EaseOutBounce 收尾弹跳。
	EaseOutBounce = easeOutBounce
)

// easeOutBounce 标准弹跳曲线（EaseOutBounce 的实现）。
func easeOutBounce(t float32) float32 {
	const n1, d1 = 7.5625, 2.75
	switch {
	case t < 1/d1:
		return n1 * t * t
	case t < 2/d1:
		t -= 1.5 / d1
		return n1*t*t + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	default:
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

// easeOr 空缓动退化为线性，避免调用方传 nil 时 panic。
func easeOr(fn EaseFunc) EaseFunc {
	if fn == nil {
		return EaseLinear
	}
	return fn
}

// Tween 单个补间：把归一化进度 [0,1] 经缓动后交给回调，
// 支持延迟、循环（Loop<0 无限）、乒乓与结束回调。
//
//	tw := game.NewTween(0.4).Ease(game.EaseOutCubic).
//		OnUpdate(func(p float32) { btn.Alpha = p })
//	tw.Update(dt) // 每帧调用
type Tween struct {
	// Duration 单次时长（秒，≤0 视为立即完成）。
	Duration float32
	// Delay 启动前延迟（秒，仅第一次启动生效）。
	Delay float32
	// Loop 循环轮数（0 = 只播一次；<0 = 无限循环）。
	Loop int
	// PingPong 循环时来回播放（1→0→1）。
	PingPong bool

	ease     EaseFunc
	onUpdate func(p float32)
	onLoop   func(times int)
	onDone   func()

	elapsed float32 // 当前轮已播放时间
	delay   float32 // 剩余延迟
	loops   int     // 已完成轮数
	dir     float32 // 1 = 正向，-1 = 乒乓反向
	started bool
	done    bool
}

// NewTween 创建补间（默认线性缓动）。
func NewTween(duration float32) *Tween {
	return &Tween{Duration: duration, dir: 1}
}

// Ease 设置缓动曲线（nil = 线性）并返回自身（链式）。
func (t *Tween) Ease(fn EaseFunc) *Tween {
	t.ease = fn
	return t
}

// WithDelay 设置启动延迟并返回自身（链式）。
func (t *Tween) WithDelay(sec float32) *Tween {
	t.Delay = sec
	return t
}

// LoopTimes 设置循环轮数（<0 无限循环）并返回自身（链式）。
func (t *Tween) LoopTimes(n int) *Tween {
	t.Loop = n
	return t
}

// WithPingPong 设置乒乓循环并返回自身（链式）。
func (t *Tween) WithPingPong(v bool) *Tween {
	t.PingPong = v
	return t
}

// OnUpdate 设置进度回调（p 已含缓动，0→1）并返回自身（链式）。
func (t *Tween) OnUpdate(fn func(p float32)) *Tween {
	t.onUpdate = fn
	return t
}

// OnLoop 设置每轮结束回调（从 1 开始计数）并返回自身（链式）。
func (t *Tween) OnLoop(fn func(times int)) *Tween {
	t.onLoop = fn
	return t
}

// OnDone 设置全部结束回调并返回自身（链式）。
func (t *Tween) OnDone(fn func()) *Tween {
	t.onDone = fn
	return t
}

// Done 是否已结束。
func (t *Tween) Done() bool { return t.done }

// Progress 当前进度（已缓动；乒乓反向时递减）。
func (t *Tween) Progress() float32 {
	if t.Duration <= 0 {
		return 1
	}
	raw := t.elapsed / t.Duration
	if raw > 1 {
		raw = 1
	}
	if raw < 0 {
		raw = 0
	}
	if t.dir < 0 {
		raw = 1 - raw
	}
	return easeOr(t.ease)(raw)
}

// Reset 重置到初始状态（保留配置）。
func (t *Tween) Reset() {
	t.elapsed, t.loops, t.dir = 0, 0, 1
	t.delay = t.Delay
	t.started, t.done = false, false
}

// Restart 重置并立即启动（忽略 Delay）。
func (t *Tween) Restart() {
	t.Reset()
	t.started = true
	t.delay = 0
}

// Update 推进补间（dt 秒）。结束时自动置 Done 并触发结束回调。
func (t *Tween) Update(dt float32) {
	if t.done || dt <= 0 {
		return
	}
	if !t.started {
		t.started = true
		t.delay = t.Delay
	}
	if t.delay > 0 {
		t.delay -= dt
		if t.delay > 0 {
			return
		}
		dt = -t.delay // 把超出延迟的部分算进本轮播放时间
		t.delay = 0
	}
	if t.Duration <= 0 {
		t.finish()
		return
	}

	t.elapsed += dt
	for t.elapsed >= t.Duration {
		t.elapsed -= t.Duration
		t.loops++
		if t.onLoop != nil {
			t.onLoop(t.loops)
		}
		if t.Loop == 0 || (t.Loop > 0 && t.loops >= t.Loop) {
			t.finish()
			return
		}
		if t.PingPong {
			t.dir = -t.dir
		}
	}
	if t.onUpdate != nil {
		t.onUpdate(t.Progress())
	}
}

// finish 触达终点并收尾。
func (t *Tween) finish() {
	t.elapsed = t.Duration
	t.dir = 1
	if t.onUpdate != nil {
		t.onUpdate(1)
	}
	t.done = true
	if t.onDone != nil {
		t.onDone()
	}
}

// TweenFloat 把 *dst 从 from 补间到 to。
//
// 例：game.TweenFloat(&p.HP, 100, 0, 0.6, game.EaseOutCubic)
func TweenFloat(dst *float32, from, to, dur float32, ease EaseFunc) *Tween {
	tw := NewTween(dur)
	tw.ease = ease
	if dst != nil {
		*dst = from
		tw.onUpdate = func(p float32) { *dst = from + (to-from)*p }
	}
	return tw
}

// TweenVec3 把 *dst 从 from 线性插值到 to。
func TweenVec3(dst *Vec3, from, to Vec3, dur float32, ease EaseFunc) *Tween {
	tw := NewTween(dur)
	tw.ease = ease
	if dst != nil {
		*dst = from
		tw.onUpdate = func(p float32) {
			dst.X = from.X + (to.X-from.X)*p
			dst.Y = from.Y + (to.Y-from.Y)*p
			dst.Z = from.Z + (to.Z-from.Z)*p
		}
	}
	return tw
}

// TweenColor 把 *dst 颜色从 from 插值到 to（含 Alpha）。
func TweenColor(dst *RGBA, from, to RGBA, dur float32, ease EaseFunc) *Tween {
	mix := func(a, b uint8, p float32) uint8 {
		v := float32(a) + (float32(b)-float32(a))*p
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	tw := NewTween(dur)
	tw.ease = ease
	if dst != nil {
		*dst = from
		tw.onUpdate = func(p float32) {
			dst.R = mix(from.R, to.R, p)
			dst.G = mix(from.G, to.G, p)
			dst.B = mix(from.B, to.B, p)
			dst.A = mix(from.A, to.A, p)
		}
	}
	return tw
}

// Timeline 时间轴：按加入顺序串行播放多个补间，
// 适合“先弹出→等待→淡出”这类脚本化演出。
//
//	tl := game.NewTimeline().
//		Add(game.NewTween(0.2).OnUpdate(scaleIn)).
//		Add(game.NewTween(0.5).WithDelay(0.3).OnUpdate(fadeOut))
//	tl.Update(dt)
type Timeline struct {
	items []*Tween
	// Loop 循环整个时间轴（0 = 一次；<0 无限）。
	Loop int

	onDone func()

	cur   int
	loops int
	done  bool
}

// NewTimeline 创建空时间轴。
func NewTimeline() *Timeline { return &Timeline{} }

// Add 追加补间（串行：前一个结束后才开始下一个）并返回自身。
func (tl *Timeline) Add(ts ...*Tween) *Timeline {
	tl.items = append(tl.items, ts...)
	return tl
}

// OnDone 设置全部播放结束回调并返回自身（链式）。
func (tl *Timeline) OnDone(fn func()) *Timeline {
	tl.onDone = fn
	return tl
}

// Len 补间数量。
func (tl *Timeline) Len() int { return len(tl.items) }

// Done 是否全部结束。
func (tl *Timeline) Done() bool { return tl.done }

// Reset 重置到起点（补间一并重置）。
func (tl *Timeline) Reset() {
	tl.resetItems()
	tl.loops, tl.done = 0, false
}

// Update 推进当前补间；结束后自动进入下一个，全部结束触发结束回调。
func (tl *Timeline) Update(dt float32) {
	if tl.done || dt <= 0 || len(tl.items) == 0 {
		return
	}
	for tl.cur < len(tl.items) {
		cur := tl.items[tl.cur]
		if !cur.done {
			cur.Update(dt)
			if !cur.done {
				return
			}
		}
		tl.cur++
		// 子补间边界吸收多余 dt，避免跨项时间穿透造成观感跳变。
		dt = 0
	}
	tl.loops++
	if tl.Loop != 0 && (tl.Loop < 0 || tl.loops < tl.Loop) {
		tl.resetItems()
		return
	}
	tl.done = true
	if tl.onDone != nil {
		tl.onDone()
	}
}

// resetItems 把所有子补间还原并回到起点。
func (tl *Timeline) resetItems() {
	for _, t := range tl.items {
		t.Reset()
	}
	tl.cur = 0
}
