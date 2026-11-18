// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"testing"
)

// animTup 把一帧的复合结果按「位移 / 缩放 / 透明度」六元组取出，方便测试读单个通道。
func animTup(n *Node, now float32) (dx, dy, sx, sy, alpha float32, active bool) {
	f := animTransformOf(n, now)
	return f.DX, f.DY, f.SX, f.SY, f.Alpha, f.Active
}

// animFakeStart 把某条效果的开始时刻改成 start（用于确定性地测进度）。
func animFakeStart(t *testing.T, n *Node, i int, start float32) {
	t.Helper()
	widgetAnimMu.Lock()
	widgetAnimStates[widgetAnimKey(n, i)] = &widgetAnimState{start: start}
	widgetAnimMu.Unlock()
}

func TestWidgetAnimFadeIn(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果，动画直接落到终态")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	n := Box().Anim(FadeIn(1))
	now := AnimTime()

	if _, _, sx, sy, alpha, active := animTup(n, now); !active || alpha > 0.01 || sx != 1 || sy != 1 {
		t.Fatalf("起始应为「透明 + 不缩放且在播放」，得到 active=%v alpha=%v scale=%v/%v", active, alpha, sx, sy)
	}

	animFakeStart(t, n, 0, now-0.5)
	if _, _, _, _, alpha, active := animTup(n, now); !active || alpha < 0.7 || alpha > 0.99 {
		t.Fatalf("半程不透明度应接近 1（EaseOutCubic），得到 active=%v alpha=%v", active, alpha)
	}

	animFakeStart(t, n, 0, now-5)
	if _, _, sx, sy, alpha, active := animTup(n, now); active || alpha != 1 || sx != 1 || sy != 1 {
		t.Fatalf("播完应回到终态，得到 active=%v alpha=%v scale=%v/%v", active, alpha, sx, sy)
	}
}

func TestWidgetAnimSlideIn(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()
	n := Box().Anim(SlideIn(SideBottom, 24, 0.5))

	dx, dy, _, _, _, active := animTup(n, now)
	if !active || dx != 0 || dy != -24 {
		t.Fatalf("向上滑入的起始位移应为 (0,-24)，得到 (%v,%v) active=%v", dx, dy, active)
	}

	// 结束时应完全回到原位
	animFakeStart(t, n, 0, now-5)
	dx, dy, _, _, _, active = animTup(n, now)
	if active || dx != 0 || dy != 0 {
		t.Fatalf("播完位移应归零，得到 (%v,%v) active=%v", dx, dy, active)
	}

	// 方向：从左侧进入 → 起始向右偏移
	m := Box().K("left-slide").Anim(SlideIn(SideLeft, 30, 0.4))
	if dx, _, _, _, _, _ := animTup(m, now); dx != 30 {
		t.Fatalf("左侧滑入的起始位移应为 +30，得到 %v", dx)
	}
}

func TestWidgetAnimPulseLoops(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()
	n := Box().Anim(Pulse(1).Amount(0.2))
	// t = 0.25 时 sin(π/2) = 1 → 缩放 1.2
	animFakeStart(t, n, 0, now-0.25)
	_, _, sx, sy, _, active := animTup(n, now)
	if !active {
		t.Fatal("循环动画应始终在播放")
	}
	if sx < 1.19 || sx > 1.21 || sy != sx {
		t.Fatalf("呼吸到峰值时缩放应为 1.2，得到 %v/%v", sx, sy)
	}
}

func TestWidgetAnimReplay(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()
	n := Box().Anim(FadeIn(1)).K("replay")
	animFakeStart(t, n, 0, now-5)
	if _, _, _, _, alpha, _ := animTup(n, now); alpha != 1 {
		t.Fatalf("先要播完（alpha 应为 1），得到 %v", alpha)
	}
	n.Replay()
	if _, _, _, _, alpha, active := animTup(n, now); !active || alpha > 0.01 {
		t.Fatalf("Replay 之后应从头播放（alpha≈0），得到 active=%v alpha=%v", active, alpha)
	}
}

func TestWidgetAnimShakeDecays(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()
	n := Box().Anim(Shake(0.5).Amount(10))
	animFakeStart(t, n, 0, now-5)
	if dx, dy, _, _, _, active := animTup(n, now); active || dx != 0 || dy != 0 {
		t.Fatalf("抖动播完应归零，得到 (%v,%v) active=%v", dx, dy, active)
	}
}

// 带动画的节点不能进组件栅格缓存：否则会贴回一张动画中途的快照。
func TestWidgetAnimExcludedFromCache(t *testing.T) {
	if markStaticTree(Box().Anim(FadeIn(0.3))) {
		t.Fatal("绑定了动画的节点不应被判定为纯静态")
	}
	if !markStaticTree(Box()) {
		t.Fatal("普通节点应仍是静态的")
	}
}

// 空节点 / 无动画节点不产生任何变换。
func TestWidgetAnimEmpty(t *testing.T) {
	now := AnimTime()
	if _, _, sx, sy, alpha, active := animTup(nil, now); active || sx != 1 || sy != 1 || alpha != 1 {
		t.Fatal("nil 节点不应产生动画")
	}
	if _, _, sx, sy, alpha, active := animTup(Box(), now); active || sx != 1 || sy != 1 || alpha != 1 {
		t.Fatal("未绑定效果的节点不应产生动画")
	}
}

// 翻展开 / 掉落 / 旋转 / 摆动：起始与终态要落在预期通道上。
func TestWidgetAnimTransformKinds(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果，动画直接落到终态")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()

	// FlipInX：起始 X 向缩到最小（不能是 0，矩阵会退化），播完回到 1
	fx := Box().K("flipx").Anim(FlipInX(0.4))
	if f := animTransformOf(fx, now); !f.Active || f.SX > 0.02 || f.SY != 1 {
		t.Fatalf("FlipInX 起始应只在 X 方向收拢，得到 sx=%v sy=%v active=%v", f.SX, f.SY, f.Active)
	}
	animFakeStart(t, fx, 0, now-5)
	if f := animTransformOf(fx, now); f.Active || f.SX != 1 || f.SY != 1 || f.Alpha != 1 {
		t.Fatalf("FlipInX 播完应回到终态，得到 sx=%v sy=%v alpha=%v", f.SX, f.SY, f.Alpha)
	}

	// DropIn：起始在基准线上方 Dist 像素，播完落回原位
	dp := Box().K("dropin").Anim(DropIn(20, 0.4))
	if f := animTransformOf(dp, now); f.DY != -20 || !f.Active {
		t.Fatalf("DropIn 起始位移应为 -20，得到 %v", f.DY)
	}
	animFakeStart(t, dp, 0, now-5)
	if f := animTransformOf(dp, now); math.Abs(float64(f.DY)) > 0.01 || f.Active {
		t.Fatalf("DropIn 播完应落回 0，得到 %v", f.DY)
	}

	// Spin：1 秒一圈，半程应转过 π，且一直算「在播放」（循环）
	sp := Box().K("spin").Anim(Spin(1))
	animFakeStart(t, sp, 0, now-0.5)
	if f := animTransformOf(sp, now); math.Abs(float64(f.Rot-math.Pi)) > 0.01 || !f.Active {
		t.Fatalf("Spin 半程旋转应为 π，得到 %v active=%v", f.Rot, f.Active)
	}

	// Swing：轴心在上边缘（PivotFY = -0.5），半程应转离正位
	sw := Box().K("swing").Anim(Swing(0.6))
	if f := animTransformOf(sw, now); f.PivotFY != -0.5 {
		t.Fatalf("Swing 轴心应在下边缘之上（-0.5），得到 %v", f.PivotFY)
	}
	animFakeStart(t, sw, 0, now-0.15)
	if f := animTransformOf(sw, now); f.Rot == 0 {
		t.Fatal("Swing 半程应有旋转角")
	}
}

// 强调类效果（果冻 / 心跳 / 强调 / 橡皮筋）只改缩放与旋转，不动透明度；闪烁改透明度。
func TestWidgetAnimEmphasisKinds(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()

	cases := []struct {
		name string
		sp   AnimSpec
	}{
		{"Jello", Jello(0.4)},
		{"Heartbeat", Heartbeat(0.4)},
		{"Tada", Tada(0.4)},
		{"RubberBand", RubberBand(0.4)},
	}
	for _, c := range cases {
		n := Box().K(c.name).Anim(c.sp)
		animFakeStart(t, n, 0, now-0.1)
		f := animTransformOf(n, now)
		if !f.Active || f.Alpha != 1 {
			t.Fatalf("%s 中途应在播放且不改透明度，得到 active=%v alpha=%v", c.name, f.Active, f.Alpha)
		}
		if f.SX == 1 && f.SY == 1 {
			t.Fatalf("%s 中途应改变缩放，得到 %v/%v", c.name, f.SX, f.SY)
		}
	}

	fl := Box().K("flash").Anim(Flash(0.4))
	animFakeStart(t, fl, 0, now-0.1)
	if f := animTransformOf(fl, now); f.Alpha >= 1 {
		t.Fatalf("Flash 中途应压暗透明度，得到 %v", f.Alpha)
	}
}

// 出场效果从终态 1 退到 0。
func TestWidgetAnimOutKinds(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()

	fo := Box().K("fadeout").Anim(FadeOut(0.4))
	if f := animTransformOf(fo, now); f.Alpha != 1 || !f.Active {
		t.Fatalf("FadeOut 起始应完全不透明，得到 alpha=%v active=%v", f.Alpha, f.Active)
	}
	animFakeStart(t, fo, 0, now-5)
	if f := animTransformOf(fo, now); f.Alpha != 0 || f.Active {
		t.Fatalf("FadeOut 播完不透明度应为 0，得到 alpha=%v active=%v", f.Alpha, f.Active)
	}

	so := Box().K("slideout").Anim(SlideOut(SideRight, 30, 0.4))
	animFakeStart(t, so, 0, now-5)
	if f := animTransformOf(so, now); f.DX != -30 || f.Alpha != 0 {
		t.Fatalf("SlideOut 播完应移出 -30 并变透明，得到 dx=%v alpha=%v", f.DX, f.Alpha)
	}

	po := Box().K("popout").Anim(PopOut(0.4))
	animFakeStart(t, po, 0, now-5)
	if f := animTransformOf(po, now); math.Abs(float64(f.SX-0.9)) > 0.001 || f.Alpha != 0 {
		t.Fatalf("PopOut 播完应缩到 0.9 且透明，得到 sx=%v alpha=%v", f.SX, f.Alpha)
	}
}

// 循环播放的强调类效果不能在每圈末尾收幅归零，否则看着像卡顿 / 没动画。
func TestWidgetAnimLoopOnKeepsAmplitude(t *testing.T) {
	if ReducedMotion() {
		t.Skip("系统开启了减弱动态效果")
	}
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	now := AnimTime()

	// 取 t≈0.98（一趟快结束）对比：LoopOn 仍保持振幅，一次性版本已收幅到接近 1
	loop := Box().K("jello-loop").Anim(Jello(0.6).LoopOn())
	animFakeStart(t, loop, 0, now-0.6*0.98)
	lf := animTransformOf(loop, now)
	if !lf.Active {
		t.Fatal("LoopOn 的效果应一直算「在播放」")
	}
	if math.Abs(float64(lf.SX-1)) < 0.02 {
		t.Fatalf("循环播放到末尾不应收幅，得到 sx=%v", lf.SX)
	}

	once := Box().K("jello-once").Anim(Jello(0.6))
	animFakeStart(t, once, 0, now-0.6*0.98)
	of := animTransformOf(once, now)
	if math.Abs(float64(of.SX-1)) > 0.005 {
		t.Fatalf("一次性效果到末尾应收幅归位，得到 sx=%v", of.SX)
	}
}

// Present：关掉后再保留 dur 秒，超时才放手；从未显示过的 key 直接 false。
func TestPresentExitWindow(t *testing.T) {
	ResetWidgetAnims()
	defer ResetWidgetAnims()
	if !Present("dlg", true, 0.2) {
		t.Fatal("显示中应返回 true")
	}
	if PresentLeaving("dlg") {
		t.Fatal("显示中不应处于退场期")
	}
	if !Present("dlg", false, 0.2) {
		t.Fatal("刚关闭时应保留")
	}
	if !PresentLeaving("dlg") {
		t.Fatal("关闭后应处于退场期")
	}
	// 把退场起点往前拨，模拟时间流逝
	presenceMu.Lock()
	presenceStates["dlg"].since -= 1
	presenceMu.Unlock()
	if Present("dlg", false, 0.2) {
		t.Fatal("超过退场时长后应返回 false")
	}
	if Present("never-shown", false, 0.2) {
		t.Fatal("从未显示的 key 不应保留")
	}
	// 重新打开要清掉上一轮的退场计时
	if !Present("dlg", true, 0.2) || PresentLeaving("dlg") {
		t.Fatal("重新打开后应立即回到非退场状态")
	}
}
