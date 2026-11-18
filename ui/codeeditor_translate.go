// guis — 框架：编辑器内的异步翻译钩子（悬停说明 + 诊断消息）
//
// 编辑器不关心翻译来源：页面设置 OnTranslate 之后，悬停 Markdown 与诊断消息
// 先按原文显示，done(译文) 回来了再重排一次 —— 既不卡开面板，也不会被旧译文
// 覆盖新悬停。译文按原文缓存在编辑器内，同一条内容只请求一次。
// OnTranslate 为 nil（默认）时完全走原来的路径，行为不变。
package ui

import "strings"

// codeTransCacheMax 翻译缓存上限：超了整体重建，避免长期开着编辑器无限增长。
const codeTransCacheMax = 512

// translateCached 取译文缓存。
func (e *CodeEditorState) translateCached(raw string) (string, bool) {
	if e.transCache == nil {
		return "", false
	}
	v, ok := e.transCache[raw]
	return v, ok
}

// translateStore 写译文缓存（只在 UI 线程调用）。
func (e *CodeEditorState) translateStore(raw, zh string) {
	if raw == "" || zh == "" || zh == raw {
		return
	}
	if len(e.transCache) >= codeTransCacheMax {
		e.transCache = make(map[string]string, 8)
	}
	if e.transCache == nil {
		e.transCache = make(map[string]string, 8)
	}
	e.transCache[raw] = zh
}

// translateHoverMarkdown 悬停文本过一遍翻译钩子。
//
// 返回原文立刻显示；译文回来后投递 lspHoverTranslated，由 drainLSP 校验
// 「当前悬停仍是这条原文」再重排，避免鼠标移走后旧译文覆盖新内容。
func (e *CodeEditorState) translateHoverMarkdown(md string) string {
	if e.OnTranslate == nil || strings.TrimSpace(md) == "" {
		return md
	}
	if v, ok := e.translateCached(md); ok {
		return v
	}
	raw := md
	e.hoverRaw = raw
	e.OnTranslate(raw, func(zh string) {
		pushLSP(lspPending{kind: lspHoverTranslated, md: raw, text: zh})
	})
	return raw
}

// translateDiags 诊断消息过翻译钩子：命中缓存直接换中文，未命中的先显示原文
// 并异步请求（同一原文在途只请求一次）。
func (e *CodeEditorState) translateDiags(ds []CodeDiagnostic) []CodeDiagnostic {
	e.diagRaw = ds
	if len(ds) == 0 || e.OnTranslate == nil {
		return ds
	}
	out := make([]CodeDiagnostic, len(ds))
	copy(out, ds)
	for i := range out {
		raw := ds[i].Message
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if v, ok := e.translateCached(raw); ok {
			out[i].Message = v
			continue
		}
		e.requestDiagTranslate(raw)
	}
	return out
}

// requestDiagTranslate 为一条诊断原文发起翻译（在途去重，结果回到 UI 线程再落缓存）。
func (e *CodeEditorState) requestDiagTranslate(raw string) {
	if e.diagInFlight == nil {
		e.diagInFlight = make(map[string]bool, 4)
	}
	if e.diagInFlight[raw] {
		return
	}
	e.diagInFlight[raw] = true
	e.OnTranslate(raw, func(zh string) {
		pushLSP(lspPending{kind: lspDiagsTranslated, md: raw, text: zh})
	})
}

// applyHoverTranslated 悬停译文回来：仍是当前悬停内容才重排。
func (e *CodeEditorState) applyHoverTranslated(raw, zh string) {
	if !e.hoverOpen || e.hoverRaw != raw {
		return
	}
	zh = strings.TrimSpace(zh)
	if zh == "" || zh == raw {
		return
	}
	e.translateStore(raw, zh)
	e.setHoverMarkdown(zh) // 此时缓存命中，不会再发起请求
}

// applyDiagsTranslated 诊断译文回来：落缓存后按最近一次诊断原文重建列表。
func (e *CodeEditorState) applyDiagsTranslated(raw, zh string) {
	delete(e.diagInFlight, raw)
	zh = strings.TrimSpace(zh)
	if zh == "" || zh == raw {
		return
	}
	e.translateStore(raw, zh)
	if len(e.diagRaw) == 0 {
		return
	}
	e.lspDiags = e.translateDiags(e.diagRaw)
	RequestRedraw()
}
