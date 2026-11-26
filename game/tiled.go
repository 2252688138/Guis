// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
)

// TileLayer 图块图层。
type TileLayer struct {
	Name    string
	W, H    int
	Data    []int // 图块 GID（0 = 空）
	Visible bool
	Offset  Vec2
}

// TiledMap Tiled 正交地图（JSON 子集：图块层 + 首个图块集）。
type TiledMap struct {
	W, H         int
	TileW, TileH int
	Layers       []TileLayer
	// Atlas 图块集图集（缺图时为 nil，此时不绘制）。
	Atlas *Atlas
	// FirstGID 首个图块集起始 GID。
	FirstGID int
}

// tiledFile Tiled JSON 结构子集。
type tiledFile struct {
	Width      int `json:"width"`
	Height     int `json:"height"`
	TileWidth  int `json:"tilewidth"`
	TileHeight int `json:"tileheight"`
	Layers     []struct {
		Type    string  `json:"type"`
		Name    string  `json:"name"`
		Width   int     `json:"width"`
		Height  int     `json:"height"`
		Data    []int   `json:"data"`
		Visible *bool   `json:"visible"`
		OffsetX float64 `json:"offsetx"`
		OffsetY float64 `json:"offsety"`
	} `json:"layers"`
	Tilesets []struct {
		FirstGID   int    `json:"firstgid"`
		Image      string `json:"image"`
		ImageWidth int    `json:"imagewidth"`
		TileWidth  int    `json:"tilewidth"`
		TileHeight int    `json:"tileheight"`
		Columns    int    `json:"columns"`
	} `json:"tilesets"`
}

// LoadTiledJSON 读取 Tiled 导出的 .tmj/.json（正交地图）。
//
// 图块集图片路径相对于 JSON 所在目录；图片缺失不报错，只置 Atlas = nil。
func LoadTiledJSON(path string) (*TiledMap, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadTiledJSONBytes(b, filepath.Dir(path))
}

// LoadTiledJSONBytes 从内存解析 Tiled JSON，baseDir 用于定位图块集图片。
func LoadTiledJSONBytes(b []byte, baseDir string) (*TiledMap, error) {
	var f tiledFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	m := &TiledMap{W: f.Width, H: f.Height, TileW: f.TileWidth, TileH: f.TileHeight}
	if m.TileW <= 0 {
		m.TileW = 16
	}
	if m.TileH <= 0 {
		m.TileH = 16
	}
	for _, l := range f.Layers {
		if l.Type != "tilelayer" {
			continue
		}
		tl := TileLayer{Name: l.Name, W: l.Width, H: l.Height, Data: l.Data, Visible: true}
		if tl.W <= 0 {
			tl.W = m.W
		}
		if tl.H <= 0 {
			tl.H = m.H
		}
		if l.Visible != nil {
			tl.Visible = *l.Visible
		}
		tl.Offset = Vec2{X: float32(l.OffsetX), Y: float32(l.OffsetY)}
		m.Layers = append(m.Layers, tl)
	}
	if len(f.Tilesets) > 0 {
		ts := f.Tilesets[0]
		m.FirstGID = ts.FirstGID
		if ts.Image != "" {
			p := ts.Image
			if baseDir != "" && !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			if tex, err := LoadTexture(p); err == nil {
				cols := ts.Columns
				if cols <= 0 && ts.TileWidth > 0 {
					cols = tex.W / ts.TileWidth
				}
				cw, ch := ts.TileWidth, ts.TileHeight
				if cw <= 0 {
					cw = m.TileW
				}
				if ch <= 0 {
					ch = m.TileH
				}
				if cols > 0 {
					m.Atlas = NewAtlas(tex, cw, ch, cols)
				}
			}
		}
	}
	return m, nil
}

// Layer 按名字取图层。
func (m *TiledMap) Layer(name string) *TileLayer {
	if m == nil {
		return nil
	}
	for i := range m.Layers {
		if m.Layers[i].Name == name {
			return &m.Layers[i]
		}
	}
	return nil
}

// Tile 取图层某格的 GID（越界返回 0）。
func (m *TiledMap) Tile(layer string, tx, ty int) int {
	l := m.Layer(layer)
	if l == nil || tx < 0 || ty < 0 || tx >= l.W || ty >= l.H {
		return 0
	}
	i := ty*l.W + tx
	if i >= len(l.Data) {
		return 0
	}
	return l.Data[i]
}

// DrawSprites 实现 Sprite2D：按图层顺序画可见图块（自动剔除屏外）。
func (m *TiledMap) DrawSprites(dst *image.RGBA, w, h int) {
	if m == nil || m.Atlas == nil || dst == nil {
		return
	}
	for _, l := range m.Layers {
		if !l.Visible {
			continue
		}
		for ty := 0; ty < l.H; ty++ {
			y := float32(ty*m.TileH) + l.Offset.Y
			if y > float32(h) || y+float32(m.TileH) < 0 {
				continue
			}
			for tx := 0; tx < l.W; tx++ {
				i := ty*l.W + tx
				if i >= len(l.Data) {
					break
				}
				gid := l.Data[i]
				if gid <= 0 {
					continue
				}
				x := float32(tx*m.TileW) + l.Offset.X
				if x > float32(w) || x+float32(m.TileW) < 0 {
					continue
				}
				src, ok := m.Atlas.Cell(gid - m.FirstGID)
				if !ok {
					continue
				}
				DrawSprite(dst, Sprite{
					Pos:     Vec2{X: x, Y: y},
					Size:    Vec2{X: float32(m.TileW), Y: float32(m.TileH)},
					Texture: m.Atlas.Tex,
					Src:     src,
				})
			}
		}
	}
}

// CollisionGrid 网格碰撞体（二维，世界坐标为 (x, z) 平面，格子边长 Cell）。
type CollisionGrid struct {
	W, H int
	Cell float32
	// Origin 格子 (0,0) 在世界坐标中的左上角。
	Origin Vec3
	// CellHeight 实体格的高度（≤ 0 表示无限高，默认 0）。
	CellHeight float32
	solid      []bool
}

// NewCollisionGrid 创建 w×h 网格（Cell ≤ 0 时按 1）。
func NewCollisionGrid(w, h int, cell float32) *CollisionGrid {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if cell <= 0 {
		cell = 1
	}
	return &CollisionGrid{W: w, H: h, Cell: cell, solid: make([]bool, w*h)}
}

// SetSolid 设置格子是否为实体。
func (g *CollisionGrid) SetSolid(tx, ty int, v bool) {
	if g == nil || tx < 0 || ty < 0 || tx >= g.W || ty >= g.H {
		return
	}
	g.solid[ty*g.W+tx] = v
}

// Solid 查询格子是否实体（越界视为实体，防止跑出地图）。
func (g *CollisionGrid) Solid(tx, ty int) bool {
	if g == nil {
		return true
	}
	if tx < 0 || ty < 0 || tx >= g.W || ty >= g.H {
		return true
	}
	return g.solid[ty*g.W+tx]
}

// SolidAt 世界坐标是否落在实体格。
func (g *CollisionGrid) SolidAt(x, z float32) bool {
	if g == nil {
		return true
	}
	tx := int((x - g.Origin.X) / g.Cell)
	tz := int((z - g.Origin.Z) / g.Cell)
	return g.Solid(tx, tz)
}

// Bounds 整个世界包围盒（y 方向 [0,1]）。
func (g *CollisionGrid) Bounds() AABB {
	if g == nil {
		return AABB{}
	}
	b := AABB{
		Min: Vec3{X: g.Origin.X, Y: g.Origin.Y, Z: g.Origin.Z},
		Max: Vec3{X: g.Origin.X + float32(g.W)*g.Cell, Y: g.Origin.Y + 1, Z: g.Origin.Z + float32(g.H)*g.Cell},
	}
	return b
}

// OverlapAABB 盒体是否与任意实体格相交（只考虑 x/z 平面）。
func (g *CollisionGrid) OverlapAABB(b AABB) bool {
	if g == nil {
		return true
	}
	x0 := int((b.Min.X - g.Origin.X) / g.Cell)
	x1 := int((b.Max.X - g.Origin.X) / g.Cell)
	z0 := int((b.Min.Z - g.Origin.Z) / g.Cell)
	z1 := int((b.Max.Z - g.Origin.Z) / g.Cell)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if z1 < z0 {
		z0, z1 = z1, z0
	}
	for tz := z0; tz <= z1; tz++ {
		for tx := x0; tx <= x1; tx++ {
			if g.Solid(tx, tz) {
				return true
			}
		}
	}
	return false
}

// OverlapBox 盒体是否与任意实体格相交（考虑 CellHeight：盒体高于格子顶部则不阻挡）。
func (g *CollisionGrid) OverlapBox(b AABB) bool {
	if g == nil {
		return true
	}
	if g.CellHeight > 0 {
		if b.Min.Y >= g.Origin.Y+g.CellHeight || b.Max.Y <= g.Origin.Y {
			return false
		}
	}
	return g.OverlapAABB(b)
}

// CollisionGrid 从地图生成碰撞网格：非 0 图块视为实体（layer 为空取第一个可见图层）。
func (m *TiledMap) CollisionGrid(layer string) *CollisionGrid {
	if m == nil {
		return NewCollisionGrid(0, 0, 1)
	}
	l := m.Layer(layer)
	if l == nil && len(m.Layers) > 0 {
		l = &m.Layers[0]
	}
	g := NewCollisionGrid(m.W, m.H, float32(m.TileW))
	if l == nil {
		return g
	}
	for ty := 0; ty < m.H; ty++ {
		for tx := 0; tx < m.W; tx++ {
			i := ty*l.W + tx
			if i < len(l.Data) && l.Data[i] > 0 {
				g.SetSolid(tx, ty, true)
			}
		}
	}
	return g
}

// FindPath 网格 A* 寻路（四向 + 对角，8 向不允许穿墙角），返回格子路径（含起点与终点）。
func (g *CollisionGrid) FindPath(fromX, fromY, toX, toY int) [][2]int {
	if g == nil || g.Solid(toX, toY) || g.Solid(fromX, fromY) {
		return nil
	}
	n := g.W * g.H
	if n <= 0 {
		return nil
	}
	start := fromY*g.W + fromX
	goal := toY*g.W + toX
	if start == goal {
		return [][2]int{{fromX, fromY}}
	}
	gScore := make([]float32, n)
	came := make([]int, n)
	closed := make([]bool, n)
	for i := range gScore {
		gScore[i] = -1
		came[i] = -1
	}
	h := func(i int) float32 {
		dx := float32(i%g.W - toX)
		dy := float32(i/g.W - toY)
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		return dx + dy
	}
	gScore[start] = 0
	open := []int{start}
	fScore := map[int]float32{start: h(start)}
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for len(open) > 0 {
		// 取 f 最小项
		best := 0
		for i := 1; i < len(open); i++ {
			if fScore[open[i]] < fScore[open[best]] {
				best = i
			}
		}
		cur := open[best]
		open = append(open[:best], open[best+1:]...)
		if cur == goal {
			return g.reconstruct(came, cur)
		}
		closed[cur] = true
		cx, cy := cur%g.W, cur/g.W
		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= g.W || ny >= g.H || g.Solid(nx, ny) {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (g.Solid(cx+d[0], cy) || g.Solid(cx, cy+d[1])) {
				continue // 不穿墙角
			}
			ni := ny*g.W + nx
			if closed[ni] {
				continue
			}
			step := float32(1)
			if d[0] != 0 && d[1] != 0 {
				step = 1.41421356
			}
			ng := gScore[cur] + step
			if gScore[ni] < 0 || ng < gScore[ni] {
				gScore[ni] = ng
				came[ni] = cur
				fScore[ni] = ng + h(ni)
				found := false
				for _, o := range open {
					if o == ni {
						found = true
						break
					}
				}
				if !found {
					open = append(open, ni)
				}
			}
		}
	}
	return nil
}

func (g *CollisionGrid) reconstruct(came []int, cur int) [][2]int {
	var rev [][2]int
	for cur >= 0 {
		rev = append(rev, [2]int{cur % g.W, cur / g.W})
		cur = came[cur]
	}
	out := make([][2]int, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		out = append(out, rev[i])
	}
	return out
}
