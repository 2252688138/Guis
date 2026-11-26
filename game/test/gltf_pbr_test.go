// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"

	"guis/game"
)

// pngURI 生成一张 w×h 单色 PNG 的 data URI。
func pngURI(t *testing.T, w, h int, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码 PNG 失败: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// minimalGLTF 构造一个带 1 个三角形、1 个材质的最小 glTF。
// extra 用来往根对象里塞 materials / images 等额外字段。
func minimalGLTF(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	var pos bytes.Buffer
	for _, v := range []float32{0, 0, 0, 1, 0, 0, 0, 1, 0} {
		if err := binary.Write(&pos, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	doc := map[string]any{
		"asset":   map[string]any{"version": "2.0"},
		"scene":   0,
		"scenes":  []any{map[string]any{"nodes": []any{0}}},
		"nodes":   []any{map[string]any{"mesh": 0}},
		"meshes":  []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]any{"POSITION": 0}, "material": 0}}}},
		"accessors":   []any{map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}},
		"bufferViews": []any{map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": pos.Len()}},
		"buffers": []any{map[string]any{
			"uri":        "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(pos.Bytes()),
			"byteLength": pos.Len(),
		}},
	}
	for k, v := range extra {
		doc[k] = v
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化 glTF 失败: %v", err)
	}
	return raw
}

func TestGLTFPBRTextureMapping(t *testing.T) {
	base := pngURI(t, 2, 2, color.RGBA{255, 0, 0, 255})
	norm := pngURI(t, 2, 2, color.RGBA{128, 128, 255, 255})
	mr := pngURI(t, 2, 2, color.RGBA{0, 200, 30, 255})
	ao := pngURI(t, 4, 4, color.RGBA{255, 255, 255, 255})

	data := minimalGLTF(t, map[string]any{
		"images": []any{
			map[string]any{"uri": base},
			map[string]any{"uri": norm},
			map[string]any{"uri": mr},
			map[string]any{"uri": ao},
		},
		"textures": []any{
			map[string]any{"source": 0, "sampler": 0},
			map[string]any{"source": 1},
			map[string]any{"source": 2},
			map[string]any{"source": 3},
		},
		"samplers": []any{map[string]any{"magFilter": 9728, "minFilter": 9987}},
		"materials": []any{map[string]any{
			"pbrMetallicRoughness": map[string]any{
				"metallicFactor":           0.2,
				"roughnessFactor":          0.6,
				"baseColorTexture":         map[string]any{"index": 0},
				"metallicRoughnessTexture": map[string]any{"index": 2},
			},
			"normalTexture":    map[string]any{"index": 1, "scale": 0.8},
			"occlusionTexture": map[string]any{"index": 3},
		}},
	})

	g, err := game.LoadGLTFBytes(data)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("应有 1 个网格，实际 %d", g.Len())
	}
	mat := g.Meshes[0].Material
	if mat == nil {
		t.Fatal("材质为空")
	}
	if mat.Shade != game.ShadePBR {
		t.Fatalf("有贴图的材质应走 PBR，实际 %v", mat.Shade)
	}
	if mat.Texture == nil || mat.Texture.W != 2 || mat.Texture.H != 2 {
		t.Fatalf("baseColor 贴图未加载：%+v", mat.Texture)
	}
	if mat.NormalMap == nil || mat.MetallicRoughnessMap == nil || mat.AOMap == nil {
		t.Fatalf("贴图槽缺失: normal=%v mr=%v ao=%v", mat.NormalMap, mat.MetallicRoughnessMap, mat.AOMap)
	}
	if mat.NormalScale != 0.8 {
		t.Fatalf("normalScale 应为 0.8，实际 %v", mat.NormalScale)
	}
	if !mat.Nearest || !mat.Mipmap {
		t.Fatalf("采样器映射不对：Nearest=%v Mipmap=%v", mat.Nearest, mat.Mipmap)
	}
	// 贴图内容应正确（PNG 红色 → 纹理像素红）
	if c := mat.Texture.At(0, 0); c.R != 255 || c.G != 0 {
		t.Fatalf("贴图像素不对：%v", c)
	}
}

func TestGLTFTextureReuse(t *testing.T) {
	uri := pngURI(t, 2, 2, color.RGBA{10, 20, 30, 255})
	data := minimalGLTF(t, map[string]any{
		"images":   []any{map[string]any{"uri": uri}},
		"textures": []any{map[string]any{"source": 0}},
		"materials": []any{
			map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}},
			map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}},
		},
	})
	g, err := game.LoadGLTFBytes(data)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	// 两个材质不在同一网格上，用逐个 primitive 的方式另测：这里至少验证解析不崩且贴图可用
	if g.Len() == 0 || g.Meshes[0].Material == nil || g.Meshes[0].Material.Texture == nil {
		t.Fatal("贴图应被加载")
	}
}

func TestGLTFBadImageFallsBack(t *testing.T) {
	data := minimalGLTF(t, map[string]any{
		"images":   []any{map[string]any{"uri": "data:image/png;base64,bm90LWEtcG5n"}},
		"textures": []any{map[string]any{"source": 0}},
		"materials": []any{map[string]any{
			"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}},
		}},
	})
	g, err := game.LoadGLTFBytes(data)
	if err != nil {
		t.Fatalf("坏贴图不应导致整个模型导入失败: %v", err)
	}
	if mat := g.Meshes[0].Material; mat == nil || mat.Texture != nil {
		t.Fatalf("坏图应回退为无贴图，实际 %+v", mat)
	}
}

func TestGLTFMissingImageIndex(t *testing.T) {
	data := minimalGLTF(t, map[string]any{
		"textures": []any{map[string]any{}},
		"materials": []any{map[string]any{
			"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 7}},
		}},
	})
	if _, err := game.LoadGLTFBytes(data); err != nil {
		t.Fatalf("缺失 texture 引用不应报错: %v", err)
	}
}
