// guis — 原生代码编辑器（十二）：鼠标悬停语法提示
//
// 交互：鼠标停在标识符上约 450ms 后，先显示「词 + 本行诊断」占位，
// 再异步换成 LSP hover 结果（gopls 的类型 / 文档）；移到别的词、离开编辑器、
// 点击或输入时自动收起。
//
// 线程模型：坐标换算与状态修改都在 UI 线程；去抖用 time.AfterFunc 只投递一个
// 「到期」讯号（lspHoverDue），真正的取词请求在 drainLSP 里做，避免跨线程改状态。
package ui

import "time"

// codeHoverDelay 鼠标在标识符上停留多久弹出提示（太短会被扫过的鼠标频繁触发）。
const codeHoverDelay = 450 * time.Millisecond

// 悬停面板的截断参数与排版常量见 codeeditor_hover_render.go（codeHoverMaxRows 等）。

// mouseMoveAt 鼠标在编辑器内移动（每帧一次，坐标相对节点左上角）。
func (e *CodeEditorState) mouseMoveAt(x, y float32) {
	// 菜单 / 查找面板 / 补全面板 / 拖动过程中不弹提示
	if e.menuOpen || e.findOpen || e.compOpen || e.dragActive {
		e.hoverLeave()
		return
	}
	// 只在文本区取词：行号槽、小地图、滚动条不弹
	if x < e.gutterTotal()-e.ScrollX {
		e.hoverLeave()
		return
	}
	p := e.posAtXY(x, y)
	word := e.wordAt(p.Line, p.Col)
	if word == "" {
		e.hoverLeave()
		return
	}
	key := codeItoa(p.Line) + ":" + codeItoa(p.Col) + ":" + word
	e.hoverMouseX, e.hoverMouseY = x, y
	if key == e.hoverKeyNow {
		return
	}
	// 换了词：旧提示立刻收起（否则会带着上一个词的信息再挂 450ms）
	e.hideHover()
	e.hoverKeyNow = key
	e.hoverSeq++
	seq := e.hoverSeq
	// 每次换词都重新计时：鼠标快速扫过多个符号时，只有最后一个词的计时器
	// 能匹配当前序号（其余到期后自行丢弃），不会出现「扫完停下来却永不弹」。
	time.AfterFunc(codeHoverDelay, func() {
		pushLSP(lspPending{kind: lspHoverDue, seq: uint64(seq)})
	})
}

// hoverDue 去抖到期（UI 线程）：仍停在同一个符号上才真正请求。
func (e *CodeEditorState) hoverDue(seq uint64) {
	if int(seq) != e.hoverSeq || e.hoverKeyNow == "" {
		return
	}
	if e.menuOpen || e.findOpen || e.compOpen || e.dragActive {
		return
	}
	p := e.posAtXY(e.hoverMouseX, e.hoverMouseY)
	word := e.wordAt(p.Line, p.Col)
	if word == "" {
		return
	}
	e.hoverPinned = true
	e.requestHoverAt(p.Line, p.Col, e.hoverMouseX, e.hoverMouseY)
}

// hoverLeave 鼠标离开符号 / 编辑器：收起鼠标弹出的提示（F1 弹出的不在这里收）。
func (e *CodeEditorState) hoverLeave() {
	e.hoverKeyNow = ""
	e.hoverSeq++
	e.hideHover()
}

// hideHover 收起鼠标悬停弹出的提示。
func (e *CodeEditorState) hideHover() {
	if !e.hoverPinned {
		return
	}
	e.hoverPinned = false
	e.hoverOpen = false
	e.hoverLines = nil
	RequestRedraw()
}
