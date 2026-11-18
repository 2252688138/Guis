// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 日期区间选择：两个日期框（各带一个日历浮层）+ 区间合法性提示。
// 日历浮层由 DateLayers() 统一渲染，所以页面里记得带上它（与 DateBox 相同要求）。

// DateRangeState 日期区间状态（跨帧，由调用方持有）。
type DateRangeState struct {
	From *DateState
	To   *DateState
}

// NewDateRange 创建日期区间状态（两个日历浮层各自独立，key 不能重复）。
func NewDateRange(key string) *DateRangeState {
	return &DateRangeState{From: NewDate(key + "-from"), To: NewDate(key + "-to")}
}

// DateRangeBox 日期区间选择（开始 → 结束），值格式 YYYY-MM-DD。
//
// 开始日期晚于结束日期时给出红色提示，但不自动交换：避免把用户正在输入的
// 半成品值改掉（先改成合法值再提示消失）。
func DateRangeBox(st *DateRangeState, from, to string, onChange func(from, to string)) *Node {
	if st == nil {
		return nil
	}
	invalid := from != "" && to != "" && from > to

	emit := func(f, t string) {
		if onChange != nil {
			onChange(f, t)
		}
	}
	// 每个日期框外层给固定宽度：DateBox 自身是 100% 宽，直接并排会互抢空间。
	wrap := func(n *Node) *Node { return Stack(n).St(St().W(150)) }

	fromBox := wrap(DateBox(st.From, from, func(v string) { emit(v, st.To.Value) }))
	toBox := wrap(DateBox(st.To, to, func(v string) { emit(st.From.Value, v) }))
	sep := Text("→").St(St().Text(13, 400, ColTextMuted))
	row := Row(fromBox, sep, toBox).St(St().Wp(100).GapOf(8)).AlignTo(AlignCenter)
	if !invalid {
		return row
	}
	hint := Text("开始日期不能晚于结束日期").St(St().Text(11, 400, ColDanger))
	return Column(row, hint).St(St().GapOf(4))
}

// DateRangeValid 区间是否合法（两端都填且先后顺序正确）。
func DateRangeValid(from, to string) bool {
	return from != "" && to != "" && from <= to
}
