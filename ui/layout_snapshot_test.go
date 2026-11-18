// guis — 布局快照回归
//
// 目的：布局算法（flex / grid / wrap / stack / 百分比 / 绝对定位 / 约束）
// 的改动最容易「静默改坏某个角落」，而像素级对比又受字体与显卡影响。
// 这里用「几何快照」代替像素快照：把布局后的每个节点 Frame 序列化成文本，
// 与 testdata/layout_snapshot/*.txt 比对。跨平台稳定（不依赖字体、不依赖 GPU）。
//
// 更新快照：
//
//	$env:UPDATE_SNAPSHOTS=1; go test ./ui/ -run TestLayoutSnapshot
//
// 只覆盖纯盒模型（不含文本），文本排版由 layout/textwrap 的其它用例覆盖。

package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// snapshotLayout 把整棵树的几何序列化成稳定文本。
func snapshotLayout(root *Node) string {
	var sb strings.Builder
	var walk func(n *Node, path string)
	walk = func(n *Node, path string) {
		if n == nil {
			return
		}
		f := n.Frame
		fmt.Fprintf(&sb, "%s %s %s\n", path, kindName(n.Kind),
			fmt.Sprintf("%s,%s %s,%s", snapNum(f.X), snapNum(f.Y), snapNum(f.W), snapNum(f.H)))
		for i, c := range n.Children {
			walk(c, path+"/"+strconv.Itoa(i))
		}
	}
	walk(root, "0")
	return sb.String()
}

// snapNum 量化到 0.5 像素，吸收跨平台的浮点尾差。
func snapNum(v float32) string {
	q := math.Round(float64(v)*2) / 2
	if q == math.Trunc(q) {
		return strconv.Itoa(int(q))
	}
	return strconv.FormatFloat(q, 'f', 1, 64)
}

// layoutCase 一个快照场景。
type layoutCase struct {
	name  string
	w, h  float32
	build func() *Node
}

func layoutCases() []layoutCase {
	box := func(w, h float32) *Node {
		n := Box()
		if w > 0 {
			n.St(St().W(w))
		}
		if h > 0 {
			n.St(St().H(h))
		}
		return n
	}
	return []layoutCase{
		{
			name: "flex_row_gap_pad", w: 400, h: 200,
			build: func() *Node {
				return Row(box(50, 20), box(60, 30), box(70, 40)).
					St(St().GapOf(10).Padding(16))
			},
		},
		{
			name: "flex_col_grow", w: 300, h: 300,
			build: func() *Node {
				return Column(
					box(100, 20),
					Box().Grow(1),
					Box().Grow(2),
				).St(St().GapOf(12).Padding(10))
			},
		},
		{
			name: "flex_row_grow_equal", w: 320, h: 80,
			build: func() *Node {
				return Row(Box().Grow(1), Box().Grow(1), Box().Grow(1)).St(St().GapOf(8))
			},
		},
		{
			name: "grid_3cols", w: 330, h: 220,
			build: func() *Node {
				return Grid(box(0, 0), box(0, 0), box(0, 0), box(0, 0), box(0, 0)).
					St(St().ColsN(3).GapOf(10).Padding(8))
			},
		},
		{
			name: "stack_center", w: 200, h: 120,
			build: func() *Node {
				return Stack(Box().St(St().W(80).H(40))).AlignTo(AlignCenter).JustifyTo(AlignCenter)
			},
		},
		{
			name: "pct_size", w: 400, h: 200,
			build: func() *Node {
				return Column(
					Box().St(St().Wp(50).H(30)),
					Box().St(St().Wp(100).H(40)),
				)
			},
		},
		{
			name: "absolute_pos", w: 300, h: 200,
			build: func() *Node {
				return Stack(
					box(300, 200),
					Box().St(St().W(40).H(40).AbsTL(20, 30)),
				)
			},
		},
		{
			name: "min_max_constraint", w: 200, h: 100,
			build: func() *Node {
				return Row(
					Box().St(St().W(10).MinWidth(60)),
					Box().St(St().W(400).MaxWidth(80)),
				).St(St().GapOf(6))
			},
		},
		{
			name: "wrap_rows", w: 220, h: 200,
			build: func() *Node {
				return Wrap(box(80, 20), box(80, 20), box(80, 20), box(80, 20)).
					St(St().GapOf(8).Padding(6))
			},
		},
		{
			name: "nested_flex", w: 500, h: 300,
			build: func() *Node {
				return Row(
					Column(box(100, 30), Box().Grow(1)).St(St().GapOf(6)),
					Column(box(120, 30), box(120, 30)).St(St().GapOf(6)),
				).St(St().GapOf(14).Padding(12))
			},
		},
	}
}

func TestLayoutSnapshot(t *testing.T) {
	update := os.Getenv("UPDATE_SNAPSHOTS") == "1"
	dir := filepath.Join("testdata", "layout_snapshot")

	for _, c := range layoutCases() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			root := c.build()
			Layout(root, c.w, c.h, nil)
			got := snapshotLayout(root)

			path := filepath.Join(dir, c.name+".txt")
			if update {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("建目录失败：%v", err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("写快照失败：%v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("缺少快照 %s（用 UPDATE_SNAPSHOTS=1 生成）：%v", path, err)
			}
			if got != string(want) {
				t.Fatalf("布局与快照不一致：%s\n%s", path, snapshotDiff(string(want), got))
			}
		})
	}
}

// snapshotDiff 逐行对比，只列出第一处差异附近的上下文。
func snapshotDiff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			from := i - 3
			if from < 0 {
				from = 0
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "第 %d 行开始不一致：\n  期望: %s\n  实际: %s\n", i+1, w, g)
			for j := from; j < len(gl) && j < i+4; j++ {
				fmt.Fprintf(&sb, "  [%d] %s\n", j+1, gl[j])
			}
			return sb.String()
		}
	}
	return "内容一致（不可见字符差异）"
}
