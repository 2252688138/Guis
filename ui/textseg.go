// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// TextSegNodes 把 [start,end) 这段文本按当前选区切段渲染成节点。
// centerY=true 时每段按行高中线垂直居中（多行输入用）。
func TextSegNodes(t *TextSel, start, end int, size float32, weight int32, color string, centerY bool) []*Node {
	segs := t.SelLineSegments(start, end)
	out := make([]*Node, 0, len(segs))
	for _, sg := range segs {
		st := St().Text(size, weight, color)
		if sg.Sel {
			st.BgC(paintWithAlpha(ColPrimary, 0.30))
		}
		if centerY {
			st.CenterYOn()
		}
		out = append(out, Text(sg.Text).St(st))
	}
	return out
}
