// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// JSON 查看器：把任意值转成可折叠的树形表格（复用 TreeTable 的展开/选中能力）。
// 适合调试面板、接口返回预览、配置查看。

// JSONOptions JSON 查看器选项。
type JSONOptions struct {
	Key       string // 状态键（展开状态按此键保存）
	RootLabel string // 根节点名，默认「root」
	MaxDepth  int    // 最大展开层级（<=0 不限制）
	MaxString int    // 字符串截断长度，默认 48
	DefaultOpen bool // 首帧展开全部
	OnPick    func(path []string, preview string)
}

// JsonViewer 渲染任意 JSON 兼容值（map / slice / 基础类型）。
func JsonViewer(v any, opts JSONOptions) *Node {
	if opts.RootLabel == "" {
		opts.RootLabel = "root"
	}
	if opts.MaxString <= 0 {
		opts.MaxString = 48
	}
	rows := jsonRows(opts.RootLabel, v, 0, opts.MaxDepth, opts.MaxString)
	return TreeTable(rows, TreeTableOptions{
		Key:         opts.Key,
		Headers:     []string{"键", "类型", "值"},
		Widths:      []float32{200, 80, 0},
		DefaultOpen: opts.DefaultOpen,
		EmptyText:   "没有数据",
		OnClick: func(path []string, row TreeTableRow) {
			if opts.OnPick == nil {
				return
			}
			preview := ""
			if len(row.Texts) > 2 {
				preview = row.Texts[2]
			}
			opts.OnPick(path, preview)
		},
	})
}

// jsonRows 递归把值转成 TreeTableRow。
func jsonRows(label string, v any, depth, maxDepth, maxStr int) []TreeTableRow {
	row := TreeTableRow{Key: jsonKey(label), Texts: []string{label, jsonTypeName(v), jsonPreview(v, maxStr)}}
	if maxDepth > 0 && depth >= maxDepth {
		return []TreeTableRow{row}
	}
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		row.Texts[2] = "{" + strconv.Itoa(len(keys)) + " 个键}"
		for _, k := range keys {
			row.Children = append(row.Children, jsonRows(k, val[k], depth+1, maxDepth, maxStr)...)
		}
	case []any:
		row.Texts[2] = "[" + strconv.Itoa(len(val)) + " 项]"
		for i, item := range val {
			row.Children = append(row.Children, jsonRows("["+strconv.Itoa(i)+"]", item, depth+1, maxDepth, maxStr)...)
		}
	}
	return []TreeTableRow{row}
}

// jsonKey 生成稳定的行键（层级路径由 TreeTable 自动拼接）。
func jsonKey(label string) string {
	if label == "" {
		return "_"
	}
	r := strings.NewReplacer("/", "_", " ", "_")
	return r.Replace(label)
}

func jsonTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case float64, float32, int, int64, int32, uint, uint64:
		return "number"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return "value"
}

func jsonPreview(v any, maxStr int) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case string:
		return jsonClip(val, maxStr)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case map[string]any:
		return "{" + strconv.Itoa(len(val)) + " 个键}"
	case []any:
		return "[" + strconv.Itoa(len(val)) + " 项]"
	}
	if b, err := json.Marshal(v); err == nil {
		return jsonClip(string(b), maxStr)
	}
	return ""
}

// jsonClip 超长字符串截断并补省略号。
func jsonClip(s string, max int) string {
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}
