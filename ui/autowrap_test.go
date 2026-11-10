// guis — 文本自动换行的布局回归测试。
//
// 规则：纵向流容器（Column / Box / Scroll / Stack / Grid）里的文本，一旦自然宽度
// 超出可用宽度就自动折行；横向容器（Row / Wrap）里的文本保持单行。
// 前者解决窄屏手机上「说明文案溢出被裁掉」，后者保证页签 / 按钮 / 表格单元格不被撑成两行。

package ui

import "testing"

// TestTextAutoWrapInVerticalFlow 纵向容器自动折行、横向容器不折行。
func TestTextAutoWrapInVerticalFlow(t *testing.T) {
	const long = "这是一段很长的说明文字，用来验证窄容器里的自动换行行为是否正确，不应该溢出被裁掉。"
	root := Column(
		Text(long).K("para"),
		Row(Text(long).K("rowtext")),
	).St(St().GapOf(8))
	Layout(root, 200, 600, newTestCache())

	para := findNodeKey(root, "para")
	if para == nil {
		t.Fatal("缺少 para 节点")
	}
	if para.wrapW <= 0 {
		t.Fatal("纵向容器里的超宽文本应自动折行（wrapW = 0）")
	}
	if para.Frame.W > 200.5 {
		t.Fatalf("折行后宽度 %.1f 超出容器宽度 200", para.Frame.W)
	}
	if para.Frame.H <= para.Style.FontSize*1.6 {
		t.Fatalf("折行后高度 %.1f 仍接近单行，说明没有真正折行", para.Frame.H)
	}

	row := findNodeKey(root, "rowtext")
	if row == nil {
		t.Fatal("缺少 rowtext 节点")
	}
	if row.wrapW != 0 {
		t.Fatalf("Row 里的文本不应自动折行（wrapW = %.1f）", row.wrapW)
	}
}

// TestTextNoAutoWrapWhenFits 放得下时不折行（桌面宽窗口行为不变）。
func TestTextNoAutoWrapWhenFits(t *testing.T) {
	root := Column(Text("短文本").K("short"))
	Layout(root, 600, 400, newTestCache())
	n := findNodeKey(root, "short")
	if n == nil {
		t.Fatal("缺少 short 节点")
	}
	if n.wrapW != 0 {
		t.Fatalf("放得下的文本不应折行（wrapW = %.1f）", n.wrapW)
	}
}
