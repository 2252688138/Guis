// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"strings"
	"sync"
	"unicode/utf8"

	"guis/internal/text"
)

// forEachLine 按 '\n' 逐行回调，不分配行切片。
// 绘制（paintText）与测量（layoutCtx.measure）每帧都会遍历同一段文本，
// 旧实现的 splitLines 会为每个文本节点、每帧生成 1~2 个切片，滚动时 GC 压力明显。
func forEachLine(s string, fn func(line string)) {
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			fn(s[start:i])
			start = i + 1
		}
	}
	fn(s[start:])
}

// lineCount 行数（与 forEachLine 一致：空串算 1 行）。
func lineCount(s string) int { return strings.Count(s, "\n") + 1 }

// wrapTextRuns 按可用宽度折行并逐行回调（字节区间，不分配行切片/ rune 切片）。
//
// 断行优先级与旧实现完全一致：空格 / 连字符 / 斜杠优先，CJK 逐字可断，
// 收尾标点在它之前断行。
func wrapTextRuns(f *text.Font, s string, maxW float32, fn func(line string)) {
	if f == nil {
		fn(s)
		return
	}
	if maxW <= 0 {
		maxW = 1
	}
	start := 0
	width := float32(0)
	breakAt := -1 // 断行位置（字节下标，其之前的内容成行）
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\n' {
			fn(s[start:i])
			i += size
			start = i
			width = 0
			breakAt = -1
			continue
		}
		adv := f.Advance(r)
		if width+adv > maxW && i > start {
			cut := i
			if breakAt > start && breakAt < i {
				cut = breakAt
			}
			fn(s[start:cut])
			start = cut
			// 断行点后的空格不带到下一行
			for start < len(s) && s[start] == ' ' {
				start++
			}
			width = 0
			breakAt = -1
			i = start
			continue
		}
		width += adv
		switch {
		case r == ' ' || r == '\t' || r == '-' || r == '/' || r == '\u00a0':
			breakAt = i + size
		case isWideRune(r):
			// 中文/日文/韩文逐字可断；收尾标点前不断（断在标点之前）
			if isCJKPunct(r) {
				breakAt = i
			} else {
				breakAt = i + size
			}
		}
		i += size
	}
	fn(s[start:])
}

// wrapTextCount 折行后的行数（只数不建切片）。
func wrapTextCount(f *text.Font, s string, maxW float32) int {
	n := 0
	wrapTextRuns(f, s, maxW, func(string) { n++ })
	return n
}

// wrapTextLines 按可用宽度折行（贪心 + 断行优先级：空格 / 连字符 / CJK 字符边界）。
//
// 布局测量（layoutCtx.measure）与绘制（App.paintText）共用同一规则，
// 保证「测量出来的行数/高度」与「实际绘制的行数」完全一致。
// 热路径请直接用 wrapTextRuns 或 wrapLinesCached —— 它们不分配行切片 / 不重复计算。
func wrapTextLines(f *text.Font, s string, maxW float32) []string {
	out := make([]string, 0, 4)
	wrapTextRuns(f, s, maxW, func(ln string) { out = append(out, ln) })
	return out
}

// ---- 折行结果缓存 ----
//
// Markdown / 长文本页每帧都会对同一段落折行两次（测量 + 绘制），
// 每次都要逐字符取推进量（带锁）。按「字体代次 + 文本 + 宽度」缓存后，
// 命中时只剩一次 map 查找。缓存里的字符串是原文的子串（不复制内容）。

type wrapKey struct {
	// gen 字体代次而不是 *text.Font：字体被回收后地址会被新字体复用，
	// 用指针做键会命中上一代字体留下的行宽结果。
	gen uint32
	s   string
	w   int32 // 宽度按 0.5 像素量化，避免浮点抖动导致的永不命中
}

type wrapEntry struct {
	lines []string
	n     int // 缓存的行数（总预算控制用）
}

var (
	wrapMu        sync.RWMutex
	wrapMemo      = make(map[wrapKey]wrapEntry, 256)
	wrapLineCount int // 缓存里的总行数（预算控制用）
)

const (
	// wrapMemoMax 折行缓存条数上限。
	//
	// 旧实现只有 512 条、且超出时「整体清空」：长会话/长文档一屏的段落数
	// 就足以超过 512，结果每帧都在「清空 → 全部重新折行」，缓存等于不存在。
	wrapMemoMax = 4096
	// wrapLineBudget 缓存里的总行数预算（一行 ≈ 16B 切片 + 子串引用）。
	wrapLineBudget = 40000
)

// wrapLinesCached 折行并缓存结果。
// 返回的切片为缓存共享数据，调用方只读，禁止 append / 修改元素。
//
// 超出预算时按 map 的随机迭代顺序淘汰一批（约 1/8），不做整体清空：
// 整体清空会让长页面每帧都回到「全量重新折行」。
func wrapLinesCached(f *text.Font, s string, maxW float32) []string {
	if f == nil {
		return []string{s}
	}
	key := wrapKey{gen: f.Gen(), s: s, w: int32(maxW*2 + 0.5)}
	wrapMu.RLock()
	if e, ok := wrapMemo[key]; ok {
		wrapMu.RUnlock()
		return e.lines
	}
	wrapMu.RUnlock()

	lines := wrapTextLines(f, s, maxW)

	wrapMu.Lock()
	if _, exists := wrapMemo[key]; !exists {
		dropBudget := wrapMemoMax / 8
		if dropBudget < 32 {
			dropBudget = 32
		}
		for drop := 0; drop < dropBudget && (len(wrapMemo) >= wrapMemoMax || wrapLineCount+len(lines) > wrapLineBudget); drop++ {
			for k, e := range wrapMemo {
				wrapLineCount -= e.n
				delete(wrapMemo, k)
				break
			}
		}
		wrapMemo[key] = wrapEntry{lines: lines, n: len(lines)}
		wrapLineCount += len(lines)
	}
	wrapMu.Unlock()
	return lines
}

// clearWrapMemo 清空折行缓存（全局缩放 / 锐度变化时调用）。
//
// 单个字体被回收时**不需要**清空：缓存键里带字体代次，旧条目永远不会命中，
// 交给预算淘汰即可——以前每次字体回收都清全局缓存，等于把整页文本重新折行一遍。
func clearWrapMemo() {
	wrapMu.Lock()
	if len(wrapMemo) > 0 {
		wrapMemo = make(map[wrapKey]wrapEntry, 256)
		wrapLineCount = 0
	}
	wrapMu.Unlock()
}

// isWideRune 是否为全角（CJK）字符。
func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E, // CJK Radicals / Kangxi / CJK Symbols
		r >= 0x3041 && r <= 0x33FF, // Hiragana..CJK Compatibility
		r >= 0x3400 && r <= 0x4DBF, // CJK Ext A
		r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
		r >= 0xA000 && r <= 0xA4CF, // Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul Syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK Compatibility Ideographs
		r >= 0xFE30 && r <= 0xFE4F, // CJK Compatibility Forms
		r >= 0xFF00 && r <= 0xFF60, // Fullwidth Forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1FAFF: // Emoji
		return true
	}
	return false
}

// isCJKPunct 中文收尾标点（不在其前面断行会更好看，这里断在其之前）。
func isCJKPunct(r rune) bool {
	switch r {
	case '，', '。', '、', '；', '：', '！', '？', '）', '】', '》', '」', '』', '”', '’',
		',', '.', ';', ':', '!', '?', ')', ']', '}':
		return true
	}
	return false
}
