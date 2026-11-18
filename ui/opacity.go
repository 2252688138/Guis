// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "guis/internal/gpu"

// fadedPaint 返回把 alpha 乘以 op 的颜色（op <= 0、op >= 1 或颜色本就透明时原样返回）。
//
// 供 Style.Opacity / Fade(x) 在绘制阶段调制节点自身的外观：以前 Opacity 只被当成
// 「是否绘制」的开关（<=0 直接跳过），Fade(0.16) 这类写法等于完全没生效 ——
// 典型症状是同色底板 + 同色图标时图标整块看不见。
func fadedPaint(c gpu.Paint, op float32) gpu.Paint {
	if op <= 0 || op >= 1 || c.A <= 0 {
		return c
	}
	c.A *= op
	return c
}

// fadedStyle 返回把 Fade(x) / Opacity 应用到自身配色后的样式副本（op 不在 (0,1) 时原样返回）。
func fadedStyle(st *Style) *Style {
	if st == nil {
		return &Style{}
	}
	op := st.Opacity
	if op <= 0 || op >= 1 {
		return st
	}
	cp := *st
	cp.Bg, cp.BgTo = fadedPaint(cp.Bg, op), fadedPaint(cp.BgTo, op)
	cp.BorderColor, cp.ShadowColor = fadedPaint(cp.BorderColor, op), fadedPaint(cp.ShadowColor, op)
	cp.Color = fadedPaint(cp.Color, op)
	return &cp
}
