// guis — 原生代码编辑器（十一）：LSP 语义高亮
//
// 词法高亮（内置解析器）只能区分关键字 / 字符串 / 注释；语义高亮由语言服务器
// 给出「这个标识符是函数 / 类型 / 参数 / 枚举成员」，颜色更贴近 IDE。
//
// 数据流：semanticTokens/full（后台）→ pushLSP → drainLSP（UI 线程）→
// semantic[行] = []codeSpan。绘制时与词法片段合并，语义优先。
package ui

import (
	"sort"
	"strings"
	"time"
)

// codeSemToken 一条语义 token（行列按编辑器习惯：rune 列）。
type codeSemToken struct {
	Line, Col, Len int
	Kind           string
}

// semLegend 服务端 token 类型表（initialize 结果的 capabilities 里）。
func (c *LSPClient) semLegend() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st, ok := c.capabs["semanticTokensProvider"].(map[string]any); ok {
		if lg, ok := st["legend"].(map[string]any); ok {
			if arr, ok := lg["tokenTypes"].([]any); ok {
				out := make([]string, 0, len(arr))
				for _, v := range arr {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
		}
	}
	return nil
}

// SupportsSemantic 服务端是否支持语义 token。
func (c *LSPClient) SupportsSemantic() bool { return len(c.semLegend()) > 0 }

// SemanticTokens 请求全文语义 token（同步，调用方在后台协程里跑）。
func (c *LSPClient) SemanticTokens(uri string) ([]codeSemToken, bool) {
	if !c.Started() {
		return nil, false
	}
	legend := c.semLegend()
	if len(legend) == 0 {
		return nil, false
	}
	res, err := c.request("textDocument/semanticTokens/full", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
	if err != nil || res == nil {
		return nil, false
	}
	raw, ok := res["data"].([]any)
	if !ok || len(raw) < 5 {
		return nil, false
	}
	out := make([]codeSemToken, 0, len(raw)/5)
	line, col := 0, 0
	for i := 0; i+4 < len(raw); i += 5 {
		dl := lspInt(raw[i])
		dc := lspInt(raw[i+1])
		ln := lspInt(raw[i+2])
		ti := lspInt(raw[i+3])
		if i == 0 {
			line, col = dl, dc
		} else {
			line += dl
			if dl == 0 {
				col += dc
			} else {
				col = dc
			}
		}
		kind := ""
		if ti >= 0 && ti < len(legend) {
			kind = legend[ti]
		}
		if ln <= 0 {
			continue
		}
		out = append(out, codeSemToken{Line: line, Col: col, Len: ln, Kind: kind})
		if len(out) > 200000 {
			break
		}
	}
	return out, true
}

// lspInt JSON 数字统一是 float64；兼容整数。
func lspInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// semKindPaint 语义 token 类型 → 词法类别。
func semKindPaint(name string) (codeTokKind, bool) {
	switch name {
	case "keyword", "modifier":
		return codeKeyword, true
	case "type", "class", "interface", "enum", "struct", "namespace", "typeParameter":
		return codeType, true
	case "string", "regexp":
		return codeString, true
	case "comment":
		return codeComment, true
	case "number":
		return codeNumber, true
	case "function", "method", "macro":
		return codeFunction, true
	case "operator":
		return codeOperator, true
	case "constant", "enumMember":
		return codeConstant, true
	case "decorator", "annotation":
		return codeAttribute, true
	case "tag":
		return codeTag, true
	case "variable", "parameter", "property", "label":
		return codePlain, true // 变量不额外着色（保持文本色）
	}
	return codePlain, false
}

// ---- 编辑器侧 ----

// SetSemantic 开关语义高亮（UI 线程调用；开启且已接 LSP 时立即请求一次）。
func (e *CodeEditorState) SetSemantic(on bool) { e.setSemanticNow(on) }

// SetSemanticAsync 开关语义高亮（可在后台协程调用，下一帧在 UI 线程生效）。
func (e *CodeEditorState) SetSemanticAsync(on bool) {
	pushLSP(lspPending{kind: lspSemanticOn, on: on})
}

func (e *CodeEditorState) setSemanticNow(on bool) {
	if e.semanticOn == on {
		return
	}
	e.semanticOn = on
	if !on {
		e.semantic = nil
		e.semanticStamp++
		e.semanticBusy = false
		e.semanticEmpty = false
		RequestRedraw()
		return
	}
	e.RequestSemantic()
}

// SemanticOn 语义高亮是否开启。
func (e *CodeEditorState) SemanticOn() bool { return e.semanticOn }

// RequestSemantic 异步请求语义 token（服务端不支持时静默返回）。
//
// 服务端刚启动 / 刚改完代码时 workspace 可能还没 type-check 完，此时
// semanticTokens/full 会返回空数组（gopls 的常见行为）。这里按退避重试几次，
// 否则页面会一直停在「语义高亮：等待服务端」。
func (e *CodeEditorState) RequestSemantic() {
	if !e.semanticOn || e.semanticBusy {
		return
	}
	if e.srv == nil || e.Path == "" || !e.srv.Started() || !e.srv.SupportsSemantic() {
		return
	}
	e.semanticSeq++
	seq := e.semanticSeq
	e.semanticBusy = true
	e.semanticEmpty = false
	srv, uri := e.srv, FileURI(e.Path)
	go func() {
		// 结果（含空）都要回一条，UI 线程靠它清 busy 标记。
		defer func() { pushLSP(lspPending{kind: lspSemantic, seq: seq}) }()
		for i := 0; i < codeSemanticRetries; i++ {
			if i > 0 {
				time.Sleep(time.Duration(i) * 400 * time.Millisecond)
			}
			toks, ok := srv.SemanticTokens(uri)
			if !ok {
				return
			}
			if len(toks) > 0 {
				pushLSP(lspPending{kind: lspSemantic, semToks: toks, seq: seq})
				return
			}
		}
	}()
}

// applySemantic 把语义 token 展开成按行的着色片段（UI 线程）。
func (e *CodeEditorState) applySemantic(toks []codeSemToken) {
	if len(toks) == 0 {
		e.semantic = nil
		e.semanticStamp++
		return
	}
	out := make(map[int][]codeSpan, 256)
	for _, t := range toks {
		kind, ok := semKindPaint(t.Kind)
		if !ok || t.Line < 0 || t.Line >= len(e.Lines) {
			continue
		}
		n := runeLen(e.line(t.Line))
		start := t.Col
		end := t.Col + t.Len
		if start >= n {
			continue
		}
		if end > n {
			end = n
		}
		if end <= start {
			continue
		}
		out[t.Line] = append(out[t.Line], codeSpan{Pos: start, End: end, Kind: kind})
	}
	for line := range out {
		sort.Slice(out[line], func(i, j int) bool { return out[line][i].Pos < out[line][j].Pos })
	}
	e.semantic = out
	e.semanticStamp++
	RequestRedraw()
}

// mergeSemanticSpans 词法片段 + 语义片段合并（语义优先），结果按列升序、无重叠。
//
// 只对「本行有语义 token」的情况付出 O(行长) 的合并成本，其余行仍走词法切片。
func (e *CodeEditorState) mergeSemanticSpans(line int, base []codeSpan) []codeSpan {
	if len(e.semantic) == 0 {
		return base
	}
	sem := e.semantic[line]
	if len(sem) == 0 {
		return base
	}
	info := e.cache.lines[line]
	if info != nil && info.mergedStamp == e.semanticStamp && info.mergedText == e.linesText(line) {
		return info.merged
	}
	n := runeLen(e.line(line))
	if n == 0 {
		return nil
	}
	grid := make([]codeTokKind, n)
	for _, sp := range base {
		for c := sp.Pos; c < sp.End && c < n; c++ {
			if c >= 0 {
				grid[c] = sp.Kind
			}
		}
	}
	for _, sp := range sem {
		for c := sp.Pos; c < sp.End && c < n; c++ {
			if c >= 0 {
				grid[c] = sp.Kind
			}
		}
	}
	out := make([]codeSpan, 0, len(base)+len(sem))
	c := 0
	for c < n {
		kind := grid[c]
		start := c
		for c < n && grid[c] == kind {
			c++
		}
		if kind == codePlain && start == 0 {
			// 行首普通文本不单独成段（绘制时按 gap 用默认色）
			continue
		}
		if kind == codePlain {
			continue
		}
		out = append(out, codeSpan{Pos: start, End: c, Kind: kind})
	}
	if info != nil {
		info.merged, info.mergedStamp, info.mergedText = out, e.semanticStamp, e.linesText(line)
	}
	return out
}

// linesText 取行文本（带边界保护）。
func (e *CodeEditorState) linesText(line int) string {
	if line < 0 || line >= len(e.Lines) {
		return ""
	}
	return e.Lines[line]
}

// semanticLineCount 有语义着色的行数（状态栏 / 调试用）。
func (e *CodeEditorState) semanticLineCount() int { return len(e.semantic) }

// semanticSummary 语义高亮来源摘要（页面状态栏展示）。
func (e *CodeEditorState) SemanticSummary() string {
	if !e.semanticOn {
		return ""
	}
	if len(e.semantic) == 0 {
		if e.semanticEmpty {
			return "语义高亮：服务端未返回 token"
		}
		return "语义高亮：等待服务端"
	}
	return "语义高亮：" + codeItoa(e.semanticLineCount()) + " 行"
}

// trimSpaceLeft 去掉行首空白（绘制软换行续段时不重复缩进）。
func trimSpaceLeft(s string) string { return strings.TrimLeft(s, " \t") }
