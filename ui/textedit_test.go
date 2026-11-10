// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestInputSelectionReplace 输入框接线回归：
//  1. 必须注册 OnPress / OnDrag，否则 App 建立不了拖拽目标，拖选完全没反应；
//  2. 必须登记文本编辑能力，Ctrl+A/C/X/V 才会被焦点控件消费；
//  3. 全选后输入字符应替换整段文本。
func TestInputSelectionReplace(t *testing.T) {
	fc := newTestCache()
	a := &App{fonts: fc, scrolls: map[string]float32{}}
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)
	defer delete(textEditHandlers, "t-in")

	value := "hello"
	root := Column(Input("t-in", value, "", func(s string) { value = s })).St(St().W(200))
	Layout(root, 200, 38, fc)
	a.root = root
	SetFocus("t-in")

	f := findKey(root, "t-in")
	if f == nil {
		t.Fatal("找不到输入框节点")
	}
	if f.OnPress == nil || f.OnDrag == nil {
		t.Fatal("输入框缺少 OnPress/OnDrag，拖选不生效")
	}
	if !TextEditBound("t-in") {
		t.Fatal("输入框应登记文本编辑能力（Ctrl+A/C/X/V）")
	}
	if !dispatchTextEditTo("t-in", 'A') {
		t.Fatal("Ctrl+A 全选未生效")
	}
	f.OnChar('X')
	if value != "X" {
		t.Fatalf("全选后输入应替换整段文本，实际 %q", value)
	}
}

// TestInputDragSelect 拖选全链路：命中输入框 → OnPress 定位 → OnDrag 拉出选区。
// App 只有在 pressNode.OnDrag != nil 时才会建立 dragKey，这条链路任一环缺失
// 都会表现为「拖不动 / 选不中」。
func TestInputDragSelect(t *testing.T) {
	fc := newTestCache()
	a := &App{fonts: fc, scrolls: map[string]float32{}}
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)
	defer delete(textEditHandlers, "t-drag")
	defer delete(inputAnchor, "t-drag")
	defer delete(inputCaret, "t-drag")

	root := Column(Input("t-drag", "hello world", "", nil)).St(St().W(300).Padding(10))
	Layout(root, 300, 60, fc)
	a.root = root

	in := findKey(root, "t-drag")
	if in == nil {
		t.Fatal("找不到输入框节点")
	}
	// 模拟按下：命中测试必须返回输入框本身（否则 dragKey 建不起来）
	// 按下点取文本左边缘（输入框内边距 12），锚点落在 0
	a.mouseX, a.mouseY = in.Frame.X+12, in.Frame.Y+in.Frame.H/2
	press := hitInteractive(root, a.mouseX, a.mouseY, 0, a)
	if press == nil || press.Key != "t-drag" {
		t.Fatalf("按下应命中输入框，实际 %+v", press)
	}
	if press.OnDrag == nil || press.OnPress == nil {
		t.Fatal("输入框必须同时注册 OnPress 与 OnDrag")
	}
	press.OnPress()

	// 模拟拖动到靠右位置：光标右移、锚点留在起点，形成选区
	if n := findKey(root, "t-drag"); n != nil && n.OnDrag != nil {
		n.OnDrag(in.Frame.W-6, 6)
	}
	anchor := inputAnchor["t-drag"]
	caret := inputCaret["t-drag"]
	if anchor != 0 {
		t.Fatalf("拖选起点应为 0，实际 %d", anchor)
	}
	if caret <= anchor {
		t.Fatalf("拖到最右应选中整段文本，caret=%d anchor=%d", caret, anchor)
	}
	if IsFocused("t-drag") == false {
		t.Fatal("按下即应获得焦点，否则拖选过程中看不到高亮")
	}
}

// TestTextSelIndices 选区下标与光标夹取。
func TestTextSelIndices(t *testing.T) {
	s := NewTextSel("hello world", 5, 0)
	lo, hi, ok := s.Selection()
	if !ok || lo != 0 || hi != 5 {
		t.Fatalf("选区应为 [0,5)，实际 [%d,%d) ok=%v", lo, hi, ok)
	}
	if s.Selected() != "hello" {
		t.Fatalf("选中文本应为 hello，实际 %q", s.Selected())
	}
	// 锚点在光标右侧时同样成立
	s.Anchor, s.Caret = 11, 6
	lo, hi, ok = s.Selection()
	if !ok || lo != 6 || hi != 11 {
		t.Fatalf("反向选区应为 [6,11)，实际 [%d,%d)", lo, hi)
	}
	s.SetCaret(99, false)
	if s.Caret != 11 || s.Anchor != 11 {
		t.Fatalf("光标应被夹取到 11，实际 caret=%d anchor=%d", s.Caret, s.Anchor)
	}
}

// TestTextSelEdit 插入 / 删除 / 替换选区。
func TestTextSelEdit(t *testing.T) {
	s := NewTextSel("abcd", 4, 4)
	s.Insert("e")
	if s.Text() != "abcde" || s.Caret != 5 {
		t.Fatalf("插入失败：%q caret=%d", s.Text(), s.Caret)
	}
	// 退格删前一个字符
	s.Backspace()
	if s.Text() != "abcd" || s.Caret != 4 {
		t.Fatalf("退格失败：%q caret=%d", s.Text(), s.Caret)
	}
	// 有选区时插入会替换选区
	s.Anchor, s.Caret = 1, 3
	s.Insert("XY")
	if s.Text() != "aXYd" || s.Caret != 3 {
		t.Fatalf("替换选区失败：%q caret=%d", s.Text(), s.Caret)
	}
	// 全选后删除
	s.SelectAll()
	if !s.DeleteSelection() || s.Text() != "" {
		t.Fatalf("全选删除失败：%q", s.Text())
	}
}

// TestTextSelLineSegments 按行切段（选区高亮渲染用）。
func TestTextSelLineSegments(t *testing.T) {
	s := NewTextSel("abc\ndef", 1, 6)
	// 第一行 [0,3) 命中选区前段
	segs := s.SelLineSegments(0, 3)
	if len(segs) != 2 || segs[0].Sel || segs[0].Text != "a" || !segs[1].Sel || segs[1].Text != "bc" {
		t.Fatalf("第一行切段不对：%+v", segs)
	}
	// 第二行 [4,7) 命中选区后段
	segs = s.SelLineSegments(4, 7)
	if len(segs) != 2 || !segs[0].Sel || segs[0].Text != "de" || segs[1].Sel {
		t.Fatalf("第二行切段不对：%+v", segs)
	}
}

// TestTextEditDispatch 焦点文本控件优先接收 Ctrl+A/C/X/V。
func TestTextEditDispatch(t *testing.T) {
	defer delete(textEditHandlers, "test-input")

	got := TextEditAction(-1)
	RegisterTextEdit("test-input", func(a TextEditAction) bool {
		got = a
		return true
	})
	if !dispatchTextEditTo("test-input", 'C') {
		t.Fatal("Ctrl+C 应被文本控件消费")
	}
	if got != TextEditCopy {
		t.Fatalf("应收到复制动作，实际 %v", got)
	}
	if !dispatchTextEditTo("test-input", 'A') || got != TextEditSelectAll {
		t.Fatalf("Ctrl+A 应映射为全选，实际 %v", got)
	}
	if !dispatchTextEditTo("test-input", 'V') || got != TextEditPaste {
		t.Fatalf("Ctrl+V 应映射为粘贴，实际 %v", got)
	}
	// 非文本控件不得拦截（否则全局快捷键与选区复制失效）
	if dispatchTextEditTo("no-such-input", 'C') {
		t.Fatal("未登记的控件不应消费 Ctrl+C")
	}
	if dispatchTextEditTo("test-input", 'S') {
		t.Fatal("Ctrl+S 不属于文本编辑动作")
	}
}
