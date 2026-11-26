// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// World 游戏世界（Engine 的语义别名，便于业务代码按“世界”理解）。
type World = Engine

// Game 游戏逻辑接口（由业务实现）。
type Game interface {
	// Init 引擎首次 Tick 时调用。
	Init(w *World)
	// Update 每帧调用一次（dt 为秒）。
	Update(dt float32)
	// FixedUpdate 按固定步长调用（物理/确定性逻辑放这里）。
	FixedUpdate(step float32)
	// Draw 每帧在渲染前调用，允许动态调整场景。
	Draw(sc *Scene)
}

// Loop 固定步长循环驱动器。
type Loop struct {
	// FixedStep 固定步长（秒，默认 1/60）。
	FixedStep float32
	// MaxCatchUp 单帧最多追赶的步数（默认 8，防止卡顿后“死亡螺旋”）。
	MaxCatchUp int
	// Game 业务逻辑（可空）。
	Game Game

	accum  float32
	inited bool
	paused bool
	steps  int
}

// NewLoop 创建循环驱动器。
func NewLoop(g Game) *Loop {
	return &Loop{FixedStep: 1.0 / 60.0, MaxCatchUp: 8, Game: g}
}

// Paused 暂停/恢复（暂停时仍调用 Update 与 Draw，但不跑 FixedUpdate）。
func (l *Loop) Paused(v bool) { l.paused = v }

// IsPaused 是否暂停。
func (l *Loop) IsPaused() bool { return l.paused }

// Steps 上一帧执行的固定步数。
func (l *Loop) Steps() int { return l.steps }

// Step 推进一帧；返回本帧执行的固定更新次数。
func (l *Loop) Step(dt float32, w *World) int {
	if l == nil {
		return 0
	}
	if l.FixedStep <= 0 {
		l.FixedStep = 1.0 / 60.0
	}
	if l.MaxCatchUp <= 0 {
		l.MaxCatchUp = 8
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.25 {
		dt = 0.25 // 长卡顿时截断，避免一次补上百帧
	}
	if l.Game != nil {
		if !l.inited {
			l.Game.Init(w)
			l.inited = true
		}
		l.Game.Update(dt)
	}
	l.steps = 0
	if l.paused {
		return 0
	}
	l.accum += dt
	n := 0
	for l.accum >= l.FixedStep-1e-6 && n < l.MaxCatchUp {
		if l.Game != nil {
			l.Game.FixedUpdate(l.FixedStep)
		}
		l.accum -= l.FixedStep
		n++
	}
	if n == l.MaxCatchUp && l.accum >= l.FixedStep-1e-6 {
		l.accum = 0 // 追不上就丢弃余量
	}
	l.steps = n
	return n
}

// Reset 重置累加器与初始化标记。
func (l *Loop) Reset() {
	l.accum = 0
	l.inited = false
	l.steps = 0
}
