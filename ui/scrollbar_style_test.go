// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
package ui

import (
	"testing"

	"guis/internal/geom"
)

// TestScrollbarOnFlag ScrollbarOn 打开「常显」标记；默认不常显（滚动/悬停后淡出）。
func TestScrollbarOnFlag(t *testing.T) {
	if St().Scrollbar {
		t.Fatal("默认不应常显滚动条")
	}
	if !St().ScrollbarOn().Scrollbar {
		t.Fatal("ScrollbarOn 应把常显标记置为 true")
	}
	// 节点级传播：Scroll 节点合并样式后必须带上常显标记，否则 paintScrollbar 仍会淡出
	if !Scroll().St(St().GrowTo(1).ScrollbarOn()).Style.Scrollbar {
		t.Fatal("节点未继承 Scrollbar 常显标记")
	}
}

// TestScrollbarMetricsOverflow 内容不溢出时不算滚动条；溢出时滑块高度按可视比例。
func TestScrollbarMetricsOverflow(t *testing.T) {
	n := Scroll().K("test-scroll").St(St().H(200).ScrollbarOn())
	n.Frame = geom.Rect{X: 0, Y: 0, W: 100, H: 200}

	n.ContentH = 200
	if _, _, ok := scrollbarMetrics(n, n.Frame); ok {
		t.Fatal("内容恰好不溢出时不该显示滚动条")
	}

	n.ContentH = 800
	thumb, maxScroll, ok := scrollbarMetrics(n, n.Frame)
	if !ok {
		t.Fatal("内容溢出时应显示滚动条")
	}
	if maxScroll != 600 {
		t.Fatalf("可滚动距离错误：期望 600，实际 %.1f", maxScroll)
	}
	if thumb != 50 {
		t.Fatalf("滑块高度错误：期望 50，实际 %.1f", thumb)
	}
}
