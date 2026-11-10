// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "strings"

// 文本编辑公共能力：选区（anchor + caret）、剪贴板快捷键、拖选定光标。
//
// 输入框（Input / PasswordInput）、多行输入（Textarea）和自绘编辑框共用这套状态机。
// 之前输入框只有 caret 没有 anchor，也没有处理 Ctrl+A/C/X/V —— 复制粘贴完全不生效，
// 而 Ctrl+C 还会被全局「选区复制」快捷键抢先吃掉（DispatchShortcuts 在焦点控件之前），
// 拖选更是无从谈起。

// TextEditAction 文本编辑动作（剪贴板 / 全选）。
type TextEditAction int

const (
	// TextEditCopy 复制选中文本。
	TextEditCopy TextEditAction = iota
	// TextEditCut 剪切选中文本。
	TextEditCut
	// TextEditPaste 粘贴剪贴板文本（替换选区）。
	TextEditPaste
	// TextEditSelectAll 全选。
	TextEditSelectAll
)

// textEditHandlers 已登记文本编辑能力的控件（按节点 Key，每帧构建时覆盖）。
var textEditHandlers = map[string]func(TextEditAction) bool{}

// RegisterTextEdit 登记一个控件的文本编辑动作（输入框内部调用）。
// 同一 Key 每帧覆盖，闭包捕获当帧的文本状态。
func RegisterTextEdit(key string, fn func(TextEditAction) bool) {
	if key == "" || fn == nil {
		return
	}
	textEditHandlers[key] = fn
}

// TextEditBound 判断 Key 对应控件是否支持文本编辑。
func TextEditBound(key string) bool { return key != "" && textEditHandlers[key] != nil }

// textEditKeyAction 按键 → 编辑动作（仅 Ctrl+A / Ctrl+C / Ctrl+X / Ctrl+V）。
func textEditKeyAction(vk uint32) (TextEditAction, bool) {
	switch vk {
	case 'A', 'a':
		return TextEditSelectAll, true
	case 'C', 'c':
		return TextEditCopy, true
	case 'X', 'x':
		return TextEditCut, true
	case 'V', 'v':
		return TextEditPaste, true
	}
	return TextEditCopy, false
}

// DispatchTextEdit 把 Ctrl+A/C/X/V 交给获得焦点的文本控件。
//
// 返回 false 表示当前焦点不是文本控件，调用方继续走全局快捷键
// （SelectionArea 的「Ctrl+C 复制选区」仍然生效）。
func DispatchTextEdit(vk uint32) bool {
	if !dispatchTextEditTo(FocusedKey(), vk) {
		return false
	}
	RequestRedraw()
	return true
}

// dispatchTextEditTo 把按键对应的编辑动作投递给指定控件（Key）。
// 返回 false 表示该控件不存在或按键不是编辑动作。
func dispatchTextEditTo(key string, vk uint32) bool {
	act, ok := textEditKeyAction(vk)
	if !ok {
		return false
	}
	fn := textEditHandlers[key]
	if fn == nil {
		return false
	}
	return fn(act)
}

// ---- 选区状态机 ----

// TextSel 文本编辑状态：文本 + 光标 + 选区锚点（下标均按 rune 计）。
// Anchor == Caret 表示没有选区；Anchor 是拖选起点，Caret 随鼠标移动。
type TextSel struct {
	Runes  []rune
	Caret  int
	Anchor int
}

// NewTextSel 用当前文本与记忆的光标 / 锚点构造状态（越界自动夹取）。
func NewTextSel(value string, caret, anchor int) *TextSel {
	t := &TextSel{Runes: []rune(value), Caret: caret, Anchor: anchor}
	t.Clamp()
	return t
}

// Clamp 把光标与锚点夹取到 [0, 文本长度]。
func (t *TextSel) Clamp() {
	n := len(t.Runes)
	if t.Caret < 0 {
		t.Caret = 0
	}
	if t.Caret > n {
		t.Caret = n
	}
	if t.Anchor < 0 {
		t.Anchor = 0
	}
	if t.Anchor > n {
		t.Anchor = n
	}
}

// Text 当前文本。
func (t *TextSel) Text() string { return string(t.Runes) }

// Selection 返回选区区间 [lo, hi) 与是否存在选区。
func (t *TextSel) Selection() (lo, hi int, ok bool) {
	if t.Anchor == t.Caret {
		return t.Caret, t.Caret, false
	}
	if t.Anchor < t.Caret {
		return t.Anchor, t.Caret, true
	}
	return t.Caret, t.Anchor, true
}

// Selected 选中的文本（无选区返回空串）。
func (t *TextSel) Selected() string {
	lo, hi, ok := t.Selection()
	if !ok {
		return ""
	}
	return string(t.Runes[lo:hi])
}

// Move 移动光标（delta 按 rune）；keep=false 时折叠选区（普通方向键）。
func (t *TextSel) Move(delta int, keep bool) {
	t.Caret += delta
	t.Clamp()
	if !keep {
		t.Anchor = t.Caret
	}
}

// SetCaret 定位光标；keep=false 时折叠选区。
func (t *TextSel) SetCaret(pos int, keep bool) {
	t.Caret = pos
	t.Clamp()
	if !keep {
		t.Anchor = t.Caret
	}
}

// SelectAll 全选。
func (t *TextSel) SelectAll() {
	t.Anchor, t.Caret = 0, len(t.Runes)
}

// DeleteSelection 删除选中区间（无选区返回 false）。
func (t *TextSel) DeleteSelection() bool {
	lo, hi, ok := t.Selection()
	if !ok {
		return false
	}
	t.Runes = append(t.Runes[:lo:lo], t.Runes[hi:]...)
	t.Caret, t.Anchor = lo, lo
	return true
}

// Insert 在光标处插入文本（有选区先删除选区）。
func (t *TextSel) Insert(s string) bool {
	if s == "" {
		return false
	}
	t.DeleteSelection()
	rs := []rune(s)
	out := make([]rune, 0, len(t.Runes)+len(rs))
	out = append(out, t.Runes[:t.Caret]...)
	out = append(out, rs...)
	out = append(out, t.Runes[t.Caret:]...)
	t.Runes = out
	t.Caret += len(rs)
	t.Anchor = t.Caret
	return true
}

// Backspace 删除光标前一个 rune（有选区时先删选区）。
func (t *TextSel) Backspace() bool {
	if t.DeleteSelection() {
		return true
	}
	if t.Caret <= 0 {
		return false
	}
	t.Runes = append(t.Runes[:t.Caret-1], t.Runes[t.Caret:]...)
	t.Caret--
	t.Anchor = t.Caret
	return true
}

// DeleteForward 删除光标后一个 rune（有选区时先删选区）。
func (t *TextSel) DeleteForward() bool {
	if t.DeleteSelection() {
		return true
	}
	if t.Caret >= len(t.Runes) {
		return false
	}
	t.Runes = append(t.Runes[:t.Caret], t.Runes[t.Caret+1:]...)
	t.Anchor = t.Caret
	return true
}

// Copy 把选中文本写入系统剪贴板。
func (t *TextSel) Copy() bool {
	s := t.Selected()
	if s == "" {
		return false
	}
	return ClipboardSetText(s)
}

// Cut 剪切选中文本到剪贴板。
func (t *TextSel) Cut() bool {
	if t.Selected() == "" {
		return false
	}
	ok := t.Copy()
	t.DeleteSelection()
	return ok
}

// Paste 粘贴剪贴板文本（multiline=false 时把换行替换为空格）。
func (t *TextSel) Paste(multiline bool) bool {
	s := ClipboardText()
	if s == "" {
		return false
	}
	if !multiline {
		s = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
	}
	return t.Insert(s)
}

// ---- 坐标 → 光标 ----

// IndexAtX 单行文本按横向偏移定位光标（x 相对文本起点）。
func (t *TextSel) IndexAtX(x, size float32, weight int32) int {
	if x <= 0 || len(t.Runes) == 0 {
		return 0
	}
	acc := float32(0)
	for i, r := range t.Runes {
		w := TextWidth(string(r), size, weight)
		if x < acc+w/2 {
			return i
		}
		acc += w
	}
	return len(t.Runes)
}

// IndexAtPoint 多行文本按坐标定位光标（x/y 相对文本左上角，行高 lineH）。
func (t *TextSel) IndexAtPoint(x, y, lineH, size float32, weight int32) int {
	lines := strings.Split(string(t.Runes), "\n")
	row := int(y / lineH)
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	base := 0
	for i := 0; i < row; i++ {
		base += len([]rune(lines[i])) + 1
	}
	ln := []rune(lines[row])
	acc := float32(0)
	for i, r := range ln {
		w := TextWidth(string(r), size, weight)
		if x < acc+w/2 {
			return base + i
		}
		acc += w
	}
	return base + len(ln)
}

// TextSeg 一行按选区切分后的片段（渲染高亮用）。
type TextSeg struct {
	Text string
	Sel  bool
}

// SelLineSegments 把 [start,end) 这段 rune（一行）按当前选区切成若干片段。
// 返回片段按从左到右排列，Sel=true 的片段是选中部分（调用方加高亮底色）。
func (t *TextSel) SelLineSegments(start, end int) []TextSeg {
	if start < 0 {
		start = 0
	}
	if end > len(t.Runes) {
		end = len(t.Runes)
	}
	if start > end {
		start = end
	}
	lo, hi, has := t.Selection()
	if !has || hi <= start || lo >= end {
		if start == end {
			return nil
		}
		return []TextSeg{{Text: string(t.Runes[start:end])}}
	}
	var out []TextSeg
	if lo > start {
		out = append(out, TextSeg{Text: string(t.Runes[start:lo])})
	}
	ms, me := lo, hi
	if ms < start {
		ms = start
	}
	if me > end {
		me = end
	}
	out = append(out, TextSeg{Text: string(t.Runes[ms:me]), Sel: true})
	if hi < end {
		out = append(out, TextSeg{Text: string(t.Runes[hi:end])})
	}
	return out
}
