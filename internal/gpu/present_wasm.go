//go:build js && wasm

package gpu

// PresentTarget 浏览器端的「离屏呈现目标」。
//
// 为什么需要它：WebGL 的默认帧缓冲没有双缓冲语义。如果每帧都「先清屏、再逐个图元重画」，
// 浏览器合成器有可能在两次 JS 任务之间抓到一个只画了一半的画面——鼠标移动触发
// 频繁重绘时，这种中间态就会出现，观感就是「界面在闪」。
//
// 做法：整帧先画进 FBO（不可见），画完后用 blitFramebuffer 一次性拷到默认缓冲。
// 这样每次上屏都是一个完整帧，中间态永远不会被看到。
type PresentTarget struct {
	FBO  uint32
	Tex  uint32
	W, H int32
}

// NewPresentTarget 新建呈现目标（首次 Ensure 时分配显存）。
func NewPresentTarget() *PresentTarget { return &PresentTarget{} }

// Ensure 按像素尺寸准备 FBO；尺寸不变时直接复用。
func (p *PresentTarget) Ensure(w, h int32) uint32 {
	if p == nil || w < 1 || h < 1 {
		return 0
	}
	if p.FBO != 0 && p.W == w && p.H == h {
		return p.FBO
	}
	p.Free()

	const (
		textureMagFilter = 0x2800
		textureWrapS     = 0x2802
		textureWrapT     = 0x2803
		clampToEdge      = 0x812F
		nearest          = 0x2600
	)

	var tex uint32
	glGenTextures(1, &tex)
	glBindTexture(GL_TEXTURE_2D, tex)
	glTexParameteri(GL_TEXTURE_2D, textureMagFilter, nearest)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, nearest)
	glTexParameteri(GL_TEXTURE_2D, textureWrapS, clampToEdge)
	glTexParameteri(GL_TEXTURE_2D, textureWrapT, clampToEdge)
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, w, h, GL_RGBA, GL_UNSIGNED_BYTE, nil)

	var fbo uint32
	glGenFramebuffers(1, &fbo)
	glBindFramebuffer(GL_FRAMEBUFFER, fbo)
	glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, tex, 0)
	glBindFramebuffer(GL_FRAMEBUFFER, 0)

	p.FBO, p.Tex, p.W, p.H = fbo, tex, w, h
	return fbo
}

// Blit 把离屏内容一次性拷到浏览器默认帧缓冲（单次操作，无中间态）。
func (p *PresentTarget) Blit() {
	if p == nil || p.FBO == 0 {
		return
	}
	const (
		readFramebuffer  = 0x8CA8
		drawFramebuffer  = 0x8CA9
		colorBufferBit   = 0x4000
		nearest          = 0x2600
	)
	glBindFramebuffer(readFramebuffer, p.FBO)
	glBindFramebuffer(drawFramebuffer, 0)
	glCall("blitFramebuffer", 0, 0, int(p.W), int(p.H), 0, 0, int(p.W), int(p.H), colorBufferBit, nearest)
	glBindFramebuffer(GL_FRAMEBUFFER, 0)
}

// Free 释放显存。
func (p *PresentTarget) Free() {
	if p == nil {
		return
	}
	if p.FBO != 0 {
		glDeleteFramebuffers(1, &p.FBO)
	}
	if p.Tex != 0 {
		glDeleteTextures(1, &p.Tex)
	}
	p.FBO, p.Tex, p.W, p.H = 0, 0, 0, 0
}
