// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// RTL 布局镜像。
//
// 思路：布局引擎本身只按 LTR 摆放（列 / 行 / 网格 / 流式 / 绝对定位全部保持原逻辑），
// 布局完成后再做一次「整树水平翻转」——把每个子节点在父容器内边距区里镜像到右侧。
// 这样做的好处：
//   - 只改一个函数，行 / 列 / 换行 / 网格 / 瀑布流 / 叠层 / 绝对定位一次性全部镜像；
//   - 间距、对齐、Grow 分配结果都自动跟着镜像，不需要每个组件单独适配；
//   - 子树递归翻转，嵌套容器天然正确。
//
// 文本对齐在绘制期单独处理（见 app.go 的 drawTextLine）：RTL 下 AlignStart 画到右边。
// 需要保持方向不变的子树（时间轴、代码编辑器内部、图表坐标系等）把 Node.NoMirror
// 置为 true 即可跳过（含整棵子树）。

// mirrorTree 把整棵树的子节点在各自父容器内做水平镜像。
func mirrorTree(n *Node) {
	if n == nil || n.NoMirror || n.rtlDone {
		return
	}
	n.rtlDone = true
	st := &n.Style
	innerW := n.Frame.W - st.Pad.Horizontal()
	if innerW > 0 && len(n.Children) > 0 {
		ox := n.Frame.X + st.Pad.Left
		for _, c := range n.Children {
			if c == nil {
				continue
			}
			// 左右外边距互换：镜像后左侧间距仍等于原来的左侧间距。
			c.Style.Margin.Left, c.Style.Margin.Right = c.Style.Margin.Right, c.Style.Margin.Left
			c.Frame.X = ox + innerW - (c.Frame.X - ox) - c.Frame.W
			mirrorTree(c)
		}
		return
	}
	for _, c := range n.Children {
		mirrorTree(c)
	}
}

// mirrorX 单点/矩形水平镜像（绘制期给自绘控件用）。
//
//	// 在 RTL 下把图标画到右边
//	x = ui.MirrorX(parentX, parentW, x, w)
func MirrorX(originX, boxW, x, w float32) float32 {
	if !IsRTL() {
		return x
	}
	return originX + boxW - (x - originX) - w
}
