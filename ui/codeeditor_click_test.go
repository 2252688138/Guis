package ui

import "testing"

// TestCodeEditorPlainClickClearsCursors 普通单击必须收起额外光标。
//
// 回归：额外光标一直留在文档里，界面上同时画出两个插入符（双光标），
// 之后的输入 / 退格按所有光标批量生效，点别处看起来就像「选中了多行」。
func TestCodeEditorPlainClickClearsCursors(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("click1", "aaa\nbbb\nccc\nddd")
	viewSetup(e, 400, 300)
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	if !e.AddCursor(1) || e.CursorCount() != 2 {
		t.Fatalf("准备多光标失败：%d", e.CursorCount())
	}

	// 普通单击第 3 行第 2 列
	e.TextClick(7, codeTopPad+2*e.lineH()+2, false)
	if e.CursorCount() != 1 {
		t.Fatalf("普通单击后应只剩主光标，实际 %d 个", e.CursorCount())
	}
	if e.Pos.Line != 2 {
		t.Fatalf("普通单击应定位到第 3 行，实际 %+v", e.Pos)
	}
	if _, ok := e.Selection(); ok {
		t.Fatal("普通单击后不应有选区")
	}

	// Alt+点击仍应新增光标
	e.TextClick(7, codeTopPad+3*e.lineH()+2, true)
	if e.CursorCount() != 2 {
		t.Fatalf("Alt+点击应新增一个光标，实际 %d 个", e.CursorCount())
	}
}

// TestCodeEditorMouseDragSelectsFromPressPoint 拖动选区从按下点开始，且按下帧不改选区。
//
// 回归：框架的 OnDrag 在按下帧就会回调，旧实现直接 DragTo，于是「只是点一下」
// 也会从上一个光标位置拉出一条跨行选区。
func TestCodeEditorMouseDragSelectsFromPressPoint(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("drag1", "aaa\nbbb\nccc\nddd")
	viewSetup(e, 400, 300)
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)
	e.AddCursor(1) // 拖动前残留的额外光标应被收起

	px, py := float32(7), codeTopPad+e.lineH()+2 // 第 2 行第 1 列
	e.MouseDrag(px, py)                          // 按下帧
	if r, ok := e.Selection(); ok {
		t.Fatalf("按下帧不应产生选区：%+v", r)
	}
	if e.Pos != (CodePos{Line: 0, Col: 0}) {
		t.Fatalf("按下帧不应移动光标：%+v", e.Pos)
	}

	e.MouseDrag(px+14, py) // 越过 3px 阈值，同一行往后拖
	r, ok := e.Selection()
	if !ok || r.Start.Line != 1 || r.Start.Col != 1 || r.End.Line != 1 {
		t.Fatalf("拖动选区应从按下点开始且不跨行：%+v ok=%v", r, ok)
	}
	if e.CursorCount() != 1 {
		t.Fatalf("拖动开始后应收起额外光标，实际 %d 个", e.CursorCount())
	}
	e.MouseDragEnd()
	if !e.skipClick {
		t.Fatal("拖动结束后应忽略紧随其后的松手点击")
	}
}

// TestCodeEditorMouseDragJitter 轻微抖动（<3px）不算拖动，不应出现选区。
func TestCodeEditorMouseDragJitter(t *testing.T) {
	withCodeMeasure(t, 7)
	e := NewCodeEditorState("drag2", "aaa\nbbb\nccc")
	viewSetup(e, 400, 300)
	e.SetCaret(CodePos{Line: 0, Col: 0}, false)

	px, py := float32(7), codeTopPad+e.lineH()+2
	e.MouseDrag(px, py)
	e.MouseDrag(px+2, py+1)
	if r, ok := e.Selection(); ok {
		t.Fatalf("抖动不应产生选区：%+v", r)
	}
	e.MouseDragEnd()
	if e.skipClick {
		t.Fatal("未越过阈值的按下不应吞掉松手点击")
	}
}
