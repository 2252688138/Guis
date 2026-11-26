package ui

import "testing"

func TestScrollAtBottomWithoutApp(t *testing.T) {
	if ScrollAtBottom("no-such-scroll", 10) {
		t.Fatal("没有应用/容器时不应判定为触底")
	}
}

func TestInfiniteScrollFooter(t *testing.T) {
	key := "t-infinite"
	InfiniteScrollReset(key)
	defer InfiniteScrollReset(key)

	items := []*Node{
		Text("第一行").St(St().PadXY(12, 8)),
		Text("第二行").St(St().PadXY(12, 8)),
	}
	n := InfiniteScroll(items, InfiniteScrollOptions{
		Key: key, Height: 200, HasMore: true, OnLoadMore: func() {}, EmptyText: "空",
	})
	if n == nil || n.Key != key || n.Kind != KScroll {
		t.Fatal("应返回 KScroll 容器并保留 Key")
	}
	if got := len(n.Children); got != 3 {
		t.Fatalf("两项数据 + 状态行应为 3 个子节点，实际 %d", got)
	}

	// 加载中：底部行存在
	n = InfiniteScroll(items, InfiniteScrollOptions{Key: key, Height: 200, Loading: true, HasMore: true})
	if got := len(n.Children); got != 3 {
		t.Fatalf("加载中底部应渲染状态行，实际子节点 %d", got)
	}

	// 无更多：底部行存在；无数据时套空状态
	n = InfiniteScroll(nil, InfiniteScrollOptions{Key: key, Height: 200, HasMore: false})
	if got := len(n.Children); got != 2 {
		t.Fatalf("空数据 + 结束状态应为 2 个子节点，实际 %d", got)
	}
}
