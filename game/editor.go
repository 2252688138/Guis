// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"fmt"
	"math"
)

// 场景编辑器运行时。
//
// 引擎本身只管渲染与逻辑，这个文件补齐「改场景」需要的那一层：选择集、
// 射线拾取、轨道相机、平移/旋转/缩放、撤销重做、场景存取。它不依赖任何 GUI，
// 宿主（guis/ui 或自定义工具窗口）把鼠标键盘事件翻译成下面的方法调用即可。
//
//	ed := game.NewEditor(scene)
//	ed.Orbit(dx, dy)                       // 右键拖拽
//	ed.Pan(dx, dy)                         // 中键拖拽
//	ed.Zoom(step)                          // 滚轮
//	if n, _, _, ok := ed.PickScreen(cam, aspect, x, y, w, h); ok {
//	    ed.Select(n)                       // 左键点选
//	}
//	ed.BeginTransform()                    // 拖 gizmo 之前
//	ed.TranslateSelected(game.V3(1, 0, 0), true)
//	ed.CommitTransform("移动")              // 拖完之后（进撤销栈）
//	ed.DrawGizmo(game.NewDebugDraw(cam))   // 每帧画包围盒与坐标轴
type Editor struct {
	// Scene 编辑中的场景。
	Scene *Scene
	// Cam 编辑器相机（为空时用 Scene.Camera）。
	Cam *Camera
	// MaxUndo 撤销栈上限（默认 64）。
	MaxUndo int
	// Grid 是否绘制地面网格（默认 true）。
	Grid bool
	// Snap 平移吸附步长（0 = 不吸附）。
	Snap float32
	// GizmoSize 坐标轴 gizmo 长度（默认 1.5）。
	GizmoSize float32

	sel   []*Node
	undo  []editCmd
	redo  []editCmd
	snap0 []trsSnap // BeginTransform 时记录的原始状态
}

// NewEditor 创建编辑器（相机默认取场景相机）。
func NewEditor(sc *Scene) *Editor {
	ed := &Editor{Scene: sc, MaxUndo: 64, Grid: true, GizmoSize: 1.5}
	if sc != nil {
		ed.Cam = sc.Camera
	}
	if ed.Cam == nil {
		ed.Cam = NewCamera()
	}
	return ed
}

// Camera 编辑器相机。
func (ed *Editor) Camera() *Camera {
	if ed == nil {
		return nil
	}
	if ed.Cam == nil {
		if ed.Scene != nil && ed.Scene.Camera != nil {
			ed.Cam = ed.Scene.Camera
		} else {
			ed.Cam = NewCamera()
		}
	}
	return ed.Cam
}

// ---------------------------------------------------------------- 选择集

// Select 直接设定选择（nil 元素被忽略）。
func (ed *Editor) Select(ns ...*Node) {
	ed.sel = ed.sel[:0]
	for _, n := range ns {
		if n != nil {
			ed.sel = append(ed.sel, n)
		}
	}
}

// SelectAdd 追加选择（已选中则跳过）。
func (ed *Editor) SelectAdd(n *Node) {
	if n == nil {
		return
	}
	for _, s := range ed.sel {
		if s == n {
			return
		}
	}
	ed.sel = append(ed.sel, n)
}

// Toggle 切换选择状态。
func (ed *Editor) Toggle(n *Node) {
	if n == nil {
		return
	}
	for i, s := range ed.sel {
		if s == n {
			ed.sel = append(ed.sel[:i], ed.sel[i+1:]...)
			return
		}
	}
	ed.sel = append(ed.sel, n)
}

// ClearSelection 清空选择。
func (ed *Editor) ClearSelection() { ed.sel = ed.sel[:0] }

// Selection 当前选择集（只读切片）。
func (ed *Editor) Selection() []*Node { return ed.sel }

// Primary 主选中节点（第一个），无选择时为 nil。
func (ed *Editor) Primary() *Node {
	if len(ed.sel) == 0 {
		return nil
	}
	return ed.sel[0]
}

// IsSelected 是否已选中。
func (ed *Editor) IsSelected(n *Node) bool {
	for _, s := range ed.sel {
		if s == n {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- 拾取

// PickRay 用世界空间射线拾取最近的节点，返回节点、命中点与距离。
// 有网格的节点做逐三角形求交，没有网格的节点按其原点半径做球形近似。
func (ed *Editor) PickRay(origin, dir Vec3) (*Node, Vec3, float32, bool) {
	if ed == nil || ed.Scene == nil || ed.Scene.Root == nil {
		return nil, Vec3{}, 0, false
	}
	dir = dir.Normalize()
	ed.Scene.BuildWorld()
	var (
		best    *Node
		bestT   float32
		bestHit Vec3
		found   bool
	)
	ed.Scene.Each(func(n *Node) {
		if n.Mesh != nil && len(n.Mesh.Positions) > 0 {
			if t, _, ok := RayMesh(origin, dir, n.Mesh, n.WorldMatrix()); ok && (!found || t < bestT) {
				best, bestT, bestHit, found = n, t, origin.Add(dir.Scale(t)), true
			}
			return
		}
		// 空节点：把原点当成半径 0.25 的球
		r := float32(0.25) * float32(math.Max(1, float64(n.GizmoScaleHint())))
		c := n.WorldPos()
		if t, ok := RaySphere(origin, dir, Sphere{C: c, R: r}); ok && (!found || t < bestT) {
			best, bestT, bestHit, found = n, t, origin.Add(dir.Scale(t)), true
		}
	})
	return best, bestHit, bestT, found
}

// GizmoScaleHint 节点缩放的粗略上限，用于无网格节点的拾取半径。
func (n *Node) GizmoScaleHint() float32 {
	if n == nil {
		return 1
	}
	s := n.Scl
	return float32(math.Max(1, math.Max(float64(s.X), math.Max(float64(s.Y), float64(s.Z)))))
}

// PickScreen 把控件像素坐标转成射线后拾取。
// px/py 是相对控件左上角的像素坐标，w/h 是控件像素尺寸，aspect = w/h。
func (ed *Editor) PickScreen(cam *Camera, aspect, px, py, w, h float32) (*Node, Vec3, float32, bool) {
	if cam == nil {
		cam = ed.Camera()
	}
	if cam == nil || w <= 0 || h <= 0 {
		return nil, Vec3{}, 0, false
	}
	origin, dir, ok := ScreenRay(cam, aspect, px, py, w, h)
	if !ok {
		return nil, Vec3{}, 0, false
	}
	return ed.PickRay(origin, dir)
}

// ScreenRay 由屏幕像素坐标构造世界空间射线。
func ScreenRay(cam *Camera, aspect, px, py, w, h float32) (Vec3, Vec3, bool) {
	if cam == nil || w <= 0 || h <= 0 {
		return Vec3{}, Vec3{}, false
	}
	inv, ok := cam.ViewProj(aspect).Invert()
	if !ok {
		return Vec3{}, Vec3{}, false
	}
	nx := px/w*2 - 1
	ny := 1 - py/h*2
	near := inv.TransformVec4(V4(nx, ny, -1, 1))
	far := inv.TransformVec4(V4(nx, ny, 1, 1))
	if near.W == 0 || far.W == 0 {
		return Vec3{}, Vec3{}, false
	}
	a := V3(near.X/near.W, near.Y/near.W, near.Z/near.W)
	b := V3(far.X/far.W, far.Y/far.W, far.Z/far.W)
	d := b.Sub(a)
	if d.Len() < 1e-6 {
		return Vec3{}, Vec3{}, false
	}
	return a, d.Normalize(), true
}

// ---------------------------------------------------------------- 相机操作

// Orbit 轨道旋转（绕相机注视点，dx/dy 为像素增量）。
func (ed *Editor) Orbit(dx, dy float32) {
	c := ed.Camera()
	if c == nil {
		return
	}
	off := c.Pos.Sub(c.Target)
	dist := off.Len()
	if dist < 1e-4 {
		off = V3(0, 0, 5)
		dist = 5
	}
	yaw := float32(math.Atan2(float64(off.X), float64(off.Z)))
	pitch := float32(math.Asin(float64(clampF(off.Y/dist, -1, 1))))
	yaw += dx * 0.01
	pitch = clampF(pitch+dy*0.01, -1.5, 1.5)
	cp, sp := float32(math.Cos(float64(pitch))), float32(math.Sin(float64(pitch)))
	cy, sy := float32(math.Cos(float64(yaw))), float32(math.Sin(float64(yaw)))
	c.Pos = c.Target.Add(V3(dist*cp*sy, dist*sp, dist*cp*cy))
}

// Pan 平移相机（沿屏幕右/上方向，dx/dy 为像素增量）。
func (ed *Editor) Pan(dx, dy float32) {
	c := ed.Camera()
	if c == nil {
		return
	}
	fwd := c.Target.Sub(c.Pos)
	dist := fwd.Len()
	if dist < 1e-4 {
		dist = 5
	}
	fwd = fwd.Normalize()
	right := fwd.Cross(c.Up)
	if right.Len() < 1e-5 {
		right = V3(1, 0, 0)
	}
	right = right.Normalize()
	up := right.Cross(fwd).Normalize()
	k := dist * 0.0018
	d := right.Scale(-dx * k).Add(up.Scale(dy * k))
	c.Pos = c.Pos.Add(d)
	c.Target = c.Target.Add(d)
}

// Zoom 推拉相机（step > 0 拉远，step < 0 推近）。
func (ed *Editor) Zoom(step float32) {
	c := ed.Camera()
	if c == nil {
		return
	}
	off := c.Pos.Sub(c.Target)
	dist := off.Len()
	if dist < 1e-4 {
		dist = 5
	}
	nd := clampF(dist*(1+step*0.1), 0.2, 10000)
	c.Pos = c.Target.Add(off.Normalize().Scale(nd))
}

// FrameNode 把相机拉近到刚好装下该节点（无网格节点按其缩放估尺寸）。
func (ed *Editor) FrameNode(n *Node) {
	c := ed.Camera()
	if n == nil || c == nil {
		return
	}
	ed.Scene.BuildWorld()
	center := n.WorldPos()
	r := float32(1)
	if n.Mesh != nil {
		if b := AABBFromMesh(n.Mesh); b.Size().Len() > 0 {
			r = b.Size().Len() * 0.5
			center = n.WorldMatrix().TransformPoint(b.Center())
		}
	}
	if r < 0.2 {
		r = 0.2
	}
	dir := c.Pos.Sub(c.Target)
	if dir.Len() < 1e-4 {
		dir = V3(0, 0, 1)
	}
	dist := r / float32(math.Tan(float64(c.FOVY*math.Pi/360)))
	if dist < r*1.2 {
		dist = r * 1.2
	}
	c.Target = center
	c.Pos = center.Add(dir.Normalize().Scale(dist))
}

// FrameSelected 聚焦到选择集（空选择时回到场景原点）。
func (ed *Editor) FrameSelected() {
	if len(ed.sel) == 0 {
		c := ed.Camera()
		if c != nil {
			c.Target = V3(0, 0, 0)
		}
		return
	}
	ed.FrameNode(ed.sel[0])
}

// ---------------------------------------------------------------- 变换与撤销

// BeginTransform 在拖动 gizmo 之前记录原始状态，CommitTransform 时生成一条撤销命令。
func (ed *Editor) BeginTransform() {
	ed.snap0 = ed.snap0[:0]
	for _, n := range ed.sel {
		ed.snap0 = append(ed.snap0, snapOf(n))
	}
}

// CommitTransform 结束一次变换（label 为空时用默认名）。没变化则不产生撤销记录。
func (ed *Editor) CommitTransform(label string) {
	if len(ed.snap0) == 0 {
		return
	}
	before := ed.snap0
	ed.snap0 = nil
	after := make([]trsSnap, 0, len(before))
	changed := false
	for _, b := range before {
		a := snapOf(b.n)
		if !a.equal(b) {
			changed = true
		}
		after = append(after, a)
	}
	if !changed {
		return
	}
	if label == "" {
		label = "变换"
	}
	ed.push(&trsCmd{label: label, before: before, after: after})
}

// TranslateSelected 平移选中节点（world=true 时按世界轴，否则按父空间轴）。
func (ed *Editor) TranslateSelected(d Vec3, world bool) {
	if world {
		if ed.Snap > 0 {
			d = V3(snapF(d.X, ed.Snap), snapF(d.Y, ed.Snap), snapF(d.Z, ed.Snap))
		}
	}
	for _, n := range ed.sel {
		n.Pos = n.Pos.Add(d)
	}
}

// RotateSelected 以自身原点旋转选中节点（四元数累乘，并同步欧拉缓存）。
func (ed *Editor) RotateSelected(q Quat) {
	q = q.Normalize()
	for _, n := range ed.sel {
		cur := n.Quat
		if cur.W == 0 && cur.X == 0 && cur.Y == 0 && cur.Z == 0 {
			cur = QuatAxisAngle(V3(0, 1, 0), 0) // 单位四元数
		}
		n.Quat = q.Mul(cur).Normalize()
		n.Rot = eulerOf(n.Quat)
	}
}

// ScaleSelected 按倍率缩放选中节点。
func (ed *Editor) ScaleSelected(f float32) {
	if f <= 0 {
		return
	}
	for _, n := range ed.sel {
		s := n.Scl
		if s.X == 0 && s.Y == 0 && s.Z == 0 {
			s = V3(1, 1, 1)
		}
		n.Scl = s.Scale(f)
	}
}

// ParentOf 查找节点的父节点（Root 的父为 nil）。
func (ed *Editor) ParentOf(n *Node) *Node {
	if ed == nil || ed.Scene == nil || ed.Scene.Root == nil || n == nil {
		return nil
	}
	var parent *Node
	ed.Scene.Each(func(c *Node) {
		if parent != nil {
			return
		}
		for _, ch := range c.Children {
			if ch == n {
				parent = c
				return
			}
		}
	})
	return parent
}

// AddNode 把节点挂到场景根节点下，并记录撤销。
func (ed *Editor) AddNode(n *Node) *Node {
	if n == nil || ed.Scene == nil {
		return nil
	}
	if ed.Scene.Root == nil {
		ed.Scene.Root = &Node{Name: "root", Visible: true}
	}
	ed.Scene.Root.Add(n)
	ed.push(&addCmd{parent: ed.Scene.Root, n: n})
	return n
}

// DeleteSelected 删除选中节点（记录撤销后可从同位置恢复）。
func (ed *Editor) DeleteSelected() int {
	if len(ed.sel) == 0 {
		return 0
	}
	cmds := make([]editCmd, 0, len(ed.sel))
	for _, n := range ed.sel {
		p := ed.ParentOf(n)
		if p == nil {
			continue
		}
		idx := -1
		for i, c := range p.Children {
			if c == n {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		p.Children = append(p.Children[:idx], p.Children[idx+1:]...)
		cmds = append(cmds, &deleteCmd{parent: p, n: n, idx: idx})
	}
	if len(cmds) == 0 {
		return 0
	}
	ed.push(&multiCmd{label: "删除", cmds: cmds})
	ed.ClearSelection()
	return len(cmds)
}

// Undo 撤销一步，返回该步名称。
func (ed *Editor) Undo() (string, bool) {
	if len(ed.undo) == 0 {
		return "", false
	}
	c := ed.undo[len(ed.undo)-1]
	ed.undo = ed.undo[:len(ed.undo)-1]
	c.Undo()
	ed.redo = append(ed.redo, c)
	return c.Label(), true
}

// Redo 重做一步，返回该步名称。
func (ed *Editor) Redo() (string, bool) {
	if len(ed.redo) == 0 {
		return "", false
	}
	c := ed.redo[len(ed.redo)-1]
	ed.redo = ed.redo[:len(ed.redo)-1]
	c.Redo()
	ed.undo = append(ed.undo, c)
	return c.Label(), true
}

// CanUndo / CanRedo 是否有可撤销 / 可重做的操作。
func (ed *Editor) CanUndo() bool { return len(ed.undo) > 0 }

// CanRedo 是否可重做。
func (ed *Editor) CanRedo() bool { return len(ed.redo) > 0 }

// HistoryDepth 返回撤销栈深度。
func (ed *Editor) HistoryDepth() int { return len(ed.undo) }

func (ed *Editor) push(c editCmd) {
	ed.undo = append(ed.undo, c)
	ed.redo = ed.redo[:0]
	if ed.MaxUndo > 0 && len(ed.undo) > ed.MaxUndo {
		ed.undo = ed.undo[len(ed.undo)-ed.MaxUndo:]
	}
}

// ---------------------------------------------------------------- 场景存取

// SaveScene 保存当前场景到 JSON（meshName 用于把网格引用写成名字）。
func (ed *Editor) SaveScene(path string, meshName func(*Mesh) string) error {
	if ed == nil || ed.Scene == nil {
		return errors.New("game: 编辑器没有场景")
	}
	if path == "" {
		return errors.New("game: 保存路径为空")
	}
	return SaveScene(ed.Scene, path, meshName)
}

// LoadSceneFile 从 JSON 加载场景并重置编辑器状态（选择集、撤销栈）。
func (ed *Editor) LoadSceneFile(path string, resolve func(string) *Mesh) error {
	sc, err := LoadScene(path, resolve)
	if err != nil {
		return err
	}
	ed.Scene = sc
	if sc.Camera != nil {
		ed.Cam = sc.Camera
	}
	ed.sel = nil
	ed.undo = nil
	ed.redo = nil
	return nil
}

// ---------------------------------------------------------------- 撤销命令

type editCmd interface {
	Undo()
	Redo()
	Label() string
}

// trsSnap 节点变换快照。
type trsSnap struct {
	n        *Node
	pos, rot Vec3
	scl      Vec3
	q        Quat
}

func snapOf(n *Node) trsSnap {
	return trsSnap{n: n, pos: n.Pos, rot: n.Rot, scl: n.Scl, q: n.Quat}
}

func (s trsSnap) apply() {
	s.n.Pos, s.n.Rot, s.n.Scl, s.n.Quat = s.pos, s.rot, s.scl, s.q
}

func (s trsSnap) equal(o trsSnap) bool {
	return s.pos.Equal(o.pos) && s.rot.Equal(o.rot) && s.scl.Equal(o.scl) &&
		s.q.X == o.q.X && s.q.Y == o.q.Y && s.q.Z == o.q.Z && s.q.W == o.q.W
}

type trsCmd struct {
	label        string
	before, after []trsSnap
}

func (c *trsCmd) Undo() {
	for _, s := range c.before {
		s.apply()
	}
}

func (c *trsCmd) Redo() {
	for _, s := range c.after {
		s.apply()
	}
}

func (c *trsCmd) Label() string { return c.label }

type addCmd struct {
	parent *Node
	n      *Node
}

func (c *addCmd) Undo() {
	removeChild(c.parent, c.n)
}

func (c *addCmd) Redo() { c.parent.Add(c.n) }

func (c *addCmd) Label() string { return "新增 " + c.n.Name }

type deleteCmd struct {
	parent *Node
	n      *Node
	idx    int
}

func (c *deleteCmd) Undo() { insertChild(c.parent, c.idx, c.n) }

func (c *deleteCmd) Redo() { removeChild(c.parent, c.n) }

func (c *deleteCmd) Label() string { return "删除 " + c.n.Name }

type multiCmd struct {
	label string
	cmds  []editCmd
}

func (c *multiCmd) Undo() {
	for i := len(c.cmds) - 1; i >= 0; i-- {
		c.cmds[i].Undo()
	}
}

func (c *multiCmd) Redo() {
	for _, x := range c.cmds {
		x.Redo()
	}
}

func (c *multiCmd) Label() string { return c.label }

func removeChild(parent, n *Node) {
	if parent == nil || n == nil {
		return
	}
	for i, c := range parent.Children {
		if c == n {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

func insertChild(parent *Node, idx int, n *Node) {
	if parent == nil || n == nil {
		return
	}
	if idx < 0 || idx > len(parent.Children) {
		idx = len(parent.Children)
	}
	parent.Children = append(parent.Children, nil)
	copy(parent.Children[idx+1:], parent.Children[idx:])
	parent.Children[idx] = n
}

// eulerOf 把四元数拆成欧拉角（与 RotateEuler 的顺序一致）。
func eulerOf(q Quat) Vec3 {
	p, y, r := q.Mat4().Euler()
	return V3(p, y, r)
}

func snapF(v, step float32) float32 {
	if step <= 0 {
		return v
	}
	return float32(math.Round(float64(v/step))) * step
}

// 保证 fmt 在只用到 %v 调试时仍被引用（避免后续删日志时误删 import）。
var _ = fmt.Sprintf
