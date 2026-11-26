package game_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"guis/game"
)

// TestPipelineIntegration 端到端串一遍「编辑器 → 导航 → 物理 → 脚本」，
// 验证五项新能力之间能互通（单项行为由各自测试覆盖）。
func TestPipelineIntegration(t *testing.T) {
	eng := game.NewEngine(200, 150)
	if eng.Scene.Root == nil {
		eng.Scene.Root = &game.Node{Name: "root", Visible: true}
	}

	// ---------- ① 编辑器：加节点、拖动、撤销/重做 ----------
	ed := game.NewEditor(eng.Scene)
	box := game.NewMeshNode("crate", game.NewBox(1, 1, 1), game.DefaultMaterial())
	ed.AddNode(box)
	if ed.ParentOf(box) == nil {
		t.Fatalf("AddNode 未挂到场景")
	}
	ed.Select(box)
	if ed.Primary() != box {
		t.Fatalf("选中状态不对")
	}
	ed.BeginTransform()
	ed.TranslateSelected(game.V3(2, 0, 0), true)
	ed.CommitTransform("移动")
	moved := box.Pos
	if math.Abs(float64(moved.X-2)) > 1e-4 {
		t.Fatalf("平移失败: %v", moved)
	}
	if label, ok := ed.Undo(); !ok {
		t.Fatalf("撤销栈为空")
	} else if label == "" {
		t.Fatalf("撤销命令缺少描述")
	}
	if math.Abs(float64(box.Pos.X)) > 1e-4 {
		t.Fatalf("撤销后应回到原点: %v", box.Pos)
	}
	if _, ok := ed.Redo(); !ok {
		t.Fatalf("重做失败")
	}
	if math.Abs(float64(box.Pos.X-2)) > 1e-4 {
		t.Fatalf("重做后应回到 x=2: %v", box.Pos)
	}

	// ---------- ② 导航：高度图建网 + 绕开障碍寻路 ----------
	hm := game.NewHeightmap(8, 8, 1)
	for i := range hm.Heights {
		hm.Heights[i] = 0
	}
	nav, err := game.BuildNavMeshFromHeightmap(hm, game.Identity(), 45)
	if err != nil {
		t.Fatalf("NavMesh 构建失败: %v", err)
	}
	if nav.PolygonCount() != 49 {
		t.Fatalf("7×7 应得 49 个多边形，实际 %d", nav.PolygonCount())
	}
	// 挖一条墙（把若干格从网格里剔除：直接重建并跳过）
	nav2 := game.NewNavMesh()
	for z := 0; z < 7; z++ {
		for x := 0; x < 7; x++ {
			if x == 3 && z != 6 { // 只留最上方一格通行
				continue
			}
			fx, fz := float32(x), float32(z)
			nav2.AddPolygon([]game.Vec3{
				game.V3(fx, 0, fz), game.V3(fx+1, 0, fz),
				game.V3(fx+1, 0, fz+1), game.V3(fx, 0, fz+1),
			})
		}
	}
	nav2.LinkNeighbors()
	start := game.V3(0.5, 0, 0.5)
	goal := game.V3(6.5, 0, 0.5)
	path, ok := nav2.FindPath(start, goal)
	if !ok {
		t.Fatalf("绕行寻路失败")
	}
	if got, direct := nav2.PathLength(path), start.Dist(goal); got <= direct {
		t.Fatalf("应绕路：%.2f vs %.2f", got, direct)
	}
	agent := nav2.NewAgent(start, 6, 0.25)
	if !agent.SetDestination(nav2, goal) {
		t.Fatalf("代理设置目标失败")
	}
	frames := 0
	for !agent.Update(nav2, 1.0/60) && frames < 2000 {
		frames++
	}
	if frames >= 2000 {
		t.Fatalf("代理未到达，位置 %v", agent.Pos)
	}

	// ---------- ③ 物理：地面 + 两箱，落到地面并稳定 ----------
	rw := game.NewRigidWorld(game.V3(0, -9.8, 0))
	rw.SetIterations(12)
	ground := game.NewRigidBody(game.V3(0, 0, 0), 0, game.PlaneShape(game.V3(0, 1, 0)))
	rw.Add(ground)
	b1 := game.NewRigidBody(game.V3(0, 2, 0), 1, game.BoxShape(0.5, 0.5, 0.5))
	b2 := game.NewRigidBody(game.V3(0, 4, 0), 1, game.BoxShape(0.5, 0.5, 0.5))
	rw.Add(b1)
	rw.Add(b2)
	for i := 0; i < 300; i++ {
		rw.Step(1.0 / 60)
	}
	if math.Abs(float64(b1.Pos.Y-0.5)) > 0.15 {
		t.Fatalf("底箱应停在地面上方 0.5，实际 y=%.3f", b1.Pos.Y)
	}
	if b2.Pos.Y <= b1.Pos.Y+0.8 {
		t.Fatalf("上箱应叠在底箱上：b1=%.3f b2=%.3f", b1.Pos.Y, b2.Pos.Y)
	}
	if math.Abs(float64(b1.Vel.Y)) > 1.0 {
		t.Fatalf("底箱应静止，速度 %v", b1.Vel)
	}

	// ---------- ④ 脚本：读实体、按导航路径推进、改相机 ----------
	vm := game.NewScriptVM()
	vm.BindEngine(eng)
	vm.Register("waypoint", func(args []game.ScriptValue) (game.ScriptValue, error) {
		i := 0
		if len(args) > 0 {
			i = int(args[0].AsFloat())
		}
		if i < 0 || i >= len(path) {
			return game.ScriptNilValue(), nil
		}
		p := path[i]
		return game.ScriptObjectValue(map[string]game.ScriptValue{
			"x": game.ScriptNum(float64(p.X)),
			"y": game.ScriptNum(float64(p.Y)),
			"z": game.ScriptNum(float64(p.Z)),
		}), nil
	})
	if err := vm.Exec(`
		let crate = entity_find("crate")
		fn advance(idx) {
			let wp = waypoint(idx)
			if wp == nil { return false }
			entity_set_pos(crate, wp.x, wp.y, wp.z)
			return true
		}
		let steps = 0
		for i in range(32) {
			if advance(i) { steps = steps + 1 } else { break }
		}
		camera_set_pos(0, 8, 12)
		camera_look_at(3, 0, 0)
	`); err != nil {
		t.Fatalf("脚本执行失败: %v", err)
	}
	if v, _ := vm.GetGlobal("steps"); v.AsFloat() != float64(len(path)) {
		t.Fatalf("脚本应逐个走完 %d 个路径点，实际 %v", len(path), v.AsFloat())
	}
	if box.Pos.Dist(path[len(path)-1]) > 1e-3 {
		t.Fatalf("脚本未能移动实体到终点: %v", box.Pos)
	}
	if eng.Scene.Camera.Pos.Y != 8 {
		t.Fatalf("相机未被脚本设置")
	}

	// ---------- ⑤ 热重载：场景文件改动后重新加载 ----------
	dir := t.TempDir()
	scenePath := filepath.Join(dir, "level.json")
	level := `{"nodes":[{"name":"从热重载来","pos":[1,2,3]}]}`
	if err := os.WriteFile(scenePath, []byte(level), 0o644); err != nil {
		t.Fatalf("写临时场景失败: %v", err)
	}
	hr := game.NewHotReloader()
	loaded := 0
	var lastScene *game.Scene
	hr.WatchScene(scenePath, func(string) *game.Mesh { return game.NewBox(1, 1, 1) }, func(sc *game.Scene) {
		loaded++
		lastScene = sc
	})
	if err := hr.Reload(scenePath); err != nil {
		t.Fatalf("热重载失败: %v", err)
	}
	if loaded != 1 || lastScene == nil {
		t.Fatalf("热重载回调未触发")
	}
	if lastScene.Root == nil || len(lastScene.Root.Children) == 0 {
		t.Fatalf("热重载后的场景为空")
	}
	if lastScene.Root.Children[0].Name != "从热重载来" {
		t.Fatalf("节点名不符: %q", lastScene.Root.Children[0].Name)
	}
	if hr.Paths() == nil || len(hr.Paths()) == 0 {
		t.Fatalf("已监视路径为空")
	}
}
