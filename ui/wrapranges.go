// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "sync"

// 折行区间：把一段文本按可用宽度切成若干视觉行，每行是 [start,end) 的 rune 下标。
//
// 输入框（WrapInput）、文本域与业务自绘的输入区（cmd/deepseek 的 composer）共用这套算法：
// 之前两处各写一份，且都没有缓存——长文本每帧重建时要把整段逐字测量一遍
//（1900 rune ≈ 216µs / 2005 次分配），输入框一刷新就白烧一帧的时间。

// wrapRangesKey 折行区间缓存的键。
//
// 宽度按 0.5 像素量化：布局算出来的宽度常带浮点尾数，不量化会永不命中。
type wrapRangesKey struct {
	s      string
	avail  int32
	size   float32
	weight int32
}

type wrapRangesEntry struct {
	rows [][2]int
	n    int // 缓存的行数（总预算控制用）
}

var (
	wrapRangesMu    sync.RWMutex
	wrapRangesMemo  = make(map[wrapRangesKey]wrapRangesEntry, 64)
	wrapRangesCount int // 缓存里的总行数（预算控制用）
)

const (
	// wrapRangesMemoMax 折行区间缓存条数上限（同时存在的输入框不会太多）。
	wrapRangesMemoMax = 256
	// wrapRangesBudget 缓存里的总行数预算（一行 ≈ 16B 切片头）。
	wrapRangesBudget = 20000
)

// emptyWrapRanges 空文本的折行结果（空输入框也要有 1 行，光标才有地方落）。
var emptyWrapRanges = [][2]int{{0, 0}}

// WrapRanges 把文本按可用宽度折成视觉行（每行是 [start,end) 的 rune 区间）。
// avail <= 0 表示宽度未知：只按 '\n' 分行；'\n' 本身不占行内字符。
//
// 结果按「文本 + 可用宽度 + 字号/字重」缓存，同一段文本在测量、绘制与多帧之间只算一次。
// 返回的切片为缓存共享数据，调用方只读，禁止 append / 修改元素。
func WrapRanges(rs []rune, avail, size float32, weight int32) [][2]int {
	if len(rs) == 0 {
		return emptyWrapRanges
	}
	key := wrapRangesKey{s: string(rs), avail: int32(avail*2 + 0.5), size: size, weight: weight}
	wrapRangesMu.RLock()
	if e, ok := wrapRangesMemo[key]; ok {
		wrapRangesMu.RUnlock()
		return e.rows
	}
	wrapRangesMu.RUnlock()

	rows := calcWrapRanges(rs, avail, size, weight)
	wrapRangesMu.Lock()
	for len(wrapRangesMemo) >= wrapRangesMemoMax || wrapRangesCount+len(rows) > wrapRangesBudget {
		if len(wrapRangesMemo) == 0 {
			break
		}
		// 按 map 的随机迭代顺序淘汰一条：不做整体清空，否则长页面会每帧全量重算
		for k, e := range wrapRangesMemo {
			wrapRangesCount -= e.n
			delete(wrapRangesMemo, k)
			break
		}
	}
	wrapRangesMemo[key] = wrapRangesEntry{rows: rows, n: len(rows)}
	wrapRangesCount += len(rows)
	wrapRangesMu.Unlock()
	return rows
}

// calcWrapRanges 逐字贪心折行（CJK 与西文一视同仁，按 rune 宽度累加）。
func calcWrapRanges(rs []rune, avail, size float32, weight int32) [][2]int {
	out := make([][2]int, 0, 4)
	start := 0
	acc := float32(0)
	for i, r := range rs {
		if r == '\n' {
			out = append(out, [2]int{start, i})
			start, acc = i+1, 0
			continue
		}
		w := TextWidth(string(r), size, weight)
		if avail > 0 && acc+w > avail && i > start {
			out = append(out, [2]int{start, i})
			start, acc = i, 0
		}
		acc += w
	}
	return append(out, [2]int{start, len(rs)})
}

// ---- 单行横向滚动起点 ----

// scrollStartKey 滚动起点缓存的键：结果只取决于光标前的文本与可用宽度。
// 宽度按 0.5 像素量化，避免布局浮点尾数导致永不命中。
type scrollStartKey struct {
	head  string
	avail int32
}

var (
	scrollStartMu   sync.RWMutex
	scrollStartMemo = make(map[scrollStartKey]int, 32)
)

const (
	// scrollStartMemoMax 滚动起点缓存条数上限。
	scrollStartMemoMax = 128
	// scrollStartMaxText 参与缓存的最大前缀长度（rune）：再长就现算，
	// 免得每帧为了查缓存拷一份上百 KB 的字符串。
	scrollStartMaxText = 4096
)

// ScrollStartCached 单行输入框横向滚动的起点（0 表示不滚动）：光标前文本超出 avail 时，
// 返回需要从左侧滚出可视区的 rune 数。结果按「光标前文本 + 可用宽度」缓存——
// 光标不动时每帧都会问同一个问题，而它的计算要对几十个 rune 逐个测量
//（1900 rune 文本约 20µs / 55 次分配）。
func ScrollStartCached(head []rune, avail float32) int {
	if avail <= 0 {
		return 0
	}
	if len(head) == 0 {
		return 0
	}
	if len(head) > scrollStartMaxText {
		return calcScrollStart(head, avail)
	}
	key := scrollStartKey{head: string(head), avail: int32(avail*2 + 0.5)}
	scrollStartMu.RLock()
	v, ok := scrollStartMemo[key]
	scrollStartMu.RUnlock()
	if ok {
		return v
	}
	v = calcScrollStart(head, avail)
	scrollStartMu.Lock()
	for len(scrollStartMemo) >= scrollStartMemoMax {
		for k := range scrollStartMemo {
			delete(scrollStartMemo, k)
			break
		}
	}
	scrollStartMemo[key] = v
	scrollStartMu.Unlock()
	return v
}

// calcScrollStart 从光标处向前累加宽度，找出还能塞进 avail 的起点。
func calcScrollStart(head []rune, avail float32) int {
	const caretPad = 6 // 光标贴边时留一点余量，不顶住右边框
	limit := avail - caretPad
	if limit <= 0 {
		return len(head)
	}
	if TextWidth(string(head), 13, 400) <= limit {
		return 0
	}
	acc := float32(0)
	i := len(head)
	for i > 0 && acc < limit {
		i--
		acc += TextWidth(string(head[i]), 13, 400)
	}
	if acc > limit && i < len(head) {
		i++
	}
	return i
}
