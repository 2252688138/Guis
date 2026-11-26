// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestIconAutoAlignToText 水平行 + AlignCenter 时，矢量图标对齐到同行文字的墨迹中心。
//
// 文字盒高是整个行高，图标盒高等于字号：两边都按盒子居中时，图标与文字墨迹之间
// 总有 1~2px 的偏差（含降部的西文偏低、全大写/数字偏高）。框架按同行文字的墨迹
// 包围盒算出差值，用绘制期 DrawY 抹平，布局与命中区域保持不变。
func TestIconAutoAlignToText(t *testing.T) {
	skipOnNonWindows(t, "图标对齐依赖 Microsoft YaHei UI 的字体度量")
	fc := newTestCache()
	defer fc.close()
	f := fc.getFamily(13, 500, "")
	if f == nil {
		t.Skip("缺少测试字体，跳过")
	}
	cases := []string{"DeepSeek-V4", "HTTP GET", "工作区内修改", "gogui"}
	for _, sample := range cases {
		icon := IconColored("star", 13, "#ffffff")
		label := Text(sample).St(St().Text(13, 500, "#ffffff"))
		root := Column(Row(icon, label).St(St().H(24).GapOf(6)).AlignTo(AlignCenter))
		Layout(root, 300, 40, fc)

		s := sample
		if rs := []rune(s); len(rs) > 4 {
			s = string(rs[:4])
		}
		top, gh := textGlyphBoundsOf(f, s)
		want := (top + gh/2) - f.LineHeight/2
		if lim := f.LineHeight * 0.15; want < -lim {
			want = -lim
		} else if want > lim {
			want = lim
		}
		if d := icon.Style.DrawY - want; d > 0.02 || d < -0.02 {
			t.Fatalf("%q 的对齐量不对：DrawY=%.3f 期望 %.3f", sample, icon.Style.DrawY, want)
		}
		// 只影响绘制，布局不变：图标仍在行内垂直居中
		wantY := float32(24-13) / 2
		if d := icon.Frame.Y - wantY; d > 0.01 || d < -0.01 {
			t.Fatalf("%q 图标布局被改动：Frame.Y=%.2f 期望 %.2f", sample, icon.Frame.Y, wantY)
		}
	}
}

// TestIconAutoAlignSkips 已显式平移的图标、纯图标行、非居中行都不修正。
func TestIconAutoAlignSkips(t *testing.T) {
	fc := newTestCache()
	defer fc.close()
	if fc.getFamily(13, 500, "") == nil {
		t.Skip("缺少测试字体，跳过")
	}
	label := func() *Node { return Text("标题文字").St(St().Text(13, 500, "#ffffff")) }

	// 1. 业务已用 Translate 微调过
	tuned := IconColored("star", 13, "#ffffff").St(St().Translate(0, -2))
	Layout(Row(tuned, label()).St(St().H(24)).AlignTo(AlignCenter), 300, 40, fc)
	if tuned.Style.DrawY != -2 {
		t.Fatalf("已显式平移的图标被覆盖：DrawY=%.3f", tuned.Style.DrawY)
	}

	// 2. 纯图标行（没有同行文字可参考）
	alone := IconColored("star", 13, "#ffffff")
	Layout(Row(alone).St(St().H(24)).AlignTo(AlignCenter), 300, 40, fc)
	if alone.Style.DrawY != 0 {
		t.Fatalf("纯图标行不应修正：DrawY=%.3f", alone.Style.DrawY)
	}

	// 3. 起点对齐的行（图标本就贴行顶，不该再动）
	start := IconColored("star", 13, "#ffffff")
	Layout(Row(start, label()).St(St().H(24)), 300, 40, fc)
	if start.Style.DrawY != 0 {
		t.Fatalf("非居中行不应修正：DrawY=%.3f", start.Style.DrawY)
	}
}
