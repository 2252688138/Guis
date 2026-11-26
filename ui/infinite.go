// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 无限滚动：滚动到接近底部时回调 OnLoadMore，底部自带「加载中 / 没有更多」状态行。
// 约定与 vlist 一致——内容由调用方按需追加，组件只负责判定触底与去重。

// InfiniteScrollOptions 无限滚动列表选项。
type InfiniteScrollOptions struct {
	Key          string
	Height       float32 // 视口高度，0 = 撑满父容器
	Threshold    float32 // 距底部多少像素触发加载，默认 120
	Loading      bool
	HasMore      bool
	LoadingLabel string // 默认「加载中…」
	EndLabel     string // 默认「没有更多了」
	EmptyText    string // 默认「暂无数据」
	OnLoadMore   func()
}

// infiniteLast 每个列表上一次触发加载时的滚动偏移（同一偏移只触发一次）。
var infiniteLast = map[string]float32{}

// ScrollAtBottom 判断滚动容器是否已接近底部（容器不存在或无需滚动都算到底）。
func ScrollAtBottom(key string, threshold float32) bool {
	if threshold <= 0 {
		threshold = 1
	}
	root := currentRoot()
	if root == nil {
		return false
	}
	n := findKey(root, key)
	if n == nil || n.Kind != KScroll {
		return false
	}
	maxScroll := n.ContentH - n.Frame.H
	if maxScroll <= 0 {
		return true
	}
	return maxScroll-ScrollOffset(key) <= threshold
}

// InfiniteScroll 无限滚动列表。
func InfiniteScroll(items []*Node, opts InfiniteScrollOptions) *Node {
	if opts.Threshold <= 0 {
		opts.Threshold = 120
	}
	if opts.LoadingLabel == "" {
		opts.LoadingLabel = "加载中…"
	}
	if opts.EndLabel == "" {
		opts.EndLabel = "没有更多了"
	}
	if opts.EmptyText == "" {
		opts.EmptyText = "暂无数据"
	}
	key := opts.Key

	children := make([]*Node, 0, len(items)+1)
	if len(items) == 0 && !opts.Loading {
		children = append(children, EmptyState("", opts.EmptyText, "", "", nil))
	} else {
		children = append(children, items...)
	}
	children = append(children, infiniteFooter(opts))

	n := Scroll(children...).K(key)
	if opts.Height > 0 {
		n.St(St().H(opts.Height))
	}

	loading, hasMore, fire := opts.Loading, opts.HasMore, opts.OnLoadMore
	return n.ScrollEvt(func(off float32) {
		if loading || !hasMore || fire == nil {
			return
		}
		if !ScrollAtBottom(key, opts.Threshold) {
			return
		}
		if last, ok := infiniteLast[key]; ok && last == off {
			return
		}
		infiniteLast[key] = off
		fire()
	})
}

// infiniteFooter 列表底部状态行（加载中 / 没有更多；还有下一页时不渲染）。
func infiniteFooter(opts InfiniteScrollOptions) *Node {
	var n *Node
	switch {
	case opts.Loading:
		n = Row(
			Spinner(14, ColTextMuted),
			Text(opts.LoadingLabel).St(St().Text(12, 400, ColTextMuted)),
		).St(St().GapOf(8)).AlignTo(AlignCenter)
	case !opts.HasMore:
		n = Text(opts.EndLabel).St(St().Text(12, 400, ColTextMuted)).St(St().TAlign(AlignCenter))
	default:
		return nil
	}
	return Row(Center(n)).St(St().PadXY(0, 14))
}

// InfiniteScrollReset 清空某个列表的触发记录（重新加载数据后调用，允许再次触底加载）。
func InfiniteScrollReset(key string) {
	delete(infiniteLast, key)
}
