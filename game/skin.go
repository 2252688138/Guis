// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Bone 骨骼（绑定姿势 + 逆绑定矩阵）。
//
// 约束：Bones 必须按拓扑顺序排列（父骨索引 < 自身索引），这样每帧
// 算世界矩阵只需一遍扫描，不用递归。用 Skeleton.Validate() 校验。
type Bone struct {
	Name string
	// Parent 父骨索引（-1 = 根）。
	Parent int
	// Pos / Rot / Scl 绑定姿势下的局部变换（动画没覆盖时用它们）。
	Pos Vec3
	Rot Quat
	Scl Vec3
	// InverseBind 绑定姿势下「世界 → 骨骼空间」的逆矩阵（蒙皮用）。
	InverseBind Mat4
}

// LocalMatrix 绑定姿势的局部矩阵（T * R * S）。
func (b Bone) LocalMatrix() Mat4 {
	scl := b.Scl
	if scl.X == 0 && scl.Y == 0 && scl.Z == 0 {
		scl = Vec3{X: 1, Y: 1, Z: 1}
	}
	return Translate(b.Pos).Mul(b.Rot.Mat4()).Mul(Scale(scl))
}

// Skeleton 骨架。
type Skeleton struct {
	Bones []Bone
}

// Validate 检查父索引合法性与拓扑顺序，返回第一个出问题的骨索引（-1 = 无问题）。
func (s *Skeleton) Validate() int {
	if s == nil {
		return -1
	}
	for i := range s.Bones {
		p := s.Bones[i].Parent
		if p < -1 || p >= len(s.Bones) {
			return i
		}
		if p >= i {
			return i
		}
	}
	return -1
}

// SkinVertex 单顶点的骨骼权重（最多 4 骨、权重会自动归一化）。
type SkinVertex struct {
	Joints  [4]uint8
	Weights [4]float32
}

// SkinnedMesh 蒙皮网格：网格顶点 + 骨架 + 每顶点权重。
type SkinnedMesh struct {
	Mesh     *Mesh
	Skeleton *Skeleton
	// Skin 长度应与 Mesh.Positions 一致（不足的顶点按绑定位输出）。
	Skin []SkinVertex

	posed []Vec3
	tmp   *Mesh
}

// NewSkinnedMesh 构造蒙皮网格（Skin 为空时自动全部绑到 0 号骨）。
func NewSkinnedMesh(m *Mesh, sk *Skeleton, skin []SkinVertex) *SkinnedMesh {
	sm := &SkinnedMesh{Mesh: m, Skeleton: sk, Skin: skin}
	if sm.Skeleton != nil && len(sm.Skeleton.Bones) == 0 {
		sm.Skeleton = nil
	}
	if len(sm.Skin) == 0 && m != nil && sm.Skeleton != nil {
		sm.Skin = make([]SkinVertex, len(m.Positions))
		for i := range sm.Skin {
			sm.Skin[i] = SkinVertex{Joints: [4]uint8{0}, Weights: [4]float32{1}}
		}
	}
	return sm
}

// BindSingleBone 把整张网格绑到 0 号骨（静态模型过渡用）。
func (sm *SkinnedMesh) BindSingleBone() {
	if sm == nil || sm.Mesh == nil {
		return
	}
	sm.Skin = make([]SkinVertex, len(sm.Mesh.Positions))
	for i := range sm.Skin {
		sm.Skin[i] = SkinVertex{Joints: [4]uint8{0}, Weights: [4]float32{1}}
	}
}

// SingleBoneSkeleton 只有一根根骨的骨架（绑定逆矩阵为单位矩阵）。
//
// 用途：给静态 OBJ / 程序生成网格套上蒙皮管线过渡，让整张网格能随手一根
// 动画骨整体变换（旋转、缩放、下落……）。
func SingleBoneSkeleton(name string) *Skeleton {
	if name == "" {
		name = "root"
	}
	return &Skeleton{Bones: []Bone{{
		Name: name, Parent: -1, Scl: Vec3{X: 1, Y: 1, Z: 1}, InverseBind: Identity(),
	}}}
}

// NewSkinnedMeshFromOBJ 加载 OBJ 并绑定到单骨骨架（整网格受同一根骨驱动）。
func NewSkinnedMeshFromOBJ(path string) (*SkinnedMesh, error) {
	m, err := LoadOBJ(path)
	if err != nil {
		return nil, err
	}
	sm := NewSkinnedMesh(m, SingleBoneSkeleton("root"), nil)
	sm.BindSingleBone()
	return sm, nil
}

// BindPose 绑定姿势下的关节矩阵（= 单位矩阵）：蒙皮后顶点与原始网格一致。
func (sm *SkinnedMesh) BindPose() []Mat4 {
	if sm == nil || sm.Skeleton == nil {
		return nil
	}
	out := make([]Mat4, len(sm.Skeleton.Bones))
	for i := range out {
		out[i] = Identity()
	}
	return out
}

// RenderMesh 生成用于光栅的临时网格：顶点蒙皮、法线按主导骨变换、包围盒重算。
//
// 临时网格复用同一块内存（每帧只写不分配），适合常规低模角色；
// 高模 / 大量角色时建议配合 LOD 降低顶点数。
func (sm *SkinnedMesh) RenderMesh(pose []Mat4) *Mesh {
	if sm == nil || sm.Mesh == nil {
		return nil
	}
	verts := sm.SkinnedVertices(pose)
	if len(verts) == 0 {
		return nil
	}
	if sm.tmp == nil {
		sm.tmp = &Mesh{}
	}
	m := sm.tmp
	m.Positions = verts
	m.Indices = sm.Mesh.Indices
	m.UVs = sm.Mesh.UVs
	m.Colors = sm.Mesh.Colors
	if len(m.Normals) != len(verts) {
		m.Normals = make([]Vec3, len(verts))
	}
	// 法线：用每个顶点主导骨（权重最大的那根）的旋转部分变换，
	// 比逐骨加权更便宜，光照方向误差在低模上视觉可接受。
	for i := range verts {
		var n Vec3
		if i < len(sm.Mesh.Normals) {
			n = sm.Mesh.Normals[i]
		}
		if len(pose) > 0 {
			j := 0
			if i < len(sm.Skin) {
				best := float32(0)
				for k := 0; k < 4; k++ {
					if w := sm.Skin[i].Weights[k]; w > best {
						best = w
						j = int(sm.Skin[i].Joints[k])
					}
				}
			}
			if j >= 0 && j < len(pose) {
				n = pose[j].TransformDir(n)
			}
		}
		m.Normals[i] = n.Normalize()
	}
	box := AABB{Min: verts[0], Max: verts[0]}
	for _, v := range verts[1:] {
		box.Min = box.Min.Min(v)
		box.Max = box.Max.Max(v)
	}
	m.Bounds = box
	return m
}

// SkinnedVertices 按骨骼矩阵 pose（长度 = 骨数）蒙皮，返回复用缓冲的顶点数组。
//
// pose[i] 是第 i 根骨的「关节矩阵」= 骨骼当前世界矩阵 * 绑定逆矩阵。
func (sm *SkinnedMesh) SkinnedVertices(pose []Mat4) []Vec3 {
	if sm == nil || sm.Mesh == nil {
		return nil
	}
	n := len(sm.Mesh.Positions)
	if sm.posed == nil || len(sm.posed) < n {
		sm.posed = make([]Vec3, n)
	}
	out := sm.posed[:n]
	for i := 0; i < n; i++ {
		p := sm.Mesh.Positions[i]
		if i >= len(sm.Skin) {
			out[i] = p
			continue
		}
		sv := sm.Skin[i]
		var acc Vec3
		var wsum float32
		for k := 0; k < 4; k++ {
			w := sv.Weights[k]
			if w == 0 {
				continue
			}
			j := int(sv.Joints[k])
			if j < 0 || j >= len(pose) {
				continue
			}
			acc = acc.Add(pose[j].TransformPoint(p).Scale(w))
			wsum += w
		}
		switch {
		case wsum == 0:
			out[i] = p
		case wsum > 0.999 && wsum < 1.001:
			out[i] = acc
		default:
			out[i] = acc.Scale(1 / wsum)
		}
	}
	return out
}
