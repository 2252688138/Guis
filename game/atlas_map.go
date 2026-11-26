// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AtlasMap 非均匀图集（名字 → 源矩形），用于 PackAtlas / tools/atlaspack 的产物。
//
// 与按固定格子切分的 Atlas 互补：AtlasMap 每帧尺寸任意，适合角色、UI、特效混合图集。
type AtlasMap struct {
	Tex    *Texture
	frames map[string]Rect2
}

// AtlasMapJSON 图集坐标表（atlas.json）的结构。
type AtlasMapJSON struct {
	Texture struct {
		W int `json:"w"`
		H int `json:"h"`
	} `json:"texture"`
	Frames []AtlasFrameJSON `json:"frames"`
}

// AtlasFrameJSON 单帧坐标。
type AtlasFrameJSON struct {
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

// NewAtlasMap 创建空图集（tex 可为 nil，之后再设）。
func NewAtlasMap(tex *Texture) *AtlasMap {
	return &AtlasMap{Tex: tex, frames: map[string]Rect2{}}
}

// Add 登记一帧（同名覆盖）。
func (a *AtlasMap) Add(name string, r Rect2) {
	if a == nil {
		return
	}
	if a.frames == nil {
		a.frames = map[string]Rect2{}
	}
	a.frames[name] = r
}

// Rect 按名字取源矩形。
func (a *AtlasMap) Rect(name string) (Rect2, bool) {
	if a == nil || a.frames == nil {
		return Rect2{}, false
	}
	r, ok := a.frames[name]
	return r, ok
}

// Names 返回全部帧名（按字典序，便于稳定输出）。
func (a *AtlasMap) Names() []string {
	if a == nil {
		return nil
	}
	out := make([]string, 0, len(a.frames))
	for n := range a.frames {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Len 帧数量。
func (a *AtlasMap) Len() int {
	if a == nil {
		return 0
	}
	return len(a.frames)
}

// Sprite 按名字构造精灵（x,y 为左上角像素，w,h 为绘制尺寸；w/h ≤ 0 时用帧原始尺寸）。
func (a *AtlasMap) Sprite(name string, x, y, w, h float32) (Sprite, bool) {
	r, ok := a.Rect(name)
	if !ok || a.Tex == nil {
		return Sprite{}, false
	}
	if w <= 0 {
		w = r.W
	}
	if h <= 0 {
		h = r.H
	}
	return Sprite{Pos: V2(x, y), Size: V2(w, h), Texture: a.Tex, Src: r, Color: RGB(255, 255, 255)}, true
}

// Marshal 导出坐标表 JSON（可直接落地为 atlas.json）。
func (a *AtlasMap) Marshal() ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("game: 空图集")
	}
	var doc AtlasMapJSON
	if a.Tex != nil {
		doc.Texture.W = a.Tex.W
		doc.Texture.H = a.Tex.H
	}
	for _, n := range a.Names() {
		r := a.frames[n]
		doc.Frames = append(doc.Frames, AtlasFrameJSON{
			Name: n, X: int(r.X), Y: int(r.Y), W: int(r.W), H: int(r.H),
		})
	}
	return json.MarshalIndent(doc, "", "  ")
}

// LoadAtlasMap 从坐标表 JSON 构造图集（tex 为图集纹理，可为 nil）。
func LoadAtlasMap(tex *Texture, data []byte) (*AtlasMap, error) {
	var doc AtlasMapJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("game: 解析图集坐标表失败: %w", err)
	}
	a := NewAtlasMap(tex)
	for _, f := range doc.Frames {
		if f.Name == "" {
			continue
		}
		a.Add(f.Name, Rect2{X: float32(f.X), Y: float32(f.Y), W: float32(f.W), H: float32(f.H)})
	}
	return a, nil
}

// LoadAtlasMapFile 从文件加载坐标表，并按同目录同名 .png 加载纹理。
//
// jsonPath 为 atlas.json 路径；纹理默认取同目录的 atlas.png（不存在也返回图集，Tex 为 nil）。
func LoadAtlasMapFile(jsonPath string) (*AtlasMap, error) {
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	tex, _ := LoadTexture(atlasTexturePath(jsonPath))
	return LoadAtlasMap(tex, b)
}

// atlasTexturePath 由坐标表路径推导同目录的 atlas.png。
func atlasTexturePath(jsonPath string) string {
	return strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + ".png"
}
