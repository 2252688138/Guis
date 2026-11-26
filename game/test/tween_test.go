// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

// nearF 浮点近似比较（补间测试用）。
func nearF(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func TestEaseEndpoints(t *testing.T) {
	cases := map[string]game.EaseFunc{
		"linear":     game.EaseLinear,
		"in":         game.EaseIn,
		"out":        game.EaseOut,
		"inout":      game.EaseInOut,
		"inQuad":     game.EaseInQuad,
		"outQuad":    game.EaseOutQuad,
		"inoutQuad":  game.EaseInOutQuad,
		"inCubic":    game.EaseInCubic,
		"outCubic":   game.EaseOutCubic,
		"inoutCubic": game.EaseInOutCubic,
		"inSine":     game.EaseInSine,
		"outSine":    game.EaseOutSine,
		"inoutSine":  game.EaseInOutSine,
		"inExpo":     game.EaseInExpo,
		"outExpo":    game.EaseOutExpo,
		"inCirc":     game.EaseInCirc,
		"outCirc":    game.EaseOutCirc,
		"inBack":     game.EaseInBack,
		"outBack":    game.EaseOutBack,
		"outElastic": game.EaseOutElastic,
		"outBounce":  game.EaseOutBounce,
	}
	for name, fn := range cases {
		if got := fn(0); !nearF(got, 0, 1e-3) {
			t.Fatalf("%s(0)=%v 期望 0", name, got)
		}
		if got := fn(1); !nearF(got, 1, 1e-3) {
			t.Fatalf("%s(1)=%v 期望 1", name, got)
		}
	}
}

func TestTweenFloatProgress(t *testing.T) {
	var got float32 = -1
	tw := game.TweenFloat(&got, 0, 100, 1, game.EaseLinear)
	tw.Update(0.25)
	if !nearF(got, 25, 0.01) {
		t.Fatalf("0.25s 后应为 25，实际 %v", got)
	}
	tw.Update(0.5)
	if !nearF(got, 75, 0.01) {
		t.Fatalf("0.75s 后应为 75，实际 %v", got)
	}
	done := false
	tw.OnDone(func() { done = true })
	tw.Update(1)
	if !nearF(got, 100, 0.01) {
		t.Fatalf("结束时应为 100，实际 %v", got)
	}
	if !tw.Done() || !done {
		t.Fatal("补间应完成并触发 OnDone")
	}
	tw.Update(1)
	if !tw.Done() {
		t.Fatal("完成后的补间应保持 Done")
	}
}

func TestTweenDelay(t *testing.T) {
	var got float32
	tw := game.TweenFloat(&got, 0, 10, 1, game.EaseLinear).WithDelay(0.5)
	tw.Update(0.4)
	if got != 0 {
		t.Fatalf("延迟期间不应推进，实际 %v", got)
	}
	tw.Update(0.1)
	if !nearF(got, 0, 1e-5) {
		t.Fatalf("延迟刚好结束时应仍为 0，实际 %v", got)
	}
	tw.Update(0.5)
	if !nearF(got, 5, 0.01) {
		t.Fatalf("延迟后 0.5s 应为 5，实际 %v", got)
	}
}

func TestTweenLoop(t *testing.T) {
	loops := 0
	var last float32
	tw := game.NewTween(0.5).LoopTimes(2)
	tw.OnUpdate(func(p float32) { last = p })
	tw.OnLoop(func(n int) { loops = n })
	tw.Update(0.5)
	if loops != 1 {
		t.Fatalf("第一轮后 loops=%d", loops)
	}
	if tw.Done() {
		t.Fatal("应开始第二轮")
	}
	if !nearF(last, 0, 0.01) && !nearF(last, 1, 0.01) {
		t.Fatalf("轮次边界进度异常: %v", last)
	}
	tw.Update(0.5)
	if loops != 2 || !tw.Done() {
		t.Fatalf("第二轮后 loops=%d done=%v", loops, tw.Done())
	}
}

func TestTweenPingPong(t *testing.T) {
	tw := game.NewTween(1).LoopTimes(2).WithPingPong(true)
	tw.Update(0.999)
	if p := tw.Progress(); p < 0.99 {
		t.Fatalf("正向接近终点，Progress=%v", p)
	}
	tw.Update(0.001) // 完成第一轮，进入反向
	if tw.Done() {
		t.Fatal("乒乓模式应进入反向轮")
	}
	tw.Update(0.25)
	if p := tw.Progress(); !nearF(p, 0.75, 0.02) {
		t.Fatalf("反向 0.25s 时 Progress 应约 0.75，实际 %v", p)
	}
	if tw.Done() {
		t.Fatal("第二轮反向进行中不应完成")
	}
	tw.Update(1)
	if !tw.Done() {
		t.Fatal("两轮乒乓结束后应完成")
	}
}

func TestTweenRestart(t *testing.T) {
	var got float32
	tw := game.TweenFloat(&got, 0, 10, 1, game.EaseLinear)
	tw.Update(1)
	if !tw.Done() {
		t.Fatal("应完成")
	}
	tw.Restart()
	if tw.Done() {
		t.Fatal("Restart 后不应为 Done")
	}
	tw.Update(0.5)
	if !nearF(got, 5, 0.01) {
		t.Fatalf("Restart 后 0.5s 应为 5，实际 %v", got)
	}
}

func TestTweenVec3AndColor(t *testing.T) {
	var v game.Vec3
	tw := game.TweenVec3(&v, game.V3(0, 0, 0), game.V3(2, 4, 6), 1, game.EaseLinear)
	tw.Update(0.5)
	if !v.Equal(game.V3(1, 2, 3)) {
		t.Fatalf("Vec3 补间: %v", v)
	}

	var c game.RGBA
	tc := game.TweenColor(&c, game.Color(0, 0, 0, 0), game.Color(255, 128, 64, 255), 1, game.EaseLinear)
	tc.Update(0.5)
	if c.R != 128 || c.G != 64 || c.B != 32 || c.A != 128 {
		t.Fatalf("Color 补间: %+v", c)
	}
}

func TestTimelineSequence(t *testing.T) {
	var order []string
	tl := game.NewTimeline().
		Add(game.NewTween(0.2).OnDone(func() { order = append(order, "a") })).
		Add(game.NewTween(0.3).OnDone(func() { order = append(order, "b") }))

	tl.Update(0.2)
	if len(order) != 1 || order[0] != "a" {
		t.Fatalf("第一步后 order=%v", order)
	}
	if tl.Done() {
		t.Fatal("时间轴不应完成")
	}
	tl.Update(0.3)
	if len(order) != 2 || order[1] != "b" {
		t.Fatalf("第二步后 order=%v", order)
	}
	if !tl.Done() {
		t.Fatal("时间轴应已完成")
	}
	if tl.Len() != 2 {
		t.Fatalf("Len=%d", tl.Len())
	}
}

func TestTimelineLoopAndReset(t *testing.T) {
	runs := 0
	tl := game.NewTimeline().Add(game.NewTween(0.1).OnDone(func() { runs++ }))
	tl.Loop = 2
	tl.Update(0.1)
	if runs != 1 || tl.Done() {
		t.Fatalf("第一圈后 runs=%d done=%v", runs, tl.Done())
	}
	tl.Update(0.1)
	if runs != 2 || !tl.Done() {
		t.Fatalf("第二圈后 runs=%d done=%v", runs, tl.Done())
	}
	tl.Reset()
	if tl.Done() || runs != 2 {
		t.Fatal("Reset 后应可重新播放")
	}
}
