// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"strconv"
	"strings"
)

// 树形表格：Table 的层级版。节点树每帧重建，展开/选中状态按 Key 存在包级 map。
// 与 Table 的差异：首列自动加缩进与展开箭头，父行可折叠整棵子树。

// TreeTableRow 树形表格的一行（Children 非空即可展开）。
type TreeTableRow struct {
	Key      string // 行键（同一表格内唯一；留空按「层级路径 + 序号」自动生成）
	Texts    []string
	Cells    []*Node // 自定义单元格，非空时优先于 Texts
	Children []TreeTableRow
	Disabled bool
}

// TreeTableOptions 树形表格选项。
type TreeTableOptions struct {
	Key         string // 状态键：展开/选中按此键存在包级 map
	Headers     []string
	Widths      []float32 // 与 Headers 对齐，<=0 为弹性列
	Indent      float32   // 每级缩进，默认 18
	RowPadY     float32   // 行上下内边距，默认 10
	Zebra       bool      // 斑马纹，默认开
	DefaultOpen bool      // 首帧默认展开全部层级
	EmptyText   string    // 无数据文案，默认「暂无数据」
	OnClick     func(path []string, row TreeTableRow)
}

type treeTableState struct {
	open   map[string]bool
	sel    string
	inited bool
}

// treeTableStates 已创建的树形表格状态（按 Key 隔离）。
var treeTableStates = map[string]*treeTableState{}

func treeTableStateOf(key string) *treeTableState {
	st := treeTableStates[key]
	if st == nil {
		st = &treeTableState{open: map[string]bool{}}
		treeTableStates[key] = st
	}
	return st
}

// TreeTable 树形表格。
func TreeTable(rows []TreeTableRow, opts TreeTableOptions) *Node {
	if opts.Indent <= 0 {
		opts.Indent = 18
	}
	if opts.RowPadY <= 0 {
		opts.RowPadY = 10
	}
	if opts.EmptyText == "" {
		opts.EmptyText = "暂无数据"
	}
	st := treeTableStateOf(opts.Key)
	if !st.inited {
		st.inited = true
		if opts.DefaultOpen {
			treeTableSetAll(rows, "", true, st)
		}
	}

	head := Row().St(St().GapOf(12).PadXY(16, 10).BgC(ColHeader))
	for i, h := range opts.Headers {
		cell := Text(h).St(St().Text(11.5, 600, ColTextMuted))
		if i < len(opts.Widths) && opts.Widths[i] > 0 {
			cell.St(St().W(opts.Widths[i]))
		} else {
			cell.Grow(1)
		}
		head.Add(cell)
	}

	body := make([]*Node, 0, len(rows))
	ri := 0
	treeTableWalk(rows, "", 0, nil, &opts, st, &body, &ri)
	if len(body) == 0 {
		body = append(body, Row(
			Text(opts.EmptyText).St(St().Text(12.5, 400, ColTextMuted)),
		).St(St().PadXY(16, 18)))
	}
	return Column(append([]*Node{Divider(), head, Divider()}, body...)...).
		K(opts.Key).St(St().R(12).Stroke(1, ColStroke).Clipped())
}

// treeTableWalk 深度优先展开可见行。
func treeTableWalk(rows []TreeTableRow, parent string, depth int, path []string,
	opts *TreeTableOptions, st *treeTableState, out *[]*Node, ri *int) {
	for i, row := range rows {
		rk := treeTableRowKey(row, i)
		cur := parent + "/" + rk
		curPath := append(append([]string{}, path...), rk)
		*out = append(*out, treeTableRowNode(row, cur, curPath, depth, opts, st, *ri))
		*ri++
		if len(row.Children) > 0 && st.open[cur] {
			treeTableWalk(row.Children, cur, depth+1, curPath, opts, st, out, ri)
		}
	}
}

func treeTableRowKey(row TreeTableRow, i int) string {
	if row.Key != "" {
		return row.Key
	}
	if len(row.Texts) > 0 && row.Texts[0] != "" {
		return row.Texts[0] + "#" + strconv.Itoa(i)
	}
	return "r" + strconv.Itoa(i)
}

func treeTableRowNode(row TreeTableRow, key string, path []string, depth int,
	opts *TreeTableOptions, st *treeTableState, ri int) *Node {
	bg := ColRow
	if opts.Zebra && ri%2 == 1 {
		bg = ColRowAlt
	}
	if st.sel == key {
		bg = ColHover
	}
	r := Row().St(St().GapOf(12).PadOnly(opts.RowPadY, 16, opts.RowPadY, 14).BgC(bg)).
		AlignTo(AlignCenter).K(key)

	// 首列：缩进 + 展开箭头 + 文本
	first := Row().AlignTo(AlignCenter)
	if depth > 0 {
		first.Add(Box().St(St().W(opts.Indent * float32(depth))))
	}
	if len(row.Children) > 0 {
		icon := "chevron-right"
		if st.open[key] {
			icon = "chevron-down"
		}
		caret := IconColored(icon, 14, ColTextMuted).K(key + ":caret").
			Tap(func() {
				st.open[key] = !st.open[key]
				RequestRedraw()
			})
		first.Add(caret)
	} else {
		first.Add(Box().St(St().W(14)))
	}
	label := ""
	if len(row.Texts) > 0 {
		label = row.Texts[0]
	}
	color := any(ColText)
	if row.Disabled {
		color = ColTextMuted
	}
	first.Add(Text(label).St(St().Text(12.5, 500, color)))
	if i := 0; i < len(opts.Widths) && opts.Widths[0] > 0 {
		first.St(St().W(opts.Widths[0]))
	} else {
		first.Grow(1)
	}
	r.Add(first)

	// 其余列
	cols := len(opts.Headers)
	if len(opts.Widths) > cols {
		cols = len(opts.Widths)
	}
	for ci := 1; ci < cols; ci++ {
		var c *Node
		switch {
		case ci < len(row.Cells) && row.Cells[ci] != nil:
			c = row.Cells[ci]
		case ci < len(row.Texts):
			c = Text(row.Texts[ci]).St(St().Text(12, 400, ColTextDim))
		default:
			c = Text("—").St(St().Text(12, 400, ColTextMuted))
		}
		if ci < len(opts.Widths) && opts.Widths[ci] > 0 {
			c.St(St().W(opts.Widths[ci]))
		} else {
			c.Grow(1)
		}
		r.Add(c)
	}

	if !row.Disabled {
		cp := append([]string{}, path...)
		snapshot := row
		r.Tap(func() {
			st.sel = key
			if opts.OnClick != nil {
				opts.OnClick(cp, snapshot)
			}
			RequestRedraw()
		})
	}
	return r
}

func treeTableSetAll(rows []TreeTableRow, parent string, open bool, st *treeTableState) {
	for i, row := range rows {
		key := parent + "/" + treeTableRowKey(row, i)
		if len(row.Children) > 0 {
			st.open[key] = open
			treeTableSetAll(row.Children, key, open, st)
		}
	}
}

// TreeTableSetOpen 设置某一行展开/收起（rowKey 为层级路径，如 "a/b"）。
func TreeTableSetOpen(key, rowKey string, open bool) {
	st := treeTableStateOf(key)
	st.open["/"+strings.TrimPrefix(rowKey, "/")] = open
	RequestRedraw()
}

// TreeTableToggle 切换某一行展开/收起。
func TreeTableToggle(key, rowKey string) {
	st := treeTableStateOf(key)
	p := "/" + strings.TrimPrefix(rowKey, "/")
	st.open[p] = !st.open[p]
	RequestRedraw()
}

// TreeTableIsOpen 查询某一行是否展开。
func TreeTableIsOpen(key, rowKey string) bool {
	return treeTableStateOf(key).open["/"+strings.TrimPrefix(rowKey, "/")]
}

// TreeTableSetAllOpen 展开/收起全部层级。
func TreeTableSetAllOpen(key string, rows []TreeTableRow, open bool) {
	st := treeTableStateOf(key)
	treeTableSetAll(rows, "", open, st)
	RequestRedraw()
}

// TreeTableSelected 当前选中行的层级路径（未选中为 ""）。
func TreeTableSelected(key string) string {
	return strings.TrimPrefix(treeTableStateOf(key).sel, "/")
}

// TreeTableSetSelected 设置选中行（rowKey 为层级路径，"" 表示取消选中）。
func TreeTableSetSelected(key, rowKey string) {
	st := treeTableStateOf(key)
	if rowKey == "" {
		st.sel = ""
	} else {
		st.sel = "/" + strings.TrimPrefix(rowKey, "/")
	}
	RequestRedraw()
}

// TreeTableReset 清空某张树形表格的状态（展开/选中）。
func TreeTableReset(key string) {
	delete(treeTableStates, key)
}
