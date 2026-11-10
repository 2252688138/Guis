// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"guis/internal/geom"
	"guis/internal/gpu"
)

// 图标绘制与纹理缓存。
//
// 图标纹理是「白色 + 覆盖率 alpha」：绘制时用 gpu.Image.Tint 染成任意颜色，
// 因此同尺寸的不同颜色图标共用一张纹理。纹理按（名称、像素边长、线宽）缓存，
// 尺寸变化（拖动窗口 / DPI 切换）才会重新光栅化，正常帧只有一次 DrawImage。

// iconTexKey 图标纹理缓存键。
type iconTexKey struct {
	Name string
	Size int   // 纹理边长（设备像素）
	// Stroke 线宽 ×100（设计网格单位）：量化后避免浮点误差把缓存拆成多份
	Stroke int16
}

// iconTexture 取（必要时光栅化）图标纹理。
func (a *App) iconTexture(name string, stroke float32, px int) *gpu.Texture {
	def, ok := IconDefOf(name)
	if !ok || len(def.Paths) == 0 {
		return nil
	}
	if stroke <= 0 {
		stroke = def.Stroke
	}
	if px < 4 {
		px = 4
	}
	if px > 512 {
		px = 512
	}
	key := iconTexKey{Name: name, Size: px, Stroke: int16(stroke*100 + 0.5)}
	if a.icons == nil {
		a.icons = map[iconTexKey]*gpu.Texture{}
	}
	if t := a.icons[key]; t != nil && t.ID != 0 {
		return t
	}

	subs := make([]iconSubPath, 0, len(def.Paths))
	for _, d := range def.Paths {
		subs = append(subs, iconFlattenPath(d)...)
	}
	t := a.r.NewTextureRGBA(px, px, iconRaster(subs, def.Fill, float64(stroke), px))
	if t == nil || t.ID == 0 {
		return nil
	}
	a.icons[key] = t
	return t
}

// paintIcon 绘制图标节点：按短边等比居中，颜色取 Style.Color（未设置用次要文字色）。
func (a *App) paintIcon(n *Node, r geom.Rect, st *Style) {
	if n.IconName == "" || r.W <= 0 || r.H <= 0 {
		return
	}
	side := minf(r.W, r.H)
	px := int(side*(a.effScale()*a.textureSS()) + 0.5)
	tex := a.iconTexture(n.IconName, n.IconStroke, px)
	if tex == nil {
		return
	}
	color := st.Color
	if color.A <= 0 {
		color = C(ColTextDim)
	}
	rect := geom.Rect{
		X: r.X + (r.W-side)/2,
		Y: r.Y + (r.H-side)/2,
		W: side,
		H: side,
	}
	a.r.DrawImage(gpu.Image{Rect: rect, Tex: tex, Tint: color})
}

// iconReleaseAll 释放全部图标纹理（空闲回收 / 关窗时调用，下一帧按需重建）。
func (a *App) iconReleaseAll() {
	if a == nil || a.r == nil {
		return
	}
	for k, t := range a.icons {
		if t != nil {
			a.r.DeleteTexture(t)
		}
		delete(a.icons, k)
	}
}
