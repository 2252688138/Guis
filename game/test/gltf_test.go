// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"guis/game"
)

// triBuffer 构造一个三角形的二进制数据：3 个 vec3 位置 + 3 个 uint16 索引（末尾补 0 对齐）。
func triBuffer() []byte {
	var buf []byte
	f32 := func(v float32) {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, math.Float32bits(v))
		buf = append(buf, b...)
	}
	for _, v := range []float32{0, 0, 0, 1, 0, 0, 0, 1, 0} {
		f32(v)
	}
	for _, i := range []uint16{0, 1, 2} {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, i)
		buf = append(buf, b...)
	}
	for len(buf)%4 != 0 {
		buf = append(buf, 0)
	}
	return buf
}

// triGltf 生成一份最小可解析的 glTF 文档（dataURI=true 时把二进制内嵌成 base64）。
func triGltf(t *testing.T, bin []byte, dataURI bool) []byte {
	t.Helper()
	buffer := map[string]any{"byteLength": len(bin)}
	if dataURI {
		buffer["uri"] = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)
	}
	doc := map[string]any{
		"asset":   map[string]any{"version": "2.0"},
		"scene":   0,
		"scenes":  []any{map[string]any{"name": "tri-scene", "nodes": []any{0}}},
		"buffers": []any{buffer},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36},
			map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": 6},
		},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 1, "componentType": 5123, "count": 3, "type": "SCALAR"},
		},
		"materials": []any{map[string]any{
			"name": "red",
			"pbrMetallicRoughness": map[string]any{
				"baseColorFactor": []any{1, 0, 0, 1},
			},
		}},
		"meshes": []any{map[string]any{
			"name": "tri",
			"primitives": []any{map[string]any{
				"attributes": map[string]any{"POSITION": 0},
				"indices":    1,
				"material":   0,
				"mode":       4,
			}},
		}},
		"nodes": []any{map[string]any{
			"name": "triNode", "mesh": 0,
			"translation": []any{0, 2, 0},
		}},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("构造 glTF JSON 失败: %v", err)
	}
	return data
}

// buildGLB 把 JSON 与 BIN 打包成 GLB 容器（GLB 的 buffer 不再写 uri）。
func buildGLB(t *testing.T, jsonPart, bin []byte) []byte {
	t.Helper()
	for len(jsonPart)%4 != 0 {
		jsonPart = append(jsonPart, ' ')
	}
	for len(bin)%4 != 0 {
		bin = append(bin, 0)
	}
	total := 12 + 8 + len(jsonPart) + 8 + len(bin)
	out := make([]byte, 0, total)
	le := func(v uint32) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, v)
		return b
	}
	out = append(out, []byte("glTF")...)
	out = append(out, le(2)...)
	out = append(out, le(uint32(total))...)
	out = append(out, le(uint32(len(jsonPart)))...)
	out = append(out, le(0x4E4F534A)...)
	out = append(out, jsonPart...)
	out = append(out, le(uint32(len(bin)))...)
	out = append(out, le(0x004E4942)...)
	out = append(out, bin...)
	return out
}

func TestLoadGLTFDataURI(t *testing.T) {
	bin := triBuffer()
	g, err := game.LoadGLTFBytes(triGltf(t, bin, true))
	if err != nil {
		t.Fatalf("加载 glTF 失败: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("期望 1 个网格，实际 %d", g.Len())
	}
	m := g.Meshes[0]
	if m.Name != "triNode" {
		t.Fatalf("网格名应取节点名，实际 %q", m.Name)
	}
	if len(m.Mesh.Positions) != 3 || len(m.Mesh.Indices) != 3 {
		t.Fatalf("顶点/索引数量错误: %d/%d", len(m.Mesh.Positions), len(m.Mesh.Indices))
	}
	// 节点 translation [0,2,0] 应烘焙进顶点。
	if !nearF(m.Mesh.Positions[1].Y, 2, 1e-5) {
		t.Fatalf("节点平移未生效: %v", m.Mesh.Positions[1])
	}
	if !nearF(m.Mesh.Bounds.Max.Y, 3, 1e-5) {
		t.Fatalf("包围盒错误: %v", m.Mesh.Bounds)
	}
	if len(m.Mesh.Normals) != 3 {
		t.Fatal("缺法线时应自动重算")
	}
	if m.Material == nil || m.Material.BaseColor.R != 255 || m.Material.BaseColor.G != 0 {
		t.Fatalf("材质颜色未应用: %+v", m.Material.BaseColor)
	}
}

func TestLoadGLTFGLB(t *testing.T) {
	bin := triBuffer()
	jsonPart := triGltf(t, bin, false)
	g, err := game.LoadGLTFBytes(buildGLB(t, jsonPart, bin))
	if err != nil {
		t.Fatalf("加载 GLB 失败: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("期望 1 个网格，实际 %d", g.Len())
	}
	if g.Name != "tri-scene" {
		t.Fatalf("模型名应取 scene 名，实际 %q", g.Name)
	}
	box := g.Bounds()
	if !nearF(box.Min.Y, 2, 1e-5) || !nearF(box.Max.Y, 3, 1e-5) {
		t.Fatalf("Bounds 错误: %v", box)
	}
}

func TestLoadGLTFFileExternalBin(t *testing.T) {
	dir := t.TempDir()
	bin := triBuffer()
	if err := os.WriteFile(filepath.Join(dir, "tri.bin"), bin, 0o644); err != nil {
		t.Fatalf("写 bin 失败: %v", err)
	}
	// 外部文件模式：buffer 写 uri 而不是 base64。
	var doc map[string]any
	if err := json.Unmarshal(triGltf(t, bin, false), &doc); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	doc["buffers"] = []any{map[string]any{"byteLength": len(bin), "uri": "tri.bin"}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	path := filepath.Join(dir, "tri.gltf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写 gltf 失败: %v", err)
	}

	g, err := game.LoadGLTFFile(path)
	if err != nil {
		t.Fatalf("加载外部 bin 失败: %v", err)
	}
	if g.Len() != 1 || g.Name != "tri-scene" {
		t.Fatalf("结果异常: len=%d name=%q", g.Len(), g.Name)
	}

	// AddTo 应把网格挂进场景。
	sc := game.NewScene()
	nodes := g.AddTo(sc)
	if len(nodes) != 1 || nodes[0].Mesh == nil {
		t.Fatalf("AddTo 失败: %v", nodes)
	}
}

func TestLoadGLTFBytesRejectsExternalBin(t *testing.T) {
	bin := triBuffer()
	var doc map[string]any
	if err := json.Unmarshal(triGltf(t, bin, false), &doc); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	doc["buffers"] = []any{map[string]any{"byteLength": len(bin), "uri": "tri.bin"}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if _, err := game.LoadGLTFBytes(data); err == nil {
		t.Fatal("引用外部 bin 的内存加载应报错并提示用 LoadGLTFFile")
	}
}

func TestLoadGLTFErrors(t *testing.T) {
	if _, err := game.LoadGLTFBytes([]byte("not json")); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	// 缺 POSITION 的图元。
	bin := triBuffer()
	var doc map[string]any
	if err := json.Unmarshal(triGltf(t, bin, true), &doc); err != nil {
		t.Fatalf("解析测试 JSON 失败: %v", err)
	}
	doc["meshes"] = []any{map[string]any{
		"primitives": []any{map[string]any{"attributes": map[string]any{}}},
	}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if _, err := game.LoadGLTFBytes(data); err == nil {
		t.Fatal("缺 POSITION 应报错")
	}
}
