// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"container/heap"
	"math"
)

// NavMesh 寻路：多边形级 A*（cost 用门户中点距离）+ 漏斗算法平滑拐点。
//
//	finder := game.NewNavMesh()
//	... 构建 ...
//	path, ok := finder.FindPath(from, to)   // 首尾已吸附到网格表面上
//	agent := finder.NewAgent(from, 3, 0.4)  // 半径 0.4、速度 3
//	agent.SetDestination(finder, to)
//	agent.Update(finder, dt)                // 每帧推进，返回是否到达

// NavPathOpts 寻路参数（零值即默认：平滑开启、启发权重 1）。
type NavPathOpts struct {
	// Smooth 是否用漏斗算法把多边形序列压成最少拐点（默认 true）。
	Smooth bool
	// Heuristic 启发权重（>1 更快但可能不最优，默认 1）。
	Heuristic float32
}

// FindPath 求从 start 到 goal 的路径（顶点序列，首尾为吸附到网格上的点）。
func (m *NavMesh) FindPath(start, goal Vec3) ([]Vec3, bool) {
	return m.FindPathOpts(start, goal, NavPathOpts{Smooth: true, Heuristic: 1})
}

// FindPathOpts 带参数寻路。
func (m *NavMesh) FindPathOpts(start, goal Vec3, o NavPathOpts) ([]Vec3, bool) {
	if m == nil || len(m.polys) == 0 {
		return nil, false
	}
	if o.Heuristic <= 0 {
		o.Heuristic = 1
	}
	if !o.Smooth {
		// 零值 NavPathOpts 的 Smooth=false 视为"不平滑"，但 FindPath 显式传 true
	}
	sPos, sIdx, ok := m.ClosestPoint(start)
	if !ok {
		return nil, false
	}
	gPos, gIdx, ok := m.ClosestPoint(goal)
	if !ok {
		return nil, false
	}
	if sIdx == gIdx {
		return []Vec3{sPos, gPos}, true
	}

	// A*
	n := len(m.polys)
	gScore := make([]float32, n)
	came := make([]int32, n)
	closed := make([]bool, n)
	for i := range gScore {
		gScore[i] = math.MaxFloat32
		came[i] = -1
	}
	gScore[sIdx] = 0
	open := &navMeshOpen{}
	heap.Init(open)
	heap.Push(open, navMeshNode{idx: sIdx, f: m.navHeur(sIdx, gPos) * o.Heuristic})
	found := false
	for open.Len() > 0 {
		cur := heap.Pop(open).(navMeshNode)
		if closed[cur.idx] {
			continue
		}
		if cur.idx == gIdx {
			found = true
			break
		}
		closed[cur.idx] = true
		for _, nb := range m.polys[cur.idx].Neighbors {
			if closed[nb] {
				continue
			}
			step := m.navStepCost(cur.idx, nb)
			ng := gScore[cur.idx] + step
			if ng >= gScore[nb] {
				continue
			}
			gScore[nb] = ng
			came[nb] = int32(cur.idx)
			heap.Push(open, navMeshNode{idx: nb, f: ng + m.navHeur(nb, gPos)*o.Heuristic})
		}
	}
	if !found {
		return nil, false
	}

	// 回溯多边形序列 s...g
	var chain []int
	for i := int32(gIdx); i >= 0; i = came[i] {
		chain = append(chain, int(i))
		if int(i) == sIdx {
			break
		}
	}
	for l, r := 0, len(chain)-1; l < r; l, r = l+1, r-1 {
		chain[l], chain[r] = chain[r], chain[l]
	}
	if chain[0] != sIdx {
		return nil, false
	}

	// 同一多边形内直线可达
	if len(chain) == 1 {
		return []Vec3{sPos, gPos}, true
	}

	// 构造门户序列（每项 [left, right]，相对行进方向）
	portals := make([][2]Vec3, 0, len(chain))
	prev := sPos
	for k := 0; k+1 < len(chain); k++ {
		a, b, ok := m.Portal(chain[k], chain[k+1])
		if !ok {
			// 邻接但找不到门户：退化成中心点串
			a, b = m.polys[chain[k]].Center, m.polys[chain[k+1]].Center
		}
		left, right := a, b
		if navTriArea2(prev, a, b) < 0 {
			left, right = b, a
		}
		portals = append(portals, [2]Vec3{left, right})
		prev = a.Add(b).Scale(0.5)
	}
	path := navFunnel(portals, sPos, gPos)
	// 去掉与端点重复的点
	out := path[:0]
	for _, p := range path {
		if len(out) > 0 && out[len(out)-1].Dist(p) < 1e-4 {
			continue
		}
		out = append(out, p)
	}
	if len(out) < 2 {
		return []Vec3{sPos, gPos}, true
	}
	return out, true
}

// navStepCost 进入相邻多边形的代价。
func (m *NavMesh) navStepCost(from, to int) float32 {
	a, b, ok := m.Portal(from, to)
	if !ok {
		return m.polys[from].Center.Dist(m.polys[to].Center)
	}
	mid := a.Add(b).Scale(0.5)
	c := m.polys[to].Cost
	if c <= 0 {
		c = 1
	}
	return m.polys[from].Center.Dist(mid)*(1/c) + mid.Dist(m.polys[to].Center)*(1/c)
}

// navHeur A* 启发：多边形中心到目标点的直线距离。
func (m *NavMesh) navHeur(i int, goal Vec3) float32 {
	return m.polys[i].Center.Dist(goal)
}

// navTriArea2 三角面积（与 Recast 的 triarea2 同号）：
// 用于漏斗算法的左右侧判定。
func navTriArea2(a, b, c Vec3) float32 {
	return (c.X-a.X)*(b.Z-a.Z) - (b.X-a.X)*(c.Z-a.Z)
}

// navFunnel simple stupid funnel：把门户序列压成拐点路径。
func navFunnel(portals [][2]Vec3, start, end Vec3) []Vec3 {
	if len(portals) == 0 {
		return []Vec3{start, end}
	}
	apex := start
	left := portals[0][0]
	right := portals[0][1]
	apexIdx, leftIdx, rightIdx := 0, 0, 0
	pts := []Vec3{start}

	for i := 1; i < len(portals); i++ {
		l := portals[i][0]
		r := portals[i][1]

		// 收紧右侧
		if navTriArea2(apex, right, r) <= 0 {
			if navNearlyEqual(apex, right) || navTriArea2(apex, left, r) > 0 {
				right = r
				rightIdx = i
			} else {
				// 右侧越过左侧：左侧点成为新拐点
				pts = append(pts, left)
				apex = left
				apexIdx = leftIdx
				left, right = apex, apex
				leftIdx, rightIdx = apexIdx, apexIdx
				i = apexIdx
				continue
			}
		}
		// 收紧左侧
		if navTriArea2(apex, left, l) >= 0 {
			if navNearlyEqual(apex, left) || navTriArea2(apex, right, l) < 0 {
				left = l
				leftIdx = i
			} else {
				pts = append(pts, right)
				apex = right
				apexIdx = rightIdx
				left, right = apex, apex
				leftIdx, rightIdx = apexIdx, apexIdx
				i = apexIdx
				continue
			}
		}
		_ = rightIdx
	}
	pts = append(pts, end)
	return pts
}

// navNearlyEqual XZ 平面近似相等。
func navNearlyEqual(a, b Vec3) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-4 && math.Abs(float64(a.Z-b.Z)) < 1e-4
}

// ---------------------------------------------------------------- A* 容器

type navMeshNode struct {
	idx int
	f   float32
}

type navMeshOpen []navMeshNode

func (h navMeshOpen) Len() int            { return len(h) }
func (h navMeshOpen) Less(i, j int) bool  { return h[i].f < h[j].f }
func (h navMeshOpen) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *navMeshOpen) Push(x interface{}) { *h = append(*h, x.(navMeshNode)) }
func (h *navMeshOpen) Pop() interface{} {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}

// ---------------------------------------------------------------- 代理

// NavAgent 在导航网格上移动的简单代理。
type NavAgent struct {
	// Pos 当前位置。
	Pos Vec3
	// Speed 移动速度（单位/秒）。
	Speed float32
	// Radius 到达判定半径。
	Radius float32
	// Path 当前路径（首点为起点）。
	Path []Vec3
	// Cursor 下一个要到达的路径点下标。
	Cursor int
}

// NewAgent 在网格上创建代理（位置会被吸附到网格表面）。
func (m *NavMesh) NewAgent(pos Vec3, speed, radius float32) *NavAgent {
	if speed <= 0 {
		speed = 1
	}
	if radius <= 0 {
		radius = 0.1
	}
	a := &NavAgent{Pos: pos, Speed: speed, Radius: radius}
	if m != nil {
		if q, _, ok := m.ClosestPoint(pos); ok {
			a.Pos = q
		}
	}
	return a
}

// SetDestination 规划到 dst 的路径，返回是否成功。
func (a *NavAgent) SetDestination(m *NavMesh, dst Vec3) bool {
	if a == nil || m == nil {
		return false
	}
	path, ok := m.FindPath(a.Pos, dst)
	if !ok || len(path) < 2 {
		a.Path = nil
		return false
	}
	a.Path = path
	a.Cursor = 1
	return true
}

// Update 沿路径推进 dt 秒，返回是否已到达终点。
func (a *NavAgent) Update(m *NavMesh, dt float32) bool {
	if a == nil {
		return true
	}
	if len(a.Path) == 0 || a.Cursor >= len(a.Path) {
		return true
	}
	remain := a.Speed * dt
	for remain > 0 && a.Cursor < len(a.Path) {
		target := a.Path[a.Cursor]
		d := target.Sub(a.Pos)
		dist := d.Len()
		reach := a.Radius * 0.5
		if reach < 0.02 {
			reach = 0.02
		}
		if dist <= reach {
			a.Pos = target
			a.Cursor++
			continue
		}
		step := remain
		if step >= dist {
			a.Pos = target
			a.Cursor++
			remain -= dist
			continue
		}
		a.Pos = a.Pos.Add(d.Scale(step / dist))
		remain = 0
	}
	if m != nil {
		if q, _, ok := m.ClosestPoint(a.Pos); ok {
			a.Pos = q
		}
	}
	return a.Cursor >= len(a.Path)
}

// Done 是否已到达终点。
func (a *NavAgent) Done() bool {
	return a == nil || len(a.Path) == 0 || a.Cursor >= len(a.Path)
}
