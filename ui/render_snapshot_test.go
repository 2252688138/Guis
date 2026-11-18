// guis — 绘制属性快照回归
//
// 与像素级 golden（ui/golden_test.go）互补：
//   - golden 看「最终长什么样」，但需要 GL 上下文，无显卡环境跑不了；
//   - 这里看「交给绘制层的参数」（颜色 / 圆角 / 描边 / 阴影 / 字体 / 淡化 /
//     裁剪 / 文本内容），纯 CPU、跨平台稳定，任何环境都能跑。
//
// 主题色、圆角、字号这类「改一行影响全局观感」的改动，会被本快照第一时间抓到。
//
// 更新快照：
//
//	$env:UPDATE_SNAPSHOTS=1; go test ./ui/ -run TestRenderSnapshot

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guis/internal/gpu"
)

// snapshotPaint 序列化一幅绘制样式（颜色或渐变端点），保留 3 位小数。
func snapshotPaint(p gpu.Paint) string {
	if p.R == 0 && p.G == 0 && p.B == 0 && p.A == 0 {
		return "-"
	}
	return fmt.Sprintf("%.3f/%.3f/%.3f/%.3f", p.R, p.G, p.B, p.A)
}

// snapshotRender 把整棵树的绘制参数序列化成稳定文本。
func snapshotRender(root *Node) string {
	var sb strings.Builder
	var walk func(n *Node, path string, depth int)
	walk = func(n *Node, path string, depth int) {
		if n == nil {
			return
		}
		st := n.Style
		fmt.Fprintf(&sb, "%s%s kind=%s key=%q box=%.1f,%.1f,%.1f,%.1f",
			strings.Repeat("  ", depth), path, kindName(n.Kind), n.Key,
			n.Frame.X, n.Frame.Y, n.Frame.W, n.Frame.H)
		if n.Text != "" {
			fmt.Fprintf(&sb, " text=%q", n.Text)
		}
		fmt.Fprintf(&sb, " bg=%s", snapshotPaint(st.Bg))
		if st.BgToSet {
			fmt.Fprintf(&sb, " bgTo=%s ang=%.1f", snapshotPaint(st.BgTo), st.BgAngle)
		}
		var r [4]float32 = st.Radius
		if r != [4]float32{} {
			fmt.Fprintf(&sb, " r=%.1f/%.1f/%.1f/%.1f", r[0], r[1], r[2], r[3])
		}
		if st.Border > 0 {
			fmt.Fprintf(&sb, " border=%.1f:%s", st.Border, snapshotPaint(st.BorderColor))
		}
		if st.ShadowBlur > 0 || st.ShadowSpread > 0 {
			fmt.Fprintf(&sb, " shadow=%.1f/%.1f/%.1f:%s", st.ShadowBlur, st.ShadowY,
				st.ShadowSpread, snapshotPaint(st.ShadowColor))
		}
		if st.FontSize > 0 {
			fmt.Fprintf(&sb, " font=%.1f/%d/%s", st.FontSize, st.FontWeight, snapshotPaint(st.Color))
		}
		if st.LineHeight > 0 {
			fmt.Fprintf(&sb, " lh=%.1f", st.LineHeight)
		}
		if st.Opacity != 1 {
			fmt.Fprintf(&sb, " opacity=%.2f", st.Opacity)
		}
		if st.Wrap {
			sb.WriteString(" wrap")
		}
		if st.Clip {
			sb.WriteString(" clip")
		}
		if st.PosAbs {
			sb.WriteString(" abs")
		}
		sb.WriteString("\n")
		for i, c := range n.Children {
			walk(c, fmt.Sprintf("%s/%d", path, i), depth+1)
		}
	}
	walk(root, "0", 0)
	return sb.String()
}

// renderSnapshotCases 覆盖基础盒模型 + 常用组件 + 主题色。
func renderSnapshotCases() []GoldenCase {
	out := GoldenCases()
	out = append(out, GoldenCase{
		Name: "widgets", Width: 420, Height: 260,
		View: func() *Node {
			return Column(
				Text("标题").St(St().Text(16, 700, ColText)),
				Text("说明文字，视觉回归要能抓到字号/颜色的变化").St(St().Text(12.5, 400, ColTextMuted)),
				Row(
					Button("主要", func() {}),
					Button("次要", func() {}).St(St().BgC(ColSurfaceMuted)),
					Chip("标签", true, nil),
				).St(St().GapOf(8)),
				Progress(0.42, ""),
			).St(St().Padding(16).GapOf(10).BgC(ColBg).Hp(100))
		},
	})
	return out
}

// TestRenderSnapshot 绘制属性快照（UPDATE_SNAPSHOTS=1 生成 / 更新）。
func TestRenderSnapshot(t *testing.T) {
	withTestFonts(t)
	update := os.Getenv("UPDATE_SNAPSHOTS") != ""
	dir := filepath.Join("testdata", "render_snapshot")
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("创建快照目录失败：%v", err)
		}
	}
	for _, c := range renderSnapshotCases() {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			root := c.View()
			Layout(root, float32(c.Width), float32(c.Height), newTestCache())
			got := snapshotRender(root)
			path := filepath.Join(dir, c.Name+".txt")
			if update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("写快照失败：%v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("缺少快照 %s（UPDATE_SNAPSHOTS=1 生成）", path)
			}
			if string(want) != got {
				t.Fatalf("绘制属性回归：%s\n--- 基线 ---\n%s\n--- 本次 ---\n%s", c.Name, want, got)
			}
		})
	}
}
