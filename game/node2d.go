// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Node2D 2D 场景节点（父子层级 + 局部变换）。
//
// 用于把角色、手持武器、坐骑、头像、血条等组织成一棵树，
// 再由 SpriteBatch.AddNode2D 一次性展开成世界坐标精灵。
//
// 约定：
//   - Pos 是父节点坐标系下的位置，默认落在本节点锚点上（Origin 缺省为几何中心）。
//   - Scl 零值按 (1,1) 处理；Hidden 零值 = 可见；Alpha 零值按不透明处理。
//   - Sprite 非空时本节点自身也绘制一个精灵（其 Pos/Size 为本地矩形，
//     Pos 相对节点锚点）。
//   - Layer / Z 会累加到子树的精灵排序键上。
type Node2D struct {
	Name string
	Pos  Vec2
	Scl  Vec2
	Rot  float32
	// Origin 本节点锚点（0~1，相对 Sprite.Size；缺省 0.5,0.5 = 中心）。
	Origin Vec2
	// Sprite 本节点绘制的精灵（nil 表示纯变换容器）。
	Sprite *Sprite
	// Alpha 本节点透明度（0~1，缺省 1），沿子树累乘。
	Alpha float32
	// Layer / Z 参与排序的键（Layer 是整数层，Z 是层内深度）。
	Layer int
	Z     float32
	// Hidden 隐藏本节点及其子树（零值 = 可见）。
	Hidden bool

	parent   *Node2D
	Children []*Node2D
}

// NewNode2D 创建 2D 节点（Scl 初始化为 1,1，Alpha 为不透明）。
func NewNode2D(name string) *Node2D {
	return &Node2D{Name: name, Scl: V2(1, 1), Alpha: 1}
}

// Add 挂载子节点（自动接管 parent，同一节点重复 Add 会被去重）。
func (n *Node2D) Add(children ...*Node2D) *Node2D {
	if n == nil {
		return n
	}
	for _, c := range children {
		if c == nil || c == n {
			continue
		}
		if c.parent != nil {
			c.parent.Remove(c)
		}
		c.parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

// Remove 移除直接子节点。
func (n *Node2D) Remove(c *Node2D) {
	if n == nil || c == nil {
		return
	}
	for i, x := range n.Children {
		if x == c {
			n.Children = append(n.Children[:i], n.Children[i+1:]...)
			c.parent = nil
			return
		}
	}
}

// Parent 父节点（根节点返回 nil）。
func (n *Node2D) Parent() *Node2D {
	if n == nil {
		return nil
	}
	return n.parent
}

// Traverse 深度优先遍历本节点及子树（fn 收到 nil 时忽略）。
func (n *Node2D) Traverse(fn func(*Node2D)) {
	if n == nil || fn == nil {
		return
	}
	fn(n)
	for _, c := range n.Children {
		c.Traverse(fn)
	}
}

// Find 按名字递归查找（先序）。
func (n *Node2D) Find(name string) *Node2D {
	if n == nil {
		return nil
	}
	var found *Node2D
	n.Traverse(func(x *Node2D) {
		if found == nil && x.Name == name {
			found = x
		}
	})
	return found
}

// WorldPos / WorldRot / WorldScale / WorldAlpha 返回累积到根的变换。
func (n *Node2D) WorldPos() Vec2 {
	p, _, _, _ := n.world()
	return p
}

// WorldRot 累积旋转（弧度）。
func (n *Node2D) WorldRot() float32 {
	_, r, _, _ := n.world()
	return r
}

// WorldScale 累积缩放。
func (n *Node2D) WorldScale() Vec2 {
	_, _, s, _ := n.world()
	return s
}

// WorldAlpha 累积透明度（0~1）。
func (n *Node2D) WorldAlpha() float32 {
	_, _, _, a := n.world()
	return a
}

func (n *Node2D) world() (Vec2, float32, Vec2, float32) {
	if n == nil {
		return Vec2{}, 0, V2(1, 1), 1
	}
	pos, rot, scl, alpha := n.Pos, n.Rot, n.scale(), n.alpha()
	for p := n.parent; p != nil; p = p.parent {
		local := Vec2{X: pos.X * p.scale().X, Y: pos.Y * p.scale().Y}
		c, s := node2dTrig(p.Rot)
		pos = Vec2{X: p.Pos.X + local.X*c - local.Y*s, Y: p.Pos.Y + local.X*s + local.Y*c}
		rot += p.Rot
		scl = Vec2{X: scl.X * p.scale().X, Y: scl.Y * p.scale().Y}
		alpha *= p.alpha()
	}
	return pos, rot, scl, alpha
}

func (n *Node2D) scale() Vec2 {
	if n == nil {
		return V2(1, 1)
	}
	s := n.Scl
	if s.X == 0 {
		s.X = 1
	}
	if s.Y == 0 {
		s.Y = 1
	}
	return s
}

func (n *Node2D) alpha() float32 {
	if n == nil || n.Alpha <= 0 {
		return 1
	}
	if n.Alpha > 1 {
		return 1
	}
	return n.Alpha
}

// AddNode2D 递归展开节点树，把每个节点自身的精灵按世界变换加入批。
//
// 展开后精灵的 UseOrigin 为 false（Pos/Size 已是世界坐包围盒），
// 透明度沿子树累乘到精灵 Color.A。调用方可随后 b.SortY() 排序。
func (b *SpriteBatch) AddNode2D(root *Node2D) {
	if b == nil || root == nil {
		return
	}
	b.addNode2D(root, Vec2{}, 0, V2(1, 1), 1)
}

func (b *SpriteBatch) addNode2D(n *Node2D, pos Vec2, rot float32, scl Vec2, alpha float32) {
	if n == nil || n.Hidden {
		return
	}
	local := Vec2{X: n.Pos.X * scl.X, Y: n.Pos.Y * scl.Y}
	c, s := node2dTrig(rot)
	wPos := Vec2{X: pos.X + local.X*c - local.Y*s, Y: pos.Y + local.X*s + local.Y*c}
	wRot := rot + n.Rot
	wScl := Vec2{X: n.scale().X * scl.X, Y: n.scale().Y * scl.Y}
	wAlpha := alpha * n.alpha()

	if n.Sprite != nil {
		sp := *n.Sprite
		w, h := sp.Size.X, sp.Size.Y
		ax, ay := float32(0.5), float32(0.5)
		if sp.UseOrigin {
			ax, ay = sp.Origin.X, sp.Origin.Y
		}
		// 本地矩形中心相对节点锚点的偏移
		lcx, lcy := w*(0.5-ax), h*(0.5-ay)
		c2, s2 := node2dTrig(wRot)
		wcx := wPos.X + lcx*wScl.X*c2 - lcy*wScl.Y*s2
		wcy := wPos.Y + lcx*wScl.X*s2 + lcy*wScl.Y*c2
		sp.Size = V2(w*wScl.X, h*wScl.Y)
		sp.Pos = V2(wcx-sp.Size.X/2, wcy-sp.Size.Y/2)
		sp.Rot = wRot
		sp.UseOrigin = false
		sp.Origin = Vec2{}
		sp.Color = mulSpriteAlpha(sp.Color, wAlpha)
		sp.Layer += n.Layer
		sp.Z += n.Z
		b.Add(sp)
	}
	for _, c := range n.Children {
		b.addNode2D(c, wPos, wRot, wScl, wAlpha)
	}
}

func node2dTrig(rot float32) (float32, float32) {
	if rot == 0 {
		return 1, 0
	}
	return float32(math.Cos(float64(rot))), float32(math.Sin(float64(rot)))
}

// mulSpriteAlpha 把透明度乘进颜色：A=0 视为不透明（与 Sprite.Color 语义一致）。
func mulSpriteAlpha(c RGBA, a float32) RGBA {
	if a >= 1 {
		return c
	}
	if a <= 0 {
		c.A = 0
		return c
	}
	if c.A == 0 {
		c.A = uint8(a*255 + 0.5)
		return c
	}
	c.A = uint8(float32(c.A)*a + 0.5)
	return c
}
