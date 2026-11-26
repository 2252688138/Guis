// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"sync"
	"sync/atomic"
)

// 声明式焦点作用域：焦点陷阱（模态）与 roving tabindex。
//
// PushFocusScope/PopFocusScope 是命令式 API，调用方要自己保证成对调用；声明式
// 节点树每帧重建，配对很容易漏（弹窗被条件分支跳过时就会漏 Pop）。这里把两种
// 最常见语义做成节点标记，由框架每帧从节点树推导：
//
//	n.Trap()       子树成为焦点陷阱（模态）：Tab/Shift+Tab 只在子树内循环；
//	               子树出现的当帧若焦点不在其中，自动聚焦子树内第一个可聚焦控件。
//	n.Rove("tab")  本节点的直接子节点组成 roving tabindex 组：组内只有活动项占
//	               一个 Tab 位，方向键（←↑→↓）在组内移动（Tabs / 工具栏 / 单选组）。

// trapUsed 记录「本帧是否有节点调用了 Trap()」。
// 每帧重建节点树时在 Node.Trap 里置位、帧末 clear，用于让无模态的帧完全跳过
// 全树遍历（1 万节点的表格不该为「没有弹窗」多付一次遍历）。
var trapUsed atomic.Bool

// roveMu 保护 roveActive：方向键处理与 Tab 遍历都在主循环，但 golden 截图
// 测试会在 goroutine 里建树，加锁比假设单线程安全。
var roveMu sync.Mutex

// roveActive 记录每个 roving 组的活动项标识（组 Key → 项标识）。
// 声明式树每帧重建，活动项这类交互状态只能放包级。
var roveActive = map[string]string{}

// Trap 把本子树标记为焦点陷阱（模态弹窗 / 抽屉 / 全屏浮层）。
//
// 生效范围：
//   - Tab / Shift+Tab 只在子树内循环，背景控件拿不到焦点；
//   - 子树出现的当帧自动聚焦其内第一个可聚焦控件（等价 Web 的 autofocus）；
//   - 显式 PushFocusScope 的作用域仍然优先，两者可共存。
func (n *Node) Trap() *Node {
	n.FocusTrap = true
	trapUsed.Store(true)
	return n
}

// Rove 把本节点的直接子节点声明为 roving tabindex 组（group 为组标识，需在
// 应用内唯一）。组内只有活动项占一个 Tab 位，方向键在组内循环移动；焦点离开
// 组后活动项保持不变，下次 Tab 回到组里仍是同一项。
//
// 输入框等文本编辑控件不吃这层拦截（方向键要留给光标），所以 roving 组适合
// Tabs / 工具栏 / 单选组这类「一项一按钮」的容器。
func (n *Node) Rove(group string) *Node {
	n.RovingKey = group
	return n
}

// subtreeHasKey 判断 key 是否落在 root 子树内（含 root 自身）。
func subtreeHasKey(root *Node, key string) bool {
	if root == nil || key == "" {
		return false
	}
	if root.Key == key {
		return true
	}
	for _, c := range root.Children {
		if subtreeHasKey(c, key) {
			return true
		}
	}
	return false
}

// visibleNode 节点是否还参与交互（禁用 / 完全透明的子树整棵跳过）。
func visibleNode(n *Node) bool {
	return n != nil && !n.Disabled && n.Style.Opacity > 0
}

// deepestTrap 返回树中最后出现的焦点陷阱节点（先序遇到即替换，嵌套时取最内层）。
func deepestTrap(root *Node) *Node {
	var found *Node
	var walk func(n *Node)
	walk = func(n *Node) {
		if !visibleNode(n) {
			return
		}
		if n.FocusTrap {
			found = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstFocusableIn 返回子树内第一个可聚焦节点（先序）。
func firstFocusableIn(root *Node) *Node {
	if !visibleNode(root) {
		return nil
	}
	if root.Style.Focusable && root.Key != "" {
		return root
	}
	for _, c := range root.Children {
		if f := firstFocusableIn(c); f != nil {
			return f
		}
	}
	return nil
}

// syncFocusTrap 每帧调用：焦点陷阱出现时把焦点收进子树。
// 焦点已经在子树内则原样保留（用户点到哪个控件就停在哪个控件）。
func (a *App) syncFocusTrap() {
	if a == nil || a.root == nil || !trapUsed.Load() {
		return
	}
	trap := deepestTrap(a.root)
	if trap == nil {
		return
	}
	if a.focusKey != "" && subtreeHasKey(trap, a.focusKey) {
		return
	}
	if f := firstFocusableIn(trap); f != nil {
		a.focusKey = f.Key
		a.redrawRequest.Store(true)
	}
}

// roveItems 返回 roving 组的项（直接子节点，跳过禁用 / 透明项）。
func roveItems(group *Node) []*Node {
	if group == nil {
		return nil
	}
	out := make([]*Node, 0, len(group.Children))
	for _, c := range group.Children {
		if visibleNode(c) {
			out = append(out, c)
		}
	}
	return out
}

// roveItemID 项的稳定标识：项自身有 Key 用 Key，否则用其内第一个可聚焦控件的
// Key（「一项 = 一个带 padding 的容器包一个按钮」是很常见的写法）。
func roveItemID(item *Node) string {
	if item == nil {
		return ""
	}
	if item.Key != "" {
		return item.Key
	}
	if f := firstFocusableIn(item); f != nil {
		return f.Key
	}
	return ""
}

// roveActiveItem 返回当前活动项：记录失效（项被删 / 重建换 Key）时回落到首项，
// 并顺手把回落结果写回，避免每帧重复搜索。
func roveActiveItem(group *Node) *Node {
	items := roveItems(group)
	if len(items) == 0 {
		return nil
	}
	roveMu.Lock()
	want := roveActive[group.RovingKey]
	roveMu.Unlock()
	if want != "" {
		for _, it := range items {
			if roveItemID(it) == want && firstFocusableIn(it) != nil {
				return it
			}
		}
	}
	for _, it := range items {
		if firstFocusableIn(it) != nil {
			id := roveItemID(it)
			roveMu.Lock()
			roveActive[group.RovingKey] = id
			roveMu.Unlock()
			return it
		}
	}
	return nil
}

// RoveActive 返回 roving 组当前活动项的标识（供组件渲染选中态）。
func RoveActive(group string) string {
	roveMu.Lock()
	defer roveMu.Unlock()
	return roveActive[group]
}

// SetRoveActive 手动指定 roving 组的活动项（点击某一项后同步键盘活动项用）。
func SetRoveActive(group, item string) {
	roveMu.Lock()
	defer roveMu.Unlock()
	if group == "" {
		return
	}
	roveActive[group] = item
}

// ResetRoveState 清空全部 roving 组状态（页面整体切换 / 窗口重建时调用）。
func ResetRoveState() {
	roveMu.Lock()
	defer roveMu.Unlock()
	roveActive = map[string]string{}
}

// findRoveGroup 找到包含 key 的最内层 roving 组。
func findRoveGroup(root *Node, key string) *Node {
	var found *Node
	var walk func(n *Node)
	walk = func(n *Node) {
		if !visibleNode(n) {
			return
		}
		if n.RovingKey != "" && subtreeHasKey(n, key) {
			found = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return found
}

// rovingHandleKey 处理 roving 组内的方向键。返回 true 表示已消费。
// 文本编辑控件（输入框 / 代码编辑器 / 终端）不吃这层：方向键要留给光标。
func rovingHandleKey(key uint32) bool {
	var dir int
	switch key {
	case VK_LEFT, VK_UP:
		dir = -1
	case VK_RIGHT, VK_DOWN:
		dir = 1
	default:
		return false
	}
	a := currentApp.Load()
	if a == nil || a.root == nil || a.focusKey == "" {
		return false
	}
		if TextEditBound(a.focusKey) {
		return false
	}
	group := findRoveGroup(a.root, a.focusKey)
	if group == nil {
		return false
	}
	items := roveItems(group)
	if len(items) < 2 {
		return false
	}
	idx := -1
	for i, it := range items {
		if subtreeHasKey(it, a.focusKey) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	next := items[(idx+dir+len(items))%len(items)]
	f := firstFocusableIn(next)
	if f == nil {
		return false
	}
	roveMu.Lock()
	roveActive[group.RovingKey] = roveItemID(next)
	roveMu.Unlock()
	SetFocus(f.Key)
	SetFocusVisible(true)
	return true
}
