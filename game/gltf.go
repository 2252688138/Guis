// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// GLTFMesh 一个 glTF 图元渲染出来的网格（顶点已烘焙节点变换）。
type GLTFMesh struct {
	// Name 网格名（取节点名，缺省用 mesh 名或序号）。
	Name string
	// Mesh 三角网格。
	Mesh *Mesh
	// Material 材质（取 baseColorFactor / emissiveFactor / doubleSided）。
	Material *Material
	// Node 节点在模型里的原始名字（用于定位部件）。
	Node string
}

// GLTF 加载后的模型（可能含多个网格）。
type GLTF struct {
	// Meshes 按节点顺序展开的网格列表。
	Meshes []GLTFMesh
	// Name 模型名（scene 名或文件名派生，可为空）。
	Name string
}

// Len 网格数量。
func (g *GLTF) Len() int {
	if g == nil {
		return 0
	}
	return len(g.Meshes)
}

// AddTo 把所有网格作为子节点加进场景，返回创建的节点（材质已就绪）。
func (g *GLTF) AddTo(sc *Scene) []*Node {
	if g == nil || sc == nil {
		return nil
	}
	out := make([]*Node, 0, len(g.Meshes))
	for _, m := range g.Meshes {
		n := NewMeshNode(m.Name, m.Mesh, m.Material)
		sc.Add(n)
		out = append(out, n)
	}
	return out
}

// Bounds 整个模型的包围盒（无网格时返回零盒）。
func (g *GLTF) Bounds() AABB {
	var box AABB
	first := true
	if g == nil {
		return box
	}
	for _, m := range g.Meshes {
		if m.Mesh == nil {
			continue
		}
		if first {
			box, first = m.Mesh.Bounds, false
			continue
		}
		box.Min = box.Min.Min(m.Mesh.Bounds.Min)
		box.Max = box.Max.Max(m.Mesh.Bounds.Max)
	}
	return box
}

// LoadGLTFFile 从 .gltf / .glb 文件加载模型（外部 .bin 与贴图按同目录解析）。
func LoadGLTFFile(path string) (*GLTF, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g, err := loadGLTF(data, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("game: 解析 glTF %s 失败: %w", path, err)
	}
	if g.Name == "" {
		base := filepath.Base(path)
		g.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return g, nil
}

// LoadGLTFBytes 从内存加载 glTF/GLB。
//
// 只支持内嵌数据（GLB 的 BIN chunk 或 base64 data URI）；引用外部 .bin 时必须用
// LoadGLTFFile。仅加载三角形图元（mode 4），跳过点/线。
func LoadGLTFBytes(data []byte) (*GLTF, error) {
	return loadGLTF(data, "")
}

// glTF JSON 结构（只取本项目用得到的字段）。
type gltfRoot struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	Scene  *int `json:"scene"`
	Scenes []struct {
		Name  string `json:"name"`
		Nodes []int  `json:"nodes"`
	} `json:"scenes"`
	Buffers []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
	Accessors []struct {
		BufferView    int    `json:"bufferView"`
		ByteOffset    int    `json:"byteOffset"`
		ComponentType int    `json:"componentType"`
		Count         int    `json:"count"`
		Type          string `json:"type"`
	} `json:"accessors"`
	Meshes []struct {
		Name       string `json:"name"`
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Material   *int           `json:"material"`
			Mode       *int           `json:"mode"`
		} `json:"primitives"`
	} `json:"meshes"`
	Materials []struct {
		Name                 string    `json:"name"`
		DoubleSided          bool      `json:"doubleSided"`
		EmissiveFactor       []float32 `json:"emissiveFactor"`
		AlphaMode            string    `json:"alphaMode"`
		PBRMetallicRoughness struct {
			BaseColorFactor  []float32 `json:"baseColorFactor"`
			MetallicFactor   *float32  `json:"metallicFactor"`
			RoughnessFactor  *float32  `json:"roughnessFactor"`
			BaseColorTexture *struct {
				Index *int `json:"index"`
			} `json:"baseColorTexture"`
			MetallicRoughnessTexture *struct {
				Index *int `json:"index"`
			} `json:"metallicRoughnessTexture"`
		} `json:"pbrMetallicRoughness"`
		NormalTexture *struct {
			Index *int     `json:"index"`
			Scale *float32 `json:"scale"`
		} `json:"normalTexture"`
		OcclusionTexture *struct {
			Index    *int     `json:"index"`
			Strength *float32 `json:"strength"`
		} `json:"occlusionTexture"`
	} `json:"materials"`
	Images []struct {
		URI        string `json:"uri"`
		BufferView *int   `json:"bufferView"`
		MimeType   string `json:"mimeType"`
	} `json:"images"`
	Textures []struct {
		Source  *int `json:"source"`
		Sampler *int `json:"sampler"`
	} `json:"textures"`
	Samplers []struct {
		MagFilter *int `json:"magFilter"`
		MinFilter *int `json:"minFilter"`
	} `json:"samplers"`
	Nodes []gltfNode `json:"nodes"`
}

// gltfNode 一个节点（TRS 或 4×4 matrix）。
type gltfNode struct {
	Name        string    `json:"name"`
	Mesh        *int      `json:"mesh"`
	Children    []int     `json:"children"`
	Translation []float32 `json:"translation"`
	Rotation    []float32 `json:"rotation"`
	Scale       []float32 `json:"scale"`
	Matrix      []float32 `json:"matrix"`
}

// gltfLoader 解析过程中的临时状态。
type gltfLoader struct {
	root gltfRoot
	bin  []byte   // GLB 的 BIN chunk
	dir  string   // 外部资源目录（空表示不允许外部文件）
	bufs [][]byte // 已解析的 buffer 数据
	// texCache 已解码贴图（按 texture 索引复用，避免同一张图被多个材质重复解码）。
	texCache map[int]*Texture
}

// loadGLTF 解析 glTF/GLB 字节。
func loadGLTF(data []byte, dir string) (*GLTF, error) {
	jsonPart, binPart, err := splitGLB(data)
	if err != nil {
		return nil, err
	}
	if jsonPart == nil {
		jsonPart = data
	}
	l := &gltfLoader{bin: binPart, dir: dir}
	if err := json.Unmarshal(jsonPart, &l.root); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if len(l.root.Meshes) == 0 && len(l.root.Nodes) == 0 {
		return nil, fmt.Errorf("没有任何 mesh/node")
	}
	if err := l.loadBuffers(); err != nil {
		return nil, err
	}

	g := &GLTF{}
	if l.root.Scene != nil && *l.root.Scene >= 0 && *l.root.Scene < len(l.root.Scenes) {
		g.Name = l.root.Scenes[*l.root.Scene].Name
	}
	roots := l.rootNodes()
	seen := make(map[int]bool, len(l.root.Nodes))
	for _, idx := range roots {
		if err := l.walk(idx, Identity(), 0, &g.Meshes, seen); err != nil {
			return nil, err
		}
	}
	if len(g.Meshes) == 0 {
		return nil, fmt.Errorf("没有可渲染的三角网格（可能是点/线图元或缺 POSITION）")
	}
	return g, nil
}

// splitGLB 拆出 GLB 的 JSON 与 BIN 块；非 GLB 时返回 (nil, nil, nil)。
func splitGLB(data []byte) (jsonPart, binPart []byte, err error) {
	if len(data) < 12 || string(data[:4]) != "glTF" {
		return nil, nil, nil
	}
	version := binary.LittleEndian.Uint32(data[4:])
	if version != 2 {
		return nil, nil, fmt.Errorf("不支持的 GLB 版本 %d", version)
	}
	off := 12
	for off+8 <= len(data) {
		clen := int(binary.LittleEndian.Uint32(data[off:]))
		ctype := binary.LittleEndian.Uint32(data[off+4:])
		off += 8
		if clen < 0 || off+clen > len(data) {
			return nil, nil, fmt.Errorf("GLB 块长度越界")
		}
		chunk := data[off : off+clen]
		switch ctype {
		case 0x4E4F534A: // "JSON"
			jsonPart = chunk
		case 0x004E4942: // "BIN\0"
			binPart = chunk
		}
		off += clen
		if clen%4 != 0 {
			off += 4 - clen%4
		}
	}
	if jsonPart == nil {
		return nil, nil, fmt.Errorf("GLB 缺少 JSON 块")
	}
	return jsonPart, binPart, nil
}

// loadBuffers 解析 buffers（内嵌 BIN / data URI / 外部文件）。
func (l *gltfLoader) loadBuffers() error {
	l.bufs = make([][]byte, len(l.root.Buffers))
	for i, b := range l.root.Buffers {
		switch {
		case b.URI == "":
			if len(l.bin) == 0 {
				return fmt.Errorf("buffer %d 没有 uri 且不是 GLB", i)
			}
			l.bufs[i] = l.bin
		case strings.HasPrefix(b.URI, "data:"):
			idx := strings.IndexByte(b.URI, ',')
			if idx < 0 {
				return fmt.Errorf("buffer %d 的 data URI 无效", i)
			}
			raw, err := base64.StdEncoding.DecodeString(b.URI[idx+1:])
			if err != nil {
				return fmt.Errorf("buffer %d 的 base64 解码失败: %w", i, err)
			}
			l.bufs[i] = raw
		default:
			if l.dir == "" {
				return fmt.Errorf("buffer %d 引用外部文件 %q，请改用 LoadGLTFFile", i, b.URI)
			}
			raw, err := os.ReadFile(filepath.Join(l.dir, b.URI))
			if err != nil {
				return fmt.Errorf("buffer %d 读取失败: %w", i, err)
			}
			l.bufs[i] = raw
		}
	}
	return nil
}

// rootNodes 返回要遍历的根节点索引。
func (l *gltfLoader) rootNodes() []int {
	if l.root.Scene != nil && *l.root.Scene >= 0 && *l.root.Scene < len(l.root.Scenes) {
		if ns := l.root.Scenes[*l.root.Scene].Nodes; len(ns) > 0 {
			return ns
		}
	}
	if len(l.root.Scenes) > 0 && len(l.root.Scenes[0].Nodes) > 0 {
		return l.root.Scenes[0].Nodes
	}
	// 没有 scene：把未被任何节点引用的节点当根。
	child := make(map[int]bool, len(l.root.Nodes))
	for _, n := range l.root.Nodes {
		for _, c := range n.Children {
			child[c] = true
		}
	}
	var roots []int
	for i := range l.root.Nodes {
		if !child[i] {
			roots = append(roots, i)
		}
	}
	return roots
}

// walk 递归遍历节点，把网格烘焙到世界空间。
func (l *gltfLoader) walk(idx int, parent Mat4, depth int, out *[]GLTFMesh, seen map[int]bool) error {
	if idx < 0 || idx >= len(l.root.Nodes) {
		return nil
	}
	if seen[idx] || depth > 64 {
		return fmt.Errorf("节点层级出现环或过深（节点 %d）", idx)
	}
	seen[idx] = true
	defer delete(seen, idx)

	n := l.root.Nodes[idx]
	world := parent.Mul(l.nodeMatrix(n))
	if n.Mesh != nil && *n.Mesh >= 0 && *n.Mesh < len(l.root.Meshes) {
		def := l.root.Meshes[*n.Mesh]
		for pi := range def.Primitives {
			if gm, ok, err := l.primitive(*n.Mesh, pi, world, n.Name, def.Name); err != nil {
				return err
			} else if ok {
				*out = append(*out, gm)
			}
		}
	}
	for _, c := range n.Children {
		if err := l.walk(c, world, depth+1, out, seen); err != nil {
			return err
		}
	}
	return nil
}

// nodeMatrix 节点本地矩阵（matrix 优先，否则 TRS）。
func (l *gltfLoader) nodeMatrix(n gltfNode) Mat4 {
	if len(n.Matrix) == 16 {
		// glTF 的 matrix 是列主序，转成项目的行主序。
		var m Mat4
		for r := 0; r < 4; r++ {
			for c := 0; c < 4; c++ {
				m[r*4+c] = n.Matrix[c*4+r]
			}
		}
		return m
	}
	t := V3(0, 0, 0)
	if len(n.Translation) == 3 {
		t = V3(n.Translation[0], n.Translation[1], n.Translation[2])
	}
	s := V3(1, 1, 1)
	if len(n.Scale) == 3 {
		s = V3(n.Scale[0], n.Scale[1], n.Scale[2])
	}
	q := QuatIdentity()
	if len(n.Rotation) == 4 {
		q = Quat{X: n.Rotation[0], Y: n.Rotation[1], Z: n.Rotation[2], W: n.Rotation[3]}.Normalize()
	}
	return Translate(t).Mul(q.Mat4()).Mul(Scale(s))
}

// primitive 把一个图元转成 *Mesh（world 已烘焙）。
func (l *gltfLoader) primitive(meshIdx, primIdx int, world Mat4, nodeName, meshName string) (GLTFMesh, bool, error) {
	p := l.root.Meshes[meshIdx].Primitives[primIdx]
	if p.Mode != nil && *p.Mode != 4 {
		return GLTFMesh{}, false, nil // 只支持三角形
	}
	posAttr, ok := p.Attributes["POSITION"]
	if !ok {
		return GLTFMesh{}, false, fmt.Errorf("mesh %d 图元 %d 缺少 POSITION", meshIdx, primIdx)
	}
	pos, err := l.accessor(posAttr)
	if err != nil {
		return GLTFMesh{}, false, err
	}
	if len(pos) == 0 {
		return GLTFMesh{}, false, nil
	}
	m := &Mesh{Positions: make([]Vec3, len(pos))}
	for i, v := range pos {
		m.Positions[i] = V3(v[0], v[1], v[2])
	}
	if ai, ok := p.Attributes["NORMAL"]; ok {
		if nrm, err := l.accessor(ai); err == nil && len(nrm) == len(pos) {
			m.Normals = make([]Vec3, len(nrm))
			for i, v := range nrm {
				m.Normals[i] = V3(v[0], v[1], v[2]).Normalize()
			}
		}
	}
	if ai, ok := p.Attributes["TEXCOORD_0"]; ok {
		if uvs, err := l.accessor(ai); err == nil && len(uvs) == len(pos) {
			m.UVs = make([]Vec2, len(uvs))
			for i, v := range uvs {
				m.UVs[i] = V2(v[0], v[1])
			}
		}
	}

	if p.Indices != nil {
		idx, err := l.accessor(*p.Indices)
		if err != nil {
			return GLTFMesh{}, false, err
		}
		m.Indices = make([]uint32, 0, len(idx))
		for _, v := range idx {
			i := uint32(v[0])
			if int(i) >= len(m.Positions) {
				return GLTFMesh{}, false, fmt.Errorf("索引 %d 越界（顶点数 %d）", i, len(m.Positions))
			}
			m.Indices = append(m.Indices, i)
		}
	} else {
		m.Indices = make([]uint32, len(m.Positions))
		for i := range m.Indices {
			m.Indices[i] = uint32(i)
		}
	}
	if len(m.Indices)%3 != 0 {
		m.Indices = m.Indices[:len(m.Indices)/3*3]
	}
	if len(m.Normals) != len(m.Positions) {
		m.RecomputeNormals()
	}
	m.RecomputeBounds()
	if !matIsIdentity(world) {
		if t := m.Transformed(world); t != nil {
			m = t
		}
	}

	name := nodeName
	if name == "" {
		name = meshName
	}
	if name == "" {
		name = fmt.Sprintf("gltf-mesh-%d", meshIdx)
	}
	return GLTFMesh{Name: name, Node: nodeName, Mesh: m, Material: l.material(p.Material)}, true, nil
}

// material 把 glTF 材质转成项目材质（缺省用 DefaultMaterial）。
func (l *gltfLoader) material(idx *int) *Material {
	m := DefaultMaterial()
	if m == nil {
		m = &Material{BaseColor: Color(200, 205, 215, 255)}
	}
	if idx == nil || *idx < 0 || *idx >= len(l.root.Materials) {
		return m
	}
	g := l.root.Materials[*idx]
	if f := g.PBRMetallicRoughness.BaseColorFactor; len(f) == 4 {
		m.BaseColor = Color(unit8(f[0]), unit8(f[1]), unit8(f[2]), unit8(f[3]))
	}
	if len(g.EmissiveFactor) == 3 {
		m.Emissive = Color(unit8(g.EmissiveFactor[0]), unit8(g.EmissiveFactor[1]), unit8(g.EmissiveFactor[2]), 255)
	}
	if g.DoubleSided {
		m.DoubleSided = true
	}
	// PBR：只有模型显式声明了金属度/粗糙度/法线贴图才切成 ShadePBR，
	// 否则保持 Phong，旧资源观感不变。
	pr := g.PBRMetallicRoughness
	pbr := false
	if pr.MetallicFactor != nil {
		m.Metallic = Clamp(*pr.MetallicFactor, 0, 1)
		pbr = true
	}
	if pr.RoughnessFactor != nil {
		m.Roughness = Clamp(*pr.RoughnessFactor, pbrMinRough, 1)
		pbr = true
	}
	if g.NormalTexture != nil {
		pbr = true
		if g.NormalTexture.Scale != nil {
			m.NormalScale = *g.NormalTexture.Scale
		}
		if g.NormalTexture.Index != nil {
			m.NormalMap = l.texture(*g.NormalTexture.Index)
		}
	}
	// 贴图：baseColor → Texture、metallicRoughness → MetallicRoughnessMap
	// （glTF 是 G=粗糙度 B=金属度，与 Material 约定一致）、occlusion → AOMap
	if pr.BaseColorTexture != nil && pr.BaseColorTexture.Index != nil {
		if t := l.texture(*pr.BaseColorTexture.Index); t != nil {
			m.Texture = t
			pbr = true
		}
	}
	if pr.MetallicRoughnessTexture != nil && pr.MetallicRoughnessTexture.Index != nil {
		if t := l.texture(*pr.MetallicRoughnessTexture.Index); t != nil {
			m.MetallicRoughnessMap = t
			pbr = true
		}
	}
	if g.OcclusionTexture != nil && g.OcclusionTexture.Index != nil {
		if t := l.texture(*g.OcclusionTexture.Index); t != nil {
			m.AOMap = t
			pbr = true
		}
	}
	if pr.BaseColorTexture != nil && pr.BaseColorTexture.Index != nil {
		near, mip := l.samplerModes(*pr.BaseColorTexture.Index)
		if near {
			m.Nearest = true
		}
		if mip {
			m.Mipmap = true
		}
	}
	if pbr {
		m.Shade = ShadePBR
	}
	return m
}

// accessor 读取 accessor 为最多 4 分量的 float32 表（索引也走这条路径）。
func (l *gltfLoader) accessor(i int) ([][4]float32, error) {
	if i < 0 || i >= len(l.root.Accessors) {
		return nil, fmt.Errorf("accessor %d 不存在", i)
	}
	a := l.root.Accessors[i]
	if a.Count < 0 {
		return nil, fmt.Errorf("accessor %d 数量非法", i)
	}
	comps := gltfComponentCount(a.Type)
	if comps <= 0 {
		return nil, fmt.Errorf("accessor %d 的类型 %q 不支持", i, a.Type)
	}
	if a.BufferView < 0 || a.BufferView >= len(l.root.BufferViews) {
		return nil, fmt.Errorf("accessor %d 的 bufferView 非法", i)
	}
	bv := l.root.BufferViews[a.BufferView]
	if bv.Buffer < 0 || bv.Buffer >= len(l.bufs) || l.bufs[bv.Buffer] == nil {
		return nil, fmt.Errorf("accessor %d 引用的 buffer 不可用", i)
	}
	buf := l.bufs[bv.Buffer]
	size := gltfCompSize(a.ComponentType)
	if size <= 0 {
		return nil, fmt.Errorf("accessor %d 的 componentType %d 不支持", i, a.ComponentType)
	}
	stride := bv.ByteStride
	if stride <= 0 {
		stride = comps * size
	}
	base := bv.ByteOffset + a.ByteOffset
	out := make([][4]float32, a.Count)
	for k := 0; k < a.Count; k++ {
		off := base + k*stride
		for c := 0; c < comps; c++ {
			p := off + c*size
			if p < 0 || p+size > len(buf) {
				return nil, fmt.Errorf("accessor %d 读取越界（数据 %d 字节）", i, len(buf))
			}
			out[k][c] = gltfReadComp(buf[p:], a.ComponentType)
		}
	}
	return out, nil
}

// gltfComponentCount glTF 类型的分量数。
func gltfComponentCount(t string) int {
	switch t {
	case "SCALAR":
		return 1
	case "VEC2":
		return 2
	case "VEC3":
		return 3
	case "VEC4", "MAT2":
		return 4
	}
	return 0
}

// gltfCompSize 分量字节数。
func gltfCompSize(ct int) int {
	switch ct {
	case 5120, 5121: // byte / ubyte
		return 1
	case 5122, 5123: // short / ushort
		return 2
	case 5125, 5126: // uint / float
		return 4
	}
	return 0
}

// gltfReadComp 按 componentType 读取一个分量并转成 float32。
func gltfReadComp(b []byte, ct int) float32 {
	switch ct {
	case 5126:
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	case 5125:
		return float32(binary.LittleEndian.Uint32(b))
	case 5123:
		return float32(binary.LittleEndian.Uint16(b))
	case 5121:
		return float32(b[0])
	case 5122:
		return float32(int16(binary.LittleEndian.Uint16(b)))
	case 5120:
		return float32(int8(b[0]))
	}
	return 0
}

// matIsIdentity 是否单位矩阵（用于跳过无意义的顶点烘焙）。
func matIsIdentity(m Mat4) bool {
	for i := 0; i < 16; i++ {
		want := float32(0)
		if i%5 == 0 {
			want = 1
		}
		if d := m[i] - want; d > 1e-6 || d < -1e-6 {
			return false
		}
	}
	return true
}

// unit8 把 0~1 浮点颜色分量转成 0~255。
func unit8(v float32) uint8 {
	x := float64(v)*255 + 0.5
	if x < 0 {
		return 0
	}
	if x > 255 {
		return 255
	}
	return uint8(x)
}
