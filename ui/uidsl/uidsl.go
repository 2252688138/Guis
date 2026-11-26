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
// spacer / text / button / icon / image / input / textarea / checkbox / switch /
// slider / progress。
// 样式字段与 ui.St() 的方法一一对应，未填（0 / 空串）即不设置。
type Node struct {
	Type     string `json:"type"`
	Key      string `json:"key,omitempty"`
	Text     string `json:"text,omitempty"`    // 文本内容 / 按钮文案 / 图片路径
	Icon     string `json:"icon,omitempty"`    // type=icon 时的图标名
	OnClick     string  `json:"onClick,omitempty"`     // 点击事件名，由 Actions 解析
	OnChange    string  `json:"onChange,omitempty"`    // 值变化事件名（checkbox/switch → bool，input → string，slider → number）
	OnSubmit    string  `json:"onSubmit,omitempty"`    // 输入框回车事件名（参数 string）
	Value       string  `json:"value,omitempty"`       // input / textarea 的当前值
	Placeholder string  `json:"placeholder,omitempty"` // 输入框占位文字
	Checked     bool    `json:"checked,omitempty"`     // checkbox / switch 的开关状态
	Pct         float32 `json:"pct,omitempty"`         // slider / progress 的进度（0~1）
	Children    []Node  `json:"children,omitempty"`

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

	// ---- 更多事件（写函数名或事件名，未注册的事件名不绑定、不报错）----
	OnKey      string `json:"onKey,omitempty"`      // 按键（键码）
	OnKeyUp    string `json:"onKeyUp,omitempty"`    // 抬键（键码）
	OnChar     string `json:"onChar,omitempty"`     // 字符输入
	OnHover    string `json:"onHover,omitempty"`    // 悬停进入/离开（bool）
	OnScroll   string `json:"onScroll,omitempty"`   // 滚轮（dy）
	OnDrag     string `json:"onDrag,omitempty"`     // 拖拽（x, y）
	OnDrop     string `json:"onDrop,omitempty"`     // 文件拖放（路径数组）
	OnPress    string `json:"onPress,omitempty"`    // 按下
	OnActivate string `json:"onActivate,omitempty"` // 激活（回车 / 空格）
	OnClickAt  string `json:"onClickAt,omitempty"`  // 带坐标的点击（x, y）
	OnCancel   string `json:"onCancel,omitempty"`   // confirm 取消回调

	// ---- 扩展组件数据 ----
	Label    string      `json:"label,omitempty"`    // chip 文本
	Count    int         `json:"count,omitempty"`    // badge 数字
	Lang     string      `json:"lang,omitempty"`     // codeeditor 语言标识
	Open     bool        `json:"open,omitempty"`     // alert / confirm / drawer 是否显示
	DlgKind  string      `json:"dlgKind,omitempty"`  // alert / confirm 类型：info / warn / error / success
	Title    string      `json:"title,omitempty"`    // 对话框 / drawer 标题
	Body     string      `json:"body,omitempty"`     // 对话框正文 / drawer 内容
	OKLabel  string      `json:"okLabel,omitempty"`  // 确认按钮文案
	CType    string      `json:"ctype,omitempty"`    // chart 类型：line / area / bar / pie / donut
	Labels   []string    `json:"labels,omitempty"`   // chart X 轴 / segmented 选项键
	Series   []SeriesDef `json:"series,omitempty"`   // chart 数据系列
	Items    []OptionDef `json:"items,omitempty"`    // select / segmented 选项
	Active   string      `json:"active,omitempty"`   // segmented 当前选中值
	Spans    []SpanDef   `json:"spans,omitempty"`    // richtext 片段
	Src      string      `json:"src,omitempty"`      // markdown / image 的内容或路径
	ReadOnly bool        `json:"readonly,omitempty"` // codeeditor 只读

	// ---- 第三阶段：动画 / 终端 / 媒体 / 浏览器 / 菜单栏 ----
	Anim      []AnimDef   `json:"anim,omitempty"`      // 节点动画（可多条叠加）
	Lines     []string    `json:"lines,omitempty"`     // term 行内容
	Menus     []MenuDefDef `json:"menus,omitempty"`    // menubar 菜单定义
	HTML      string      `json:"html,omitempty"`      // webview 直接加载 HTML
	Zoom      float64     `json:"zoom,omitempty"`      // webview 缩放
	UserAgent string      `json:"userAgent,omitempty"` // webview 自定义 UA
	UserDataDir string    `json:"userDataDir,omitempty"` // webview 用户数据目录
	Shapes    []CanvasShapeDef `json:"shapes,omitempty"` // canvas 绘制指令
}

// Actions 事件名 → 回调。未注册的事件名不会绑定（不报错，便于先搭界面后接逻辑）。
type Actions map[string]func()

// Handlers 事件表：点击、值变化、回车提交。
//
// Change 的参数随控件而定：checkbox / switch → bool，input / textarea → string，
// slider → float32；未注册的事件名不绑定（与 Actions 一致）。
type Handlers struct {
	Click    map[string]func()
	Change   map[string]func(any)
	Submit   map[string]func(string)
	Key      map[string]func(uint32)
	KeyUp    map[string]func(uint32)
	Char     map[string]func(rune)
	Hover    map[string]func(bool)
	Scroll   map[string]func(float32)
	Drag     map[string]func(x, y float32)
	Drop     map[string]func([]string)
	Press    map[string]func()
	Activate map[string]func()
	ClickAt  map[string]func(x, y float32)
}

// SeriesDef 图表数据系列（脚本 / JSON 描述用）。
type SeriesDef struct {
	Name   string    `json:"name,omitempty"`
	Color  string    `json:"color,omitempty"`
	Values []float64 `json:"values,omitempty"`
}

// OptionDef 下拉框 / 分段控件的选项。
type OptionDef struct {
	Value    string `json:"value,omitempty"`
	Label    string `json:"label,omitempty"`
	Hint     string `json:"hint,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// SpanDef 富文本片段。
type SpanDef struct {
	Text      string  `json:"text,omitempty"`
	Color     string  `json:"color,omitempty"`
	BgColor   string  `json:"bgColor,omitempty"`
	Size      float32 `json:"size,omitempty"`
	Weight    int32   `json:"weight,omitempty"`
	Link      string  `json:"link,omitempty"`
	Underline bool    `json:"underline,omitempty"`
	Strike    bool    `json:"strike,omitempty"`
	Code      bool    `json:"code,omitempty"`
}

// Build 把声明树构造成 guis/ui 节点树。
func Build(root Node, actions Actions) *ui.Node {
	return BuildWith(root, Handlers{Click: actions})
}

// BuildWith 同 Build，但事件表支持带值回调（输入 / 勾选 / 滑块）。
func BuildWith(root Node, h Handlers) *ui.Node {
	return build(root, h, "0")
}

func build(d Node, h Handlers, path string) *ui.Node {
	kids := make([]*ui.Node, 0, len(d.Children))
	for i, c := range d.Children {
		kids = append(kids, build(c, h, path+"."+itoa(i)))
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
	case "input", "textfield", "field":
		n = ui.Input(d.Key, d.Value, d.Placeholder, changeStr(d, h))
	case "textarea":
		n = ui.Textarea(d.Key, d.Value, d.Placeholder, d.H, changeStr(d, h))
	case "checkbox", "check":
		n = ui.Checkbox(d.Key, d.Text, d.Checked, changeBool(d, h))
	case "switch", "toggle":
		n = ui.Switch(d.Key, d.Checked, changeBool(d, h))
	case "slider":
		n = ui.Slider(d.Key, d.Pct, changeNum(d, h))
	case "progress", "bar":
		n = ui.Progress(d.Pct, d.Color)
	case "divider":
		n = ui.Divider()
	case "card":
		n = ui.Card(kids...)
	case "badge":
		n = ui.Badge(d.Count)
	case "avatar":
		n = ui.Avatar(d.Text, d.Color)
	case "chip":
		n = ui.Chip(d.Label, d.Checked, clickOf(d.OnClick, h))
	case "spinner":
		n = ui.Spinner(d.Size, d.Color)
	case "skeleton":
		n = ui.Skeleton(d.W, d.H)
	case "markdown":
		n = ui.Markdown(d.Key, markdownSource(d))
	case "richtext":
		n = ui.RichText(toSpans(d.Spans), ui.RichTextOptions{
			Size: d.Size, Weight: d.Weight, Color: d.Color,
		})
	case "masonry":
		n = ui.Masonry(kids...)
	case "arrow":
		n = ui.Arrow(d.Color)
	case "segmented":
		keys, labels := optionKeys(d.Items, d.Labels)
		n = ui.Segmented(keys, labels, d.Active, submitOf(d.OnSubmit, h))
	case "select":
		n = ui.SelectBox(selectStateFor(d, path), toOptions(d.Items), d.Value, d.Placeholder, changeStr(d, h))
	case "chart":
		n = &ui.Node{Kind: ui.KChart, Chart: ui.NewChart(chartTypeOf(d.CType), d.Labels, toSeries(d.Series)...)}
	case "codeeditor":
		n = ui.CodeEditor(codeStateFor(d, path))
	case "alert":
		n = ui.AlertDlg(d.Key, d.Open, dlgKindOf(d.DlgKind), d.Title, d.Body, clickOf(d.OnClick, h))
	case "confirm":
		n = ui.ConfirmDlg(d.Key, d.Open, dlgKindOf(d.DlgKind), d.Title, d.Body, d.OKLabel,
			clickOf(d.OnCancel, h), clickOf(d.OnClick, h))
	case "drawer":
		n = ui.Drawer(ui.DrawerOpts{Key: d.Key, Open: d.Open, Title: d.Title, Body: first(kids), Width: d.W,
			OnClose: clickOf(d.OnClick, h)})
	case "term":
		n = ui.Terminal(termStateFor(d, path))
	case "video":
		n = videoNode(d, path)
	case "webview":
		n = webviewNode(d, path)
	case "menubar":
		n = ui.MenuBar(menuBarStateFor(d, path), toMenuDefs(d.Menus, h))
	case "viewport":
		n = viewportNode(d, path)
	case "canvas":
		n = ui.Canvas(canvasStateFor(d, path), toCanvasShapes(d.Shapes),
			ui.CanvasOptions{W: d.W, H: d.H, BG: d.BG})
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
	} else if n.Kind == ui.KScroll || n.Kind == ui.KButton || keyedKind(t) {
		n.K("uidsl-" + path)
	}
	if name := strings.TrimSpace(d.OnClick); name != "" && h.Click != nil {
		n.OnClick = h.Click[name]
	}
	if name := strings.TrimSpace(d.OnKey); name != "" && h.Key != nil {
		n.OnKey = h.Key[name]
	}
	if name := strings.TrimSpace(d.OnKeyUp); name != "" && h.KeyUp != nil {
		n.OnKeyUp = h.KeyUp[name]
	}
	if name := strings.TrimSpace(d.OnChar); name != "" && h.Char != nil {
		n.OnChar = h.Char[name]
	}
	if name := strings.TrimSpace(d.OnHover); name != "" && h.Hover != nil {
		n.OnHover = h.Hover[name]
	}
	if name := strings.TrimSpace(d.OnScroll); name != "" && h.Scroll != nil {
		n.OnScroll = h.Scroll[name]
	}
	if name := strings.TrimSpace(d.OnDrag); name != "" && h.Drag != nil {
		n.OnDrag = h.Drag[name]
	}
	if name := strings.TrimSpace(d.OnDrop); name != "" && h.Drop != nil {
		n.OnDrop = h.Drop[name]
	}
	if name := strings.TrimSpace(d.OnPress); name != "" && h.Press != nil {
		n.OnPress = h.Press[name]
	}
	if name := strings.TrimSpace(d.OnActivate); name != "" && h.Activate != nil {
		n.OnActivate = h.Activate[name]
	}
	if name := strings.TrimSpace(d.OnClickAt); name != "" && h.ClickAt != nil {
		n.OnClickAt = h.ClickAt[name]
	}
	if len(d.Anim) > 0 {
		ui.Animate(n, toAnimSpecs(d.Anim)...)
	}
	return n
}

// keyedKind 需要跨帧记住状态的控件（没写 key 时用路径兵底）。
func keyedKind(t string) bool {
	switch t {
	case "input", "textfield", "field", "textarea", "checkbox", "check", "switch", "toggle", "slider":
		return true
	}
	return false
}

// changeStr / changeBool / changeNum 把带值事件转成控件要的回调（未注册返回 nil）。
func changeStr(d Node, h Handlers) func(string) {
	fn := h.Change[d.OnChange]
	if fn == nil {
		return nil
	}
	return func(v string) { fn(v) }
}

func changeBool(d Node, h Handlers) func(bool) {
	fn := h.Change[d.OnChange]
	if fn == nil {
		return nil
	}
	return func(v bool) { fn(v) }
}

func changeNum(d Node, h Handlers) func(float32) {
	fn := h.Change[d.OnChange]
	if fn == nil {
		return nil
	}
	return func(v float32) { fn(v) }
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
