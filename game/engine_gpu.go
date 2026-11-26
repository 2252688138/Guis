// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "errors"

// Engine 的硬件渲染路径。
//
// Engine 默认用软件 Renderer（CPU 光栅化，结果是一块 image.RGBA），宿主拿到
// 像素后还要上传纹理。AttachGPU 之后引擎改走 GPU 后端：渲染进离屏 FBO，再
// 通过 RenderViewportNative 把颜色纹理句柄直接交给宿主合成，跳过回读与上传。
//
//	if err := eng.AttachGPU(gpu); err != nil { /* 回退软件路径 */ }
//	tex, w, h, ok := eng.RenderViewportNative(cw, ch, dt)
//	if ok { renderer.DrawImage(gpu.Image{Tex: renderer.WrapTexture(tex, w, h), FlipY: true, ...}) }

// AttachGPU 让引擎改用硬件后端渲染；传 nil 恢复软件 Renderer。
// 必须在持有 GL 上下文的线程调用（宿主渲染线程）。
func (e *Engine) AttachGPU(g *GPU) error {
	if e == nil {
		return errors.New("game: 引擎为空")
	}
	if g == nil {
		if e.gpuScene != nil {
			e.gpuScene.Close()
		}
		e.gpu, e.gpuScene = nil, nil
		return nil
	}
	w, h := e.BaseW, e.BaseH
	if w < 16 {
		w = 16
	}
	if h < 16 {
		h = 16
	}
	gs, err := NewGPUScene(g, w, h)
	if err != nil {
		return err
	}
	if e.gpuScene != nil {
		e.gpuScene.Close()
	}
	e.gpu, e.gpuScene = g, gs
	return nil
}

// GPU 当前硬件后端（未 AttachGPU 时为 nil）。
func (e *Engine) GPU() *GPU {
	if e == nil {
		return nil
	}
	return e.gpu
}

// GPUScene 硬件渲染场景（未 AttachGPU 时为 nil）。
func (e *Engine) GPUScene() *GPUScene {
	if e == nil {
		return nil
	}
	return e.gpuScene
}

// ZeroCopy 引擎是否已接通零回读呈现。
func (e *Engine) ZeroCopy() bool {
	return e != nil && e.gpu != nil && e.gpuScene != nil && e.gpu.ZeroCopy()
}

// targetSize 按控件宽高比算内部渲染尺寸；返回 0,0 表示保持现状。
func (e *Engine) targetSize(w, h int) (int, int) {
	if !e.KeepAspect || w <= 0 || h <= 0 {
		return 0, 0
	}
	tw := maxInt(e.BaseW, 16)
	th := int(float32(tw) * float32(h) / float32(w))
	if th < 16 {
		th = 16
	}
	if th > 1440 {
		th = 1440
	}
	return tw, th
}

// RenderViewportNative 渲染一帧并把颜色纹理句柄交给宿主（零回读）。
// 返回 ok=false 表示当前引擎没有可用的硬件路径，调用方应回退 RenderViewport。
func (e *Engine) RenderViewportNative(w, h int, dt float32) (texID uint32, tw, th int, ok bool) {
	if e == nil || e.gpu == nil || e.gpuScene == nil {
		return 0, 0, 0, false
	}
	e.Tick(dt)
	if nw, nh := e.targetSize(w, h); nw > 0 {
		if err := e.gpuScene.Resize(nw, nh); err != nil {
			return 0, 0, 0, false
		}
	}
	fr, err := e.gpuScene.RenderNative(e.Scene, e.Camera)
	if err != nil || fr == nil || fr.TextureID == 0 {
		return 0, 0, 0, false
	}
	e.lastW, e.lastH = fr.W, fr.H
	e.clientW, e.clientH = w, h
	return fr.TextureID, fr.W, fr.H, true
}
