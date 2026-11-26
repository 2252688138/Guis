// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func TestNavStraightPath(t *testing.T) {
	g := game.NewNavGrid(8, 8)
	p := g.FindPath(0, 0, 5, 0)
	if len(p) != 6 {
		t.Fatalf("直线路径长度 = %d，期望 6", len(p))
	}
	if p[0] != (game.Cell{X: 0, Y: 0}) || p[5] != (game.Cell{X: 5, Y: 0}) {
		t.Fatalf("路径端点不正确: %v -> %v", p[0], p[5])
	}
}

func TestNavAroundObstacle(t *testing.T) {
	g := game.NewNavGrid(10, 10)
	// 竖墙（除顶部与底部各留一格）
	for y := 1; y < 9; y++ {
		g.SetBlocked(5, y, true)
	}
	p := g.FindPath(0, 4, 9, 4)
	if len(p) == 0 {
		t.Fatal("应该能绕过障碍")
	}
	for _, c := range p {
		if g.Blocked(c.X, c.Y) {
			t.Fatalf("路径穿过了障碍: %v", c)
		}
	}
	// 必须绕行，长度明显大于直线距离
	if len(p) <= 10 {
		t.Fatalf("绕行路径长度 = %d，应该更长", len(p))
	}
}

func TestNavNoPath(t *testing.T) {
	g := game.NewNavGrid(6, 6)
	for y := 0; y < 6; y++ {
		g.SetBlocked(3, y, true)
	}
	if p := g.FindPath(0, 0, 5, 5); p != nil {
		t.Fatalf("完全隔断时应返回 nil，实际 %d 个格子", len(p))
	}
	if p := g.FindPath(-1, 0, 5, 5); p != nil {
		t.Fatal("越界起点应返回 nil")
	}
	g.SetBlocked(0, 0, true)
	if p := g.FindPath(0, 0, 1, 1); p != nil {
		t.Fatal("阻塞起点应返回 nil")
	}
}

func TestNavNoCornerCut(t *testing.T) {
	g := game.NewNavGrid(3, 3)
	g.SetBlocked(1, 0, true)
	g.SetBlocked(0, 1, true)
	// 不允许穿角：从 (0,0) 到 (1,1) 的斜线两侧都堵住，应该绕出去
	p := g.FindPath(0, 0, 1, 1)
	if len(p) == 2 {
		t.Fatalf("不应允许穿角直达，路径 = %v", p)
	}
	// 允许穿角时可以直接对角一步
	g.AllowCornerCut = true
	p = g.FindPath(0, 0, 1, 1)
	if len(p) != 2 {
		t.Fatalf("允许穿角时应直达，路径长度 = %d", len(p))
	}
}

func TestNavWeight(t *testing.T) {
	g := game.NewNavGrid(5, 3)
	// 中间一行代价高，寻路应绕行
	for x := 0; x < 5; x++ {
		g.SetCost(x, 1, 50)
	}
	p := g.FindPath(0, 1, 4, 1)
	if len(p) == 0 {
		t.Fatal("应该找到路径")
	}
	middle := 0
	for _, c := range p {
		if c.Y == 1 {
			middle++
		}
	}
	if middle > 2 {
		t.Fatalf("高代价格经过 %d 次，应尽量绕行（路径 %v）", middle, p)
	}
}

func TestNavLineOfSightAndSmooth(t *testing.T) {
	g := game.NewNavGrid(10, 10)
	if !g.LineOfSight(game.Cell{X: 0, Y: 0}, game.Cell{X: 5, Y: 5}) {
		t.Fatal("空白区域应有视线")
	}
	g.SetBlocked(2, 2, true)
	if g.LineOfSight(game.Cell{X: 0, Y: 0}, game.Cell{X: 5, Y: 5}) {
		t.Fatal("被遮挡时应无视线")
	}

	g2 := game.NewNavGrid(12, 12)
	for y := 3; y < 9; y++ {
		g2.SetBlocked(6, y, true)
	}
	raw := g2.FindPath(0, 0, 11, 11)
	if len(raw) == 0 {
		t.Fatal("应能绕行")
	}
	sm := game.SmoothPath(g2, raw)
	if len(sm) >= len(raw) {
		t.Fatalf("平滑后应更短: %d -> %d", len(raw), len(sm))
	}
	// 平滑后的相邻两点应有视线，且路径保持连通（相邻格曼哈顿距离 ≤ 1）
	for i := 0; i < len(sm)-1; i++ {
		if !g2.LineOfSight(sm[i], sm[i+1]) {
			t.Fatalf("平滑后 (%v)->(%v) 无视线", sm[i], sm[i+1])
		}
	}
}

func TestNavFromTileLayerAndWorld(t *testing.T) {
	l := &game.TileLayer{Name: "ground", W: 3, H: 2, Data: []int{0, 1, 0, 0, 0, 0}, Visible: true}
	g := game.NavGridFromTileLayer(l, nil)
	if !g.Blocked(1, 0) || g.Blocked(0, 0) {
		t.Fatal("瓦片非 0 应阻塞，0 应可走")
	}
	w := game.CellsToWorld([]game.Cell{{X: 1, Y: 1}}, 16, 16, game.V2(0, 0))
	if len(w) != 1 || w[0].X != 24 || w[0].Y != 24 {
		t.Fatalf("世界坐标 = %v，期望 (24,24)", w)
	}
	// 最近可走格：从一个阻塞格向外找
	c, ok := g.NearestWalkable(1, 0, 5)
	if !ok || g.Blocked(c.X, c.Y) {
		t.Fatalf("NearestWalkable = %v ok=%v", c, ok)
	}
}

func TestStateMachineCallbacks(t *testing.T) {
	m := game.NewStateMachine()
	var order []string
	m.Add("idle",
		func(prev string) { order = append(order, "idle.enter:"+prev) },
		func(dt float32) { order = append(order, "idle.update") },
		func(next string) { order = append(order, "idle.exit:"+next) })
	m.Add("run",
		func(prev string) { order = append(order, "run.enter:"+prev) },
		func(dt float32) { order = append(order, "run.update") },
		func(next string) { order = append(order, "run.exit:"+next) })
	if err := m.Set("idle"); err != nil {
		t.Fatalf("Set(idle) 失败: %v", err)
	}
	if m.Current() != "idle" {
		t.Fatalf("当前状态 = %q", m.Current())
	}
	if err := m.Set("run"); err != nil {
		t.Fatalf("Set(run) 失败: %v", err)
	}
	m.Update(0.016)
	if m.Previous() != "idle" {
		t.Fatalf("上一状态 = %q", m.Previous())
	}
	want := []string{"idle.enter:", "idle.exit:run", "run.enter:idle", "run.update"}
	if len(order) != len(want) {
		t.Fatalf("回调序列 = %v，期望 %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("回调序列 = %v，期望 %v", order, want)
		}
	}
	if err := m.Set("missing"); err == nil {
		t.Fatal("切换到未注册状态应返回错误")
	}
}

func TestStateMachineTransitions(t *testing.T) {
	m := game.NewStateMachine()
	m.Add("idle", nil, nil, nil)
	m.Add("walk", nil, nil, nil)
	m.Add("jump", nil, nil, nil)
	m.Set("idle")

	m.AddTransition("idle", "walk", func() bool { return m.Bool("moving") })
	m.On("jump", "*", "jump")

	m.SetBool("moving", true)
	m.Update(0.016)
	if m.Current() != "walk" {
		t.Fatalf("条件转换后状态 = %q，期望 walk", m.Current())
	}
	// 事件转换支持通配 from
	if !m.Trigger("jump") || m.Current() != "jump" {
		t.Fatalf("事件转换后状态 = %q", m.Current())
	}
	// 未注册事件不改变状态
	if m.Trigger("nope") || m.Current() != "jump" {
		t.Fatalf("未知事件不应转换，当前 = %q", m.Current())
	}
	m.Reset()
	if m.Current() != "" || m.Bool("moving") {
		t.Fatal("Reset 未清空状态与黑板")
	}
}
