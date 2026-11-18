// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "time"

// 文件树 / 目录树组件（可展开、可选中、按 Key 记忆展开状态）。
//
//	ui.FileTree(nodes, ui.TreeOptions{
//	    Key:      "file-tree",
//	    Expanded: []string{"cmd", "cmd/pages"}, // 仅首次构建生效
//	    Selected: "ui/layout.go",
//	    OnPick:   func(n ui.TreeNode) { sel = n.Key },
//	})

// TreeKind 树节点类型（决定图标）。
type TreeKind uint8

const (
	TreeFolder TreeKind = iota // 目录：带展开箭头 + 文件夹图标
	TreeFile                   // 文件：叶子节点
	TreeLink                   // 链接 / 远程节点：圆点图标
)

// TreeNode 文件树节点。Children 非空即为可展开分支。
type TreeNode struct {
	Key      string // 唯一键（展开 / 选中按它保存）；为空时按父路径自动生成
	Label    string
	Kind     TreeKind
	Tag      string // 右侧标签（如「12」「已改」）
	TagColor string
	Disabled bool
	Children []TreeNode
}

// TreeOptions 文件树参数。
type TreeOptions struct {
	Key      string
	Expanded []string // 默认展开的节点 Key（仅首次构建时生效，之后由点击记忆）
	Selected string
	Indent   float32 // 每级缩进，默认 16
	RowH     float32 // 行高，默认 30
	OnPick   func(n TreeNode)
	OnToggle func(n TreeNode, open bool)
	// OnOpen 双击节点（同一节点 600ms 内二次点击）。配置后双击那一下只触发
	// OnOpen，不再触发 OnPick / 展开切换，避免「打开」时又插入一次引用。
	OnOpen func(n TreeNode)
	// OnContextMenu 鼠标右键点击节点（坐标相对行左上角）：右键菜单用。
	// 回调触发前会先把该节点设为选中，便于菜单操作定位目标。
	OnContextMenu func(n TreeNode, x, y float32)
}

var (
	// treeOpen 每棵树（按 Key）的展开集合。
	treeOpen = map[string]map[string]bool{}
	// treeSel 每棵树（按 Key）当前选中的节点。
	treeSel = map[string]string{}
)

// FileTree 文件树：行内「箭头 + 图标 + 名称 + 右侧标签」，选中行高亮。
func FileTree(items []TreeNode, opts TreeOptions) *Node {
	if opts.Key == "" {
		opts.Key = "tree"
	}
	if opts.Indent <= 0 {
		opts.Indent = 16
	}
	if opts.RowH <= 0 {
		opts.RowH = 30
	}
	open, ok := treeOpen[opts.Key]
	if !ok {
		open = make(map[string]bool, len(opts.Expanded))
		for _, k := range opts.Expanded {
			open[k] = true
		}
		treeOpen[opts.Key] = open
	}
	sel := opts.Selected
	if v, ok := treeSel[opts.Key]; ok {
		sel = v
	}

	rows := make([]*Node, 0, 32)
	var walk func(list []TreeNode, depth int, prefix string)
	walk = func(list []TreeNode, depth int, prefix string) {
		for _, it := range list {
			n := it
			if n.Key == "" {
				n.Key = prefix + "/" + n.Label
			}
			isOpen := open[n.Key]
			rows = append(rows, buildTreeRow(opts, n, depth, isOpen, sel))
			if len(n.Children) > 0 && isOpen {
				walk(n.Children, depth+1, n.Key)
			}
		}
	}
	walk(items, 0, opts.Key)
	return Column(rows...).St(St().GapOf(1)).K(opts.Key + "-root")
}

// TreeSetOpen 展开 / 收起某个节点（外部按钮「全部展开 / 折叠」用）。
func TreeSetOpen(key, nodeKey string, open bool) {
	m, ok := treeOpen[key]
	if !ok {
		m = map[string]bool{}
		treeOpen[key] = m
	}
	m[nodeKey] = open
	RequestRedraw()
}

// TreeSetAllOpen 展开 / 收起整棵树。
func TreeSetAllOpen(key string, items []TreeNode, open bool) {
	m, ok := treeOpen[key]
	if !ok {
		m = map[string]bool{}
		treeOpen[key] = m
	}
	var walk func(list []TreeNode, prefix string)
	walk = func(list []TreeNode, prefix string) {
		for _, it := range list {
			k := it.Key
			if k == "" {
				k = prefix + "/" + it.Label
			}
			if len(it.Children) > 0 {
				m[k] = open
				walk(it.Children, k)
			}
		}
	}
	walk(items, key)
	RequestRedraw()
}

// TreeSelected 当前选中节点的 Key（未选中返回空串）。
func TreeSelected(key string) string { return treeSel[key] }

// TreeSetSelected 设置选中节点。
func TreeSetSelected(key, nodeKey string) {
	treeSel[key] = nodeKey
	RequestRedraw()
}


// ---- 双击判定 ----

// treeDoubleClickMs 双击判定窗口（毫秒）。取 600 略宽于系统默认 500ms 双击
// 速度，避免用户手慢一点就判不成双击。
const treeDoubleClickMs = 600

// treeLastClick 上一次点击的树 / 节点 / 时间，仅用于判定双击，不参与布局。
var treeLastClick struct {
	tree, node string
	at         time.Time
}

// treeIsDoubleClick 本次点击是否构成双击（同一棵树、同一节点、间隔在窗口内）。
// 命中后清空记账，避免三连击被当成第二次双击。
func treeIsDoubleClick(tree, node string) bool {
	now := time.Now()
	hit := treeLastClick.tree == tree && treeLastClick.node == node &&
		now.Sub(treeLastClick.at) <= treeDoubleClickMs*time.Millisecond
	if hit {
		treeLastClick.tree, treeLastClick.node, treeLastClick.at = "", "", time.Time{}
		return true
	}
	treeLastClick.tree, treeLastClick.node, treeLastClick.at = tree, node, now
	return false
}

// treeResetClickState 清空双击记账（测试用）。
func treeResetClickState() {
	treeLastClick.tree, treeLastClick.node, treeLastClick.at = "", "", time.Time{}
}