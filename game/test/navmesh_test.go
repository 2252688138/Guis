package game_test

import (
	"math"
	"testing"

	"guis/game"
)

// navTestGrid 构建 n×n 的 1×1 网格导航面（可跳过若干格子模拟墙）。
func navTestGrid(n int, skip map[[2]int]bool) *game.NavMesh {
	m := game.NewNavMesh()
	for z := 0; z < n; z++ {
		for x := 0; x < n; x++ {
			if skip[[2]int{x, z}] {
				continue
			}
			fx, fz := float32(x), float32(z)
			m.AddPolygon([]game.Vec3{
				game.V3(fx, 0, fz),
				game.V3(fx+1, 0, fz),
				game.V3(fx+1, 0, fz+1),
				game.V3(fx, 0, fz+1),
			})
		}
	}
	m.LinkNeighbors()
	return m
}

// TestNavMeshStraightPath 开阔网格上斜穿应为近似直线（漏斗生效）。
func TestNavMeshStraightPath(t *testing.T) {
	m := navTestGrid(4, nil)
	if m.PolygonCount() != 16 {
		t.Fatalf("多边形数 %d", m.PolygonCount())
	}
	start := game.V3(0.5, 0, 0.5)
	goal := game.V3(3.5, 0, 3.5)
	path, ok := m.FindPath(start, goal)
	if !ok {
		t.Fatalf("寻路失败")
	}
	if len(path) < 2 {
		t.Fatalf("路径点太少: %d", len(path))
	}
	if path[0].Dist(start) > 0.01 || path[len(path)-1].Dist(goal) > 0.01 {
		t.Fatalf("首尾未对齐: %v -> %v", path[0], path[len(path)-1])
	}
	want := start.Dist(goal)
	got := m.PathLength(path)
	if got > want*1.1 {
		t.Fatalf("应近似直线：长度 %.3f > 欧氏 %.3f×1.1（拐点数 %d）", got, want, len(path)-2)
	}
	for _, p := range path {
		if !m.Contains(p) {
			t.Fatalf("路径点 %v 不在网格内", p)
		}
	}
}

// TestNavMeshDetour 障碍物（挖掉中间一列）时绕行且路径合法。
func TestNavMeshDetour(t *testing.T) {
	skip := map[[2]int]bool{}
	for z := 0; z < 4; z++ {
		if z != 3 { // 只留最上方一格作通道
			skip[[2]int{2, z}] = true
		}
	}
	m := navTestGrid(4, skip)
	start := game.V3(0.5, 0, 0.5)
	goal := game.V3(3.5, 0, 0.5)
	path, ok := m.FindPath(start, goal)
	if !ok {
		t.Fatalf("绕行寻路失败")
	}
	// 必须绕到 z>2 的通道
	maxZ := float32(-1000)
	for _, p := range path {
		if p.Z > maxZ {
			maxZ = p.Z
		}
		if !m.Contains(p) {
			t.Fatalf("路径点 %v 不在网格内（穿墙）", p)
		}
	}
	if maxZ < 3.0 {
		t.Fatalf("应从最上方通道绕过，实际最高 z=%.2f", maxZ)
	}
	if m.PathLength(path) <= start.Dist(goal) {
		t.Fatalf("绕行路径应长于直线")
	}
}

// TestNavMeshClosestPoint 最近点投影精度。
func TestNavMeshClosestPoint(t *testing.T) {
	m := navTestGrid(2, nil)
	p, idx, ok := m.ClosestPoint(game.V3(0.5, 5, 0.5))
	if !ok || idx < 0 {
		t.Fatalf("应在网格上找到投影")
	}
	if math.Abs(float64(p.Y)) > 1e-3 || math.Abs(float64(p.X-0.5)) > 1e-3 || math.Abs(float64(p.Z-0.5)) > 1e-3 {
		t.Fatalf("投影点不对: %v", p)
	}
	q, _, ok := m.ClosestPoint(game.V3(-1, 0, 0.5))
	if !ok {
		t.Fatalf("网格外的点也应给出最近点")
	}
	if math.Abs(float64(q.X)) > 1e-3 {
		t.Fatalf("最近点应贴在边界 x=0 上，实际 %v", q)
	}
	if m.Contains(game.V3(-1, 0, 0.5)) {
		t.Fatalf("网格外点不应判定为包含")
	}
}

// TestNavMeshFromHeightmap 高度图构建：全部凸、法线朝上、能连通寻路。
func TestNavMeshFromHeightmap(t *testing.T) {
	hm := game.NewHeightmap(6, 6, 1)
	for i := range hm.Heights {
		hm.Heights[i] = 0
	}
	m, err := game.BuildNavMeshFromHeightmap(hm, game.Identity(), 45)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if m.PolygonCount() != 25 {
		t.Fatalf("5×5 格应得 25 个多边形，实际 %d", m.PolygonCount())
	}
	for i := 0; i < m.PolygonCount(); i++ {
		p := m.Poly(i)
		if p.Normal.Y < 0.99 {
			t.Fatalf("法线应朝上: %v", p.Normal)
		}
		if len(p.Neighbors) == 0 {
			t.Fatalf("多边形 %d 没有邻居（未连通）", i)
		}
	}
	if _, ok := m.FindPath(game.V3(0.5, 0, 0.5), game.V3(4.5, 0, 4.5)); !ok {
		t.Fatalf("高度图网格应可寻路")
	}

	// 陡坡应被过滤：把一列抬高成悬崖
	hm2 := game.NewHeightmap(4, 4, 1)
	for z := 0; z < 4; z++ {
		for x := 2; x < 4; x++ {
			hm2.Heights[z*4+x] = 10
		}
	}
	m2, err := game.BuildNavMeshFromHeightmap(hm2, game.Identity(), 30)
	if err != nil {
		t.Fatalf("陡坡构建失败: %v", err)
	}
	if m2.PolygonCount() == 0 {
		t.Fatalf("平地区域应被保留")
	}
	if m2.PolygonCount() >= 9 {
		t.Fatalf("陡坡面应被过滤，实际仍保留 %d 个面", m2.PolygonCount())
	}
}

// TestNavMeshAgent 代理沿路径抵达终点。
func TestNavMeshAgent(t *testing.T) {
	m := navTestGrid(4, nil)
	a := m.NewAgent(game.V3(0.5, 0, 0.5), 4, 0.3)
	if !a.SetDestination(m, game.V3(3.5, 0, 3.5)) {
		t.Fatalf("设置目标失败")
	}
	arrived := false
	for i := 0; i < 300; i++ {
		if a.Update(m, 0.05) {
			arrived = true
			break
		}
	}
	if !arrived {
		t.Fatalf("代理未在限定时间内到达，当前位置 %v", a.Pos)
	}
	if a.Pos.Dist(game.V3(3.5, 0, 3.5)) > 0.5 {
		t.Fatalf("终点偏差过大: %v", a.Pos)
	}
}

// TestNavMeshBuildFromMesh 三角网格构建：坡度过滤 + 共面合并。
func TestNavMeshBuildFromMesh(t *testing.T) {
	// 4×4 的平地面（两个三角形一格的网格）
	var idx []uint32
	const n = 5
	for z := 0; z < n-1; z++ {
		for x := 0; x < n-1; x++ {
			a := uint32(z*n + x)
			b := uint32(z*n + x + 1)
			c := uint32((z+1)*n + x + 1)
			d := uint32((z+1)*n + x)
			idx = append(idx, a, b, c, a, c, d)
		}
	}
	pos := make([]game.Vec3, 0, n*n)
	for z := 0; z < n; z++ {
		for x := 0; x < n; x++ {
			pos = append(pos, game.V3(float32(x), 0, float32(z)))
		}
	}
	mesh := game.NewMesh(pos, idx)

	m := game.NewNavMesh()
	if err := m.BuildFromMesh(mesh, game.Identity(), game.NavBuildOpts{MaxSlope: 45}); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if m.PolygonCount() != 32 {
		t.Fatalf("不合并时应得 32 个三角形，实际 %d", m.PolygonCount())
	}

	m2 := game.NewNavMesh()
	if err := m2.BuildFromMesh(mesh, game.Identity(), game.NavBuildOpts{
		MaxSlope: 45, MergeCoplanar: true, MergeMaxVerts: 8,
	}); err != nil {
		t.Fatalf("合并构建失败: %v", err)
	}
	if m2.PolygonCount() >= 32 {
		t.Fatalf("共面合并应减少多边形数，实际 %d", m2.PolygonCount())
	}
	path, ok := m2.FindPath(game.V3(0.2, 0, 0.2), game.V3(3.8, 0, 3.8))
	if !ok {
		t.Fatalf("合并后的网格应可寻路")
	}
	if got, want := m2.PathLength(path), float32(math.Sqrt(2))*3.6; got > want*1.15 {
		t.Fatalf("合并后路径过长: %.3f vs %.3f", got, want)
	}

	// 竖直面应被坡度过滤掉
	wall := game.NewMesh(
		[]game.Vec3{game.V3(0, 0, 0), game.V3(0, 2, 0), game.V3(0, 2, 2), game.V3(0, 0, 2)},
		[]uint32{0, 1, 2, 0, 2, 3},
	)
	m3 := game.NewNavMesh()
	if err := m3.BuildFromMesh(wall, game.Identity(), game.NavBuildOpts{MaxSlope: 45}); err != nil {
		t.Fatalf("竖直面构建不应报错: %v", err)
	}
	if m3.PolygonCount() != 0 {
		t.Fatalf("竖直面应被过滤，实际 %d", m3.PolygonCount())
	}
}

// BenchmarkNavMeshFindPath 1000 多边形网格上的寻路耗时。
func BenchmarkNavMeshFindPath(b *testing.B) {
	m := navTestGrid(32, nil) // 1024 个多边形
	start := game.V3(0.5, 0, 0.5)
	goal := game.V3(31.5, 0, 31.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := m.FindPath(start, goal); !ok {
			b.Fatalf("寻路失败")
		}
	}
}
