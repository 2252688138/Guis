// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// inputCaretFixture 建一个单行输入框并返回 (App, 根节点, 输入框节点)。
func inputCaretFixture(t *testing.T, key, value string, onChange func(string)) (*App, *Node, *Node) {
	t.Helper()
	fc := newTestCache()
	a := &App{fonts: fc, scrolls: map[string]float32{}}
	root := Column(Input(key, value, "", onChange)).St(St().W(300).Padding(10))
	Layout(root, 300, 60, fc)
	a.root = root
	in := findKey(root, key)
	if in == nil {
		t.Fatal("找不到输入框节点")
	}
	return a, root, in
}

// TestInputCaretFollowsClickNotMouse 回归：光标只能由「按下点 / 抬起点」定位，
// 不能读实时鼠标位置。点击回调在抬起后的渲染帧里执行，这期间指针多半已经移开，
// 旧的实现用 MousePos() 定位，结果是鼠标指到哪、随后输入的字符就插到哪
// （看起来就是「没点这里却插到这里」）。
func TestInputCaretFollowsClickNotMouse(t *testing.T) {
	value := "hello world"
	a, _, in := inputCaretFixture(t, "t-click", value, func(s string) { value = s })
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)
	defer delete(inputCaret, "t-click")
	defer delete(inputAnchor, "t-click")
	defer delete(textEditHandlers, "t-click")

	// 按下与抬起都落在文本最左（内边距 12）→ 光标应在 0
	px := in.Frame.X + 12
	py := in.Frame.Y + in.Frame.H/2
	a.pressX, a.pressY = px, py
	a.clickX, a.clickY = px, py
	// 抬起后鼠标移到了行尾：定位若读实时鼠标位置就会跑到行尾
	a.mouseX, a.mouseY = in.Frame.X+in.Frame.W-16, py

	in.OnPress()
	if got := inputCaret["t-click"]; got != 0 {
		t.Fatalf("按下应按按下点定位光标：期望 0，实际 %d", got)
	}
	if !IsFocused("t-click") {
		t.Fatal("按下即应获得焦点")
	}
	in.OnClick()
	if got := inputCaret["t-click"]; got != 0 {
		t.Fatalf("点击定位必须用抬起点，不能跟着鼠标当前位置：期望 0，实际 %d", got)
	}
	// 继续输入：字符应插在光标处（行首），而不是鼠标所指处
	in.OnChar('X')
	if value != "Xhello world" {
		t.Fatalf("字符应插在光标处，实际 %q", value)
	}
}

// TestInputPressUsesPressPos 按下回调同样要按按下点定位：
// 按下与渲染之间可能夹着 mousemove，读实时位置会把光标挪到指针处。
func TestInputPressUsesPressPos(t *testing.T) {
	a, _, in := inputCaretFixture(t, "t-press", "hello world", nil)
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)
	defer delete(inputCaret, "t-press")
	defer delete(inputAnchor, "t-press")
	defer delete(textEditHandlers, "t-press")

	py := in.Frame.Y + in.Frame.H/2
	a.pressX, a.pressY = in.Frame.X+in.Frame.W-16, py // 按下在行尾
	a.mouseX, a.mouseY = in.Frame.X+12, py            // 渲染前指针已被移到行首

	in.OnPress()
	if got, want := inputCaret["t-press"], len([]rune("hello world")); got != want {
		t.Fatalf("按下点定位应落在行尾（%d），实际 %d", want, got)
	}
}

// TestInputNotActivatedByKeyboard 回归：Enter / Space 不能把输入框当按钮激活。
// 输入框的 OnClick 语义是「点哪里就把光标放哪里」，键盘激活会走一遍 OnClick，
// 光标立刻跳到鼠标所指处，接着输入的内容就插错位置。
func TestInputNotActivatedByKeyboard(t *testing.T) {
	a, _, in := inputCaretFixture(t, "t-kb", "hello world", nil)
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)
	defer delete(inputCaret, "t-kb")
	defer delete(inputAnchor, "t-kb")
	defer delete(textEditHandlers, "t-kb")

	py := in.Frame.Y + in.Frame.H/2
	a.pressX, a.pressY = in.Frame.X+12, py // 光标放到行首
	a.clickX, a.clickY = a.pressX, py
	SetFocus("t-kb")
	in.OnPress()
	if got := inputCaret["t-kb"]; got != 0 {
		t.Fatalf("按下应按按下点定位光标：期望 0，实际 %d", got)
	}
	// 鼠标停在行尾（用户只是把指针移过去，没有点击）
	a.mouseX, a.mouseY = in.Frame.X+in.Frame.W-16, py
	if a.activateFocused() {
		t.Fatal("输入框不应参与 Enter / Space 键盘激活")
	}
	if got := inputCaret["t-kb"]; got != 0 {
		t.Fatalf("键盘激活不应改动光标：期望 0，实际 %d", got)
	}
}

// TestButtonStillActivatedByKeyboard 键盘激活对普通控件（按钮）仍然有效，
// 避免上面那条「编辑类控件跳过激活」的规则误伤。
func TestButtonStillActivatedByKeyboard(t *testing.T) {
	fc := newTestCache()
	a := &App{fonts: fc, scrolls: map[string]float32{}}
	prev := currentApp.Swap(a)
	defer currentApp.Store(prev)

	hit := 0
	btn := Button("确定", func() { hit++ }).K("t-btn")
	root := Column(btn).St(St().W(300).Padding(10))
	Layout(root, 300, 60, fc)
	a.root = root
	SetFocus("t-btn")
	if !a.activateFocused() {
		t.Fatal("按钮应响应键盘激活")
	}
	if hit != 1 {
		t.Fatalf("键盘激活应触发按钮回调一次，实际 %d", hit)
	}
}
