package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// withCodeMeasure 用「每字符 7px」的假字体测量替换真实字体（无窗口时 TextWidthOf 恒为 0）。
func withCodeMeasure(t *testing.T, perRune float32) {
	t.Helper()
	old := codeMeasure
	codeMeasure = func(s string, _ float32, _ string) float32 {
		return float32(len([]rune(s))) * perRune
	}
	t.Cleanup(func() { codeMeasure = old })
}

// viewSetup 模拟一帧绘制后的度量（viewW / viewH / textW / gutterW 由 paint 写入）。
func viewSetup(e *CodeEditorState, w, h float32) {
	e.viewW, e.viewH = w, h
	e.gutterW = 0
	e.ShowGutter = false
	e.textW = w
	e.MinimapW = 0
	e.ShowMinimap = false
}

// TestCodeEditorWrapMapping 软换行：分段、视觉行总数、视觉行↔文档行、坐标↔位置。
func TestCodeEditorWrapMapping(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("wrap1", strings.Repeat("x", 100))
	viewSetup(e, 212, 400) // textW-2 = 210 → 每段 30 个 rune
	e.SetWrap(true)
	segs := e.wrapSegs(0)
	if len(segs) != 3 {
		t.Fatalf("100 字符按 30 折行应有 3 个断点，实际 %v", segs)
	}
	if segs[0] != 30 || segs[1] != 60 || segs[2] != 90 {
		t.Fatalf("断点位置错误：%v", segs)
	}
	if got := e.totalRows(); got != 4 {
		t.Fatalf("视觉行数应为 4，实际 %d", got)
	}
	if line, seg := e.lineOfRow(3); line != 0 || seg != 3 {
		t.Fatalf("视觉行 3 应映射到 (0,3)，实际 (%d,%d)", line, seg)
	}
	// 第 4 段（视觉行 3）内 x=35 → 第 90+5=95 列
	p := e.PosAt(35, codeTopPad+3*e.lineH()+2)
	if p.Line != 0 || p.Col != 95 {
		t.Fatalf("坐标换算错误：%+v", p)
	}
	// 上下移动按视觉行走（不是文档行）
	e.SetCaret(CodePos{Line: 0, Col: 5}, false)
	e.MoveCaret(1, 0, false)
	if e.Pos.Line != 0 || e.Pos.Col < 30 {
		t.Fatalf("软换行下 ↓ 应在同一行的下一段：%+v", e.Pos)
	}
	// 关掉软换行恢复 1:1
	e.SetWrap(false)
	if got := e.totalRows(); got != 1 {
		t.Fatalf("关闭软换行后视觉行数应为 1，实际 %d", got)
	}
}

// TestCodeEditorFolding 折叠：区间识别、隐藏行、视觉行总数、光标回退。
func TestCodeEditorFolding(t *testing.T) {
	e := NewCodeEditorState("fold1", "func a() {\n\tx := 1\n\ty := 2\n}\nfunc b() {}\n")
	folds := e.computeFolds()
	if len(folds) != 1 || folds[0].Start != 0 || folds[0].End != 3 {
		t.Fatalf("折叠区间错误：%+v", folds)
	}
	if got := e.totalRows(); got != 6 {
		t.Fatalf("未折叠时视觉行应为 6，实际 %d", got)
	}
	if !e.ToggleFold(0) {
		t.Fatal("折叠失败")
	}
	if got := e.totalRows(); got != 3 {
		t.Fatalf("折叠后视觉行应为 3，实际 %d", got)
	}
	if !e.isHidden(2) || e.isHidden(4) || e.isHidden(0) {
		t.Fatalf("隐藏行判定错误：hidden(2)=%v hidden(4)=%v hidden(0)=%v",
			e.isHidden(2), e.isHidden(4), e.isHidden(0))
	}
	// 光标在折叠区内时，折叠后应回到折叠起始行
	e.ToggleFold(0) // 先展开
	if e.isHidden(2) {
		t.Fatal("展开失败")
	}
	e.cursors = nil
	e.SetCaret(CodePos{Line: 2, Col: 1}, false)
	e.ToggleFold(0)
	if e.Pos.Line != 0 {
		t.Fatalf("折叠后光标应回到起始行，实际 %+v", e.Pos)
	}
	e.FoldAll(true)
	if !e.isHidden(2) {
		t.Fatal("全部折叠失败")
	}
	e.FoldAll(false)
	if e.isHidden(2) {
		t.Fatal("全部展开失败")
	}
}

// TestCodeEditorCaretBlink 光标闪烁：操作后常亮 0.5s，之后 530ms 亮 / 530ms 灭。
func TestCodeEditorCaretBlink(t *testing.T) {
	old := codeNow
	now := float32(100)
	codeNow = func() float32 { return now }
	t.Cleanup(func() { codeNow = old })

	e := NewCodeEditorState("blink", "a")
	e.afterCaret()
	if !e.caretVisible() {
		t.Fatal("刚操作后应常亮")
	}
	now = 100.4
	if !e.caretVisible() {
		t.Fatal("0.4s 时应仍常亮")
	}
	now = 100 + 1.0 // 常亮 0.5s + 亮相位 0.5s
	if !e.caretVisible() {
		t.Fatal("1.0s 时应处于亮相位")
	}
	now = 100 + 2.2 // 相位 0.64s → 熄灭窗口
	if e.caretVisible() {
		t.Fatal("2.2s 时应处于熄灭相位")
	}
	if !e.caretVisible() == false {
		t.Fatal("")
	}
	// 再次操作立刻恢复常亮
	now = 100 + 5
	e.afterCaret()
	if !e.caretVisible() {
		t.Fatal("操作后应重新常亮")
	}
}

// TestCodeEditorScrollbarDrag 滚动条 / 小地图按住拖动（不再只支持点击轨道）。
func TestCodeEditorScrollbarDrag(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("drag", strings.Repeat("line\n", 1000))
	viewSetup(e, 800, 400)
	e.ShowMinimap, e.MinimapW = true, 84
	e.textW = 800 - 84 - 8
	g := e.scrollGeom()
	if g.thumbV.H <= 0 || g.maxY <= 0 {
		t.Fatalf("应有可拖动的竖向滚动条：%+v", g)
	}
	// 点轨道中点 → 约一半
	e.startDrag(codeHitScrollV, g.trackV.X+4, g.trackV.Y+g.trackV.H/2)
	frac := e.ScrollY / e.maxScrollY()
	if frac < 0.4 || frac > 0.6 {
		t.Fatalf("点轨道中点应滚到约一半，实际 %.2f", frac)
	}
	// 连续拖动到 80% → 继续跟手
	before := e.ScrollY
	e.dragScroll(codeHitScrollV, g.trackV.X+4, g.trackV.Y+g.trackV.H*0.8)
	if e.ScrollY <= before {
		t.Fatalf("向下拖动后 ScrollY 应变大：%.1f → %.1f", before, e.ScrollY)
	}
	// 小地图拖动
	e.ScrollY = 0
	e.startDrag(codeHitMinimap, g.minimap.X+10, g.minimap.Y+g.minimap.H*0.5)
	if e.ScrollY <= 0 {
		t.Fatalf("小地图拖动未滚动：%.1f", e.ScrollY)
	}
	// 拖到底不会越过上界
	e.dragScroll(codeHitScrollV, g.trackV.X+4, g.trackV.Y+g.trackV.H+500)
	if e.ScrollY > e.maxScrollY()+0.01 {
		t.Fatalf("滚动越界：%.1f > %.1f", e.ScrollY, e.maxScrollY())
	}
}

// TestCodeEditorHitZone 命中分区：滚动条 / 小地图 / 行号栏 / 文本区互不串门。
func TestCodeEditorHitZone(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("hit", strings.Repeat("x\n", 200))
	viewSetup(e, 800, 400)
	e.ShowMinimap, e.MinimapW, e.ShowGutter, e.gutterW = true, 84, true, 30
	e.textW = 800 - 30 - 84 - 8
	if z := e.hitZone(799, 200); z != codeHitScrollV {
		t.Fatalf("右侧应命中竖向滚动条，实际 %v", z)
	}
	if z := e.hitZone(740, 200); z != codeHitMinimap {
		t.Fatalf("小地图应命中小地图，实际 %v", z)
	}
	if z := e.hitZone(10, 200); z != codeHitGutter {
		t.Fatalf("行号栏应命中 gutter，实际 %v", z)
	}
	if z := e.hitZone(300, 200); z != codeHitText {
		t.Fatalf("文本区应命中 text，实际 %v", z)
	}
}

// TestCodeEditorReplaceAll 全部替换 / 撤销 / 单次替换。
func TestCodeEditorReplaceAll(t *testing.T) {
	e := NewCodeEditorState("rep", "foo bar foo\nFOO baz\n")
	e.OpenReplace()
	if !e.FindOpen() || e.findMode != 3 {
		t.Fatal("Ctrl+H 应打开替换面板")
	}
	e.findText, e.replaceText = "foo", "qux"
	if n := e.ReplaceAll(); n != 3 {
		t.Fatalf("大小写不敏感应替换 3 处，实际 %d", n)
	}
	if e.Lines[0] != "qux bar qux" || e.Lines[1] != "qux baz" {
		t.Fatalf("替换结果错误：%q", e.Lines[:2])
	}
	if !e.Undo() {
		t.Fatal("替换应可撤销")
	}
	if e.Lines[0] != "foo bar foo" || e.Lines[1] != "FOO baz" {
		t.Fatalf("撤销后未还原：%q", e.Lines[:2])
	}
	e.SetCaret(CodePos{}, false)
	e.findText, e.replaceText = "foo", "Z"
	if !e.ReplaceCurrent() {
		t.Fatal("替换当前应成功")
	}
	if e.Lines[0] != "Z bar foo" {
		t.Fatalf("单次替换错误：%q", e.Lines[0])
	}
}

// TestCodeEditorMultiCursor 多光标：新增 / 同时插入 / 同时退格 / 一次撤销。
func TestCodeEditorMultiCursor(t *testing.T) {
	e := NewCodeEditorState("multi", "aaa\nbbb\nccc")
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	if !e.AddCursor(1) {
		t.Fatal("Ctrl+Alt+↓ 应新增光标")
	}
	if e.CursorCount() != 2 {
		t.Fatalf("光标数应为 2，实际 %d", e.CursorCount())
	}
	e.InsertAtCarets(">")
	if e.Lines[0] != ">aaa" || e.Lines[1] != ">bbb" || e.Lines[2] != "ccc" {
		t.Fatalf("多光标插入错误：%q", e.Lines)
	}
	if !e.Undo() {
		t.Fatal("多光标编辑应可一次撤销")
	}
	if e.Lines[0] != "aaa" || e.Lines[1] != "bbb" {
		t.Fatalf("多光标撤销未还原：%q", e.Lines)
	}
	// 同时退格
	e.cursors = nil
	e.SetCaret(CodePos{Line: 0, Col: 3}, false)
	e.AddCursor(1)
	e.DeleteBack()
	if e.Lines[0] != "aa" || e.Lines[1] != "bb" {
		t.Fatalf("多光标退格错误：%q", e.Lines)
	}
	// Esc 收起
	e.ClearCursors()
	if e.HasMultiCursor() {
		t.Fatal("ClearCursors 未生效")
	}
}

// TestCodeEditorMenuCompact 右键菜单控制在 20 项内，语言 / 诊断信息移到状态栏。
func TestCodeEditorMenuCompact(t *testing.T) {
	e := NewCodeEditorState("menu", "package main\n")
	e.SetLanguage("go")
	items := e.menuItems()
	if len(items) > 20 {
		t.Fatalf("右键菜单 %d 行，应 ≤ 20", len(items))
	}
	labels := make(map[string]bool, len(items))
	for _, it := range items {
		if strings.HasPrefix(it.Label, "语法/语言") || strings.HasPrefix(it.Label, "诊断：") {
			t.Fatalf("语言 / 诊断条数应移到状态栏，不应留在菜单：%q", it.Label)
		}
		labels[it.Label] = true
	}
	for _, must := range []string{"撤销", "复制", "查找", "替换", "软换行", "问题面板"} {
		if !labels[must] {
			t.Fatalf("菜单缺少 %q", must)
		}
	}
	status := e.StatusLine()
	if !strings.Contains(status, "go") || !strings.Contains(status, "诊断") {
		t.Fatalf("状态栏应包含语言与诊断条数：%q", status)
	}
}

// TestCodeEditorDiagPanel 问题面板：打开 / 点击跳转 / 上下步进。
func TestCodeEditorDiagPanel(t *testing.T) {
	e := NewCodeEditorState("diag", "func f() {\n\tx := 1\n}\n")
	viewSetup(e, 900, 500)
	e.SetDiagnostics([]CodeDiagnostic{
		{Line: 2, Col: 0, Severity: 1, Message: "多余的右括号", Source: "test"},
		{Line: 0, Col: 0, Severity: 2, Message: "未闭合", Source: "test"},
	})
	e.ToggleDiagPanel()
	if !e.diagOpen {
		t.Fatal("F8 应打开问题面板")
	}
	list := e.DiagnosticsList()
	if len(list) != 2 || list[0].Line != 0 {
		t.Fatalf("诊断列表应按行排序：%+v", list)
	}
	r := e.diagPanelRect()
	if !e.DiagPanelClick(r.X+r.W/2, r.Y+24+codeDiagPanelRowH*1.5) {
		t.Fatal("点击诊断项应被面板消费")
	}
	if e.Pos.Line != 2 {
		t.Fatalf("应跳到第二条诊断（第 2 行），实际第 %d 行", e.Pos.Line)
	}
	e.DiagStep(-1)
	if e.Pos.Line != 0 {
		t.Fatalf("↑ 应回到第一条，实际第 %d 行", e.Pos.Line)
	}
	e.ToggleDiagPanel()
	if e.diagOpen {
		t.Fatal("再次 F8 应收起面板")
	}
}

// TestCodeEditorSemanticMerge LSP 语义高亮覆盖词法高亮。
func TestCodeEditorSemanticMerge(t *testing.T) {
	e := NewCodeEditorState("sem", "let alpha = 1")
	e.SetLanguage("javascript")
	e.semanticOn = true
	e.applySemantic([]codeSemToken{{Line: 0, Col: 4, Len: 5, Kind: "function"}})
	info, _ := e.cache.get(e, 0, 0)
	spans := e.mergeSemanticSpans(0, info.spans)
	found := false
	for _, sp := range spans {
		if sp.Pos == 4 && sp.End == 9 && sp.Kind == codeFunction {
			found = true
		}
	}
	if !found {
		t.Fatalf("语义 token 未生效：%+v", spans)
	}
	if e.SemanticSummary() == "" {
		t.Fatal("语义高亮摘要不应为空")
	}
	// 关闭后回到纯词法
	e.SetSemantic(false)
	if e.semantic != nil {
		t.Fatal("关闭语义高亮应清空缓存")
	}
}

// TestCodeEditorWrapHugeDocEstimate 十万行 + 软换行：首帧不能全量测量（只看视口）。
func TestCodeEditorWrapHugeDocEstimate(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("huge", "")
	e.Lines = make([]string, 100000)
	for i := range e.Lines {
		e.Lines[i] = fmt.Sprintf("\tvalue%06d := compute(%d)", i, i)
	}
	e.cache.reset()
	e.resetViewState()
	viewSetup(e, 600, 600)
	e.SetWrap(true)
	start := time.Now()
	if got := e.sumRows(); got != 100000 {
		t.Fatalf("未走到的行应按 1 行估算，实际总数 %d", got)
	}
	first, last, _ := e.lineRange(600)
	if first != 0 || last < 10 {
		t.Fatalf("可见行区间错误：%d..%d", first, last)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("十万行软换行首帧过慢：%v", d)
	}
	// 滚到中部：仍然只补算视口附近
	e.ScrollY = 500000
	e.clampScroll()
	mid, _, _ := e.lineRange(600)
	if mid < 20000 {
		t.Fatalf("中部滚动定位错误：%d", mid)
	}
}

// TestCodeEditorLexCacheNotPoisoned 列宽 / 软换行缓存不能把「未词法分析」的占位项当成命中。
// 历史 bug：列宽缓存先建了只有 text 的行缓存，词法器命中后返回空 spans，整篇失去语法高亮。
func TestCodeEditorLexCacheNotPoisoned(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("lex", "func main() {\n\ts := \"hi\"\n}\n")
	e.SetLanguage("go")
	e.cache.reset()
	// 先算列宽（会创建占位项），再取词法结果
	_ = e.xOfCol(0, 5)
	info, next := e.cache.get(e, 0, 0)
	if len(info.spans) == 0 {
		t.Fatal("列宽缓存污染了词法缓存：spans 为空（整篇会没有语法高亮）")
	}
	kinds := map[codeTokKind]bool{}
	for _, sp := range info.spans {
		kinds[sp.Kind] = true
	}
	if !kinds[codeKeyword] || !kinds[codeFunction] {
		t.Fatalf("关键字 / 函数名未着色：%+v", info.spans)
	}
	info2, _ := e.cache.get(e, 1, next)
	if len(info2.spans) == 0 {
		t.Fatal("第二行没有词法结果")
	}
	hasStr := false
	for _, sp := range info2.spans {
		if sp.Kind == codeString {
			hasStr = true
		}
	}
	if !hasStr {
		t.Fatalf("字符串未着色：%+v", info2.spans)
	}
}

// TestCodeEditorIndentGuides Tab 缩进按档位算宽度（否则参考线会穿进正文）。
func TestCodeEditorIndentGuides(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("ind", "")
	tabPx := float32(e.tabSize()) * 7
	if got := e.indentWidthPx("\tx", tabPx); got != tabPx {
		t.Fatalf("一个 Tab 应等于一档：%.1f != %.1f", got, tabPx)
	}
	if got := e.indentWidthPx("    x", tabPx); got != tabPx {
		t.Fatalf("四个空格应等于一档：%.1f", got)
	}
	if got := e.indentWidthPx("\t\tx", tabPx); got != tabPx*2 {
		t.Fatalf("两个 Tab 应为两档：%.1f", got)
	}
	if got := e.indentWidthPx("  \tx", tabPx); got != tabPx {
		t.Fatalf("混合缩进应对齐到下一档：%.1f", got)
	}
	if got := e.indentWidthPx("func", tabPx); got != 0 {
		t.Fatalf("无缩进应为 0：%.1f", got)
	}
}
