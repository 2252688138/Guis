// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Node 场景节点（可带网格与材质，可嵌套）。
type Node struct {
	Name     string
	Mesh     *Mesh
	Material *Material
	// Pos 平移（父空间）；Rot 欧拉角（弧度，依次 Z→X→Y）；Scl 缩放。
	Pos, Rot, Scl Vec3
	// Quat 非零四元数优先于 Rot（需自行置 W 为 1）。
	Quat Quat
	// Children 子节点。
	Children []*Node
	// LODs 距离细节层级（升序）。为空时始终用 Mesh。
	LODs []LODLevel
	// Skinned 蒙皮网格：非空时每帧按 SkinAnim 的骨骼矩阵重新蒙皮
	// （此时 Mesh 只提供索引 / UV，顶点位置由 Skinned 决定）。
	Skinned *SkinnedMesh
	// SkinAnim 骨骼动画播放器（为 nil 时用绑定姿势）。
	SkinAnim *SkinAnimator
	// Instances 实例批次：非空时忽略 Mesh/Material，按批次里的网格与材质
	// 绘制每个实例（一次性提交，适合草 / 石块 / 子弹这类大量重复物体）。
	Instances *InstanceBatch
	// Visible 是否参与渲染（零值不渲染，请用 NewNode 构造）。
	Visible bool
	// Tag 业务自定义数据（碰撞体、角色状态等）。
	Tag any

	world Mat4
}

// NewNode 创建可见节点。
func NewNode(name string) *Node {
	return &Node{Name: name, Scl: V3(1, 1, 1), Visible: true}
}

// NewMeshNode 创建带网格与材质的三维节点。
func NewMeshNode(name string, mesh *Mesh, mat *Material) *Node {
	n := NewNode(name)
	n.Mesh = mesh
	n.Material = mat
	return n
}

// Add 追加子节点并返回自身（链式）。
func (n *Node) Add(c ...*Node) *Node {
	n.Children = append(n.Children, c...)
	return n
}

// Find 递归按名称查找节点。
func (n *Node) Find(name string) *Node {
	if n == nil {
		return nil
	}
	if n.Name == name {
		return n
	}
	for _, c := range n.Children {
		if got := c.Find(name); got != nil {
			return got
		}
	}
	return nil
}

// Each 递归遍历（包括自身）。
func (n *Node) Each(fn func(*Node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.Children {
		c.Each(fn)
	}
}

// Local 本地变换矩阵。
func (n *Node) Local() Mat4 {
	r := RotateEuler(n.Rot.X, n.Rot.Y, n.Rot.Z)
	if n.Quat.W != 0 || n.Quat.X != 0 || n.Quat.Y != 0 || n.Quat.Z != 0 {
		r = n.Quat.Mat4()
	}
	scl := n.Scl
	if scl.X == 0 && scl.Y == 0 && scl.Z == 0 {
		scl = V3(1, 1, 1)
	}
	return Translate(n.Pos).Mul(r).Mul(Scale(scl))
}

// BuildWorld 递归计算世界矩阵（parent 传 Identity()）。
func (n *Node) BuildWorld(parent Mat4) {
	if n == nil {
		return
	}
	n.world = parent.Mul(n.Local())
	for _, c := range n.Children {
		c.BuildWorld(n.world)
	}
}

// WorldMatrix 最近一次 BuildWorld 得到的世界矩阵。
func (n *Node) WorldMatrix() Mat4 { return n.world }

// WorldPos 世界坐标原点位置。
func (n *Node) WorldPos() Vec3 { return n.world.TransformPoint(V3(0, 0, 0)) }

// TranslateWorld 沿世界轴平移节点（把位移转回父空间）。
func (n *Node) TranslateWorld(d Vec3, parentWorld Mat4) {
	inv, ok := parentWorld.Invert()
	if !ok {
		n.Pos = n.Pos.Add(d)
		return
	}
	n.Pos = n.Pos.Add(inv.TransformDir(d))
}

// Scene 场景（根节点 + 相机 + 光源 + 全局环境）。
type Scene struct {
	Root *Node
	// Camera 参与渲染的相机（为空时不渲染 3D）。
	Camera *Camera
	// Lights 光源列表（建议 ≤ 4 个，软件光栅化逐像素遍历）。
	Lights  []Light
	Ambient RGBA
	Fog     Fog
	// Background 背景色（A=0 时用渲染器自身的 BgColor）。
	Background RGBA
	// Sky 天空盒 / 渐变天空（零值 = 不绘制，用 Background 纯色）。
	Sky Sky
	// Billboards 世界空间广告牌（血条 / 伤害数字 / 图标），每帧由游戏层填充。
	Billboards *BillboardSet
}

// NewScene 构造默认场景（灰色环境光 + 默认相机 + 深色背景）。
func NewScene() *Scene {
	return &Scene{
		Root:       NewNode("root"),
		Camera:     NewCamera(),
		Ambient:    RGB(60, 60, 70),
		Background: RGB(18, 20, 26),
	}
}

// Add 把节点挂到根节点，并确保根节点可见。
func (s *Scene) Add(n ...*Node) *Scene {
	if s.Root == nil {
		s.Root = NewNode("root")
	}
	s.Root.Visible = true
	s.Root.Add(n...)
	return s
}

// Find 递归查找节点。
func (s *Scene) Find(name string) *Node {
	if s == nil || s.Root == nil {
		return nil
	}
	return s.Root.Find(name)
}

// Each 遍历所有节点。
func (s *Scene) Each(fn func(*Node)) {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.Each(fn)
}

// BuildWorld 重算整棵树的世界矩阵。
func (s *Scene) BuildWorld() {
	if s == nil || s.Root == nil {
		return
	}
	if !s.Root.Visible && len(s.Root.Children) == 0 {
		return
	}
	s.Root.BuildWorld(Identity())
}

// SetAmbientIntensity 按 0~1 设置环境光亮度（灰度）。
func (s *Scene) SetAmbientIntensity(k float32) {
	v := clamp255(Clamp(k, 0, 1) * 255)
	s.Ambient = RGB(v, v, v)
}

// AddSun 添加方向光（方向为光线照射方向，如 (0,-1,-1) 表示自上前右射下）。
func (s *Scene) AddSun(dir Vec3, intensity float32) Light {
	l := SunLight(dir)
	l.Intensity = intensity
	s.Lights = append(s.Lights, l)
	return l
}

// AddPointLight 添加点光源。
func (s *Scene) AddPointLight(pos Vec3, rng, intensity float32) Light {
	l := PointLight(pos, rng, intensity)
	s.Lights = append(s.Lights, l)
	return l
}
