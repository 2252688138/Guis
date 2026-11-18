// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"sync"
)

// 组件动画：任意节点都能绑一组动画效果，出场 / 呼吸 / 抖动不再靠手写 Style。
//
// 和 anim.go 里的 Tween / Spring / Keyframes 的区别：那些是「拿一个数」，
// 由业务自己算尺寸或位移再塞进 Style；这里是「绑到节点上」——绘制时给节点压入
// 一层顶点变换（见 gpu.PushAnim），位置（缩放 / 旋转 / 位移）与顶点色 alpha 一起变，
// 因此文字、图标、图片、圆角卡片会作为一个整体动，而不是只有底板在动。
//
//	// 卡片从下方滑入并淡入（一次性，节点首次出现时开始）
//	ui.Card(...).Anim(ui.SlideIn(ui.SideBottom, 24, 0.3), ui.FadeIn(0.3))
//	// 弹出：0.92 倍放大到 1，带回弹
//	ui.Dialog(...).Anim(ui.PopIn(0.35))
//	// 徽标呼吸（循环）
//	ui.Badge("NEW").Anim(ui.Pulse(1.4).Amount(0.08))
//	// 校验失败抖一下：点一次就抖一次
//	if bad { node.Anim(ui.Shake(0.4)).Replay() }
//	// 图标转圈（循环）
//	ui.Icon(...).Anim(ui.Spin(1.2))
//	// 出场：Present 负责「关掉后再留一会儿」，FadeOut 负责这段时间怎么退场
//	if ui.Present("dlg", open, 0.2) {
//		ui.ModalPanel(o).Anim(ui.PopIn(0.2)).AnimIf(!open, ui.PopOut(0.2))
//	}
//
// 一次性效果以「节点首次出现」为起点，标识 = 结构路径（ui 内部每帧给节点算的
// layerKey）+ 该节点的 Key + 效果序号，因此节点树每帧重建也不影响播放进度；
// 循环类效果（Pulse / Float / Spin）一直播放。Replay() 让一次性效果从头再来。
// 系统开启「减弱动态效果」时所有效果直接落到终态（不播放）。

// AnimKind 组件动画类型。
type AnimKind uint8

const (
	// AnimFade 淡入 / 淡出：不透明度 0 → 1（可叠加在其它效果上）。
	AnimFade AnimKind = iota
	// AnimSlide 滑入 / 滑出：从 Side 方向位移 Dist 像素进入。
	AnimSlide
	// AnimScale 缩放：从 Scale 倍缩放回 1（PopIn 用 0.9 左右，ZoomIn 用 1.05 左右）。
	AnimScale
	// AnimShake 抖动：左右快速摆动并衰减（一次性强调）。
	AnimShake
	// AnimPulse 呼吸：缩放 1±Amp 往复（循环）。
	AnimPulse
	// AnimFloat 漂浮：纵向 ±Amp 往复（循环）。
	AnimFloat
	// AnimBounce 弹跳：向上跳几下再落地（一次性）。
	AnimBounce
	// AnimFlipX 横向翻展开：X 方向从 0 拉到 1（像一张卡片被翻开），自带淡入。
	AnimFlipX
	// AnimFlipY 纵向翻展开：Y 方向从 0 拉到 1。
	AnimFlipY
	// AnimRotate 旋转入场：从 Rot 弧度转到 0，自带淡入（RotateIn / RotateOut）。
	AnimRotate
	// AnimSpin 持续旋转：每圈 Dur 秒（循环，适合 Loading / 图标）。
	AnimSpin
	// AnimSwing 悬挂摆动：以节点上边缘为轴心左右摆，幅度衰减（一次性）。
	AnimSwing
	// AnimJello 果冻：横向 / 纵向交替挤压，幅度衰减（一次性强调）。
	AnimJello
	// AnimHeartbeat 心跳：两下脉冲式放大（一次性强调）。
	AnimHeartbeat
	// AnimTada 强调：放大 - 回落 - 微缩 - 归位，带一点摆动（一次性）。
	AnimTada
	// AnimFlash 闪烁：不透明度快速往返两下（一次性提示）。
	AnimFlash
	// AnimDrop 掉落：从上方 Dist 像素落下，默认带回弹（一次性）。
	AnimDrop
	// AnimRubber 橡皮筋：横向拉伸回弹，纵向反向补偿（一次性）。
	AnimRubber
)

// AnimSide 滑入 / 位移方向。
type AnimSide uint8

const (
	SideLeft   AnimSide = iota // 从左侧进入（向右位移）
	SideRight                  // 从右侧进入（向左位移）
	SideTop                    // 从上方进入（向下位移）
	SideBottom                 // 从下方进入（向上位移）
)

// AnimSpec 一条动画效果。用 FadeIn / SlideIn / PopIn / Shake / Pulse / FloatY /
// Bounce / ZoomIn / FlipInX / RotateIn / Spin / Swing / Jello / Heartbeat / Tada /
// Flash / DropIn / RubberBand 以及它们对应的 *Out 构造，再用 DelayBy / With /
// Amount / From / Distance / At 微调。
type AnimSpec struct {
	Kind   AnimKind
	Side   AnimSide
	Dur    float32 // 时长（秒），<= 0 按 0.3
	Delay  float32 // 延迟（秒）
	Ease   Ease    // 缓动，nil 按类型默认值
	Dist   float32 // 滑入 / 掉落距离（逻辑像素）
	Amp    float32 // 抖动 / 呼吸 / 漂浮 / 弹跳 / 摆动 / 果冻幅度
	Scale  float32 // 缩放起点（1 = 无缩放）
	Rot    float32 // 旋转入场角度（弧度）
	PivotX float32 // 轴心横向偏移（相对尺寸，-0.5..0.5，0 = 中心）
	PivotY float32 // 轴心纵向偏移（同上，-0.5 = 上边缘）
	Loop   bool    // true = 循环播放（呼吸 / 漂浮 / 旋转）
	Out    bool    // true = 出场方向（从 1 退到 0）
}

// ---- 构造：入场 ----

// FadeIn 淡入。
func FadeIn(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFade, Dur: dur}
}

// SlideIn 从 side 方向滑入 dist 像素（自带淡入，观感更干净）。
func SlideIn(side AnimSide, dist, dur float32) AnimSpec {
	return AnimSpec{Kind: AnimSlide, Side: side, Dist: dist, Dur: dur}
}

// PopIn 弹出：0.9 倍放大到 1，带回弹，自带淡入。
func PopIn(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimScale, Scale: 0.9, Dur: dur, Ease: EaseOutBack}
}

// ZoomIn 收拢入场：1.06 倍缩回 1，自带淡入（适合弹窗、浮层）。
func ZoomIn(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimScale, Scale: 1.06, Dur: dur}
}

// Shake 左右抖动（一次性强调，默认幅度 6px）。
func Shake(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimShake, Dur: dur, Amp: 6, Ease: EaseLinear}
}

// Pulse 呼吸缩放（循环，默认幅度 0.04）。
func Pulse(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimPulse, Dur: dur, Amp: 0.04, Loop: true, Ease: EaseLinear}
}

// FloatY 上下漂浮（循环，默认幅度 6px）。
func FloatY(dist, dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFloat, Dur: dur, Amp: dist, Loop: true, Ease: EaseLinear}
}

// Bounce 弹跳（一次性，默认幅度 12px）。
func Bounce(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimBounce, Dur: dur, Amp: 12, Ease: EaseLinear}
}

// FlipInX 横向翻展开（X 方向从 0 拉到 1），自带淡入。
func FlipInX(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFlipX, Dur: dur, Ease: EaseOutCubic}
}

// FlipInY 纵向翻展开（Y 方向从 0 拉到 1），自带淡入。
func FlipInY(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFlipY, Dur: dur, Ease: EaseOutCubic}
}

// RotateIn 旋转入场：从左上方向 -20° 转到正位，自带淡入。
func RotateIn(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimRotate, Dur: dur, Rot: -0.35, Ease: EaseOutCubic}
}

// Spin 持续旋转（循环，默认 1.2 秒一圈，顺时针）。
func Spin(dur float32) AnimSpec {
	if dur <= 0 {
		dur = 1.2
	}
	return AnimSpec{Kind: AnimSpin, Dur: dur, Loop: true, Ease: EaseLinear}
}

// Swing 悬挂摆动：以节点上边缘为轴心左右摆，默认幅度 0.3 弧度。
func Swing(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimSwing, Dur: dur, Amp: 0.3, PivotY: -0.5, Ease: EaseLinear}
}

// Jello 果冻：横纵交替挤压（一次性强调，默认幅度 0.12）。
func Jello(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimJello, Dur: dur, Amp: 0.12, Ease: EaseLinear}
}

// Heartbeat 心跳：两下脉冲放大（一次性强调，默认幅度 0.12）。
func Heartbeat(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimHeartbeat, Dur: dur, Amp: 0.12, Ease: EaseLinear}
}

// Tada 强调：放大回落 + 轻微摆动（一次性，默认幅度 0.1）。
func Tada(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimTada, Dur: dur, Amp: 0.1, Ease: EaseLinear}
}

// Flash 闪烁：不透明度往返两次（一次性提示，默认幅度 0.75）。
func Flash(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFlash, Dur: dur, Amp: 0.75, Ease: EaseLinear}
}

// DropIn 从上方 dist 像素落下，带回弹。
func DropIn(dist, dur float32) AnimSpec {
	return AnimSpec{Kind: AnimDrop, Dur: dur, Dist: dist, Ease: EaseOutBounce}
}

// RubberBand 橡皮筋：横向拉伸回弹（一次性强调，默认幅度 0.15）。
func RubberBand(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimRubber, Dur: dur, Amp: 0.15, Ease: EaseLinear}
}

// ---- 构造：出场 ----

// FadeOut 淡出（配合 Present 使用）。
func FadeOut(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFade, Dur: dur, Out: true}
}

// SlideOut 朝 side 方向滑出 dist 像素并淡出。
func SlideOut(side AnimSide, dist, dur float32) AnimSpec {
	return AnimSpec{Kind: AnimSlide, Side: side, Dist: dist, Dur: dur, Out: true}
}

// PopOut 弹出退场：缩到 0.9 并淡出。
func PopOut(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimScale, Scale: 0.9, Dur: dur, Ease: EaseInQuad, Out: true}
}

// ZoomOut 放大到 1.06 并淡出。
func ZoomOut(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimScale, Scale: 1.06, Dur: dur, Ease: EaseInQuad, Out: true}
}

// FlipOutX 横向翻收起（X 方向从 1 收到 0）。
func FlipOutX(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFlipX, Dur: dur, Ease: EaseInQuad, Out: true}
}

// FlipOutY 纵向翻收起（Y 方向从 1 收到 0）。
func FlipOutY(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimFlipY, Dur: dur, Ease: EaseInQuad, Out: true}
}

// RotateOut 转动退场：转到 -20° 并淡出。
func RotateOut(dur float32) AnimSpec {
	return AnimSpec{Kind: AnimRotate, Dur: dur, Rot: -0.35, Ease: EaseInQuad, Out: true}
}

// ---- 微调 ----

// DelayBy 延迟多少秒后开始。
func (s AnimSpec) DelayBy(d float32) AnimSpec { s.Delay = d; return s }

// With 指定缓动函数。
func (s AnimSpec) With(e Ease) AnimSpec { s.Ease = e; return s }

// Amount 设置幅度（抖动 / 呼吸 / 漂浮 / 弹跳 / 摆动 / 果冻 / 心跳 / 强调 / 闪烁）。
func (s AnimSpec) Amount(v float32) AnimSpec { s.Amp = v; return s }

// From 设置滑入方向。
func (s AnimSpec) From(side AnimSide) AnimSpec { s.Side = side; return s }

// Distance 设置滑入 / 掉落距离（逻辑像素）。
func (s AnimSpec) Distance(v float32) AnimSpec { s.Dist = v; return s }

// At 设置缩放起点。
func (s AnimSpec) At(scale float32) AnimSpec { s.Scale = scale; return s }

// Angle 设置旋转入场的起始角度（弧度）。
func (s AnimSpec) Angle(rad float32) AnimSpec { s.Rot = rad; return s }

// Pivot 设置旋转轴心（相对节点尺寸，0,0 = 中心，-0.5 表示对应方向的边缘）。
func (s AnimSpec) Pivot(fx, fy float32) AnimSpec { s.PivotX, s.PivotY = fx, fy; return s }

// LoopOn 把这条效果改成循环播放：强调类效果（Shake / Bounce / Swing / Jello /
// Heartbeat / Tada / Flash / RubberBand）默认只播一次并收幅，加上它就一直往复。
func (s AnimSpec) LoopOn() AnimSpec { s.Loop = true; return s }

// Once 把这条效果改回只播一次（收幅停在终态）。
func (s AnimSpec) Once() AnimSpec { s.Loop = false; return s }

// ---- 绑定到节点 ----

// Anim 给任意节点绑定一组动画效果（可叠加，按顺序复合：透明度相乘、位移相加、
// 缩放相乘、旋转相加）。返回节点本身，便于链式调用。
func (n *Node) Anim(specs ...AnimSpec) *Node {
	if n == nil || len(specs) == 0 {
		return n
	}
	n.anim = append(n.anim, specs...)
	return n
}

// Animate 等价于 node.Anim(...)，函数式写法。
func Animate(n *Node, specs ...AnimSpec) *Node { return n.Anim(specs...) }

// AnimIf 条件为真时才绑定这些效果，常用来给出场动画：
//
//	if !open { node.AnimIf(true, ui.FadeOut(0.2)) }
func (n *Node) AnimIf(cond bool, specs ...AnimSpec) *Node {
	if n == nil || !cond || len(specs) == 0 {
		return n
	}
	return n.Anim(specs...)
}

// NoAnim 清掉节点上已绑定的动画。
func (n *Node) NoAnim() *Node {
	if n == nil {
		return n
	}
	n.anim = nil
	return n
}

// Replay 让该节点的一次性动画（淡入 / 滑入 / 弹出 / 抖动 / 弹跳 / 翻展开 /
// 旋转 / 摆动 / 果冻 / 心跳 / 强调 / 闪烁 / 掉落 / 橡皮筋）从头再播一次。
// 循环类效果（呼吸 / 漂浮 / 旋转）不受影响。
func (n *Node) Replay() *Node {
	if n == nil {
		return n
	}
	n.animReplay = true
	return n
}

// HasAnim 节点是否绑定了动画效果。
func (n *Node) HasAnim() bool { return n != nil && len(n.anim) > 0 }

// ---- 出场保留期 ----

// Present 出场保留期：shown 为 true 时直接返回 true；shown 由 true 变 false 后
// 再返回 true 持续 dur 秒（这段时间业务继续构建该节点并给它挂 *Out 效果），
// 超时后返回 false，业务就可以不再构建它了。
//
//	if ui.Present("dlg", open, 0.2) {
//		ui.ModalPanel(o).Anim(ui.PopIn(0.2)).AnimIf(!open, ui.PopOut(0.2))
//	}
func Present(key string, shown bool, dur float32) bool {
	if dur <= 0 {
		dur = 0.2
	}
	now := AnimTime()
	presenceMu.Lock()
	if shown {
		if presenceStates == nil {
			presenceStates = map[string]*presenceState{}
		}
		st := presenceStates[key]
		if st == nil {
			presenceStates[key] = &presenceState{since: now}
		} else {
			// 重新打开：清掉上一轮的退场计时
			st.since = now
			st.leaving = false
		}
		presenceMu.Unlock()
		return true
	}
	st := presenceStates[key]
	if st == nil {
		presenceMu.Unlock()
		return false
	}
	if !st.leaving {
		st.leaving = true
		st.since = now
	}
	el := now - st.since
	if el < dur {
		presenceMu.Unlock()
		markAnim()
		return true
	}
	delete(presenceStates, key)
	presenceMu.Unlock()
	return false
}

// PresentLeaving 该 key 当前是否处在退场期（已关闭、但还没播完退场动画）。
func PresentLeaving(key string) bool {
	presenceMu.Lock()
	defer presenceMu.Unlock()
	st := presenceStates[key]
	return st != nil && st.leaving
}

type presenceState struct {
	since   float32
	leaving bool
}

var (
	presenceMu     sync.Mutex
	presenceStates map[string]*presenceState
)

// ---- 运行时 ----

// widgetAnimState 一条效果的播放状态（只记开始时刻，进度按帧算）。
type widgetAnimState struct {
	start float32
}

var (
	widgetAnimMu     sync.Mutex
	widgetAnimStates = map[uint64]*widgetAnimState{}
)

// widgetAnimKey 效果的稳定标识：结构路径（layerKey） + 节点 Key + 效果序号。
func widgetAnimKey(n *Node, i int) uint64 {
	k := fnvU64(n.layerKey, uint64(i)+1)
	if n.Key != "" {
		var h uint64 = 0xCBF29CE484222325
		for j := 0; j < len(n.Key); j++ {
			h ^= uint64(n.Key[j])
			h *= 0x100000001B3
		}
		k = fnvU64(k, h)
	}
	return k
}

// animDefaultEase 各类型的默认缓动：入场类用缓出，往复类用线性。
func animDefaultEase(sp AnimSpec) Ease {
	switch sp.Kind {
	case AnimShake, AnimBounce, AnimPulse, AnimFloat, AnimSpin, AnimSwing,
		AnimJello, AnimHeartbeat, AnimTada, AnimFlash, AnimRubber:
		return EaseLinear
	case AnimScale:
		if sp.Scale > 1 {
			return EaseOutCubic
		}
		return EaseOutBack
	}
	return EaseOutCubic
}

// animAmplitudeKind 以「幅度往复」表达的效果：出场时由调用方额外乘上剩余量，
// 因为这类效果自己不改透明度。
func animAmplitudeKind(k AnimKind) bool {
	switch k {
	case AnimShake, AnimBounce, AnimPulse, AnimFloat, AnimSpin, AnimSwing,
		AnimJello, AnimHeartbeat, AnimTada, AnimFlash, AnimRubber:
		return true
	}
	return false
}

// animFrame 一帧的复合动画结果。
// SX/SY 为缩放，Rot 为弧度，DX/DY 为位移，Alpha 为透明度乘子；
// PivotFX/PivotFY 是轴心相对节点尺寸的偏移（0 = 中心，-0.5 = 上/左边缘）。
type animFrame struct {
	DX, DY           float32
	SX, SY           float32
	Rot              float32
	Alpha            float32
	PivotFX, PivotFY float32
	Active           bool
}

// animTransformOf 计算节点本帧的动画变换。
// active 表示仍有效果在播放，调用方据此持续请求重绘。
func animTransformOf(n *Node, now float32) animFrame {
	f := animFrame{SX: 1, SY: 1, Alpha: 1}
	if n == nil || len(n.anim) == 0 {
		return f
	}
	if n.animReplay {
		resetWidgetAnim(n)
		n.animReplay = false
	}
	// 系统要求减弱动态效果：直接落到终态，不产生任何动画
	if ReducedMotion() {
		return animFrame{SX: 1, SY: 1, Alpha: 1}
	}
	for i := range n.anim {
		sp := n.anim[i]
		t, playing := widgetAnimProgress(n, i, sp, now)
		if playing {
			f.Active = true
		}
		e := sp.Ease
		if e == nil {
			e = animDefaultEase(sp)
		}
		et := e(t)
		// v 是「在场程度」：入场 0→1，出场 1→0，两种方向共用下面的算式
		v := et
		if sp.Out {
			v = 1 - et
		}
		if sp.Out && animAmplitudeKind(sp.Kind) {
			f.Alpha *= clamp01(v)
		}
		if sp.PivotX != 0 || sp.PivotY != 0 {
			f.PivotFX, f.PivotFY = sp.PivotX, sp.PivotY
		}
		// decay 是「一次性强调」效果的收幅度系数：播到末尾时摆幅归零，动画自然停住。
		// 循环播放（LoopOn）时不能衰减，否则每一圈末尾都会归零、看着像卡顿。
		decay := float32(1) - t
		if sp.Loop {
			decay = 1
		}
		switch sp.Kind {
		case AnimFade:
			f.Alpha *= clamp01(v)
		case AnimSlide:
			f.Alpha *= clamp01(v)
			off := (1 - v) * sp.Dist
			switch sp.Side {
			case SideLeft:
				f.DX += off
			case SideRight:
				f.DX -= off
			case SideTop:
				f.DY += off
			case SideBottom:
				f.DY -= off
			}
		case AnimScale:
			f.Alpha *= clamp01(v)
			s := sp.Scale + (1-sp.Scale)*v
			if s <= 0 {
				s = 0.01
			}
			f.SX *= s
			f.SY *= s
		case AnimShake:
			// 三个来回，末段幅度按 decay 收掉（循环时保持等幅）
			f.DX += sp.Amp * float32(math.Sin(float64(t)*math.Pi*6)) * decay
		case AnimPulse:
			s := 1 + sp.Amp*float32(math.Sin(float64(t)*2*math.Pi))
			f.SX *= s
			f.SY *= s
		case AnimFloat:
			f.DY += sp.Amp * float32(math.Sin(float64(t)*2*math.Pi))
		case AnimBounce:
			// |sin| 让落点始终回到基准线，再乘 decay 收幅度
			k := float32(math.Abs(math.Sin(float64(t)*math.Pi*3))) * decay
			f.DY -= sp.Amp * k
		case AnimFlipX:
			f.Alpha *= clamp01(v)
			f.SX *= nonZeroScale(v)
		case AnimFlipY:
			f.Alpha *= clamp01(v)
			f.SY *= nonZeroScale(v)
		case AnimRotate:
			f.Alpha *= clamp01(v)
			f.Rot += sp.Rot * (1 - v)
		case AnimSpin:
			f.Rot += 2 * math.Pi * v
		case AnimSwing:
			// 绕上边缘摆动，三次往复后按 decay 收振幅
			f.Rot += sp.Amp * float32(math.Sin(float64(t)*math.Pi*6)) * decay
		case AnimJello:
			// 横纵反相挤压，往复三次
			s := sp.Amp * float32(math.Sin(float64(t)*math.Pi*6)) * decay
			f.SX *= 1 + s
			f.SY *= 1 - s*0.6
		case AnimHeartbeat:
			// 两下鼓包：0~0.28 一次，0.3~0.64 一次
			bump := func(x float32) float32 {
				if x <= 0 || x >= 1 {
					return 0
				}
				return float32(math.Sin(float64(x) * math.Pi))
			}
			s := sp.Amp * (bump(t/0.28) + 0.7*bump((t-0.3)/0.34))
			f.SX *= 1 + s
			f.SY *= 1 + s
		case AnimTada:
			// 1 → 放大 → 回落 → 微缩 → 1，同时轻微摆动
			s := sp.Amp * float32(math.Sin(float64(t)*math.Pi*2))
			if !sp.Loop {
				s *= 1 - 0.35*t
			}
			f.SX *= 1 + s
			f.SY *= 1 + s
			f.Rot += 0.06 * float32(math.Sin(float64(t)*math.Pi*4)) * decay
		case AnimFlash:
			f.Alpha *= 1 - sp.Amp*float32(math.Abs(math.Sin(float64(t)*math.Pi*2)))
		case AnimDrop:
			f.DY -= (1 - v) * sp.Dist
		case AnimRubber:
			s := sp.Amp * float32(math.Sin(float64(t)*math.Pi*3)) * decay
			f.SX *= 1 + s
			f.SY *= 1 - s*0.5
		}
	}
	return f
}

// nonZeroScale 缩放系数不能为 0（矩阵退化会画成一条线），最小留 0.01。
func nonZeroScale(v float32) float32 {
	if v < 0.01 {
		return 0.01
	}
	return v
}

// resetWidgetAnim 清除某节点上所有效果的播放状态（Replay 用）。
func resetWidgetAnim(n *Node) {
	widgetAnimMu.Lock()
	for i := range n.anim {
		delete(widgetAnimStates, widgetAnimKey(n, i))
	}
	widgetAnimMu.Unlock()
}

// widgetAnimProgress 返回某条效果本帧的进度（0..1）以及是否仍在播放。
func widgetAnimProgress(n *Node, i int, sp AnimSpec, now float32) (float32, bool) {
	dur := sp.Dur
	if dur <= 0 {
		dur = 0.3
	}
	key := widgetAnimKey(n, i)
	widgetAnimMu.Lock()
	st := widgetAnimStates[key]
	if st == nil {
		st = &widgetAnimState{start: now}
		widgetAnimStates[key] = st
	}
	start := st.start
	widgetAnimMu.Unlock()

	el := now - start - sp.Delay
	if el <= 0 {
		return 0, true
	}
	if sp.Loop {
		p := el / dur
		return p - float32(math.Floor(float64(p))), true
	}
	if el >= dur {
		return 1, false
	}
	return el / dur, true
}

// ResetWidgetAnims 清空所有组件动画的播放状态与出场保留期（切换页面 / 重置演示时用）。
func ResetWidgetAnims() {
	widgetAnimMu.Lock()
	widgetAnimStates = map[uint64]*widgetAnimState{}
	widgetAnimMu.Unlock()
	presenceMu.Lock()
	presenceStates = nil
	presenceMu.Unlock()
	animOpenMu.Lock()
	animOpenFlags = map[string]bool{}
	animOpenMu.Unlock()
}

// animOnOpen 判断某个浮层是不是「这一帧刚从关到开」，用于重播入场动画
// （动画进度按结构路径缓存，不重播的话第二次打开会直接落到终态）。
func animOnOpen(key string, open bool) bool {
	animOpenMu.Lock()
	defer animOpenMu.Unlock()
	if animOpenFlags == nil {
		animOpenFlags = map[string]bool{}
	}
	prev := animOpenFlags[key]
	animOpenFlags[key] = open
	return open && !prev
}

var (
	animOpenMu    sync.Mutex
	animOpenFlags = map[string]bool{}
)
