package ui

import (
	"strings"
	"testing"
)

// 折行 + 显式行高：容器高度必须按真实行数（而不是显式换行数）计算，
// 否则长消息只有一行高，后续文本会画到气泡外面。
func TestTextWrapWithLineHeight(t *testing.T) {
	long := strings.Repeat("请制作一个星空科幻宇宙飞船3D模型方案，用途为游戏动画资产，风格为硬表面科幻。", 5)
	n := Text(long).K("bubble").St(St().WrapOn().Lh(19).W(500).PadXY(12, 9))
	Layout(Column(n), 900, 800, newTestCache())

	if n.wrapW <= 0 {
		t.Fatalf("应按 500 折行（wrapW=%v）", n.wrapW)
	}
	if n.Frame.H < 60 {
		t.Fatalf("折行后高度 %.1f 明显偏小（应多行），说明高度被显式换行数覆盖", n.Frame.H)
	}
}

// 显式行高 + 显式换行：仍按换行数算高度（原行为不回退）。
func TestTextLineHeightExplicitLines(t *testing.T) {
	n := Text("第一行\n第二行\n第三行").K("three").St(St().Lh(20))
	Layout(Column(n), 600, 400, newTestCache())
	if n.Frame.H < 60 || n.Frame.H > 62 {
		t.Fatalf("三行文本高度应为 60，实际 %.1f", n.Frame.H)
	}
}
