package ui

import (
	"math"
	"testing"
)

func TestSparklineBarsAndResample(t *testing.T) {
	n := Sparkline([]float64{1, 5, 3, 9, 2}, SparkOptions{Bars: 5, Width: 60, Height: 20})
	if n == nil || len(n.Children) != 9 {
		t.Fatalf("5 根柱加 4 个间隙应为 9 个子节点，实际 %d", len(n.Children))
	}
	// 空数据也要出占位柱
	if n := Sparkline(nil, SparkOptions{Bars: 4}); n == nil || len(n.Children) != 7 {
		t.Fatal("空数据应返回占位柱")
	}
	// 多则分桶平均
	in := []float64{2, 2, 4, 4}
	got := sparkResample(in, 2)
	if len(got) != 2 || math.Abs(got[0]-2) > 1e-9 || math.Abs(got[1]-4) > 1e-9 {
		t.Fatalf("分桶平均应为 [2 4]，实际 %v", got)
	}
	// 少则原样
	if got := sparkResample([]float64{1, 2}, 5); len(got) != 2 {
		t.Fatalf("少于柱数时不应补点，实际 %v", got)
	}
	lo, hi := sparkRange([]float64{3, -1, 8})
	if lo != -1 || hi != 8 {
		t.Fatalf("范围应为 [-1 8]，实际 [%v %v]", lo, hi)
	}
}

func TestSparklineBarsClamped(t *testing.T) {
	// 柱数被夹到 3~60：极小宽度也至少 3 柱
	n := Sparkline([]float64{1, 2, 3}, SparkOptions{Width: 4, Bars: 1})
	if got := len(n.Children); got != 5 {
		t.Fatalf("柱数下限 3 → 5 个子节点，实际 %d", got)
	}
}
