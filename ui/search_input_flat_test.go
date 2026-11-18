// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// findNodeByKey 在子树里按 Key 找节点（输入框的容器带 .K(key)）。
func findNodeByKey(n *Node, key string) *Node {
	if n == nil {
		return nil
	}
	if n.Key == key {
		return n
	}
	for _, c := range n.Children {
		if got := findNodeByKey(c, key); got != nil {
			return got
		}
	}
	return nil
}

// 搜索框默认无边框（扁平）：容器不画描边、底色透明，聚焦时才给淡填充。
func TestSearchInputBorderlessByDefault(t *testing.T) {
	flat := findNodeByKey(SearchInput("t-flat", "", "搜索…", nil), "t-flat")
	if flat == nil {
		t.Fatal("没找到搜索框容器节点")
	}
	if flat.Style.Border != 0 {
		t.Fatalf("默认搜索框不应画描边，得到 %v", flat.Style.Border)
	}
	if flat.Style.Bg.A > 0.001 {
		t.Fatalf("默认搜索框底色应透明，得到 alpha=%v", flat.Style.Bg.A)
	}

	h := findNodeByKey(SearchInputH("t-flat-h", "", "搜索…", 32, nil), "t-flat-h")
	if h == nil || h.Style.Border != 0 {
		t.Fatal("SearchInputH 也应无边框")
	}
}

// 需要明确输入边界时用 SearchInputBordered / SearchInputBorderedH。
func TestSearchInputBorderedVariant(t *testing.T) {
	boxed := findNodeByKey(SearchInputBordered("t-bordered", "", "搜索…", nil), "t-bordered")
	if boxed == nil {
		t.Fatal("没找到带边框搜索框的容器节点")
	}
	if boxed.Style.Border <= 0 {
		t.Fatal("SearchInputBordered 应保留描边")
	}
	if boxed.Style.Bg.A <= 0.001 {
		t.Fatal("SearchInputBordered 应有底色")
	}
	if h := findNodeByKey(SearchInputBorderedH("t-bordered-h", "", "搜索…", 32, nil), "t-bordered-h"); h == nil || h.Style.Border <= 0 {
		t.Fatal("SearchInputBorderedH 应保留描边")
	}
}

// hasIconName 判断子树里是否包含指定图标。
func hasIconName(n *Node, name string) bool {
	if n == nil {
		return false
	}
	if n.IconName == name {
		return true
	}
	for _, c := range n.Children {
		if hasIconName(c, name) {
			return true
		}
	}
	return false
}

// 放大镜必须排在输入区之后绘制，否则会被输入区的不透明底色整块盖住
// （历史 bug：搜索框看着「没有图标」）。
func TestSearchInputGlassAboveField(t *testing.T) {
	for _, c := range []struct {
		root *Node
		key  string
	}{
		{SearchInput("t-glass-flat", "", "搜索…", nil), "t-glass-flat"},
		{SearchInputBordered("t-glass-bordered", "", "搜索…", nil), "t-glass-bordered"},
	} {
		fieldIdx, glassIdx := -1, -1
		for i, kid := range c.root.Children {
			if kid.Key == c.key {
				fieldIdx = i
			}
			if hasIconName(kid, "search") {
				glassIdx = i
			}
		}
		if fieldIdx < 0 || glassIdx < 0 {
			t.Fatalf("%s 没找到输入区或放大镜：field=%d glass=%d", c.key, fieldIdx, glassIdx)
		}
		if glassIdx < fieldIdx {
			t.Fatalf("%s 放大镜应在输入区之后绘制（field=%d glass=%d）", c.key, fieldIdx, glassIdx)
		}
	}
}

// 无边框输入框：容器透明，带边框版本有描边与底色。
func TestInputFlatNoBorder(t *testing.T) {
	flat := findNodeByKey(InputFlat("t-iflat", "", "占位", nil), "t-iflat")
	if flat == nil || flat.Style.Border != 0 {
		t.Fatal("InputFlat 不应画描边")
	}
	plain := findNodeByKey(Input("t-iplain", "", "占位", nil), "t-iplain")
	if plain == nil || plain.Style.Border <= 0 {
		t.Fatal("普通 Input 应保留描边")
	}
}
