// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// 编辑器的可视化辅助：地面网格、选中包围盒、坐标轴。
//
// 这些函数只往 DebugDraw 里塞线段，宿主最后统一 d.DrawSprites(dst, w, h) 即可：
//
//	d := game.NewDebugDraw(cam)
//	d.Clear()
//	ed.DrawGrid(d, 20, 1)
//	ed.DrawGizmo(d)
//	d.DrawSprites(frame, w, h)

// DrawGrid 在 y=0 平面上画 half*2 格、每格 step 单位的网格。
func (ed *Editor) DrawGrid(d *DebugDraw, half int, step float32) {
	if !ed.Grid || d == nil || half <= 0 || step <= 0 {
		return
	}
	var (
		minor = RGB(90, 96, 108)
		major = RGB(140, 148, 162)
		axisX = RGB(200, 80, 80)
		axisZ = RGB(80, 120, 210)
	)
	n := float32(half) * step
	for i := -half; i <= half; i++ {
		v := float32(i) * step
		c := minor
		if i%5 == 0 {
			c = major
		}
		switch {
		case v == 0:
			d.Line(V3(-n, 0, 0), V3(n, 0, 0), axisX)
			d.Line(V3(0, 0, -n), V3(0, 0, n), axisZ)
		default:
			d.Line(V3(v, 0, -n), V3(v, 0, n), c)
			d.Line(V3(-n, 0, v), V3(n, 0, v), c)
		}
	}
}

// DrawGizmo 画选中节点的世界包围盒与三轴（未选中时不画）。
func (ed *Editor) DrawGizmo(d *DebugDraw) {
	if ed == nil || d == nil || len(ed.sel) == 0 {
		return
	}
	selColor := RGB(255, 186, 66)
	ed.Scene.BuildWorld()
	for _, n := range ed.sel {
		b := WorldNodeBounds(n)
		if b.Size().Len() > 0 {
			d.Box(b.Min, b.Max, selColor)
		}
		sz := ed.GizmoSize
		if sz <= 0 {
			sz = 1.5
		}
		d.Axes(n.WorldPos(), sz)
	}
}

// WorldNodeBounds 节点的世界空间 AABB（网格为空时取原点附近的小盒）。
func WorldNodeBounds(n *Node) AABB {
	if n == nil {
		return AABB{}
	}
	wp := n.WorldPos()
	local := AABB{
		Min: wp.Sub(V3(0.1, 0.1, 0.1)),
		Max: wp.Add(V3(0.1, 0.1, 0.1)),
	}
	if n.Mesh == nil || len(n.Mesh.Positions) == 0 {
		return local
	}
	lb := AABBFromMesh(n.Mesh)
	m := n.WorldMatrix()
	first := true
	var out AABB
	for i := 0; i < 8; i++ {
		x, y, z := lb.Min.X, lb.Min.Y, lb.Min.Z
		if i&1 != 0 {
			x = lb.Max.X
		}
		if i&2 != 0 {
			y = lb.Max.Y
		}
		if i&4 != 0 {
			z = lb.Max.Z
		}
		p := m.TransformPoint(V3(x, y, z))
		if first {
			out = AABB{Min: p, Max: p}
			first = false
			continue
		}
		out.Min = out.Min.Min(p)
		out.Max = out.Max.Max(p)
	}
	return out
}
