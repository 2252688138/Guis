// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

// 外部纹理（零拷贝合成）。
//
// 3D 引擎（guis/game）在自己的 FBO 里渲染完之后，颜色结果已经是一张显存纹理。
// 老路径要 glReadPixels 拷回内存再由 GUI 上传成纹理，1080p@60fps 相当于每帧
// 白搬 8MB×2。WrapTexture 直接把那张纹理包成本渲染器的 Texture：绘制时只是
// 普通采样，像素一次也不进 CPU。
//
// 限制：两个渲染器必须处在同一个 GL 上下文（同一线程、同一 HGLRC），否则纹理
// 名字互不可见；调用方负责外部纹理的生命周期，本渲染器不会删除它。

// WrapTexture 把已有 GL 纹理包成 GUI 纹理对象（不复制像素、不拥有所有权）。
func (r *Renderer) WrapTexture(id uint32, w, h int) *Texture {
	if id == 0 || w <= 0 || h <= 0 {
		return nil
	}
	return &Texture{ID: id, W: w, H: h, External: true}
}
