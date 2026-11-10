package ui

import "testing"

// TestCodeEditorMouseHover 鼠标停在标识符上 → 去抖通过后弹提示；换词立即收起旧提示。
func TestCodeEditorMouseHover(t *testing.T) {
	withCodeMeasure(t, 7)
	//                          0         1         2
	//                          0123456789012345678901234
	e := NewCodeEditorState("hover", "package main\n\nfunc foo() int { return 1 }\n")
	e.SetLanguage("go")
	viewSetup(e, 800, 400)

	const line = 2 // 第 3 行 "func foo() int { return 1 }"
	at := func(col int) (float32, float32) {
		return e.gutterTotal() - e.ScrollX + e.xOfCol(line, col),
			codeTopPad + (float32(line)+0.5)*e.lineH() - e.ScrollY
	}

	x, y := at(6) // foo 中间
	e.mouseMoveAt(x, y)
	if e.hoverKeyNow == "" {
		t.Fatalf("鼠标停在标识符上应记录悬停目标（x=%.1f y=%.1f）", x, y)
	}
	if e.hoverSeq == 0 {
		t.Fatal("首次悬停应启动去抖计时（序号前进）")
	}
	seq := e.hoverSeq
	if e.hoverOpen {
		t.Fatal("去抖未到期不应弹提示")
	}

	// 到期（由 drainLSP 调用）→ 弹本地提示
	e.hoverDue(uint64(seq))
	if !e.hoverOpen || !e.hoverPinned {
		t.Fatalf("悬停到期应弹提示：open=%v pinned=%v", e.hoverOpen, e.hoverPinned)
	}
	if len(e.hoverLines) == 0 || e.hoverLines[0] != "foo" {
		t.Fatalf("提示首行应为鼠标下符号：%v", e.hoverLines)
	}

	// 换到同行另一个词 int：旧提示立即收起，旧序号到期讯号不得重开
	x2, _ := at(12)
	e.mouseMoveAt(x2, y)
	if e.hoverOpen || e.hoverPinned {
		t.Fatalf("换词后应立刻收起旧提示：open=%v pinned=%v", e.hoverOpen, e.hoverPinned)
	}
	e.hoverDue(uint64(seq)) // 过期讯号
	if e.hoverOpen {
		t.Fatal("过期的到期讯号不应重新弹提示")
	}
	// 新一期到期 → 换成新词
	seq2 := e.hoverSeq
	if seq2 == seq {
		t.Fatalf("换词后序号应前进：%d", seq2)
	}
	e.hoverDue(uint64(seq2))
	if !e.hoverOpen || len(e.hoverLines) == 0 || e.hoverLines[0] != "int" {
		t.Fatalf("新词提示应为 int：open=%v lines=%v", e.hoverOpen, e.hoverLines)
	}

	// 移到空白处（行尾之后）：取不到词，收起
	e.mouseMoveAt(e.gutterTotal()+600, y)
	if e.hoverOpen || e.hoverPinned || e.hoverKeyNow != "" {
		t.Fatalf("空白处应收起提示：open=%v key=%q", e.hoverOpen, e.hoverKeyNow)
	}
}
