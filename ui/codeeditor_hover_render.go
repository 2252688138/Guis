// guis — 原生代码编辑器（十三）：悬停提示的富文本排版
//
// gopls hover 返回的是 Markdown（代码块 + 标题 + 列表 + 段落）。直接压成纯文本
// 会变成一大坨，因此这里做一层轻量排版：
//   - 代码块（签名 / 字段）按编辑器语言做词法着色；
//   - 标题（# xxx）用强调色；
//   - 列表项加前缀符号；
//   - 段落之间插入一条虚线分隔（面板里不画文字的 hoverSep 行）。
package ui

import (
	"strings"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// codeHoverKind 悬停提示单行的类型（决定颜色 / 间距 / 是否虚线）。
type codeHoverKind uint8

const (
	// hoverPlainKind 普通正文。
	hoverPlainKind codeHoverKind = iota
	// hoverCodeKind 代码块内的行（签名、结构体字段、方法列表）。
	hoverCodeKind
	// hoverTitleKind 标题（# xxx / ## xxx）。
	hoverTitleKind
	// hoverBulletKind 列表项。
	hoverBulletKind
	// hoverSepKind 虚线分隔行（Text 为空，绘制时画虚线）。
	hoverSepKind
)

// 悬停面板的排版参数（宽度固定 380px）。
const (
	codeHoverRowH    = 17 // 普通行高
	codeHoverSepH    = 11 // 虚线行高
	codeHoverMaxRows = 18 // 最多行数（含虚线）
	codeHoverMaxRune = 46 // 正文单行最大字符数
	// codeHoverMaxCode 代码 / 标题行宽（签名更宽一点，尽量不被截断）
	codeHoverMaxCode = 50
	codeHoverPadX    = 12 // 左右内边距
	codeHoverPadY    = 7  // 上下内边距
)

// codeHoverLineH 行高（虚线行更矮）。
func codeHoverLineH(k codeHoverKind) float32 {
	if k == hoverSepKind {
		return codeHoverSepH
	}
	return codeHoverRowH
}

// hoverPanelHeight 面板总高（虚线行更矮）。
func (e *CodeEditorState) hoverPanelHeight() float32 {
	h := float32(codeHoverPadY * 2)
	if len(e.hoverKinds) != len(e.hoverLines) {
		// 外部直接改 hoverLines（右键菜单 / 业务代码）时没有类型信息，按普通行算
		return h + float32(len(e.hoverLines))*codeHoverRowH
	}
	for _, k := range e.hoverKinds {
		h += codeHoverLineH(k)
	}
	return h
}

// hoverKindAt 取第 i 行的类型（越界 / 未同步一律当普通行）。
func (e *CodeEditorState) hoverKindAt(i int) codeHoverKind {
	if i < 0 || i >= len(e.hoverKinds) || len(e.hoverKinds) != len(e.hoverLines) {
		return hoverPlainKind
	}
	return e.hoverKinds[i]
}

// hoverSub 取 [from, to) 区间的子串（列按 rune 计，越界自动截断）。
func hoverSub(s string, from, to int) string {
	rs := []rune(s)
	if from < 0 {
		from = 0
	}
	if to > len(rs) {
		to = len(rs)
	}
	if from >= to {
		return ""
	}
	return string(rs[from:to])
}

// setHoverLines 直接设置「文本行 + 类型」，同时把 hoverLines 同步成纯文本
// （外部 API / 测试按行取文本）。
func (e *CodeEditorState) setHoverLines(lines []string, kinds []codeHoverKind) {
	if len(kinds) != len(lines) {
		kinds = make([]codeHoverKind, len(lines))
	}
	e.hoverLines = lines
	e.hoverKinds = kinds
	e.hoverOpen = len(lines) > 0
}

// setHoverPlainLines 本地占位内容（符号名 + 本行诊断），与 Markdown 排版共用绘制。
func (e *CodeEditorState) setHoverPlainLines(lines []string) {
	if len(lines) == 0 {
		e.hoverLines, e.hoverKinds = nil, nil
		e.hoverOpen = false
		return
	}
	out := make([]string, 0, len(lines)+1)
	kinds := make([]codeHoverKind, 0, len(lines)+1)
	// 第一行是符号本身 → 用代码色
	out = append(out, clipHoverText(lines[0]))
	kinds = append(kinds, hoverCodeKind)
	for _, ln := range lines[1:] {
		out = append(out, "", "")
		kinds[len(kinds)-1] = hoverSepKind
		out = append(out, clipHoverText(ln))
		kinds = append(kinds, hoverPlainKind)
	}
	e.setHoverLines(out, kinds)
}

// setHoverMarkdown 把 gopls 的 Markdown 悬停内容排版成「行 + 类型」。
func (e *CodeEditorState) setHoverMarkdown(md string) {
	lines, kinds := renderHoverMarkdown(md)
	if len(lines) == 0 {
		e.hoverLines, e.hoverKinds = nil, nil
		e.hoverOpen = false
		return
	}
	e.setHoverLines(lines, kinds)
}

// clipHoverText 按面板宽度截断单行（超宽只留省略号）。
func clipHoverText(s string) string { return clipHoverTextN(s, codeHoverMaxRune) }

// clipHoverCode 代码 / 标题行截断（允许比正文宽几个字符）。
func clipHoverCode(s string) string { return clipHoverTextN(s, codeHoverMaxCode) }

func clipHoverTextN(s string, max int) string {
	s = strings.TrimRight(s, " \t")
	if rs := []rune(s); len(rs) > max {
		return string(rs[:max]) + "…"
	}
	return s
}

// renderHoverMarkdown 轻量 Markdown → 悬停面板行。
//
// 只处理 hover 里真正会出现的语法：围栏代码块、ATX 标题、- / * 列表、普通段落；
// 行内 **粗体** 与 `代码` 标记直接剔除，避免面板里出现星号。
func renderHoverMarkdown(md string) ([]string, []codeHoverKind) {
	raw := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	var kinds []codeHoverKind
	inCode := false
	// pendingSep 段落之间待插入的虚线
	pendingSep := false

	add := func(text string, k codeHoverKind) bool {
		if len(out) >= codeHoverMaxRows {
			return false
		}
		out = append(out, text)
		kinds = append(kinds, k)
		return true
	}
	addSep := func() {
		if len(out) == 0 {
			return
		}
		if kinds[len(kinds)-1] == hoverSepKind {
			return
		}
		if len(out) >= codeHoverMaxRows {
			return
		}
		add("", hoverSepKind)
	}

	for _, ln := range raw {
		t := strings.TrimRight(ln, " \t")
		trimmed := strings.TrimSpace(t)

		// 围栏代码块切换
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				// 代码块结束：后面接正文时补虚线
				pendingSep = true
			} else if pendingSep {
				addSep()
				pendingSep = false
			}
			inCode = !inCode
			continue
		}

		if trimmed == "" {
			if inCode {
				continue
			}
			pendingSep = true
			continue
		}

		if inCode {
			if !add(clipHoverCode(t), hoverCodeKind) {
				break
			}
			continue
		}

		if pendingSep {
			addSep()
			pendingSep = false
		}

		switch {
		case strings.HasPrefix(trimmed, "#"):
			title := strings.TrimLeft(trimmed, "#")
			addSep()
			if !add(clipHoverCode(strings.TrimSpace(title)), hoverTitleKind) {
				return out, kinds
			}
			pendingSep = true
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			if !add(clipHoverText("· "+trimHoverInline(trimmed[2:])), hoverBulletKind) {
				return out, kinds
			}
		case strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]"):
			// gopls 的符号引用列表（[Name] ...）
			if !add(clipHoverText("· "+trimHoverInline(trimmed)), hoverBulletKind) {
				return out, kinds
			}
		default:
			if !add(clipHoverText(trimHoverInline(trimmed)), hoverPlainKind) {
				return out, kinds
			}
		}
	}
	// 去掉尾部的虚线
	for len(out) > 0 && kinds[len(kinds)-1] == hoverSepKind {
		out = out[:len(out)-1]
		kinds = kinds[:len(kinds)-1]
	}
	return out, kinds
}

// trimHoverInline 去掉行内 markdown 标记（** 粗体、` 代码）。
func trimHoverInline(s string) string {
	if !strings.ContainsAny(s, "*`") {
		return s
	}
	return strings.NewReplacer("**", "", "`", "").Replace(s)
}

// hoverSepPaint 分隔虚线颜色（比面板描边略亮，深色背景上也看得清）。
func hoverSepPaint() gpu.Paint { return C("rgba(110,118,129,0.45)") }

// hoverLinePaint 悬停面板每行的颜色。
func (e *CodeEditorState) hoverLinePaint(k codeHoverKind) gpu.Paint {
	th := e.Theme
	switch k {
	case hoverCodeKind:
		return th.Type
	case hoverTitleKind:
		return th.Keyword
	case hoverBulletKind:
		return th.Number
	}
	return th.Text
}

// hoverCodeSpans 代码块行按编辑器语言做词法着色（拿不到语言就整行单一颜色）。
func (e *CodeEditorState) hoverCodeSpans(text string) []codeSpan {
	if e.Language == "" || e.Language == "plain" || e.Language == "markdown" {
		return nil
	}
	spans, _ := tokenizeCodeLine(e.Language, text, codeStateNormal)
	return spans
}

// hoverDash 虚线的一段（相对 x0 的起点 + 宽度）。
type hoverDash struct{ X, W float32 }

// hoverDashSegs 计算虚线段（4px 实 / 3px 空，末尾按剩余宽度截断）。
//
// 抽成纯函数是为了能单测：渲染器没有虚线描边能力，面板里的分隔线靠一组小矩形拼出来。
func hoverDashSegs(x0, x1 float32) []hoverDash {
	if x1 <= x0 {
		return nil
	}
	const seg, gap float32 = 4, 3
	out := make([]hoverDash, 0, int((x1-x0)/(seg+gap))+1)
	for x := x0; x < x1; x += seg + gap {
		w := seg
		if x+w > x1 {
			w = x1 - x
		}
		out = append(out, hoverDash{X: x, W: w})
	}
	return out
}

// drawHoverDashes 在 [x0, x1] 上按 4px 实 / 3px 空画一条虚线。
func drawHoverDashes(a *App, x0, x1, y float32, col gpu.Paint) {
	for _, d := range hoverDashSegs(x0, x1) {
		a.r.Rect(gpu.Shape{Rect: geom.Rect{X: d.X, Y: y, W: d.W, H: 1}, Fill: col})
	}
}
