// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// Canvas 自绘节点：描述文件与脚本共用的形状声明。
//
//	{ "type": "canvas", "key": "board", "w": 240, "h": 120, "bg": "#0b1020",
//	  "shapes": [ { "kind": "rect", "x": 10, "y": 10, "w": 60, "h": 30, "fill": "#4c8dff" } ] }

import (
	"encoding/json"

	"guis/ui"
)

// CanvasPointDef 顶点。JSON 两种写法都支持：{"x":1,"y":2} 或 [1,2]。
type CanvasPointDef struct {
	X float32 `json:"x,omitempty"`
	Y float32 `json:"y,omitempty"`
}

// UnmarshalJSON 兼容 {x,y} 与 [x,y] 两种写法。
func (p *CanvasPointDef) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		var arr []float32
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		if len(arr) > 0 {
			p.X = arr[0]
		}
		if len(arr) > 1 {
			p.Y = arr[1]
		}
		return nil
	}
	type plain CanvasPointDef
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = CanvasPointDef(v)
	return nil
}

// CanvasShapeDef 一条绘制指令（字段含义见 ui.CanvasShape）。
type CanvasShapeDef struct {
	Kind    string           `json:"kind,omitempty"`
	X       float32          `json:"x,omitempty"`
	Y       float32          `json:"y,omitempty"`
	W       float32          `json:"w,omitempty"`
	H       float32          `json:"h,omitempty"`
	R       float32          `json:"r,omitempty"`
	Points  []CanvasPointDef `json:"points,omitempty"`
	CX      float32          `json:"cx,omitempty"`
	CY      float32          `json:"cy,omitempty"`
	Radius  float32          `json:"radius,omitempty"`
	A0      float32          `json:"a0,omitempty"`
	A1      float32          `json:"a1,omitempty"`
	Fill    string           `json:"fill,omitempty"`
	Stroke  string           `json:"stroke,omitempty"`
	StrokeW float32          `json:"strokeW,omitempty"`
	Opacity float32          `json:"opacity,omitempty"`
}

var extraCanvases = map[string]*ui.CanvasState{}

// canvasStateFor 画布状态：同 key 复用。
func canvasStateFor(d Node, path string) *ui.CanvasState {
	key := nodeKeyOf(d, path)
	if st, ok := extraCanvases[key]; ok {
		return st
	}
	st := ui.NewCanvasState(key)
	extraCanvases[key] = st
	return st
}

// toCanvasShapes 声明 → ui.CanvasShape。
func toCanvasShapes(defs []CanvasShapeDef) []ui.CanvasShape {
	if len(defs) == 0 {
		return nil
	}
	out := make([]ui.CanvasShape, 0, len(defs))
	for _, d := range defs {
		sh := ui.CanvasShape{
			Kind:    d.Kind,
			X:       d.X,
			Y:       d.Y,
			W:       d.W,
			H:       d.H,
			R:       d.R,
			CX:      d.CX,
			CY:      d.CY,
			Radius:  d.Radius,
			A0:      d.A0,
			A1:      d.A1,
			Fill:    d.Fill,
			Stroke:  d.Stroke,
			StrokeW: d.StrokeW,
			Opacity: d.Opacity,
		}
		if len(d.Points) > 0 {
			pts := make([]ui.CanvasPoint, len(d.Points))
			for i, p := range d.Points {
				pts[i] = ui.CanvasPoint{X: p.X, Y: p.Y}
			}
			sh.Points = pts
		}
		out = append(out, sh)
	}
	return out
}

// resetCanvases 清掉画布缓存（热重载 / 重建界面时调用）。
func resetCanvases() {
	extraCanvases = map[string]*ui.CanvasState{}
}
