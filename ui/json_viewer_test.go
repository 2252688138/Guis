package ui

import "testing"

func TestJsonRowsNested(t *testing.T) {
	v := map[string]any{
		"name": "guis",
		"ver":  float64(3),
		"tags": []any{"ui", "go"},
		"meta": map[string]any{"ok": true},
		"none": nil,
	}
	rows := jsonRows("root", v, 0, 0, 16)
	if len(rows) != 1 || len(rows[0].Children) != 5 {
		t.Fatalf("根节点应含 5 个子行，实际 %d", len(rows[0].Children))
	}
	// 键名排序稳定：meta / name / none / tags / ver
	if got := rows[0].Children[0].Texts[0]; got != "meta" {
		t.Fatalf("子行应按键名排序，首行应为 meta，实际 %q", got)
	}
	// 数组子节点标号
	for _, r := range rows[0].Children {
		if r.Texts[0] == "tags" {
			if len(r.Children) != 2 || r.Children[0].Texts[0] != "[0]" {
				t.Fatalf("tags 应有 [0]/[1] 两行，实际 %+v", r.Children)
			}
		}
	}
}

func TestJsonRowsDepthLimit(t *testing.T) {
	v := map[string]any{"a": map[string]any{"b": map[string]any{"c": float64(1)}}}
	rows := jsonRows("root", v, 0, 1, 16)
	if len(rows[0].Children) != 1 {
		t.Fatal("第一层应展开一个子行")
	}
	if got := len(rows[0].Children[0].Children); got != 0 {
		t.Fatalf("MaxDepth=1 时子行不应继续展开，实际 %d", got)
	}
}

func TestJsonTypeNameAndPreview(t *testing.T) {
	cases := []struct {
		v    any
		typ  string
		prev string
	}{
		{nil, "null", "null"},
		{true, "bool", "true"},
		{"hello", "string", "hello"},
		{float64(2.5), "number", "2.5"},
		{map[string]any{"a": 1}, "object", "{1 个键}"},
		{[]any{1, 2}, "array", "[2 项]"},
	}
	for _, c := range cases {
		if got := jsonTypeName(c.v); got != c.typ {
			t.Fatalf("%v 类型应为 %s，实际 %s", c.v, c.typ, got)
		}
		if got := jsonPreview(c.v, 16); got != c.prev {
			t.Fatalf("%v 预览应为 %q，实际 %q", c.v, c.prev, got)
		}
	}
	if got := jsonClip("abcdefgh", 4); got != "abcd…" {
		t.Fatalf("超长字符串应截断补省略号，实际 %q", got)
	}
}

func TestJsonViewerRenders(t *testing.T) {
	key := "t-json-viewer"
	TreeTableReset(key)
	defer TreeTableReset(key)
	n := JsonViewer(map[string]any{"a": float64(1)}, JSONOptions{Key: key, DefaultOpen: true, MaxDepth: 2})
	if n == nil || n.Key != key {
		t.Fatal("JsonViewer 应返回带 Key 的树形表格")
	}
}
