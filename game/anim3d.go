// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// 内置缓动函数。
var (
	// EaseLinear 线性。
	EaseLinear = func(t float32) float32 { return t }
	// EaseInOut 平滑进出。
	EaseInOut = func(t float32) float32 { return SmoothStep(t) }
	// EaseOut 快速启动、缓慢收尾。
	EaseOut = func(t float32) float32 { return 1 - (1-t)*(1-t) }
	// EaseIn 缓慢启动。
	EaseIn = func(t float32) float32 { return t * t }
)

type trackKind uint8

const (
	trackMove trackKind = iota
	trackScale
	trackSpin
)

type track struct {
	kind     trackKind
	node     *Node
	from, to Vec3
	dur      float32
	elapsed  float32
	ease     func(float32) float32
	axis     Vec3
	speed    float32
}

// Animator 简单补间动画器（每帧 Update 即可）。
type Animator struct {
	tracks []track
}

// NewAnimator 创建空动画器。
func NewAnimator() *Animator { return &Animator{} }

// MoveTo 在 dur 秒内把节点位置补间到 to。
func (a *Animator) MoveTo(n *Node, to Vec3, dur float32) {
	if a == nil || n == nil {
		return
	}
	a.tracks = append(a.tracks, track{kind: trackMove, node: n, from: n.Pos, to: to, dur: dur, ease: EaseInOut})
}

// ScaleTo 在 dur 秒内把节点缩放补间到 to。
func (a *Animator) ScaleTo(n *Node, to Vec3, dur float32) {
	if a == nil || n == nil {
		return
	}
	a.tracks = append(a.tracks, track{kind: trackScale, node: n, from: n.Scl, to: to, dur: dur, ease: EaseInOut})
}

// Spin 让节点持续绕轴旋转（radPerSec 弧度/秒）。
func (a *Animator) Spin(n *Node, axis Vec3, radPerSec float32) {
	if a == nil || n == nil {
		return
	}
	a.tracks = append(a.tracks, track{kind: trackSpin, node: n, axis: axis.Normalize(), speed: radPerSec})
}

// Stop 移除某个节点的所有动画。
func (a *Animator) Stop(n *Node) {
	if a == nil || n == nil {
		return
	}
	out := a.tracks[:0]
	for _, t := range a.tracks {
		if t.node != n {
			out = append(out, t)
		}
	}
	a.tracks = out
}

// Clear 清空全部动画。
func (a *Animator) Clear() { a.tracks = a.tracks[:0] }

// Count 当前活动轨道数。
func (a *Animator) Count() int { return len(a.tracks) }

// Update 推进动画（dt 单位秒）。
func (a *Animator) Update(dt float32) {
	if a == nil || len(a.tracks) == 0 {
		return
	}
	out := a.tracks[:0]
	for _, t := range a.tracks {
		switch t.kind {
		case trackSpin:
			t.node.Rot = t.node.Rot.Add(t.axis.Scale(t.speed * dt))
			out = append(out, t)
			continue
		}
		t.elapsed += dt
		p := float32(1)
		if t.dur > 0 {
			p = Clamp(t.elapsed/t.dur, 0, 1)
		}
		if t.ease != nil {
			p = t.ease(p)
		}
		switch t.kind {
		case trackMove:
			t.node.Pos = t.from.Lerp(t.to, p)
		case trackScale:
			t.node.Scl = t.from.Lerp(t.to, p)
		}
		if t.elapsed < t.dur {
			out = append(out, t)
		}
	}
	a.tracks = out
}
