// guis — gallery 窄屏：说明文案自动换行的回归测试
//
// 手机逻辑宽度约 480（见 ui/main_entry_android.go 的 SetUIScale 计算），
// 页面里大量说明文本原本没有 WrapOn，窄屏时直接溢出被裁掉。布局层现在会对
// 纵向容器里的超宽文本自动折行，这里用整页布局来锁定行为。
//
// 三端共用 pages.View（同一套壳层），因此这里直接对 View() 布局：
// 窄屏走收窄侧栏分支，宽屏走标准 232px 侧栏分支。

package pages

import (
	"strings"
	"testing"

	"guis/ui"
)

// findLongNote 深度查找「包含指定关键字且宽度受限」的文本节点。
func findLongNote(n *ui.Node, keyword string) *ui.Node {
	if n == nil {
		return nil
	}
	if strings.Contains(n.Text, keyword) {
		return n
	}
	for _, c := range n.Children {
		if got := findLongNote(c, keyword); got != nil {
			return got
		}
	}
	return nil
}

// TestMobileLongTextAutoWrap 窄屏（480 逻辑宽）下长说明文本必须折行、且不超出容器。
func TestMobileLongTextAutoWrap(t *testing.T) {
	old := Current
	Current = "basic"
	defer func() { Current = old }()

	root := View()
	ui.LayoutForTest(root, 480, 853, "", 1)

	// 基础页「下拉框与多行输入」卡片里的长说明（远超 480 宽）
	n := findLongNote(root, "Textarea")
	if n == nil {
		t.Fatal("未找到基础页的长说明文本（页面结构变了？）")
	}
	if n.Frame.W > 480 {
		t.Fatalf("折行后宽度 %.1f 超出屏幕宽度 480", n.Frame.W)
	}
	// 折行后高度至少两行（wrapW 是 ui 包未导出字段，这里只看可观测的 Frame）
	if n.Frame.H < n.Style.FontSize*2.2 {
		t.Fatalf("折行后高度 %.1f 仍等于单行，说明没有真正换行", n.Frame.H)
	}
}

// TestWideScreenLongTextNoWrap 宽屏（1180 逻辑宽，桌面窗口）下同一段文本保持单行，
// 保证本次自动换行不会改变桌面端观感。
func TestWideScreenLongTextNoWrap(t *testing.T) {
	old := Current
	Current = "basic"
	defer func() { Current = old }()

	root := View()
	ui.LayoutForTest(root, 1180, 900, "", 1)

	n := findLongNote(root, "Textarea")
	if n == nil {
		t.Fatal("未找到基础页的长说明文本")
	}
	if n.Frame.H > n.Style.FontSize*2.0 {
		t.Fatalf("宽屏下不该折行：高度 %.1f（单行约 %.0f）", n.Frame.H, n.Style.FontSize*1.5)
	}
}
