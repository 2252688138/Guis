// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// Canvas 绘制：形状变化才重新光栅，然后上传纹理并走 DrawImage 合成。

import (
	"guis/internal/gpu"
	"guis/internal/geom"
)

// paintCanvas 绘制 Canvas 节点。
func (a *App) paintCanvas(n *Node, r geom.Rect) {
	st := n.Canvas
	if a == nil || st == nil {
		return
	}
	if st.w <= 0 || st.h <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	if st.needRaster() {
		st.pix = st.rasterize()
	}
	if st.pix == nil {
		return
	}	// 没有活动渲染器（离屏 / 测试）时只做光栅，不上传。
	if a.r == nil {
		return
	}
	if st.tex == nil || st.tw != st.w || st.th != st.h {
		if st.tex != nil {
			a.r.DeleteTexture(st.tex)
		}
		st.tex = a.r.NewTextureRGBA(st.w, st.h, st.pix)
		st.tw, st.th = st.w, st.h
	} else {
		a.r.UpdateTextureRGBA(st.tex, 0, 0, st.w, st.h, st.pix)
	}
	if st.tex == nil {
		return
	}
	a.r.DrawImage(gpu.Image{
		Rect:   r,
		Tex:    st.tex,
		Tint:   gpu.Paint{R: 1, G: 1, B: 1, A: 1},
		Radius: n.Style.Radius,
	})
}
