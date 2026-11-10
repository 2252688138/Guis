// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// wheelClicksFrom 把浏览器 wheel 事件的 deltaY 折算成框架约定的「滚轮格数」。
//
// mode 对应 DOM WheelEvent.deltaMode：0 = 像素、1 = 行、2 = 页。
//
// 框架里 EvMouseWheel.DY 的单位是「格」——handleWheelAt 会再乘 wheelStepSize(96px)，
// 桌面端 WM_MOUSEWHEEL 天然就是 ±120 一格。浏览器给的却是像素（一格约 ±100）或
// 行 / 页，直接透传会被当成上百格 ≈ 上万像素：一次滚动就直接顶到最底部或弹回最
// 顶部，这就是网页端「滚轮要么滚到开头要么滚到底」的根因。
//
// 返回上正下负（浏览器 deltaY > 0 表示向下滚），并夹到 ±3 格：触控板惯性一帧
// 可能抖出很大的 deltaY，不夹会出现一跳一大段。
func wheelClicksFrom(dy float64, mode int) float32 {
	switch mode {
	case 1: // DOM_DELTA_LINE：3 行 ≈ 1 格
		dy /= 3
	case 2: // DOM_DELTA_PAGE：一页最多按 3 格算（下面会夹）
		dy *= 6
	default: // DOM_DELTA_PIXEL：100px ≈ 1 格
		dy /= 100
	}
	dy = -dy
	if dy > 3 {
		dy = 3
	}
	if dy < -3 {
		dy = -3
	}
	return float32(dy)
}

// touchDragSlop 触摸拖动发出滚轮事件的最小累计位移（逻辑像素）。
//
// 太小：点击时手指抖动会变成滚动；太大：开始滑的前几像素不跟手，手感发黏。
const touchDragSlop = 3

// touchDragClicks 把手指纵向累计位移折算成「滚轮格数」（触摸拖动专用）。
//
// 移动端没有滚轮，手指拖动必须合成为 EvMouseWheel（Pointer=1 → 1:1 跟手）。
// acc 是逻辑像素的累计位移，1 格 = wheelStepSize(96px)，所以滑动 96px 就滚一格；
// 不足 touchDragSlop 的抖动返回 0（不发事件，避免点击被当成滚动）。
// 返回正数 = 向上滚（内容向下），与 wheelClicksFrom 的方向约定一致。
func touchDragClicks(acc float32) float32 {
	if acc > -touchDragSlop && acc < touchDragSlop {
		return 0
	}
	return acc / wheelStepSize
}
