// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// TileCollision 瓦片碰撞：把瓦片网格转成 2D 碰撞盒，可直接喂给
// PlatformerBody.Step（实心盒 + 单向平台盒）。
//
//	col := game.NewTileCollision(32, 32)
//	col.MarkSolid(1, 2, 3).MarkOneWay(9)
//	solid, oneWay := col.Collect(tilemap, "collision")
//	body.Step(dt, solid, oneWay)
//
// 实心瓦片默认会按行合并成长条（Merge=true），少量盒体就能描述大面积地形。
type TileCollision struct {
	// TileW, TileH 单个瓦片的像素尺寸。
	TileW, TileH float32
	// Merge 是否合并水平相邻的实心瓦片（默认 true，盒体更少）。
	Merge bool

	solid  map[int]bool
	oneWay map[int]bool
}

// NewTileCollision 创建瓦片碰撞配置。
func NewTileCollision(tileW, tileH float32) *TileCollision {
	if tileW <= 0 {
		tileW = 32
	}
	if tileH <= 0 {
		tileH = 32
	}
	return &TileCollision{TileW: tileW, TileH: tileH, Merge: true, solid: map[int]bool{}, oneWay: map[int]bool{}}
}

// MarkSolid 标记若干 GID 为实心（六面阻挡）。
func (t *TileCollision) MarkSolid(gids ...int) *TileCollision {
	if t == nil {
		return t
	}
	for _, g := range gids {
		t.solid[g] = true
	}
	return t
}

// MarkOneWay 标记若干 GID 为单向平台（仅从上方落地时阻挡）。
func (t *TileCollision) MarkOneWay(gids ...int) *TileCollision {
	if t == nil {
		return t
	}
	for _, g := range gids {
		t.oneWay[g] = true
	}
	return t
}

// IsSolid 该 GID 是否被标记为实心。
func (t *TileCollision) IsSolid(gid int) bool {
	if t == nil {
		return false
	}
	return t.solid[gid]
}

// IsOneWay 该 GID 是否被标记为单向平台。
func (t *TileCollision) IsOneWay(gid int) bool {
	if t == nil {
		return false
	}
	return t.oneWay[gid]
}

// Boxes 把一层瓦片数据转成碰撞盒（实心水平合并，单向平台逐格）。
//
// data 行优先长度为 w*h；offset 为该层的像素偏移（对应 TileLayer.Offset）。
func (t *TileCollision) Boxes(data []int, w, h int, offset Vec2) (solid, oneWay []AABB2) {
	if t == nil || w <= 0 || h <= 0 {
		return nil, nil
	}
	for ty := 0; ty < h; ty++ {
		run := -1
		for tx := 0; tx <= w; tx++ {
			var gid int
			if tx < w {
				i := ty*w + tx
				if i < len(data) {
					gid = data[i]
				}
			}
			isSolid := t.solid[gid]
			isOneWay := !isSolid && t.oneWay[gid]
			if isSolid && run < 0 {
				run = tx
			}
			if !isSolid && run >= 0 {
				// 收束一段水平实心块
				solid = append(solid, t.span(run, tx-1, ty, offset))
				run = -1
			}
			if isOneWay {
				oneWay = append(oneWay, t.cell(tx, ty, offset))
			}
		}
	}
	return solid, oneWay
}

// FromLayer 从 Tiled 地图的瓦片层生成碰撞盒。
func (t *TileCollision) FromLayer(l *TileLayer, m *TiledMap) (solid, oneWay []AABB2) {
	if t == nil || l == nil {
		return nil, nil
	}
	if m != nil {
		if m.TileW > 0 {
			t.TileW = float32(m.TileW)
		}
		if m.TileH > 0 {
			t.TileH = float32(m.TileH)
		}
	}
	return t.Boxes(l.Data, l.W, l.H, l.Offset)
}

// Collect 汇总多个瓦片层的碰撞盒（layers 为空时统计全部可见层）。
func (t *TileCollision) Collect(m *TiledMap, layers ...string) (solid, oneWay []AABB2) {
	if t == nil || m == nil {
		return nil, nil
	}
	for i := range m.Layers {
		l := &m.Layers[i]
		if len(layers) > 0 && !containsStr(layers, l.Name) {
			continue
		}
		s, o := t.FromLayer(l, m)
		solid = append(solid, s...)
		oneWay = append(oneWay, o...)
	}
	return solid, oneWay
}

// containsStr 判断切片是否包含字符串。
func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// cell 单格碰撞盒。
func (t *TileCollision) cell(tx, ty int, offset Vec2) AABB2 {
	x := float32(tx)*t.TileW + offset.X
	y := float32(ty)*t.TileH + offset.Y
	return AABB2{Min: V2(x, y), Max: V2(x+t.TileW, y+t.TileH)}
}

// span 水平合并后的碰撞盒（tx0~tx1 闭区间）。
func (t *TileCollision) span(tx0, tx1, ty int, offset Vec2) AABB2 {
	x := float32(tx0)*t.TileW + offset.X
	w := float32(tx1-tx0+1) * t.TileW
	y := float32(ty)*t.TileH + offset.Y
	return AABB2{Min: V2(x, y), Max: V2(x+w, y+t.TileH)}
}

// ResolveAABB 把一个运动中的盒体沿最小穿透方向推出（静态碰撞体的简单分轴解算）。
//
// 适合道具 / 相机限制 / 非角色实体；角色建议用 PlatformerBody.Step（带速度与单向平台）。
// 返回修正后的盒体与是否发生了碰撞。
func ResolveAABB(box AABB2, solids []AABB2) (AABB2, bool) {
	hit := false
	for _, s := range solids {
		if !box.Overlaps(s) {
			continue
		}
		hit = true
		// 四个方向的穿透深度，取最小方向推出
		left := box.Max.X - s.Min.X
		right := s.Max.X - box.Min.X
		up := box.Max.Y - s.Min.Y
		down := s.Max.Y - box.Min.Y
		minPen := left
		dir := 0
		if right < minPen {
			minPen, dir = right, 1
		}
		if up < minPen {
			minPen, dir = up, 2
		}
		if down < minPen {
			minPen, dir = down, 3
		}
		switch dir {
		case 0:
			box = box.Move(-minPen, 0)
		case 1:
			box = box.Move(minPen, 0)
		case 2:
			box = box.Move(0, -minPen)
		default:
			box = box.Move(0, minPen)
		}
	}
	return box, hit
}
