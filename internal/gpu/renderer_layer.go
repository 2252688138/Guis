// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import "guis/internal/geom"

// LayerBudget 组件栅格缓存（ui 层的 Style.CacheLayer）总预算：基准 6MB，按窗口面积自适应。
//
// 组件缓存用「显存换 CPU」：一张 1200×400 的卡片约 1.9MB。预算从 16MB 收到 6MB：
// 实测小窗口里真正命中的缓存往往只有 1~3MB，16MB 只是上限却让常驻显存长期偏高；
// 降低后超额条目按 LRU 淘汰，命中率几乎不变（重新烘焙一张卡片只有一次绘制开销）。
func (r *Renderer) LayerBudget() int64 {
	return int64(float64(6<<20) * r.TextureBudgetFactor())
}

// CaptureRGBA 把当前绘制目标里的一块矩形拷进一张 RGBA 纹理，供组件栅格缓存复用。
//
// 参数 rect 为逻辑像素矩形；纹理按设备像素创建（含 DPI/超采样倍率），
// 贴回时仍绘制到同一个逻辑矩形，因此 1:1 映射（scale=1 时不重采样，
// scale>1 时相当于把超采样结果降采样，与 Resolve 的处理一致）。
//
// *dst 传入 nil 表示按需创建；尺寸变化时自动重建。返回 false 表示本次无法捕获
// （区域非法 / 完全在目标外 / 纹理创建失败），调用方应当放弃缓存、继续直绘。
func (r *Renderer) CaptureRGBA(dst **Texture, rect geom.Rect) bool {
	if dst == nil || rect.W < 1 || rect.H < 1 || r.pw <= 0 || r.ph <= 0 {
		return false
	}
	// 先提交待上传的批次：否则拷到的是「前几批」的画面，缓存内容会缺一块。
	r.flush()

	s := r.scale
	if s <= 0 {
		s = 1
	}
	x0 := int32(round32(rect.X * s))
	// 逻辑 y 轴向下、GL 原点在左下：取矩形的底边换算
	y0 := int32(round32(float32(r.ph) - rect.Bottom()*s))
	w := int32(round32(rect.W * s))
	h := int32(round32(rect.H * s))
	if x0 < 0 {
		w += x0
		x0 = 0
	}
	if y0 < 0 {
		h += y0
		y0 = 0
	}
	if x0+w > r.pw {
		w = r.pw - x0
	}
	if y0+h > r.ph {
		h = r.ph - y0
	}
	if w < 1 || h < 1 {
		return false
	}

	t := *dst
	if t != nil && (int32(t.W) != w || int32(t.H) != h || t.ID == 0) {
		r.DeleteTexture(t)
		t = nil
	}
	if t == nil {
		t = r.NewTextureRGBA(int(w), int(h), nil)
		if t == nil || t.ID == 0 {
			return false
		}
		*dst = t
	}

	// 从当前绘制目标捕获（与毛玻璃同一条通路：glCopyTexSubImage2D 不依赖 FBO 绑定差异）
	glBindFramebuffer(GL_FRAMEBUFFER, r.drawFBO)
	r.bindTex(t.ID)
	glCopyTexSubImage2D(GL_TEXTURE_2D, 0, 0, 0, x0, y0, w, h)
	// 纹理绑定状态已被本函数改写，交给 bindTex 的缓存保持一致
	r.curTexID = t.ID
	return true
}

// round32 四舍五入到整数像素。
func round32(v float32) float32 {
	if v >= 0 {
		return float32(int32(v + 0.5))
	}
	return float32(int32(v - 0.5))
}
