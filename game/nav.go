// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "container/heap"

// Cell 网格坐标（x 向右、y 向下，与 Tiled 一致）。
type Cell struct{ X, Y int }

// NavGrid 网格导航图（A* 寻路，4/8 邻接 + 地形权重）。
//
// 默认 8 邻接且禁止穿角（对角线两侧的格子都要可走），
// 代价为每格地形权重（登录 1）。
type NavGrid struct {
	W, H int
	cost []float32 // <0 表示不可通行，否则为移动代价（缺省 1）

	// AllowDiagonal 允许斜向移动（默认 true）。
	AllowDiagonal bool
	// AllowCornerCut 允许穿角（对角线两侧有障碍也允许斜走，默认 false）。
	AllowCornerCut bool
	// MaxNodes 单次寻路的扩展节点上限（默认 200000，防止大图卡死）。
	MaxNodes int
}

// NewNavGrid 创建全部可通行的网格。
func NewNavGrid(w, h int) *NavGrid {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	cost := make([]float32, w*h)
	for i := range cost {
		cost[i] = 1
	}
	return &NavGrid{W: w, H: h, cost: cost, AllowDiagonal: true, MaxNodes: 200000}
}

// InBounds 格子是否在图内。
func (g *NavGrid) InBounds(x, y int) bool {
	return g != nil && x >= 0 && y >= 0 && x < g.W && y < g.H
}

// Blocked 格子是否不可通行（越界视为阻塞）。
func (g *NavGrid) Blocked(x, y int) bool {
	if !g.InBounds(x, y) {
		return true
	}
	return g.cost[y*g.W+x] < 0
}

// SetBlocked 设置阻塞。
func (g *NavGrid) SetBlocked(x, y int, blocked bool) {
	if !g.InBounds(x, y) {
		return
	}
	i := y*g.W + x
	if blocked {
		g.cost[i] = -1
		return
	}
	if g.cost[i] < 0 {
		g.cost[i] = 1
	}
}

// SetCost 设置地形移动代价（c <= 0 视为 1，c < 0 视为阻塞）。
func (g *NavGrid) SetCost(x, y int, c float32) {
	if !g.InBounds(x, y) {
		return
	}
	if c <= 0 {
		c = 1
	}
	g.cost[y*g.W+x] = c
}

// Cost 格子移动代价（阻塞或越界返回 -1）。
func (g *NavGrid) Cost(x, y int) float32 {
	if !g.InBounds(x, y) {
		return -1
	}
	return g.cost[y*g.W+x]
}

// Fill 批量设置一段矩形区域的可通行性（x0,y0 包含，x1,y1 不包含）。
func (g *NavGrid) Fill(x0, y0, x1, y1 int, blocked bool) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			g.SetBlocked(x, y, blocked)
		}
	}
}

type navNode struct {
	idx int
	f   float32
}

type navHeap []navNode

func (h navHeap) Len() int            { return len(h) }
func (h navHeap) Less(i, j int) bool  { return h[i].f < h[j].f }
func (h navHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *navHeap) Push(x interface{}) { *h = append(*h, x.(navNode)) }
func (h *navHeap) Pop() interface{} {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}

// FindPath A* 寻路，返回从起点到终点的格子序列（含两端）。无路返回 nil。
func (g *NavGrid) FindPath(sx, sy, tx, ty int) []Cell {
	if g == nil || g.W <= 0 || g.H <= 0 {
		return nil
	}
	if !g.InBounds(sx, sy) || !g.InBounds(tx, ty) {
		return nil
	}
	if g.Blocked(sx, sy) || g.Blocked(tx, ty) {
		return nil
	}
	if sx == tx && sy == ty {
		return []Cell{{sx, sy}}
	}
	n := g.W * g.H
	gScore := make([]float32, n)
	came := make([]int32, n)
	closed := make([]bool, n)
	for i := range gScore {
		gScore[i] = -1
		came[i] = -1
	}
	start := sy*g.W + sx
	goal := ty*g.W + tx
	gScore[start] = 0

	open := &navHeap{{idx: start, f: g.heur(sx, sy, tx, ty)}}
	heap.Init(open)
	maxNodes := g.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 200000
	}
	expanded := 0

	dirs4 := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	dirs8 := [4][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

	for open.Len() > 0 {
		cur := heap.Pop(open).(navNode)
		if closed[cur.idx] {
			continue
		}
		closed[cur.idx] = true
		expanded++
		if cur.idx == goal {
			return g.rebuild(came, goal)
		}
		if expanded > maxNodes {
			return nil
		}
		cx, cy := cur.idx%g.W, cur.idx/g.W
		for _, d := range dirs4 {
			g.tryRelax(open, gScore, came, closed, cx+d[0], cy+d[1], cur.idx, 1, tx, ty)
		}
		if g.AllowDiagonal {
			for _, d := range dirs8 {
				nx, ny := cx+d[0], cy+d[1]
				if !g.AllowCornerCut {
					if g.Blocked(cx+d[0], cy) || g.Blocked(cx, cy+d[1]) {
						continue
					}
				}
				g.tryRelax(open, gScore, came, closed, nx, ny, cur.idx, 1.41421356, tx, ty)
			}
		}
	}
	return nil
}

func (g *NavGrid) tryRelax(open *navHeap, gScore []float32, came []int32, closed []bool, nx, ny, cur int, step float32, tx, ty int) {
	if g.Blocked(nx, ny) {
		return
	}
	nidx := ny*g.W + nx
	if closed[nidx] {
		return
	}
	tentative := gScore[cur] + float32(step)*g.Cost(nx, ny)
	if gScore[nidx] >= 0 && tentative >= gScore[nidx] {
		return
	}
	gScore[nidx] = tentative
	came[nidx] = int32(cur)
	heap.Push(open, navNode{idx: nidx, f: tentative + g.heur(nx, ny, tx, ty)})
}

func (g *NavGrid) rebuild(came []int32, goal int) []Cell {
	var rev []Cell
	for i := goal; i >= 0; i = int(came[i]) {
		rev = append(rev, Cell{i % g.W, i / g.W})
		if came[i] < 0 {
			break
		}
	}
	out := make([]Cell, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out
}

func (g *NavGrid) heur(x0, y0, x1, y1 int) float32 {
	dx := x0 - x1
	if dx < 0 {
		dx = -dx
	}
	dy := y0 - y1
	if dy < 0 {
		dy = -dy
	}
	if !g.AllowDiagonal {
		return float32(dx + dy)
	}
	// octile 距离
	mn := dx
	if dy < mn {
		mn = dy
	}
	mx := dx
	if dy > mx {
		mx = dy
	}
	return float32(mx-mn) + 1.41421356*float32(mn)
}

// NavGridFromTileLayer 由 TileLayer 生成导航图：blocked(gid) 为 true 的格子不可走。
// black 为 nil 时，gid > 0 的格子视为阻塞（有实际瓦片即墙）。
func NavGridFromTileLayer(l *TileLayer, blocked func(gid int) bool) *NavGrid {
	if l == nil || l.W <= 0 || l.H <= 0 {
		return NewNavGrid(0, 0)
	}
	g := NewNavGrid(l.W, l.H)
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			i := y*l.W + x
			if i >= len(l.Data) {
				continue
			}
			gid := l.Data[i]
			b := gid > 0
			if blocked != nil {
				b = blocked(gid)
			}
			g.SetBlocked(x, y, b)
		}
	}
	return g
}

// NavGridFromTiled 由完整 Tiled 地图的指定图块层生成导航图。
// walkable 为 nil 时，gid == 0 的格子可走；否则以 walkable(gid) 为准。
func NavGridFromTiled(m *TiledMapFull, layer string, walkable func(gid int) bool) *NavGrid {
	l := m.Layer(layer)
	if l == nil || l.Type != "tile" {
		return NewNavGrid(0, 0)
	}
	g := NewNavGrid(l.W, l.H)
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			i := y*l.W + x
			if i >= len(l.Data) {
				continue
			}
			gid := l.Data[i]
			ok := gid == 0
			if walkable != nil {
				ok = walkable(gid)
			}
			g.SetBlocked(x, y, !ok)
		}
	}
	return g
}

// CellsToWorld 把格子路径转为世界坐标（取格子中心）。
func CellsToWorld(cells []Cell, tileW, tileH float32, origin Vec2) []Vec2 {
	if len(cells) == 0 {
		return nil
	}
	out := make([]Vec2, len(cells))
	for i, c := range cells {
		out[i] = V2(origin.X+float32(c.X)*tileW+tileW/2, origin.Y+float32(c.Y)*tileH+tileH/2)
	}
	return out
}

// SmoothPath 用视线检测拉直路径（只保留必要的拐点）。路径不可走时回退原路径。
func SmoothPath(g *NavGrid, cells []Cell) []Cell {
	if g == nil || len(cells) < 3 {
		return cells
	}
	out := make([]Cell, 0, len(cells))
	out = append(out, cells[0])
	i := 0
	for i < len(cells)-1 {
		best := i + 1
		for j := len(cells) - 1; j > i+1; j-- {
			if g.LineOfSight(cells[i], cells[j]) {
				best = j
				break
			}
		}
		out = append(out, cells[best])
		i = best
	}
	return out
}

// LineOfSight 两格之间是否有直线视线（Bresenham 逐格检查可通行）。
func (g *NavGrid) LineOfSight(a, b Cell) bool {
	if g == nil {
		return false
	}
	x0, y0, x1, y1 := a.X, a.Y, b.X, b.Y
	dx := absInt(x1 - x0)
	dy := absInt(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		if g.Blocked(x0, y0) {
			return false
		}
		if x0 == x1 && y0 == y1 {
			return true
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// NearestWalkable 从 (x,y) 向外螺旋搜索最近的可通行格（radius 以内）。
func (g *NavGrid) NearestWalkable(x, y, radius int) (Cell, bool) {
	if g.InBounds(x, y) && !g.Blocked(x, y) {
		return Cell{x, y}, true
	}
	for r := 1; r <= radius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if absInt(dx) != r && absInt(dy) != r {
					continue
				}
				nx, ny := x+dx, y+dy
				if g.InBounds(nx, ny) && !g.Blocked(nx, ny) {
					return Cell{nx, ny}, true
				}
			}
		}
	}
	return Cell{}, false
}
