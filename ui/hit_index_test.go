// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"testing"

	"guis/internal/geom"
)

// 命中测试性能基线。
//
// 结论（2026-11 实测）：给 hitTest 加「子树包围盒剪枝」没有收益 —— 自绘布局里
// 容器 Frame 普遍铺满父级，包围盒与 Frame 几乎重合，剪枝拒绝不掉任何子树，
// 反而多一次判断。实测 78ns/op（无剪枝）vs 83~87ns/op（剪枝），故未采纳；
// 这里保留无剪枝拷贝作为长期对照，将来换「可见 AABB 索引」方案时用它证明收益。

// hitTestNoPrune 与 ui/app.go 的 hitTest 同构（拷贝一份，便于独立度量）。
func hitTestNoPrune(n *Node, x, y, sy float32, a *App) *Node {
	if n == nil || n.Style.Opacity <= 0 || n.Disabled {
		return nil
	}
	r := n.Frame.Offset(0, sy)
	if !r.Contains(geom.Point{X: x, Y: y}) {
		return nil
	}
	childSy := sy
	if n.Kind == KScroll {
		childSy = sy - a.scrolls[n.Key]
	}
	for i := len(n.Children) - 1; i >= 0; i-- {
		if c := hitTestNoPrune(n.Children[i], x, y, childSy, a); c != nil {
			return c
		}
	}
	return n
}

// buildDeepTree 构造 depth 层、每层 fan 个分支的深树（容器铺满父级，模拟真实布局）。
func buildDeepTree(depth, fan int) *Node {
	if depth <= 0 {
		return Btn("leaf", BtnPrimary, func() {}).St(St().W(60).H(20))
	}
	kids := make([]*Node, 0, fan)
	for i := 0; i < fan; i++ {
		kids = append(kids, buildDeepTree(depth-1, fan))
	}
	return Column(kids...).St(St().W(400).H(300))
}

// BenchmarkHitTestDeepTree 命中测试在深树上的开销（剪枝方案评估用）。
func BenchmarkHitTestDeepTree(b *testing.B) {
	root := Stack(buildDeepTree(5, 4)).St(St().W(400).H(300))
	Layout(root, 400, 300, newTestCache())
	b.Run("Outside", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hitTest(root, 399, 299, 0, nil)
		}
	})
	b.Run("OutsideNoPrune", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hitTestNoPrune(root, 399, 299, 0, nil)
		}
	})
	b.Run("InsideTopLeft", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hitTest(root, 1, 1, 0, nil)
		}
	})
	b.Run("InsideTopLeftNoPrune", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hitTestNoPrune(root, 1, 1, 0, nil)
		}
	})
}