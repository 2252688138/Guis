package ui

import "testing"

// TestFindPanelInputEditing 查找 / 替换面板的插入点编辑：插入、退格、Delete、←→、Home/End、Ctrl+A。
func TestFindPanelInputEditing(t *testing.T) {
	e := NewCodeEditorState("find-edit", "alpha\nbeta\n")
	e.OpenFind(0)
	e.findText = ""
	e.findCaret = 0

	// 顺序输入 abcd → 插入点在末尾
	for _, r := range "abcd" {
		e.appendFindRune(r)
	}
	if e.findText != "abcd" || e.findCaret != 4 {
		t.Fatalf("输入后 findText=%q caret=%d", e.findText, e.findCaret)
	}

	// ← ← 把插入点移到 b|cd，退格删掉前面的 b
	e.FindCaretMove(-1)
	e.FindCaretMove(-1)
	if e.findCaret != 2 {
		t.Fatalf("← 后 caret=%d，期望 2", e.findCaret)
	}
	e.FindBackspace()
	if e.findText != "acd" || e.findCaret != 1 {
		t.Fatalf("退格后 findText=%q caret=%d，期望 acd / 1", e.findText, e.findCaret)
	}

	// Delete 删插入点处的 c → ad
	e.FindDelete()
	if e.findText != "ad" || e.findCaret != 1 {
		t.Fatalf("Delete 后 findText=%q caret=%d，期望 ad / 1", e.findText, e.findCaret)
	}

	// Home / End 边界
	e.FindCaretHome()
	e.FindDelete()
	if e.findText != "d" || e.findCaret != 0 {
		t.Fatalf("Home+Delete 后 findText=%q caret=%d，期望 d / 0", e.findText, e.findCaret)
	}
	e.FindCaretMove(-5) // 越界应被夹到 0
	if e.findCaret != 0 {
		t.Fatalf("← 越界后 caret=%d，期望 0", e.findCaret)
	}
	e.FindCaretEnd()
	if e.findCaret != 1 {
		t.Fatalf("End 后 caret=%d，期望 1", e.findCaret)
	}

	// Ctrl+A 全选后输入一个字符：整体替换
	e.FindSelectAll()
	e.appendFindRune('z')
	if e.findText != "z" || e.findCaret != 1 || e.findSelAll {
		t.Fatalf("全选替换后 findText=%q caret=%d selAll=%v", e.findText, e.findCaret, e.findSelAll)
	}

	// 替换框独立维护自己的插入点
	e.findMode = 3
	e.replaceFocus = 1
	e.replaceText, e.replaceCaret = "XY", 2
	e.appendFindRune('Z')
	if e.replaceText != "XYZ" || e.replaceCaret != 3 {
		t.Fatalf("替换框输入后 text=%q caret=%d", e.replaceText, e.replaceCaret)
	}
	if e.findText != "z" {
		t.Fatalf("替换框输入不应改动查找串：%q", e.findText)
	}
}

// TestFindPanelKeyRouting 面板打开时，编辑器不能被方向键 / 退格 / Delete 改动（焦点只在面板）。
func TestFindPanelKeyRouting(t *testing.T) {
	e := NewCodeEditorState("find-keys", "package main\n\nfunc x() {}\n")
	e.OpenFind(0)
	e.findText, e.findCaret = "zzz", 3 // 不存在的串：避免实时定位干扰断言
	linesBefore := append([]string(nil), e.Lines...)
	posBefore := e.Pos

	// 纯编辑键：不能改动编辑器文档 / 光标
	for _, vk := range []uint32{0x25, 0x27, 0x24, 0x23, 0x2E, VK_BACK} {
		e.HandleKey(vk)
	}
	if len(e.Lines) != len(linesBefore) {
		t.Fatalf("面板按键改动了行数：%d → %d", len(linesBefore), len(e.Lines))
	}
	for i := range linesBefore {
		if e.Lines[i] != linesBefore[i] {
			t.Fatalf("面板按键改动了代码：第 %d 行 %q → %q", i+1, linesBefore[i], e.Lines[i])
		}
	}
	if e.Pos != posBefore {
		t.Fatalf("面板按键移动了编辑器光标：%+v → %+v", posBefore, e.Pos)
	}
	// VK_BACK 应作用在面板缓冲上（从 "zzz" 删成 "zz"）
	if e.findText != "zz" {
		t.Fatalf("退格没有作用在面板输入框：%q", e.findText)
	}

	// ↑/↓ 属于搜索跳转：命中时移动的是“匹配位置”，不是面板输入
	e.findText, e.findCaret = "main", 4
	if !e.HandleKey(0x28) {
		t.Fatal("↓ 应被面板消费")
	}
	if e.Pos.Line != 0 || e.Pos.Col < 8 {
		t.Fatalf("↓ 应定位到 main，实际 %+v", e.Pos)
	}
	if e.findText != "main" {
		t.Fatalf("↑↓ 不应改动查找串：%q", e.findText)
	}
}
