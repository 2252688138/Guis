// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 浮层组件：Popover（锚点气泡）与 ContextMenu（右键菜单）。
//
// 两者结构一致：整窗遮罩（最底层，点它就关）+ 绝对定位面板（叠在上面）。
// 框架只负责定位、夹取窗口边界与关闭；面板内容由调用方提供。

const (
	popoverGap     = 6
	popoverEdgePad = 8
)

// PopoverState 锚点气泡状态（跨帧，由调用方持有）。
type PopoverState struct {
	Key  string
	Open bool
	// X/Y 锚点坐标（相对窗口左上角），通常是鼠标位置或某个控件的包围盒。
	X, Y float32
	// W 面板宽度；H 面板估算高度（只管下方空间够不够，不够就翻到上方）。
	W, H    float32
	OnClose func()
}

// NewPopover 创建气泡状态（默认 220 宽、200 高）。
func NewPopover(key string) *PopoverState {
	return &PopoverState{Key: key, W: 220, H: 200}
}

// IsOpen 气泡是否展开。
func (st *PopoverState) IsOpen() bool { return st != nil && st.Open }

// ShowAt 在锚点处展开（重复调用只移动位置）。
func (st *PopoverState) ShowAt(x, y float32) {
	if st == nil {
		return
	}
	st.Open, st.X, st.Y = true, x, y
	RequestRedraw()
}

// Toggle 在锚点处切换展开状态（常用于点击按钮）。
func (st *PopoverState) Toggle(x, y float32) {
	if st == nil {
		return
	}
	if st.Open {
		st.Close()
		return
	}
	st.ShowAt(x, y)
}

// Close 收起气泡。
func (st *PopoverState) Close() {
	if st == nil || !st.Open {
		return
	}
	st.Open = false
	if st.OnClose != nil {
		st.OnClose()
	}
	RequestRedraw()
}

// popoverPos 计算面板左上角：优先在锚点下方，下方放不下就翻到上方，并夹在窗口内。
func popoverPos(st *PopoverState) (float32, float32) {
	vw, vh := ViewportW(), ViewportH()
	w, h := st.W, st.H
	if w <= 0 {
		w = 220
	}
	if h <= 0 {
		h = 200
	}
	x := st.X
	if x+w > vw-popoverEdgePad {
		x = vw - w - popoverEdgePad
	}
	if x < popoverEdgePad {
		x = popoverEdgePad
	}
	y := st.Y + popoverGap
	if y+h > vh-popoverEdgePad {
		y = st.Y - h - popoverGap
		if y < popoverEdgePad {
			y = popoverEdgePad
		}
	}
	return x, y
}

// PopoverLayer 绘制气泡浮层（放在页面根 Stack 里）；未展开时返回 nil。
// content 为 nil 时只画遮罩（布尔用：等异步内容就绪后再填）。
func PopoverLayer(st *PopoverState, content *Node) *Node {
	if st == nil || !st.Open {
		return nil
	}
	x, y := popoverPos(st)
	// 遮罩只负责关掉浮层：用极低不透明度占位，视觉上看不出来。
	scrim := Stack().St(St().AbsFill().BgC("rgba(0,0,0,0.001)"))
	scrim.OnClick = func() { st.Close() }

	w := st.W
	if w <= 0 {
		w = 220
	}
	panel := Stack(content).St(St().AbsTL(x, y).W(w).R(12).BgC(ColBgDeep).
		Stroke(1, ColStroke).PadXY(12, 12).Clipped())
	return Stack(scrim, panel).St(St().AbsFill())
}

// ContextMenuState 右键菜单状态（跨帧，由调用方持有）。
type ContextMenuState struct {
	Key   string
	Open  bool
	X, Y  float32
	W     float32
	Items []MenuItem
}

// NewContextMenu 创建右键菜单状态（默认 180 宽）。
func NewContextMenu(key string) *ContextMenuState {
	return &ContextMenuState{Key: key, W: 180}
}

// IsOpen 菜单是否展开。
func (st *ContextMenuState) IsOpen() bool { return st != nil && st.Open }

// At 在指定坐标（相对窗口）弹出菜单。
func (st *ContextMenuState) At(x, y float32, items []MenuItem) {
	if st == nil {
		return
	}
	st.Open, st.X, st.Y, st.Items = true, x, y, items
	if st.Key != "" {
		SetFocus(st.Key + "-menu")
	}
	RequestRedraw()
}

// Close 关闭菜单。
func (st *ContextMenuState) Close() {
	if st == nil || !st.Open {
		return
	}
	st.Open, st.Items = false, nil
	RequestRedraw()
}

// wrapMenuItems 给每一项包一层「先关闭菜单再执行原回调」，子菜单递归处理。
func wrapMenuItems(items []MenuItem, close func()) []MenuItem {
	out := make([]MenuItem, len(items))
	for i, it := range items {
		cp := it
		if it.OnPick != nil {
			fn := it.OnPick
			cp.OnPick = func() { close(); fn() }
		}
		if len(it.Children) > 0 {
			cp.Children = wrapMenuItems(it.Children, close)
		}
		out[i] = cp
	}
	return out
}

// ContextMenuLayer 绘制右键菜单浮层（放在页面根 Stack 里）；未展开时返回 nil。
// 点遮罩或右键别处都会关闭；选中菜单项时先关闭再执行回调。
func ContextMenuLayer(st *ContextMenuState) *Node {
	if st == nil || !st.Open || len(st.Items) == 0 {
		return nil
	}
	scrim := Stack().St(St().AbsFill().BgC("rgba(0,0,0,0.001)"))
	scrim.OnClick = func() { st.Close() }
	scrim.OnRightClickAt = func(x, y float32) { st.Close() }

	w := st.W
	if w <= 0 {
		w = 180
	}
	panel := MenuPanel(st.X, st.Y, wrapMenuItems(st.Items, st.Close), w)
	return Stack(scrim, panel).St(St().AbsFill())
}

// WithContextMenu 给节点挂右键菜单：右键位置为相对该节点的点击点加上节点坐标。
// 注意：菜单坐标是布局坐标（不含祖先滚动偏移），与 SetTooltip 的约定一致。
func (n *Node) WithContextMenu(st *ContextMenuState, items []MenuItem) *Node {
	if n == nil || st == nil || len(items) == 0 {
		return n
	}
	n.OnRightClickAt = func(x, y float32) {
		st.At(n.Frame.X+x, n.Frame.Y+y, items)
	}
	return n
}
