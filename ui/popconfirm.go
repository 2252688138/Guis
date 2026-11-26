// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 就地确认：点击后在原位置展开「确认文案 + 确定 / 取消」，不依赖浮层与定位计算。
// 适用于删除、撤销、重置等一次性危险操作；状态按 Key 存包级 map。

// popconfirmOpen 「就地确认」展开状态。
var popconfirmOpen = map[string]bool{}

// Popconfirm 就地确认按钮。
//
// label 为收起时的按钮文案，question 为展开后的提示语，
// kind 为确定按钮的样式（危险操作传 BtnDanger）。
func Popconfirm(key, label, question string, kind ButtonKind, onConfirm func()) *Node {
	if popconfirmOpen[key] {
		actions := Row(
			Btn("取消", BtnGhost, func() {
				popconfirmOpen[key] = false
				RequestRedraw()
			}),
			Btn("确定", kind, func() {
				popconfirmOpen[key] = false
				if onConfirm != nil {
					onConfirm()
				}
				RequestRedraw()
			}),
		).St(St().GapOf(8))
		return Row(
			Text(question).St(St().Text(12, 400, ColTextDim).Wp(60).WrapOn()),
			actions,
		).St(St().GapOf(10).PadXY(10, 6).R(8).BgC(ColBgSoft).Stroke(1, ColStroke)).
			AlignTo(AlignCenter).K(key)
	}
	return Btn(label, BtnGhost, func() {
		popconfirmOpen[key] = true
		RequestRedraw()
	}).K(key)
}

// PopconfirmIsOpen 查询某个就地确认是否展开。
func PopconfirmIsOpen(key string) bool { return popconfirmOpen[key] }

// PopconfirmClose 收起所有就地确认（切换页面 / 提交前调用）。
func PopconfirmClose() { popconfirmOpen = map[string]bool{} }
