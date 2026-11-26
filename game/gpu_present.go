// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "errors"

// 本文件提供「零回读」呈现通道。
//
// 硬件后端渲染完场景后，颜色结果本来就在显存的 FBO 颜色纹理里；老路径会立刻
// glReadPixels 把整帧拷回内存（1080p 每帧 8MB，60fps 就是 480MB/s 纯拷贝），
// 宿主再把它上传成纹理——等于每帧把像素搬了两趟。
//
// 这里把「绘制」与「回读」拆开：绘制完成后直接把颜色纹理句柄交给宿主，由宿主
// 在自己的合成管线里当普通纹理画（一次 GPU 到 GPU 采样，零拷贝）。
//
// 前提：宿主与 game 必须在**同一个 GL 上下文**里工作，否则纹理对象互不可见。
// Windows 上 newGLContext 会自动复用当前线程已绑定的上下文，因此宿主（gui）
// 先 MakeCurrent 再创建 GPU 即可。

// NativeFrame 一帧「留在显存里」的渲染结果。
// 宿主可用 TextureID 包装成自己的纹理对象直接合成（FlipY 需置 true）。
type NativeFrame struct {
	// Backend 产生该帧的后端名（gl / soft）。
	Backend string
	// TextureID 颜色纹理的硬件句柄（gl 纹理名）。
	TextureID uint32
	// W, H 纹理尺寸。
	W, H int
	// FlipY GL 纹理第 0 行对应屏幕下方，宿主绘制时需垂直翻转。
	FlipY bool
}

// NativeID 纹理的硬件句柄（软件后端始终为 0）。
func (t *GPUTexture) NativeID() uint32 {
	if t == nil {
		return 0
	}
	return t.glID
}

// ZeroCopy 当前后端是否支持零回读呈现（只有 gl 硬件后端支持）。
func (g *GPU) ZeroCopy() bool {
	return g != nil && g.bk != nil && g.bk.name() == "gl"
}

// RenderNative 渲染场景但**不回读**像素，返回颜色纹理句柄。
// 软件后端不支持零拷贝，会返回错误；调用方应回退到 Render。
func (s *GPUScene) RenderNative(sc *Scene, cam *Camera) (*NativeFrame, error) {
	if s == nil || s.gpu == nil {
		return nil, errors.New("game: GPU 场景未初始化")
	}
	if !s.gpu.ZeroCopy() {
		return nil, errors.New("game: 当前后端不支持零回读呈现（需要 gl 后端）")
	}
	if err := s.renderPass(sc, cam); err != nil {
		return nil, err
	}
	if s.color == nil {
		return nil, errors.New("game: 帧缓冲没有颜色附件")
	}
	id := s.color.NativeID()
	if id == 0 {
		return nil, errors.New("game: 颜色纹理没有硬件句柄")
	}
	return &NativeFrame{Backend: s.gpu.Backend(), TextureID: id, W: s.W, H: s.H, FlipY: true}, nil
}
