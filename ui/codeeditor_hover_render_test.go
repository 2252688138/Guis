package ui

import (
	"strings"
	"testing"
)

// TestRenderHoverMarkdown 悬停 Markdown 排版：代码块 → 高亮行，段落间虚线，标题/列表分型。
func TestRenderHoverMarkdown(t *testing.T) {
	md := "```go\nfunc (s *Server) Run(ctx context.Context) error\n```\n\nRun 在超时时间内逐个执行处理器；任一失败即返回。\n\n# Printing\n\n- [Println] 写标准输出\n- [Sprintf] 返回字符串\n"
	lines, kinds := renderHoverMarkdown(md)

	if len(lines) != len(kinds) {
		t.Fatalf("行与类型数量不一致：%d / %d", len(lines), len(kinds))
	}
	if len(lines) == 0 {
		t.Fatal("排版结果为空")
	}
	// 首行是签名，用代码色
	if lines[0] != "func (s *Server) Run(ctx context.Context) error" || kinds[0] != hoverCodeKind {
		t.Fatalf("首行应为代码签名：%q kind=%d", lines[0], kinds[0])
	}
	joined := strings.Join(lines, "\n")
	for _, bad := range []string{"```", "**", "# Printing", "- ["} {
		if strings.Contains(joined, bad) {
			t.Fatalf("排版后不应残留 Markdown 标记 %q：\n%s", bad, joined)
		}
	}
	// 签名与正文之间要有虚线
	sepAfterSign := false
	for i := 1; i < len(kinds); i++ {
		if kinds[i] == hoverSepKind {
			sepAfterSign = true
			break
		}
	}
	if !sepAfterSign {
		t.Fatalf("代码块与正文之间缺少虚线分隔：%v", kinds)
	}
	// 标题行：去掉 # 并标记为标题
	title := -1
	for i, ln := range lines {
		if ln == "Printing" {
			title = i
		}
	}
	if title < 0 || kinds[title] != hoverTitleKind {
		t.Fatalf("标题行处理错误：%q %v", lines, kinds)
	}
	// 列表项带前缀
	bullet := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "· ") {
			bullet = i
		}
	}
	if bullet < 0 || kinds[bullet] != hoverBulletKind {
		t.Fatalf("列表项处理错误：%q %v", lines, kinds)
	}
	// 末尾不应留下孤立虚线
	if kinds[len(kinds)-1] == hoverSepKind {
		t.Fatalf("末尾不应有虚线：%v", kinds)
	}
}

// TestHoverDashes 分隔虚线：段宽 4、间隙 3、末尾截断、不超界；颜色可见（非全透明）。
func TestHoverDashes(t *testing.T) {
	ds := hoverDashSegs(10, 40)
	if len(ds) != 5 { // 30px / 7px = 4 整段 + 1 末段
		t.Fatalf("虚线段数 = %d，期望 5：%+v", len(ds), ds)
	}
	if ds[0].X != 10 || ds[0].W != 4 {
		t.Fatalf("首段应为 x=10 w=4：%+v", ds[0])
	}
	for i := 1; i < len(ds); i++ {
		if gap := ds[i].X - (ds[i-1].X + ds[i-1].W); gap != 3 {
			t.Fatalf("第 %d 段间隙 = %.0f，期望 3", i, gap)
		}
	}
	last := ds[len(ds)-1]
	if last.X+last.W != 40 {
		t.Fatalf("末段应截到 x1：%.0f", last.X+last.W)
	}
	if p := hoverSepPaint(); p.A <= 0.1 {
		t.Fatalf("虚线颜色太淡（A=%.2f），深色面板上看不见", p.A)
	}
	if segs := hoverDashSegs(50, 50); len(segs) != 0 {
		t.Fatalf("零宽度不应产生线段：%+v", segs)
	}
}
// TestHoverPanelHeight 虚线行更矮，面板高度按类型累加。
func TestHoverPanelHeight(t *testing.T) {
	e := NewCodeEditorState("hh", "x")
	e.setHoverLines([]string{"a", "", "b"}, []codeHoverKind{hoverCodeKind, hoverSepKind, hoverPlainKind})
	want := float32(codeHoverPadY*2) + codeHoverRowH + codeHoverSepH + codeHoverRowH
	if got := e.hoverPanelHeight(); got != want {
		t.Fatalf("面板高度 = %.1f，期望 %.1f", got, want)
	}
	// 外部直接改 hoverLines（无类型）时退化成等高的普通行
	e.hoverKinds = nil
	want = float32(codeHoverPadY*2) + 3*codeHoverRowH
	if got := e.hoverPanelHeight(); got != want {
		t.Fatalf("无类型信息时高度 = %.1f，期望 %.1f", got, want)
	}
}
