// guis — 节点树 → DSL 反向导出用例

package uidsl

import (
	"strings"
	"testing"

	"guis/ui"
)

// 导出结果应能被 parseDSL 重新解析（往返不报错）。
func TestFromNodeRoundTripParses(t *testing.T) {
	root := ui.Column(
		ui.Text("标题").St(ui.St().Text(20, 700, "#e8eefc")),
		ui.Row(
			ui.Box().St(ui.St().W(120).H(36).BgC("#2b7fff").R(8)).K("primary"),
		).St(ui.St().GapOf(8)),
	).K("root").St(ui.St().Padding(16).GapOf(12))

	dsl := FromNode(root)
	if dsl == "" {
		t.Fatal("导出结果为空")
	}
	if _, err := parseDSL(dsl); err != nil {
		t.Fatalf("导出的 DSL 无法解析：%v\n---\n%s", err, dsl)
	}
}

// 关键属性都要出现在文本里（缩进、类型、样式、Key）。
func TestFromNodeContainsProps(t *testing.T) {
	n := ui.Column(
		ui.Text("你好").St(ui.St().Text(18, 600, "#ffffff")),
	).K("wrap").St(ui.St().Padding(12).GapOf(6))

	dsl := FromNode(n)
	for _, want := range []string{
		"col", "key=wrap", "pad=12", "gap=6",
		"text \"你好\"", "size=18", "weight=600",
	} {
		if !strings.Contains(dsl, want) {
			t.Fatalf("导出结果缺少 %q：\n%s", want, dsl)
		}
	}
}

// 嵌套缩进：子节点比父节点多两个空格。
func TestFromNodeIndent(t *testing.T) {
	n := ui.Column(ui.Text("子")).K("p")
	dsl := FromNode(n)
	lines := strings.Split(strings.TrimRight(dsl, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("期望 2 行，实际 %d：\n%s", len(lines), dsl)
	}
	if !strings.HasPrefix(lines[0], "col") {
		t.Fatalf("首行应为 col：%q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  text") {
		t.Fatalf("子节点应缩进两格：%q", lines[1])
	}
}

// 文本里的引号 / 反斜杠要转义，不能破坏 DSL。
func TestFromNodeEscapesText(t *testing.T) {
	n := ui.Text(`他说"你好"\路径`)
	dsl := FromNode(n)
	if _, err := parseDSL(dsl); err != nil {
		t.Fatalf("转义后的 DSL 应可解析：%v\n%s", err, dsl)
	}
}

// 不支持导出的类型输出注释占位，不中断后续节点。
func TestFromNodeUnsupportedKind(t *testing.T) {
	n := ui.Column(
		ui.Video(nil),
		ui.Text("后面还要导出"),
	)
	dsl := FromNode(n)
	if !strings.Contains(dsl, "不支持导出的节点") {
		t.Fatalf("应出现占位注释：\n%s", dsl)
	}
	if !strings.Contains(dsl, "后面还要导出") {
		t.Fatalf("占位不应中断后续节点：\n%s", dsl)
	}
}

// 图片路径与图标名要带在行内。
func TestFromNodeImageAndIcon(t *testing.T) {
	img := FromNode(ui.Image("ui/assets/登录页.png"))
	if !strings.Contains(img, `"ui/assets/登录页.png"`) {
		t.Fatalf("图片路径未导出：%q", img)
	}
}

// nil 安全。
func TestFromNodeNil(t *testing.T) {
	if s := FromNode(nil); s != "" {
		t.Fatalf("nil 应导出空串，实际 %q", s)
	}
	if s := FromNodes(nil); s != "" {
		t.Fatalf("nil 切片应导出空串，实际 %q", s)
	}
}
