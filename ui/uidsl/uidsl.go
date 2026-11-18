// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package uidsl 把外部界面描述解码成 guis/ui 的节点树，并支持「改文件即刷新界面」。
//
// 两种格式共用同一份声明结构（Node，见下方）：
//
//	JSON —— 标准格式，工具 / 模型生成方便（decode.go）
//	DSL  —— 缩进嵌套的紧凑写法，手写快（dsl.go）
//
// 用法：
//
//	l, err := uidsl.Open("ui.json", uidsl.Actions{
//	    "hello": func() { log.Println("点了 hello") },
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer l.Close()
//
//	ui.SetEntry(func() (ui.Options, func() *ui.Node) {
//	    return ui.Options{Title: "uidsl", Width: 900, Height: 620}, l.Root
//	})
//
// l 内部轮询文件的修改时间（默认 200ms），一变就重新解析并把新树换上去；
// 解析失败时保留上一版并记进 LastError()，界面不会白掉。
package uidsl

import (
	"strings"

	"guis/ui"
)

// Node 一个界面节点的声明。JSON 与 DSL 解析出来的是同一种结构。
//
// 类型（type）：box / row / col / center / stack / grid / wrap / scroll /
// spacer / text / button / icon / image。
// 样式字段与 ui.St() 的方法一一对应，未填（0 / 空串）即不设置。
type Node struct {
	Type     string `json:"type"`
	Key      string `json:"key,omitempty"`
	Text     string `json:"text,omitempty"`    // 文本内容 / 按钮文案 / 图片路径
	Icon     string `json:"icon,omitempty"`    // type=icon 时的图标名
	OnClick  string `json:"onClick,omitempty"` // 事件名，由 Actions 解析
	Children []Node `json:"children,omitempty"`

	// ---- 尺寸与间距 ----
	W    float32 `json:"w,omitempty"`
	H    float32 `json:"h,omitempty"`
	Wp   float32 `json:"wp,omitempty"`
	Hp   float32 `json:"hp,omitempty"`
	MinW float32 `json:"minW,omitempty"`
	MaxW float32 `json:"maxW,omitempty"`
	MinH float32 `json:"minH,omitempty"`
	MaxH float32 `json:"maxH,omitempty"`
	Pad  float32 `json:"pad,omitempty"`
	PadX float32 `json:"padX,omitempty"`
	PadY float32 `json:"padY,omitempty"`
	Mar  float32 `json:"mar,omitempty"`
	MarX float32 `json:"marX,omitempty"`
	MarY float32 `json:"marY,omitempty"`
	Gap  float32 `json:"gap,omitempty"`
	Grow float32 `json:"grow,omitempty"`
	Cols int     `json:"cols,omitempty"`
	ColW float32 `json:"colW,omitempty"`

	// ---- 背景 / 圆角 / 描边 / 阴影 ----
	BG           string  `json:"bg,omitempty"`
	BGFrom       string  `json:"bgFrom,omitempty"`
	BGTo         string  `json:"bgTo,omitempty"`
	BGAngle      float32 `json:"bgAngle,omitempty"`
	R            float32 `json:"r,omitempty"`
	Stroke       float32 `json:"stroke,omitempty"`
	StrokeColor  string  `json:"strokeColor,omitempty"`
	ShadowBlur   float32 `json:"shadowBlur,omitempty"`
	ShadowY      float32 `json:"shadowY,omitempty"`
	ShadowSpread float32 `json:"shadowSpread,omitempty"`
	ShadowColor  string  `json:"shadowColor,omitempty"`

	// ---- 文字 / 图标 ----
	Size   float32 `json:"size,omitempty"`   // 字号；type=icon 时为图标边长
	Weight int32   `json:"weight,omitempty"` // 字重 400 / 600 / 700
	Color  string  `json:"color,omitempty"`

	// ---- 布局方向与对齐 ----
	Dir     string  `json:"dir,omitempty"`     // row / col：覆盖容器的默认方向
	Align   string  `json:"align,omitempty"`   // start / center / end / stretch
	Justify string  `json:"justify,omitempty"` // start / center / end / stretch
	Clip    bool    `json:"clip,omitempty"`
	Opacity float32 `json:"opacity,omitempty"`
}

// Actions 事件名 → 回调。未注册的事件名不会绑定（不报错，便于先搭界面后接逻辑）。
type Actions map[string]func()

// Build 把声明树构造成 guis/ui 节点树。
func Build(root Node, actions Actions) *ui.Node {
	return build(root, actions, "0")
}

func build(d Node, actions Actions, path string) *ui.Node {
	kids := make([]*ui.Node, 0, len(d.Children))
	for i, c := range d.Children {
		kids = append(kids, build(c, actions, path+"."+itoa(i)))
	}
	t := strings.ToLower(strings.TrimSpace(d.Type))
	var n *ui.Node
	switch t {
	case "", "box", "vbox":
		n = ui.Box(kids...)
	case "col", "column":
		n = ui.Column(kids...)
	case "row", "hbox":
		n = ui.Row(kids...)
	case "center":
		n = ui.Center(first(kids))
	case "stack":
		n = ui.Stack(kids...)
	case "grid":
		n = ui.Grid(kids...)
	case "wrap":
		n = ui.Wrap(kids...)
	case "scroll":
		n = ui.Scroll(kids...)
	case "spacer":
		n = ui.Spacer()
	case "text":
		n = ui.Text(d.Text)
	case "button":
		// 框架的 ui.Button 只给了高度与最小宽度，没有字号 / 文字色（不设就画不出字），
		// 这里补一套开箱可用的默认样式；描述文件里写了就以下写的为准。
		d = withButtonDefaults(d)
		n = ui.Button(d.Text, nil)
	case "icon":
		size := d.Size
		if size <= 0 {
			size = 18
		}
		n = ui.IconSized(d.Icon, size)
	case "image":
		n = ui.Image(d.Text)
	default:
		// 未知类型不炸界面：渲染成一行显眼的提示，改文件即恢复。
		n = ui.Text("[未知节点类型: " + t + "]")
	}
	if n == nil {
		n = ui.Box()
	}
	if d.Dir != "" {
		if dir, ok := dirOf(d.Dir); ok {
			n.Dir = dir
		}
	}
	applyStyle(n, d)
	// Key：显式指定优先；滚动 / 按钮这类需要跨帧状态的节点没写 Key 时，
	// 用路径生成稳定 Key，避免每次编辑描述文件都把滚动位置重置掉。
	if k := strings.TrimSpace(d.Key); k != "" {
		n.K(k)
	} else if n.Kind == ui.KScroll || n.Kind == ui.KButton {
		n.K("uidsl-" + path)
	}
	if name := strings.TrimSpace(d.OnClick); name != "" && actions != nil {
		n.OnClick = actions[name]
	}
	return n
}

// withButtonDefaults 给按钮补默认外观：字号、文字色、底色、圆角、内边距、高度。
//
// 框架的 ui.Button 只设了 Height / MinW / CenterY，没设字号与文字色，
// 而绘制文本依赖这两项——不补齐的话按钮是「看不见」的（只能靠 MinW 占位）。
func withButtonDefaults(d Node) Node {
	if d.Size <= 0 {
		d.Size = 14
	}
	if d.Weight == 0 {
		d.Weight = 500
	}
	if d.Color == "" {
		d.Color = "#e6edf7"
	}
	if d.BG == "" && d.BGFrom == "" {
		d.BG = "#2b3a55"
	}
	if d.R <= 0 {
		d.R = 8
	}
	if d.PadX <= 0 {
		d.PadX = 14
	}
	if d.H <= 0 {
		d.H = 34
	}
	return d
}

// applyStyle 把声明里的样式字段落到节点上（值为零表示不设置）。
func applyStyle(n *ui.Node, d Node) {
	st := ui.St()
	if d.W > 0 {
		st.W(d.W)
	}
	if d.H > 0 {
		st.H(d.H)
	}
	if d.Wp > 0 {
		st.Wp(d.Wp)
	}
	if d.Hp > 0 {
		st.Hp(d.Hp)
	}
	if d.MinW > 0 {
		st.MinWidth(d.MinW)
	}
	if d.MaxW > 0 {
		st.MaxWidth(d.MaxW)
	}
	if d.MinH > 0 {
		st.MinHeight(d.MinH)
	}
	if d.MaxH > 0 {
		st.MaxHeight(d.MaxH)
	}
	if d.Pad > 0 {
		st.Padding(d.Pad)
	}
	if d.PadX > 0 || d.PadY > 0 {
		st.PadXY(d.PadX, d.PadY)
	}
	if d.Mar > 0 {
		st.Mar(d.Mar)
	}
	if d.MarX > 0 || d.MarY > 0 {
		st.MarXY(d.MarX, d.MarY)
	}
	if d.Gap > 0 {
		st.GapOf(d.Gap)
	}
	if d.Grow != 0 {
		st.GrowTo(d.Grow)
	}
	if d.Cols > 0 {
		st.ColsN(d.Cols)
	}
	if d.ColW > 0 {
		st.ColWidth(d.ColW)
	}
	if d.BGFrom != "" && d.BGTo != "" {
		st.BgGrad(d.BGFrom, d.BGTo, d.BGAngle)
	} else if d.BG != "" {
		st.BgC(d.BG)
	}
	if d.R > 0 {
		st.R(d.R)
	}
	if d.Stroke > 0 {
		c := d.StrokeColor
		if c == "" {
			c = "#3b4557"
		}
		st.Stroke(d.Stroke, c)
	}
	if d.ShadowBlur > 0 || d.ShadowY != 0 || d.ShadowSpread != 0 {
		c := d.ShadowColor
		if c == "" {
			c = "rgba(0,0,0,0.35)"
		}
		st.Shadow(d.ShadowBlur, d.ShadowY, d.ShadowSpread, c)
	}
	if d.Size > 0 || d.Weight != 0 || d.Color != "" {
		st.Text(d.Size, d.Weight, d.Color)
	}
	if a, ok := alignOf(d.Align); ok {
		n.Align = a
	}
	if a, ok := alignOf(d.Justify); ok {
		n.Justify = a
	}
	if d.Clip {
		st.Clipped()
	}
	if d.Opacity > 0 {
		st.Opacity = d.Opacity
	}
	n.St(st)
}

func first(kids []*ui.Node) *ui.Node {
	if len(kids) > 0 {
		return kids[0]
	}
	return ui.Box()
}

func alignOf(s string) (ui.Align, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "start", "left", "top":
		return ui.AlignStart, true
	case "center", "middle":
		return ui.AlignCenter, true
	case "end", "right", "bottom":
		return ui.AlignEnd, true
	case "stretch", "fill":
		return ui.AlignStretch, true
	}
	return ui.AlignStretch, false
}

func dirOf(s string) (ui.Direction, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "row", "h", "horizontal":
		return ui.Horizontal, true
	case "col", "column", "v", "vertical":
		return ui.Vertical, true
	}
	return ui.Vertical, false
}

// itoa 不引入 strconv 之外的依赖，路径拼接用。
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
