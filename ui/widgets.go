// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"strconv"
	"strings"

	"guis/internal/gpu"
)

// 组件库：在基础节点（Box/Row/Column/Stack/Text/Button）之上组合出的控件。
// 约定：所有可选/可编辑状态由调用方持有，回调内修改状态并调用 ui.RequestRedraw()。
// 交互能力对应 gio 的 widget：Clickable（Tap/TapAt/DragEvt）、Editor（Input）、
// List（Scroll 自带滚动条）、Switch/CheckBox/RadioButton、Slider、Table、Tooltip 等。

// 主题 token：定义在 theme.go（深/浅色两套色板 + SetTheme 切换）。
// 组件里直接用 ColText / ColBg / ColInput / ColPanel 等变量，不要写死颜色，
// 否则切浅色主题时会出现“白底深卡”。

// ---- 基础排版 ----

// Divider 分隔线。
func Divider() *Node {
	return Box().St(St().Wp(100).H(1).BgC(ColLine))
}

// Title 区块标题（带副标题可选）。
func Title(text, sub string) *Node {
	col := Column(Text(text).St(St().Text(16, 600, ColText)))
	if sub != "" {
		col.Add(Text(sub).St(St().Text(12, 400, ColTextMuted).PadOnly(4, 0, 0, 0)))
	}
	return col
}

// Card 卡片容器。
func Card(children ...*Node) *Node {
	return Column(children...).St(St().Padding(20).R(16).GapOf(12).
		BgC(ColBg).Stroke(1, ColStroke).Shadow(20, 8, 0, "rgba(0,0,0,0.32)"))
}

// Tag 标签 / 徽章文本。
func Tag(label, color string) *Node {
	return Text(label).St(St().Text(11, 600, color).PadXY(9, 4).R(7).
		BgC(ColHover).Stroke(1, ColLine))
}

// Dot 状态圆点。
func Dot(color string, size float32) *Node {
	return Box().St(St().W(size).H(size).R(size / 2).BgC(color))
}

// Avatar 圆形头像（文字首字）。
func Avatar(initial, color string) *Node {
	return Stack(
		Box().St(St().AbsFill().R(20).BgGrad(color, "rgba(255,255,255,0.06)", 120)),
		Text(initial).St(St().AbsFill().Text(14, 600, "#ffffff").CenterYOn().TAlign(AlignCenter)),
	).St(St().W(40).H(40))
}

// Badge 角标数字（附着在父节点右上角，需父节点为 Stack）。
func Badge(count int) *Node {
	label := strconv.Itoa(count)
	if count > 99 {
		label = "99+"
	}
	return Text(label).St(St().Text(10, 700, "#ffffff").PadXY(6, 2).R(9).
		BgC(ColDanger).AbsTR(-6, -6))
}

// Link 文字链接。
func Link(label string, onClick func()) *Node {
	return Text(label).St(St().Text(12.5, 500, "#8b93ff")).Tap(func() {
		if onClick != nil {
			onClick()
		}
		RequestRedraw()
	})
}

// KeyValue 左标题右值的一行。
func KeyValue(k, v string) *Node {
	// 窄屏（手机逻辑宽度 ≈ 480）：左右排会把 value 挤到屏幕外被裁掉，
	// 改成上下堆叠 + 值折行。桌面窗口（≥640）保持原样。
	if ViewportW() < 640 {
		return Column(
			Text(k).St(St().Text(11.5, 400, ColTextMuted)),
			Text(v).St(St().Text(12.5, 500, ColTextDim).WrapOn()),
		).St(St().GapOf(2).PadOnly(0, 0, 6, 0))
	}
	return Row(
		Text(k).St(St().Text(12, 400, ColTextMuted)),
		Spacer(),
		Text(v).St(St().Text(12.5, 500, ColTextDim)),
	).St(St().PadOnly(0, 0, 6, 0)).AlignTo(AlignCenter)
}

// Empty 空状态占位。
func Empty(title, sub string) *Node {
	return Column(
		Stack(
			Box().St(St().W(54).H(54).R(16).BgC(ColHover).Stroke(1, ColStroke)),
			Text("—").St(St().AbsFill().Text(20, 500, ColPlaceholder).CenterYOn().TAlign(AlignCenter)),
		).St(St().W(54).H(54)),
		Text(title).St(St().Text(13.5, 600, ColText).PadOnly(14, 0, 0, 0)),
		Text(sub).St(St().Text(11.5, 400, ColTextMuted).WrapOn().TAlign(AlignCenter)),
	).St(St().GapOf(6).Padding(28)).AlignTo(AlignCenter)
}

// ---- 按钮 ----

// ButtonKind 按钮变体。
type ButtonKind int

const (
	BtnPrimary   ButtonKind = iota // 主行动：渐变实心
	BtnSecondary                   // 次要：深色描边
	BtnGhost                       // 幽灵：无背景
	BtnDanger
	BtnSuccess
)

// Btn 带变体与禁用态的按钮。
func Btn(label string, kind ButtonKind, onClick func()) *Node {
	st := St().Text(13, 600, "#ffffff").PadXY(16, 8).R(9)
	switch kind {
	case BtnPrimary:
		st.BgGrad(ColPrimary, ColPrimary2, 90)
	case BtnSecondary:
		st.Text(13, 500, ColText).BgC(ColBgSoft).Stroke(1, ColStroke)
	case BtnGhost:
		st.Text(13, 500, ColTextMuted)
	case BtnDanger:
		st.BgGrad("#ef4444", "#f87171", 90)
	case BtnSuccess:
		st.BgGrad("#10b981", "#34d399", 90)
	}
	return Button(label, onClick).St(st)
}

// IconButton 方形图标按钮（glyph 建议用 + - × → ← ↑ ↓ 等有字形符号）。
func IconButton(glyph string, onClick func()) *Node {
	return Button(glyph, onClick).St(St().W(34).H(34).MinWidth(34).
		Text(14, 600, ColTextDim).R(9).BgC(ColBgSoft).Stroke(1, ColStroke))
}

// ---- 表单控件 ----

// Checkbox 复选框。
func Checkbox(key, label string, checked bool, onChange func(bool)) *Node {
	box := ColHover
	stroke := ColStroke
	mark := ""
	if checked {
		box = ColPrimary
		stroke = "#8b93ff"
		mark = "√"
	}
	indicator := Stack(
		Box().St(St().AbsFill().R(6).BgC(box).Stroke(1.5, stroke)),
		Text(mark).St(St().AbsFill().Text(12, 700, "#ffffff").CenterYOn().TAlign(AlignCenter)),
	).St(St().W(18).H(18))
	row := Row(indicator, Text(label).St(St().Text(13, 500, ColTextDim))).
		St(St().GapOf(10)).AlignTo(AlignCenter)
	return row.K(key).Tap(func() {
		checked = !checked
		if onChange != nil {
			onChange(checked)
		}
		RequestRedraw()
	})
}

// Radio 单选按钮。
func Radio(key, label string, selected bool, onPick func()) *Node {
	outer := ColLine
	inner := "transparent"
	if selected {
		outer = ColPrimary
		inner = ColPrimary
	}
	ind := Stack(
		Box().St(St().AbsFill().R(9).Stroke(1.5, outer)),
		Box().St(St().W(8).H(8).R(4).BgC(inner).AbsCenter()),
	).St(St().W(18).H(18))
	row := Row(ind, Text(label).St(St().Text(13, 500, ColTextDim))).
		St(St().GapOf(10)).AlignTo(AlignCenter)
	return row.K(key).Tap(func() {
		if onPick != nil {
			onPick()
		}
		RequestRedraw()
	})
}

// switchAnim 记录开关切换时刻，用于旋钮过渡动画。
var switchAnim = map[string]float32{}

// Switch 开关（轨道 + 旋钮，切换时 140ms 过渡）。
func Switch(key string, on bool, onChange func(bool)) *Node {
	const trackW, trackH, knob float32 = 44, 24, 18
	t := float32(1)
	if start, ok := switchAnim[key]; ok {
		t = (AnimTime() - start) / 0.14
		if t >= 1 {
			t = 1
		}
	}
	if t < 1 {
		RequestRedraw()
	}
	target := float32(0)
	if on {
		target = 1
	}
	// 由目标值倒推动画起点：t=0 时位于旧位置
	from := target
	if t < 1 {
		from = 1 - target
	}
	k := from + (target-from)*easeOut(t)
	offX := 3 + k*(trackW-knob-6)
	bg := Blend(C(ColTrack), C(ColPrimary), k)
	knobC := Blend(C("#8b93ad"), C("#ffffff"), k)

	n := Stack(
		Box().St(St().AbsFill().R(trackH/2).BgC(bg)),
		Box().St(St().W(knob).H(knob).R(knob/2).BgC(knobC).
			Shadow(8, 2, 0, "rgba(0,0,0,0.45)").AbsTL(offX, (trackH-knob)/2)),
	).St(St().W(trackW).H(trackH))
	if t < 1 {
		n.St(St().AnimOn())
	}
	return n.K(key).Tap(func() {
		switchAnim[key] = AnimTime()
		on = !on
		if onChange != nil {
			onChange(on)
		}
		RequestRedraw()
	})
}

func easeOut(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	u := 1 - t
	return 1 - u*u*u
}

// Slider 滑块（0~1，支持点击定位与拖动，拖动过程中实时跟手）。
func Slider(key string, v float32, onChange func(float32)) *Node {
	const trackW, trackH, thumb float32 = 200, 18, 14
	usable := trackW - thumb
	// 轨道与滑块都按容器高度垂直居中（轨道 4px 中心、滑块 14px 中心同在 trackH/2）
	trackY := (trackH - 4) / 2
	thumbY := (trackH - thumb) / 2
	set := func(x float32) {
		nv := (x - thumb/2) / usable
		if nv < 0 {
			nv = 0
		}
		if nv > 1 {
			nv = 1
		}
		if onChange != nil {
			onChange(nv)
		}
		RequestRedraw()
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	n := Stack(
		Box().St(St().W(trackW).H(4).R(2).BgC(ColTrack).AbsTL(0, trackY)),
		Box().St(St().W(v*usable+thumb/2).H(4).R(2).BgGrad(ColPrimary, ColPrimary2, 90).AbsTL(0, trackY)),
		Box().St(St().W(thumb).H(thumb).R(thumb/2).BgC("#ffffff").
			Stroke(3, ColPrimary).Shadow(10, 2, 0, "rgba(99,102,241,0.55)").
			AbsTL(v*usable, thumbY)),
	).St(St().W(trackW).H(trackH))
	return n.K(key).TapAt(func(x, _ float32) { set(x) }).DragEvt(func(x, _ float32) { set(x) })
}

// Progress 进度条（0~1）。宽度由父容器决定（Column 拉伸 / Row 内 Grow），
// 不写死 100% 宽度，避免在不定宽容器里按父级满宽测量而溢出。
func Progress(v float32, color string) *Node {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return Stack(
		Box().St(St().AbsFill().R(4).BgC(ColTrack)),
		Box().St(St().Wp(v*100).Hp(100).R(4).BgGrad(color, "#ffffff33", 90)),
	).St(St().H(8))
}

// Ring 环形进度（分段圆点近似，gio 风格的小尺寸指示器）。
func Ring(v float32, size float32, color string) *Node {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	const segments = 40
	rad := float64(size/2 - 3)
	items := make([]*Node, 0, segments)
	for i := 0; i < segments; i++ {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/segments
		x := float32(float64(size)/2+rad*math.Cos(ang)) - 2
		y := float32(float64(size)/2+rad*math.Sin(ang)) - 2
		c := ColHover
		if float32(i) < v*segments {
			c = color
		}
		items = append(items, Box().St(St().W(4).H(4).R(2).BgC(c).AbsTL(x, y)))
	}
	return Stack(items...).St(St().W(size).H(size))
}

// ---- 输入框（gio Editor 的简化版） ----

// inputCaret 记录每个输入框的光标位置（按 rune）。
var inputCaret = map[string]int{}

// inputShadow 记录每个输入框当前文本（按 key）。
//
// 只用于移动端输入法（IME）同步：组件把文本放在闭包里，帧外无法读取，
// 所以渲染时留一份影子；输入法上屏后按「影子 → 新文本」的差异注入焦点控件。
var inputShadow = map[string]string{}

// inputKind 输入框类型：0=单行 1=多行 2=密码（决定弹出的键盘类型）。
var inputKind = map[string]int{}

// inputLeftPad 输入框内容左侧额外留白（px）：搜索框这类需要在框内左侧画图标的组件用，
// 文字与光标定位同时右移，避免图标压在文字上。
var inputLeftPad = map[string]float32{}

// inputSubmit 记录每个输入框的回车提交回调（地址栏等需要「回车即执行」的场景）。
var inputSubmit = map[string]func(){}

// inputKeyHandlers / inputCharHandlers 记录输入框自身的按键/字符处理，
// 供 AutoComplete / TagInput 这类需要在保留光标编辑的前提下追加按键语义的组件复用。
var (
	inputKeyHandlers  = map[string]func(uint32){}
	inputCharHandlers = map[string]func(rune){}
)

// callInputKey 转交给输入框自身的光标/删除处理。
func callInputKey(key string, k uint32) {
	if fn := inputKeyHandlers[key]; fn != nil {
		fn(k)
	}
}

// callInputChar 转交给输入框自身的字符插入处理。
func callInputChar(key string, r rune) {
	if fn := inputCharHandlers[key]; fn != nil {
		fn(r)
	}
}

// InputSubmit 带回车提交的单行输入框：回车触发 onSubmit（地址栏、搜索框用）。
func InputSubmit(key, value, placeholder string, onChange func(string), onSubmit func()) *Node {
	if onSubmit != nil {
		inputSubmit[key] = onSubmit
	}
	return inputNode(key, value, placeholder, false, onChange)
}

// Input 单行输入框：点击获焦、光标闪烁、左右/Home/End/退格/删除。
func Input(key, value, placeholder string, onChange func(string)) *Node {
	return inputNode(key, value, placeholder, false, onChange)
}

// InputH 指定高度的单行输入框（默认 38；紧凑工具栏 / 筛选栏用）。
func InputH(key, value, placeholder string, h float32, onChange func(string)) *Node {
	return inputNodeH(key, value, placeholder, false, h, onChange)
}

// PasswordInput 密码输入框（掩码显示）。
func PasswordInput(key, value, placeholder string, onChange func(string)) *Node {
	return inputNode(key, value, placeholder, true, onChange)
}

func inputNode(key, value, placeholder string, password bool, onChange func(string)) *Node {
	return inputNodeH(key, value, placeholder, password, 38, onChange)
}

// inputNodeH 输入框实现（h 为整体高度，光标高度随之收敛）。
func inputNodeH(key, value, placeholder string, password bool, h float32, onChange func(string)) *Node {
	if h <= 0 {
		h = 38
	}
	caretH := h - 22
	if caretH < 12 {
		caretH = 12
	}
	if caretH > 16 {
		caretH = 16
	}
	focused := IsFocused(key)
	// 影子文本 / 类型：移动端输入法同步用（见 ui/ime_android.go）
	inputShadow[key] = value
	if password {
		inputKind[key] = 2
	} else {
		inputKind[key] = 0
	}
	sel := inputTextSel(key, value)
	runes, caret := sel.Runes, sel.Caret
	inputCaret[key], inputAnchor[key] = sel.Caret, sel.Anchor

	// 显示文本：密码掩码
	display := value
	if password && len(runes) > 0 {
		display = strings.Repeat("•", len(runes))
	}

	stroke := ColStroke
	if focused {
		stroke = ColPrimary
	}

	// segCells 把 [start,end) 文本按选区切段（密码框用掩码等长替换）
	segCells := func(start, end int) []*Node {
		segs := sel.SelLineSegments(start, end)
		out := make([]*Node, 0, len(segs)+1)
		for _, sg := range segs {
			txt := sg.Text
			if password {
				txt = strings.Repeat("•", len([]rune(txt)))
			}
			st := St().Text(13, 400, ColText)
			if sg.Sel {
				st.BgC(paintWithAlpha(ColPrimary, 0.30))
			}
			out = append(out, Text(txt).St(st))
		}
		return out
	}

	var content *Node
	if focused {
		// 光标闪烁：1.0s 周期，动画节点驱动逐帧重绘
		showCaret := int(AnimTime()*2)%2 == 0
		cells := segCells(0, caret)
		// IME 组合串：高亮背景显示，光标不闪烁（输入法正在编辑）
		if comp := ComposingText(); comp != "" && !password {
			cp := ComposingCaret()
			cr := []rune(comp)
			if cp < 0 || cp > len(cr) {
				cp = len(cr)
			}
			cells = append(cells, Text(string(cr[:cp])).St(St().Text(13, 400, ColText).BgC(ColBgSoft).R(3)))
			cells = append(cells, Text(string(cr[cp:])).St(St().Text(13, 400, ColText).BgC(ColHover).R(3)))
			showCaret = false
		}
		if showCaret {
			cells = append(cells, Box().St(St().W(1.5).H(caretH).BgC(ColPrimary).MarXY(0.5, 0)))
		}
		cells = append(cells, segCells(caret, len(runes))...)
		content = Row(cells...).St(St().AnimSlowOn())
	} else if value == "" {
		content = Row(Text(placeholder).St(St().Text(13, 400, ColPlaceholder)))
	} else {
		content = Row(Text(display).St(St().Text(13, 400, ColText)))
	}

	box := Stack(
		Row(content).St(St().AbsFill().PadOnly(0, 12, 0, 12+inputLeftPad[key])).AlignTo(AlignCenter),
	).St(St().Wp(100).H(h).R(10).BgC(ColInput).Stroke(1, stroke).FocusOn().Clipped())

	n := box.K(key)
	// commit 把选区状态写回帧间存储并通知业务
	commit := func() {
		inputCaret[key] = sel.Caret
		inputAnchor[key] = sel.Anchor
		value = sel.Text()
		if onChange != nil {
			onChange(value)
		}
		RequestRedraw()
	}
	// caretAt 把节点内横坐标换算成光标位置（文本起点 padX=12，搜索框额外留白计入）
	caretAt := func(px, _ float32) int {
		return sel.IndexAtX(px-12-inputLeftPad[key], 13, 400)
	}
	n.OnClick = func() {
		SetFocus(key)
		// 刚拖选完的松手：OnClick 总会在 mouseup 后触发一次，不能在这里折叠选区
		if inputDragged[key] {
			inputDragged[key] = false
			return
		}
		sel.SetCaret(caretAtMouse(key, caretAt, sel.Caret), false)
		commit()
	}
	// 按下即定位光标并从该点开始拖选（App 只在 OnDrag != nil 时建立拖拽目标）
	n.OnPress = func() {
		inputDragged[key] = false
		// 按下即获焦：拖选过程中就要显示高亮（未聚焦只画纯文本）
		SetFocus(key)
		pos := caretAtMouse(key, caretAt, sel.Caret)
		sel.Anchor, sel.Caret = pos, pos
		commit()
	}
	n.OnDrag = func(x, _ float32) {
		inputDragged[key] = true
		sel.Caret = caretAt(x, 0)
		sel.Clamp()
		inputCaret[key], inputAnchor[key] = sel.Caret, sel.Anchor
		RequestRedraw()
	}
	n.OnChar = func(r rune) {
		switch r {
		case '\r', '\n', '\t':
			return
		case 8: // Backspace
			sel.Backspace()
		case 127: // Delete
			sel.DeleteForward()
		default:
			sel.Insert(string(r))
		}
		commit()
	}
	n.OnKey = func(k uint32) {
		shift := ShiftDown()
		ctrl := CtrlDown()
		switch k {
		case VK_BACK: // Backspace（框架不再把 WM_CHAR 0x08 当文本送入，退格走键位）
			sel.Backspace()
		case 0x0D: // Enter：提交（不录入文本，由回调决定动作）
			if fn := inputSubmit[key]; fn != nil {
				fn()
			}
			return
		case 0x25: // Left
			if ctrl {
				sel.SetCaret(wordLeft(sel.Text(), sel.Caret), shift)
			} else {
				sel.Move(-1, shift)
			}
		case 0x27: // Right
			if ctrl {
				sel.SetCaret(wordRight(sel.Text(), sel.Caret), shift)
			} else {
				sel.Move(1, shift)
			}
		case 0x24: // Home
			sel.SetCaret(0, shift)
		case 0x23: // End
			sel.SetCaret(len(sel.Runes), shift)
		case 0x2E: // Delete
			sel.DeleteForward()
		case 0x41, 0x61, 0x43, 0x63, 0x58, 0x78, 0x56, 0x76: // Ctrl+A/C/X/V 兜底
			if !ctrl {
				return
			}
			switch k {
			case 0x41, 0x61:
				sel.SelectAll()
			case 0x43, 0x63:
				sel.Copy()
			case 0x58, 0x78:
				sel.Cut()
			case 0x56, 0x76:
				sel.Paste(false)
			}
		default:
			return
		}
		commit()
	}
	// Ctrl+A/C/X/V：App 在焦点控件上优先分发（否则会被全局「复制选区」先吞掉）
	RegisterTextEdit(key, func(act TextEditAction) bool {
		switch act {
		case TextEditSelectAll:
			sel.SelectAll()
		case TextEditCopy:
			if sel.Selected() != "" {
				return sel.Copy()
			}
			// 没有选区时复制整段文本（搜索框 / 地址栏的常见习惯）
			if sel.Text() == "" {
				return false
			}
			return ClipboardSetText(sel.Text())
		case TextEditCut:
			if !sel.Cut() {
				return false
			}
		case TextEditPaste:
			if !sel.Paste(false) {
				return false
			}
		}
		commit()
		return true
	})
	inputKeyHandlers[key] = n.OnKey
	inputCharHandlers[key] = n.OnChar
	return n
}

// inputAnchor 每个输入框的选区锚点（与 inputCaret 配对；两者相等表示无选区）。
var inputAnchor = map[string]int{}

// inputDragged 标记输入框刚发生拖选：松手后的 OnClick 要跳过，否则选区被折叠。
var inputDragged = map[string]bool{}

// inputTextSel 用输入框当前文本 + 光标 / 锚点构造选区状态。
func inputTextSel(key, value string) *TextSel {
	rs := []rune(value)
	caret, ok := inputCaret[key]
	if !ok || caret > len(rs) || caret < 0 {
		caret = len(rs)
	}
	anchor, ok := inputAnchor[key]
	if !ok || anchor > len(rs) || anchor < 0 {
		anchor = caret
	}
	return NewTextSel(value, caret, anchor)
}

// caretAtMouse 按鼠标位置换算光标（点击定位 / 拖选共用）。
// at 接收「相对节点左上角」的横坐标（多行输入还要纵坐标），换算失败时返回 fallback。
func caretAtMouse(key string, at func(float32, float32) int, fallback int) int {
	x, y := MousePos()
	if r, ok := LastFrameOf(key); ok {
		return at(x-r.X, y-r.Y)
	}
	return fallback
}

// ---- 加载与反馈 ----

// Spinner 旋转加载指示器（逐帧动画）。
func Spinner(size float32, color string) *Node {
	const segments = 8
	center := size / 2
	rad := size/2 - 2
	items := make([]*Node, 0, segments)
	t := AnimTime()
	head := int(t*8) % segments
	for i := 0; i < segments; i++ {
		ang := float64(i) * math.Pi / 4
		x := float32(float64(center)+float64(rad)*math.Cos(ang)) - 2
		y := float32(float64(center)+float64(rad)*math.Sin(ang)) - 2
		alpha := 0.15 + 0.85*float64((i-head+segments)%segments)/float64(segments)
		c := C(color).WithAlpha(float32(alpha))
		items = append(items, Box().St(St().W(4).H(4).R(2).BgC(c).AbsTL(x, y)))
	}
	return Stack(items...).St(St().W(size).H(size))
}

// Skeleton 骨架屏占位（呼吸 + 流光动画）。
func Skeleton(w, h float32) *Node {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 12
	}
	base := 0.06 + 0.03*(1+float32(math.Sin(float64(AnimTime())*2.4)))/2
	p := ShimmerPhase(0.9)
	hlW := w * 0.45
	x := -hlW + p*(w+hlW*2)
	hl := Box().St(St().W(hlW).H(h).R(6).
		BgGrad(whiteA(0), whiteA(0.10), 90).AbsTL(x, 0))
	return Stack(
		Box().St(St().AbsFill().R(6).BgC(whiteA(base))),
		hl,
	).St(St().W(w).H(h).Clipped().AnimSlowOn().NoFocusRing())
}

// Toast 轻提示（悬浮于容器底部中央，需放在 Stack 中）。
// 纯渲染组件：不自动隐藏。需要「显示 N 秒后自动隐藏」请用 ToastShow / ToastLayer。
func Toast(msg string) *Node {
	if msg == "" {
		return nil
	}
	return Row(
		Box().St(St().W(6).H(6).R(3).BgC(ColSuccess)),
		Text(msg).St(St().Text(12.5, 500, ColText)),
	).St(St().AbsBottom(toastBottom).PadXY(14, 10).R(10).GapOf(8).
		BgC(ColPanel).Stroke(1, ColStroke).Shadow(24, 8, 0, "rgba(0,0,0,0.5)"))
}

// Banner 信息条（左侧色条 + 标题/正文 + 关闭）。
func Banner(kind, title, body string, onClose func()) *Node {
	color := ColInfo
	switch kind {
	case "success":
		color = ColSuccess
	case "warn":
		color = ColWarn
	case "error":
		color = ColDanger
	}
	row := Row(
		Box().St(St().W(3).Hp(100).R(2).BgC(color)),
		Column(
			Text(title).St(St().Text(12.5, 600, ColText)),
			Text(body).St(St().Text(11.5, 400, ColTextMuted).WrapOn().PadOnly(3, 0, 0, 0)),
		).Grow(1).St(St().GapOf(2)),
	)
	if onClose != nil {
		row.Add(Text("×").St(St().Text(15, 500, ColTextMuted)).Tap(onClose))
	}
	return row.St(St().GapOf(12).Padding(12).R(10).
		BgC(ColHover).Stroke(1, ColStroke)).AlignTo(AlignCenter)
}

// Modal 模态框（遮罩 + 卡片，放在 Stack 中并已 AbsFill）。
func Modal(open bool, title, body string, onClose func()) *Node {
	return ModalEx(open, title, body, "取消", "确定", onClose, onClose, onClose, 420)
}

// ModalEx 可定制按钮与宽度的模态框（带进出场过渡与焦点陷阱）。
func ModalEx(open bool, title, body, cancelLabel, okLabel string, onCancel, onConfirm, onClose func(), width float32) *Node {
	return ModalKey("modal:"+title, open, title, body, cancelLabel, okLabel, onCancel, onConfirm, onClose, width)
}

// ModalKey 带唯一 Key 的模态框：多个同标题弹窗共存时用它避免焦点/动画状态互撞。
func ModalKey(key string, open bool, title, body, cancelLabel, okLabel string, onCancel, onConfirm, onClose func(), width float32) *Node {
	if key == "" {
		key = "modal:" + title
	}
	if width <= 0 {
		width = 420
	}
	if open {
		PushFocusScope(key, true)
		OpenOverlay(key, func() bool {
			if onClose != nil {
				onClose()
			}
			return true
		})
	} else {
		PopFocusScope(key)
		CloseOverlay(key)
	}
	visible, t := KeepAlive(key, open, 0.18)
	if !visible {
		return nil
	}
	_, dy := PopOffset(t)
	scrim := Scrim(t*0.95, key+"-scrim").Tap(func() {
		if onClose != nil {
			onClose()
		}
	})
	dialog := Column(
		Row(
			Text(title).St(St().Text(16, 600, ColText)).Grow(1),
			Text("×").St(St().Text(15, 500, ColTextMuted)).Tap(onClose),
		).AlignTo(AlignCenter),
		Divider(),
		Text(body).St(St().Text(13, 400, ColTextDim).WrapOn()),
		Row(
			Spacer(),
			Btn(cancelLabel, BtnSecondary, onCancel),
			Btn(okLabel, BtnPrimary, onConfirm),
		).St(St().GapOf(10)).AlignTo(AlignCenter),
	).St(St().W(width).Padding(22).R(18).GapOf(14).
		BgC(ColPanelDeep).Stroke(1, ColStroke).Shadow(48, 18, 0, "rgba(0,0,0,0.55)").
		AbsCenter().Fade(t).Translate(0, dy))

	return Stack(scrim, dialog).St(St().AbsFill()).K(key + "-root")
}

// ---- 导航与列表 ----

// Tabs 分段页签。
func Tabs(keys, labels []string, active string, onPick func(string)) *Node {
	items := make([]*Node, 0, len(keys))
	for i, k := range keys {
		label := k
		if i < len(labels) {
			label = labels[i]
		}
		st := St().Text(12.5, 500, ColTextMuted).PadXY(14, 7).R(8)
		if k == active {
			st.Text(12.5, 600, ColText).BgC(ColBgSoft).Stroke(1, ColStroke)
		}
		id := k
		items = append(items, Text(label).St(st).Tap(func() {
			if onPick != nil {
				onPick(id)
			}
			RequestRedraw()
		}))
	}
	return Row(items...).St(St().GapOf(4).Padding(4).R(12).BgC(ColInput).Stroke(1, ColStroke))
}

// TabsBar 下划线风格页签（gio 常见导航样式）。
func TabsBar(keys, labels []string, active string, onPick func(string)) *Node {
	items := make([]*Node, 0, len(keys))
	for i, k := range keys {
		label := k
		if i < len(labels) {
			label = labels[i]
		}
		color, weight := ColTextMuted, int32(500)
		if k == active {
			color, weight = ColText, 600
		}
		cells := []*Node{Text(label).St(St().Text(13, weight, color).PadXY(2, 8))}
		if k == active {
			cells = append(cells, Box().St(St().Wp(100).H(2).R(1).BgC(ColPrimary)))
		}
		col := Column(cells...).St(St().GapOf(6))
		id := k
		items = append(items, col.Tap(func() {
			if onPick != nil {
				onPick(id)
			}
			RequestRedraw()
		}))
	}
	return Row(items...).St(St().GapOf(22)).AlignTo(AlignStart)
}

// Breadcrumb 面包屑（点击项回调，最后一项为当前页）。
func Breadcrumb(items []string, onPick func(int)) *Node {
	cells := make([]*Node, 0, len(items)*2)
	for i, s := range items {
		if i > 0 {
			cells = append(cells, Text("/").St(St().Text(11.5, 400, ColPlaceholder)))
		}
		idx := i
		color := ColTextMuted
		if i == len(items)-1 {
			color = ColTextDim
		}
		cells = append(cells, Text(s).St(St().Text(12, 500, color)).Tap(func() {
			if onPick != nil {
				onPick(idx)
			}
			RequestRedraw()
		}))
	}
	return Row(cells...).St(St().GapOf(8)).AlignTo(AlignCenter)
}

// Accordion 折叠面板。
func Accordion(key, title, body string, open bool, onToggle func()) *Node {
	head := Row(
		Text(title).St(St().Text(13, 600, ColText)).Grow(1),
		Text(map[bool]string{true: "−", false: "+"}[open]).St(St().Text(14, 600, ColTextMuted)),
	).St(St().Padding(14).R(10).BgC(ColBgSoft))
	head.K(key + "-head").Tap(func() {
		if onToggle != nil {
			onToggle()
		}
		RequestRedraw()
	})
	col := Column(head)
	if open {
		col.Add(Text(body).St(St().Text(12.5, 400, ColTextDim).WrapOn().PadOnly(12, 14, 12, 14)))
	}
	return col.St(St().GapOf(4))
}

// ListItem 可点击列表项（左侧色点 + 标题 + 右侧说明）。
func ListItem(key, title, sub, color string, onClick func()) *Node {
	row := Row(
		Dot(color, 8),
		Column(
			Text(title).St(St().Text(13, 500, ColText)),
			Text(sub).St(St().Text(11.5, 400, ColTextMuted).PadOnly(3, 0, 0, 0)),
		).Grow(1),
		Text("→").St(St().Text(13, 500, ColPlaceholder)),
	).St(St().GapOf(12).Padding(14).R(12).BgC(ColBgSoft).Stroke(1, ColStroke)).AlignTo(AlignCenter)
	return row.K(key).Tap(func() {
		if onClick != nil {
			onClick()
		}
		RequestRedraw()
	})
}

// Table 数据表格：固定/弹性列宽、表头、斑马纹、整行点击。
func Table(headers []string, widths []float32, rows [][]*Node, onRow func(int)) *Node {
	head := Row().St(St().GapOf(12).PadXY(16, 10).BgC(ColHeader))
	for i, h := range headers {
		cell := Text(h).St(St().Text(11.5, 600, ColTextMuted))
		if i < len(widths) && widths[i] > 0 {
			cell.St(St().W(widths[i]))
		} else {
			cell.Grow(1)
		}
		head.Add(cell)
	}
	body := make([]*Node, 0, len(rows))
	for ri, cells := range rows {
		bg := ColRow
		if ri%2 == 1 {
			bg = ColRowAlt
		}
		r := Row().St(St().GapOf(12).PadXY(16, 11).BgC(bg)).AlignTo(AlignCenter)
		for ci, c := range cells {
			if c == nil {
				c = Text("—").St(St().Text(12, 400, ColTextMuted))
			}
			if ci < len(widths) && widths[ci] > 0 {
				c.St(St().W(widths[ci]))
			} else {
				c.Grow(1)
			}
			r.Add(c)
		}
		idx := ri
		r.K("row-" + strconv.Itoa(ri)).Tap(func() {
			if onRow != nil {
				onRow(idx)
			}
			RequestRedraw()
		})
		body = append(body, r)
	}
	return Column(append([]*Node{Divider(), head, Divider()}, body...)...).
		St(St().R(12).Stroke(1, ColStroke).Clipped())
}

// ---- 浮层 ----

// TooltipOf 给节点绑定悬停提示。
func TooltipOf(n *Node, text string) *Node {
	if n == nil || text == "" {
		return n
	}
	n.OnHover = func(hover bool) {
		if hover {
			x, y := MousePos()
			SetTooltip(text, x, y)
		} else {
			SetTooltip("", 0, 0)
		}
	}
	return n
}

// TooltipLayer 悬停提示浮层（放在根 Stack 最后一层）。
func TooltipLayer() *Node {
	if tooltipText == "" {
		return nil
	}
	return Text(tooltipText).St(St().Text(11.5, 500, ColText).PadXY(9, 6).R(7).
		BgC(ColPanel).Stroke(1, ColStroke).Shadow(18, 6, 0, "rgba(0,0,0,0.5)").
		AbsTL(tooltipX+14, tooltipY+18))
}

// whiteA 构造白色半透明（骨架屏呼吸层）。
func whiteA(a float32) gpu.Paint { return gpu.Paint{R: 1, G: 1, B: 1, A: a} }
