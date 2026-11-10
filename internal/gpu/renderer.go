// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"guis/internal/geom"
	"guis/internal/text"
)

const (
	maxVerts        = 60000
	floatsShapeVert = 22
	floatsTextVert  = 9
	// 顶点数据切片的初始容量（顶点数）：按典型一屏预留。
	// 旧值 8192/20480 顶点对应 0.7MB/0.7MB 的常驻缓冲，而常规页面一帧只有几百个顶点；
	// 峰值场景（十万行表格/长文）超过时会按需扩容，空闲回收（ReleaseVerts）再收回。
	maxVertsShapeInitial = 4096
	maxVertsTextInitial  = 6144
)

type batchKind uint8

const (
	batchNone batchKind = iota
	batchShape
	batchText
	batchImage
)

// Renderer OpenGL 3.3 批处理 2D 渲染器，实现 Backend。
type Renderer struct {
	width, height float32
	pw, ph        int32
	scale         float32

	shapeProg, textProg uint32
	shapeVAO, shapeVBO  uint32
	textVAO, textVBO    uint32

	shapeData []float32
	textData  []float32
	shapeN    int
	textN     int

	uShapeVP, uTextVP, uTextAtlas, uTextGamma int32
	// textGamma 字形覆盖率 gamma（<1 加粗变实，=1 关闭），见 fragShaderText。
	textGamma float32

	cur          batchKind
	curFont      *text.Font
	curFontColor bool // 当前文字批次绑定的是彩色图集（Emoji）
	curTex       *Texture
	fonts        map[*text.Font]*fontAtlas
	clipSaved    []geom.Rect

	White *Texture

	// 图片批次
	imgProg, imgVAO, imgVBO uint32
	imgData                 []float32
	imgN                    int
	uImgVP, uImgTex         int32

	// 离屏模糊（backdrop-filter）
	blurProg    uint32
	blurVAO     uint32
	blurVBO     uint32
	uBlurTex    int32
	uBlurTexel  int32
	uBlurRadius int32
	srcTex      *fboRT
	halfA       *fboRT
	halfB       *fboRT
	quarter     *fboRT

	// 顶点缓冲容量（字节）：仅在超过时重新分配，其余帧用孤儿化+SubData
	shapeBufCap, textBufCap, imgBufCap int
	// 字形图集上传用的中间缓冲（避免每帧分配）
	atlasBuf []byte
	// 当前 scissor 区域（避免重复设置触发无意义的批次切分）
	clipCur geom.Rect
	clipOn  bool
	// targetFBO 呈现目标（0 = 窗口默认帧缓冲；>0 = 离屏目标，DComp 合成用）
	targetFBO uint32
	// drawFBO 本帧实际绑定的绘制目标：开启超采样时指向离屏 RT，否则同 targetFBO
	drawFBO uint32

	// GL 状态缓存：一帧内批次频繁切换（形状/文字/图片/裁剪），
	// 重复的 glUseProgram / glBindVertexArray / glBindTexture / glUniform2f
	// 都是驱动侧往返。缓存后多数切换只剩栈上比较。
	// 注意：任何直接绑定这些状态的代码路径都必须同步缓存（见 useProg / bindVAO / bindTex）
	// 或置 0 失效（删除纹理 / 销毁时）。
	curProg            uint32
	curVAO             uint32
	curTexID           uint32
	vpShapeW, vpShapeH float32
	vpTextW, vpTextH   float32
	vpImgW, vpImgH     float32

	// 超采样（SSAA）：整帧先按 ssaa 倍分辨率画进离屏 RT，再由 Resolve 降采样呈现。
	// 圆角/SDF 边缘、描边、阴影与字形都因此得到 ssaa² 倍的子像素覆盖率。
	ssaa          float32
	ssaaRT        *fboRT
	resolveProg   uint32
	resolveVAO    uint32
	resolveVBO    uint32
	uResolveTex   int32
	uResolveTexel int32
	uResolveSpan  int32
	uResolveSharp int32
}

// fontAtlas 一个字体对应的两张图集纹理：
//   - mono  单色字形（R8，1 字节/像素，占绝大多数）；
//   - color 彩色 Emoji（RGBA8，只在真的渲染过 Emoji 时才创建）。
//
// 为什么不合成一张 RGBA：普通字形只需要一字节覆盖率，用 RGBA8 存就是 4 倍白占。
// 拆开后 24 个字体实例的字形图集从 8MB 降到 2MB，且与渲染质量无关（1:1 采样）。
type fontAtlas struct {
	mono  *Texture
	color *Texture
}

// NewRenderer 创建渲染器。
func NewRenderer() *Renderer {
	return &Renderer{scale: 1, ssaa: 1, fonts: make(map[*text.Font]*fontAtlas), textGamma: defaultTextGamma()}
}

// defaultTextGamma 字形覆盖率 gamma 默认值（仅对亮字生效，见 fragShaderText）。
//
// 浏览器（Skia/DirectWrite）在 sRGB 空间混合前会对覆盖率做 gamma 校正，等效于让中间
// 覆盖率变大：亮度 0.5 的像素在浏览器里看起来接近 0.7。当前渲染器是 sRGB 空间直混，
// 同样的覆盖率就显得偏淡偏虚，尤其小字号中文的横竖笔。实测取 1/1.8 ≈ 0.55
// 与浏览器观感最接近；GUIS_TEXT_GAMMA 可覆盖（1 = 完全关闭）。
func defaultTextGamma() float32 {
	if v := os.Getenv("GUIS_TEXT_GAMMA"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f > 0 && f <= 4 {
			return float32(f)
		}
	}
	return TextGammaForLevel(text.Sharp())
}

// TextGammaForLevel 按文字锐度档位给出覆盖率 gamma（越小笔画越实）。
//
// 覆盖率锐化（smoothstep 对比度增强）在文字光栅化时就做完了（见 text.sharpCoverage），
// 这里只做小幅补充：清晰档维持 0.55 的基准值，避免两道增强叠加把笔画糊成块；
// 极致档再推到 0.5。
func TextGammaForLevel(level int) float32 {
	switch level {
	case text.SharpMax:
		return 0.5
	}
	return 0.55
}

// SetTextGamma 运行时修改字形覆盖率 gamma（<1 更实，1 = 关闭），下一帧生效。
func (r *Renderer) SetTextGamma(v float32) {
	if v < 0.2 {
		v = 0.2
	}
	if v > 1 {
		v = 1
	}
	r.textGamma = v
}

// TextGamma 当前字形覆盖率 gamma。
func (r *Renderer) TextGamma() float32 { return r.textGamma }

// textGammaFor 当前超采样下的字形覆盖率 gamma。
//
// 超采样的降采样是「子样本线性平均」，gamma 补偿作用在平均之后并不会失真，
// 因此不需要（也不应该）按 1/ss 收敛。旧实现让 2× 下的补偿从 0.55 弱化到 0.775，
// 细笔画的覆盖率被压低，文字明显发灰发虚——观感反而比 1:1 更「不清晰」。
// 现在所有倍率统一使用同一补偿值：SSAA 只提升几何/边缘精度，不改变笔画实度。
func (r *Renderer) textGammaFor() float32 {
	return r.textGamma
}

func (r *Renderer) Init() error {
	p1, err := linkProgram(vertShaderShape, fragShaderShape)
	if err != nil {
		return fmt.Errorf("shape shader: %w", err)
	}
	p2, err := linkProgram(vertShaderText, fragShaderText)
	if err != nil {
		return fmt.Errorf("text shader: %w", err)
	}
	r.shapeProg, r.textProg = p1, p2
	r.uShapeVP = glGetUniformLocation(p1, "uViewport")
	r.uTextVP = glGetUniformLocation(p2, "uViewport")
	r.uTextAtlas = glGetUniformLocation(p2, "uAtlas")
	r.uTextGamma = glGetUniformLocation(p2, "uTextGamma")

	glGenVertexArrays(1, &r.shapeVAO)
	glGenBuffers(1, &r.shapeVBO)
	glBindVertexArray(r.shapeVAO)
	glBindBuffer(GL_ARRAY_BUFFER, r.shapeVBO)
	sd := int32(floatsShapeVert * 4)
	setAttr(0, 2, sd, 0)
	setAttr(1, 2, sd, 8)
	setAttr(2, 4, sd, 16)
	setAttr(3, 4, sd, 32)
	setAttr(4, 2, sd, 48)
	setAttr(5, 4, sd, 56)
	setAttr(6, 4, sd, 72)

	glGenVertexArrays(1, &r.textVAO)
	glGenBuffers(1, &r.textVBO)
	glBindVertexArray(r.textVAO)
	glBindBuffer(GL_ARRAY_BUFFER, r.textVBO)
	td := int32(floatsTextVert * 4)
	setAttr(0, 2, td, 0)
	setAttr(1, 2, td, 8)
	setAttr(2, 4, td, 16)
	setAttr(3, 1, td, 32)
	glBindVertexArray(0)

	glDisable(GL_DEPTH_TEST)
	glDisable(GL_CULL_FACE)
	glEnable(GL_BLEND)
	glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA)

	white := make([]byte, 4*4)
	for i := range white {
		white[i] = 255
	}
	r.White = r.NewTextureRGBA(2, 2, white)

	// 顶点缓冲初始容量按「典型一屏」预留，而不是按硬上限 maxVerts 预留：
	// 旧实现一开窗就分配 60000×22×4B ≈ 5.3MB + 60000×9×4B ≈ 2.2MB 的 Go 切片，
	// 而实际一屏通常只有几百到几千个顶点。缓冲区会按需扩容（上限仍为 maxVerts）。
	r.shapeData = make([]float32, 0, maxVertsShapeInitial*floatsShapeVert)
	r.textData = make([]float32, 0, maxVertsTextInitial*floatsTextVert)
	if err := r.initImage(); err != nil {
		return err
	}
	if err := r.initBlur(); err != nil {
		return err
	}
	if err := r.initResolve(); err != nil {
		return err
	}
	return nil
}

func setAttr(loc uint32, size int32, stride, offset int32) {
	glEnableVertexAttribArray(loc)
	glVertexAttribPointer(loc, size, GL_FLOAT, 0, stride, offset)
}

func (r *Renderer) NewTextureR8(w, h int, data []byte) *Texture {
	var id uint32
	glGenTextures(1, &id)
	glBindTexture(GL_TEXTURE_2D, id)
	// 直接绑定了单元 0 的纹理，缓存作废
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)
	glTexImage2D(GL_TEXTURE_2D, 0, GL_R8, int32(w), int32(h), GL_RED, GL_UNSIGNED_BYTE, data)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_TEXTURE_WRAP_CLAMP_TO_EDGE)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_TEXTURE_WRAP_CLAMP_TO_EDGE)
	return &Texture{ID: id, W: w, H: h, R8: true}
}

func (r *Renderer) UpdateTextureR8(t *Texture, x, y, w, h int, data []byte) {
	if t == nil || len(data) == 0 {
		return
	}
	glBindTexture(GL_TEXTURE_2D, t.ID)
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)
	glTexSubImage2D(GL_TEXTURE_2D, 0, int32(x), int32(y), int32(w), int32(h), GL_RED, GL_UNSIGNED_BYTE, data)
}

func (r *Renderer) NewTextureRGBA(w, h int, data []byte) *Texture {
	var id uint32
	glGenTextures(1, &id)
	glBindTexture(GL_TEXTURE_2D, id)
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)
	glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, int32(w), int32(h), GL_RGBA, GL_UNSIGNED_BYTE, data)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_TEXTURE_WRAP_CLAMP_TO_EDGE)
	glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_TEXTURE_WRAP_CLAMP_TO_EDGE)
	return &Texture{ID: id, W: w, H: h}
}

func (r *Renderer) Resize(pw, ph int32, scale float32) {
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	r.pw, r.ph = pw, ph
	if scale <= 0 {
		scale = 1
	}
	r.scale = scale
	r.width = float32(pw) / scale
	r.height = float32(ph) / scale
	glViewport(0, 0, pw, ph)
}

// Bounds 逻辑尺寸。
func (r *Renderer) Bounds() geom.Rect { return geom.Rect{0, 0, r.width, r.height} }

// SetTarget 切换绘制目标（0 = 窗口默认帧缓冲，>0 = 离屏 FBO，用于 DComp 合成）。
func (r *Renderer) SetTarget(fbo uint32, pw, ph int32) {
	if pw <= 0 || ph <= 0 {
		return
	}
	r.targetFBO = fbo
	r.pw, r.ph = pw, ph
	r.width = float32(pw) / r.scale
	r.height = float32(ph) / r.scale
}

// Scale DPI 缩放。
func (r *Renderer) Scale() float32 { return r.scale }

// PixelSize 当前渲染目标像素尺寸（含超采样倍率）。
func (r *Renderer) PixelSize() (int, int) { return int(r.pw), int(r.ph) }

// TextureBudgetFactor 显存预算倍率：按「窗口像素面积 / 1280×800」缩放，限定 0.5~1.5。
//
// 旧实现把图片 / 图表纹理预算写成固定值（32MB / 24MB）：小窗口时明显偏多，
// 大窗口时又因为还有超采样离屏 + 合成表面而把整进程常驻推到 100MB 以上。
// 改成按可见像素面积分档后，小窗口自动收敛、大窗口按需放宽。
func (r *Renderer) TextureBudgetFactor() float64 {
	ss := float64(r.ssaa)
	if ss < 1 {
		ss = 1
	}
	area := float64(r.pw) * float64(r.ph) / (ss * ss)
	f := area / (1280 * 800)
	if f < 0.5 {
		f = 0.5
	} else if f > 1.5 {
		f = 1.5
	}
	return f
}

// ImageTexBudget 图片纹理预算（基准 8MB，按窗口面积自适应）。
func (r *Renderer) ImageTexBudget() int64 {
	return int64(float64(8<<20) * r.TextureBudgetFactor())
}

// ChartTexBudget 图表纹理预算（基准 6MB，按窗口面积自适应）。
func (r *Renderer) ChartTexBudget() int64 {
	return int64(float64(6<<20) * r.TextureBudgetFactor())
}

// SetSuperSample 设置整帧超采样倍数（1 = 关闭，推荐 1.5~2）。
// 调用后下一帧生效：绘制目标重建为 pw*ssaa × ph*ssaa，绘制结束时由 Resolve 降采样呈现。
// 倍率变化会重建离屏目标，因此不要在每帧内调整。
func (r *Renderer) SetSuperSample(n float32) {
	if n < 1 {
		n = 1
	}
	if n > 2 {
		n = 2
	}
	if n == r.ssaa {
		return
	}
	r.ssaa = n
	r.dropSSAART()
}

// SuperSample 返回当前超采样倍数。
func (r *Renderer) SuperSample() float32 { return r.ssaa }

// dropSSAART 丢弃离屏超采样目标（倍率/尺寸变化时调用）。
func (r *Renderer) dropSSAART() {
	if r.ssaaRT != nil {
		r.freeRT(r.ssaaRT)
		r.ssaaRT = nil
	}
}

// ReleaseOffscreen 释放可重建的离屏目标（超采样 RT + 毛玻璃中间层）。
//
// 窗口最小化或长时间闲置时调用：一张超采样 RT 就是「窗口像素 × ssaa² × 4B」
// （1166×746 的窗口在 2× 下约 14MB），毛玻璃还有 4 层中间纹理；
// 这些资源下一帧都会按需重建，闲置时没有必要常驻显存。
func (r *Renderer) ReleaseOffscreen() {
	r.dropSSAART()
	r.DestroyBlur()
	r.ReleaseVerts()
}

// VertBytes 顶点暂存区占用的 Go 堆字节数（估算，供内存统计输出）。
func (r *Renderer) VertBytes() int64 {
	n := int64(cap(r.shapeData)+cap(r.textData)+cap(r.imgData)) * 4
	return n
}

// ReleaseVerts 把顶点暂存区收缩回初始容量。
//
// 渲染过一个十万行表格或长文后，shapeData/textData 会按峰值扩容到几 MB
// （例如 6 万个顶点 × 22 float × 4B ≈ 5MB），而窗口最小化/长期闲置时
// 这些内存再无用处。下一帧会重新按需扩容，观感不变。
func (r *Renderer) ReleaseVerts() {
	if cap(r.shapeData) > maxVertsShapeInitial*floatsShapeVert {
		r.shapeData = make([]float32, 0, maxVertsShapeInitial*floatsShapeVert)
	} else {
		r.shapeData = r.shapeData[:0]
	}
	if cap(r.textData) > maxVertsTextInitial*floatsTextVert {
		r.textData = make([]float32, 0, maxVertsTextInitial*floatsTextVert)
	} else {
		r.textData = r.textData[:0]
	}
	if cap(r.imgData) > 4096*floatsImgVert {
		r.imgData = make([]float32, 0, 4096*floatsImgVert)
	} else {
		r.imgData = r.imgData[:0]
	}
	r.shapeN, r.textN, r.imgN = 0, 0, 0
	if cap(r.clipSaved) > 16 {
		r.clipSaved = make([]geom.Rect, 0, 16)
	}
}

// OffscreenBytes 离屏目标占用的显存字节数（估算，供内存统计输出）。
func (r *Renderer) OffscreenBytes() int64 {
	n := int64(0)
	for _, rt := range []*fboRT{r.ssaaRT, r.srcTex, r.halfA, r.halfB, r.quarter} {
		if rt != nil {
			n += int64(rt.w) * int64(rt.h) * 4
		}
	}
	return n
}

// ensureSSAA 准备超采样离屏目标并校验完整性；不可用时返回 false，绘制退回呈现目标。
func (r *Renderer) ensureSSAA() bool {
	if r.ssaa <= 1.01 || r.pw <= 0 || r.ph <= 0 {
		return false
	}
	rt := r.ensureRT(&r.ssaaRT, r.pw, r.ph)
	if rt == nil {
		return false
	}
	glBindFramebuffer(GL_FRAMEBUFFER, rt.fbo)
	if glCheckFramebufferStatus(GL_FRAMEBUFFER) != GL_FRAMEBUFFER_COMPLETE {
		glBindFramebuffer(GL_FRAMEBUFFER, r.targetFBO)
		r.dropSSAART()
		return false
	}
	return true
}

// Resolve 把超采样离屏内容降采样写入呈现目标（targetFBO，0 = 窗口帧缓冲）。
// 未开启超采样时是空操作。必须在 End() 之后、Swap/EndFrame 之前调用。
func (r *Renderer) Resolve() {
	if r.ssaa <= 1.01 || r.ssaaRT == nil || r.ssaaRT.tex == nil {
		return
	}
	ss := r.ssaa
	dw := int32(float32(r.pw) / ss)
	dh := int32(float32(r.ph) / ss)
	if dw < 1 || dh < 1 {
		return
	}
	glBindFramebuffer(GL_FRAMEBUFFER, r.targetFBO)
	glViewport(0, 0, dw, dh)
	glDisable(GL_SCISSOR_TEST)
	// 降采样是整屏覆盖写：关掉混合，避免半透明背景与目标里的旧内容再次混合
	glDisable(GL_BLEND)
	r.clipOn = false
	r.useProg(r.resolveProg)
	glUniform1i(r.uResolveTex, 0)
	glUniform2f(r.uResolveTexel, 1/float32(r.ssaaRT.w), 1/float32(r.ssaaRT.h))
	glUniform1f(r.uResolveSpan, float32(r.ssaaRT.w)/float32(dw))
	glUniform1f(r.uResolveSharp, ssaaSharpen())
	r.bindTex(r.ssaaRT.tex.ID)
	r.bindVAO(r.resolveVAO)
	glDrawArrays(GL_TRIANGLES, 0, 6)
	r.bindVAO(0)
	// 恢复混合（后续帧的常规绘制依赖它）
	glEnable(GL_BLEND)
}

// ssaaSharpen 降采样后的锐化强度（默认 0.2）。
//
// 整帧 box 降采样是「数学上正确」的，但 1px 级别的横竖笔与 1px 描边被平均后对比度下降，
// 人眼主观上会觉得「超高清还不如 1:1 清晰」。这里补一道轻度 unsharp 把被平滑掉的
// 高频成分找回一部分：0 = 关闭，0.15~0.3 为推荐区间，GUIS_SS_SHARPEN 可覆盖。
func ssaaSharpen() float32 {
	if v := os.Getenv("GUIS_SS_SHARPEN"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f >= 0 && f <= 1 {
			return float32(f)
		}
	}
	return 0.4
}

// initResolve 初始化降采样着色器与全屏四边形。
func (r *Renderer) initResolve() error {
	p, err := linkProgram(vertShaderBlur, fragShaderResolve)
	if err != nil {
		return err
	}
	r.resolveProg = p
	r.uResolveTex = glGetUniformLocation(p, "uTex")
	r.uResolveTexel = glGetUniformLocation(p, "uTexel")
	r.uResolveSpan = glGetUniformLocation(p, "uSpan")
	r.uResolveSharp = glGetUniformLocation(p, "uSharpen")

	glGenVertexArrays(1, &r.resolveVAO)
	glGenBuffers(1, &r.resolveVBO)
	glBindVertexArray(r.resolveVAO)
	glBindBuffer(GL_ARRAY_BUFFER, r.resolveVBO)
	quad := []float32{
		-1, -1, 0, 0,
		1, -1, 1, 0,
		1, 1, 1, 1,
		-1, -1, 0, 0,
		1, 1, 1, 1,
		-1, 1, 0, 1,
	}
	glBufferData(GL_ARRAY_BUFFER, quad, GL_STATIC_DRAW)
	setAttr(0, 2, 16, 0)
	setAttr(1, 2, 16, 8)
	glBindVertexArray(0)
	return nil
}

func (r *Renderer) Begin(fill Paint) {
	r.shapeData = r.shapeData[:0]
	r.textData = r.textData[:0]
	r.shapeN, r.textN = 0, 0
	r.imgN = 0
	r.cur = batchNone
	r.curFont = nil
	r.curFontColor = false
	r.curTex = nil
	r.clipSaved = r.clipSaved[:0]
	r.clipOn = false
	r.clipCur = geom.Rect{}
	// 开启超采样时整帧画进高分辨率离屏目标，End 之后由 Resolve 降采样呈现
	r.drawFBO = r.targetFBO
	if r.ensureSSAA() {
		r.drawFBO = r.ssaaRT.fbo
	}
	glBindFramebuffer(GL_FRAMEBUFFER, r.drawFBO)
	glViewport(0, 0, r.pw, r.ph)
	glDisable(GL_SCISSOR_TEST)
	glClearColor(fill.R, fill.G, fill.B, fill.A)
	glClear(GL_COLOR_BUFFER_BIT)
}

func (r *Renderer) End() { r.flush() }

// Destroy 释放渲染资源。
func (r *Renderer) Destroy() {
	// GL 状态缓存先失效：对象销毁后缓存里的名字不再有效
	r.curProg, r.curVAO, r.curTexID = 0, 0, 0
	r.vpShapeW, r.vpTextW, r.vpImgW = 0, 0, 0
	r.vpShapeH, r.vpTextH, r.vpImgH = 0, 0, 0
	r.DestroyBlur()
	r.dropSSAART()
	if r.resolveVBO != 0 {
		glDeleteBuffers(1, &r.resolveVBO)
	}
	if r.resolveVAO != 0 {
		glDeleteVertexArrays(1, &r.resolveVAO)
	}
	if r.resolveProg != 0 {
		glDeleteProgram(r.resolveProg)
	}
	if r.imgVBO != 0 {
		glDeleteBuffers(1, &r.imgVBO)
	}
	if r.imgVAO != 0 {
		glDeleteVertexArrays(1, &r.imgVAO)
	}
	if r.imgProg != 0 {
		glDeleteProgram(r.imgProg)
	}
	if r.blurVBO != 0 {
		glDeleteBuffers(1, &r.blurVBO)
	}
	if r.blurVAO != 0 {
		glDeleteVertexArrays(1, &r.blurVAO)
	}
	if r.blurProg != 0 {
		glDeleteProgram(r.blurProg)
	}
	if r.shapeVBO != 0 {
		glDeleteBuffers(1, &r.shapeVBO)
	}
	if r.textVBO != 0 {
		glDeleteBuffers(1, &r.textVBO)
	}
	if r.shapeVAO != 0 {
		glDeleteVertexArrays(1, &r.shapeVAO)
	}
	if r.textVAO != 0 {
		glDeleteVertexArrays(1, &r.textVAO)
	}
	if r.shapeProg != 0 {
		glDeleteProgram(r.shapeProg)
	}
	if r.textProg != 0 {
		glDeleteProgram(r.textProg)
	}
}

func (r *Renderer) ensureKind(k batchKind, f *text.Font) {
	if r.cur != k || (k == batchImage && r.curTex == nil) {
		r.flush()
		r.cur = k
		r.curFont = f
	}
}

// ensureText 选定文字批次。
// 一个批次只能绑定一张图集纹理，所以「字体变了」或「单色/彩色图集切换」都必须先提交。
func (r *Renderer) ensureText(f *text.Font, color bool) {
	if r.cur != batchText || f != r.curFont || color != r.curFontColor {
		r.flush()
		r.cur = batchText
		r.curFont = f
		r.curFontColor = color
	}
}

func (r *Renderer) useProg(p uint32) {
	if r.curProg != p {
		glUseProgram(p)
		r.curProg = p
	}
}

func (r *Renderer) bindVAO(v uint32) {
	if r.curVAO != v {
		glBindVertexArray(v)
		r.curVAO = v
	}
}

func (r *Renderer) bindTex(id uint32) {
	if r.curTexID != id {
		glActiveTexture(GL_TEXTURE0)
		glBindTexture(GL_TEXTURE_2D, id)
		r.curTexID = id
	}
}

func (r *Renderer) flush() {
	switch r.cur {
	case batchShape:
		if r.shapeN > 0 {
			r.useProg(r.shapeProg)
			if r.vpShapeW != r.width || r.vpShapeH != r.height {
				glUniform2f(r.uShapeVP, r.width, r.height)
				r.vpShapeW, r.vpShapeH = r.width, r.height
			}
			r.bindVAO(r.shapeVAO)
			r.uploadVerts(&r.shapeVBO, &r.shapeBufCap, r.shapeData)
			glDrawArrays(GL_TRIANGLES, 0, int32(r.shapeN))
		}
		r.shapeData = r.shapeData[:0]
		r.shapeN = 0
	case batchText:
		if r.textN > 0 && r.curFont != nil {
			tex := r.fontTexture(r.curFont, r.curFontColor)
			if tex == nil || tex.ID == 0 {
				break
			}
			r.useProg(r.textProg)
			if r.vpTextW != r.width || r.vpTextH != r.height {
				glUniform2f(r.uTextVP, r.width, r.height)
				r.vpTextW, r.vpTextH = r.width, r.height
			}
			glUniform1i(r.uTextAtlas, 0)
			glUniform1f(r.uTextGamma, r.textGammaFor())
			r.bindTex(tex.ID)
			r.bindVAO(r.textVAO)
			r.uploadVerts(&r.textVBO, &r.textBufCap, r.textData)
			glDrawArrays(GL_TRIANGLES, 0, int32(r.textN))
		}
		r.textData = r.textData[:0]
		r.textN = 0
	case batchImage:
		if r.imgN > 0 && r.curTex != nil {
			r.useProg(r.imgProg)
			if r.vpImgW != r.width || r.vpImgH != r.height {
				glUniform2f(r.uImgVP, r.width, r.height)
				r.vpImgW, r.vpImgH = r.width, r.height
			}
			glUniform1i(r.uImgTex, 0)
			r.bindTex(r.curTex.ID)
			r.bindVAO(r.imgVAO)
			r.uploadVerts(&r.imgVBO, &r.imgBufCap, r.imgData)
			glDrawArrays(GL_TRIANGLES, 0, int32(r.imgN))
		}
		r.imgData = r.imgData[:0]
		r.imgN = 0
		r.curTex = nil
	}
}

// uploadVerts 上传顶点数据。
// 旧实现每次都 GlBufferData(数据) 重新分配整块缓冲，会反复触发驱动分配 +
// 与上一帧绘制同步；现在只在容量不足时扩容，其余帧孤儿化后 GlBufferSubData 覆盖，
// 大幅减少滚动/动画时的驱动开销。
func (r *Renderer) uploadVerts(vbo *uint32, capBytes *int, data []float32) {
	if len(data) == 0 {
		return
	}
	n := len(data) * 4
	glBindBuffer(GL_ARRAY_BUFFER, *vbo)
	if *capBytes < n {
		*capBytes = n + n/2 + 4096
	}
	// 孤儿化：解除与上一帧的依赖，避免驱动等待上一帧用完这块缓冲
	glBufferAlloc(GL_ARRAY_BUFFER, *capBytes, GL_STREAM_DRAW)
	glBufferSubData(GL_ARRAY_BUFFER, 0, data)
}

// fontTexture 取（必要时创建/更新）字体对应的图集纹理；color 选择彩色图集。
func (r *Renderer) fontTexture(f *text.Font, color bool) *Texture {
	if f == nil {
		return nil
	}
	fa, ok := r.fonts[f]
	if !ok {
		fa = &fontAtlas{}
		r.fonts[f] = fa
	}
	if color {
		reset := f.TakeColorReset()
		if reset || fa.color == nil {
			if fa.color != nil {
				r.DeleteTexture(fa.color)
				fa.color = nil
			}
			dim := f.AtlasColorDimNow()
			if dim <= 0 || len(f.AtlasColor) < dim*dim*4 {
				f.TakeColorDirty()
				return nil
			}
			fa.color = r.NewTextureRGBA(dim, dim, f.AtlasColor)
			f.TakeColorDirty()
			return fa.color
		}
		if x0, y0, x1, y1, dirty := f.TakeColorDirty(); dirty {
			r.uploadColorRegion(f, fa.color, x0, y0, x1, y1)
		}
		return fa.color
	}
	// 单色字形：R8 纹理（1 字节/像素）
	reset := f.TakeReset()
	if reset || fa.mono == nil {
		if fa.mono != nil {
			r.DeleteTexture(fa.mono)
			fa.mono = nil
		}
		dim := f.AtlasDimNow()
		if len(f.Atlas) < dim*dim {
			f.TakeDirty()
			return nil
		}
		fa.mono = r.NewTextureR8(dim, dim, f.Atlas)
		f.TakeDirty()
		return fa.mono
	}
	if x0, y0, x1, y1, dirty := f.TakeDirty(); dirty {
		r.uploadAtlasRegion(f, fa.mono, x0, y0, x1, y1)
	}
	return fa.mono
}

// uploadAtlasRegion 只上传图集的脏矩形区域。
// 以前每光栅化一个新字形就把 2048²（16MB）的整张图集重传，滚动时新文字不断出现
// 会连续触发全量上传，是明显卡顿的主因；现在按行拼接后一次 glTexSubImage2D 提交。
func (r *Renderer) uploadAtlasRegion(f *text.Font, t *Texture, x0, y0, x1, y1 int) {
	dim := f.AtlasDimNow()
	if t == nil || t.ID == 0 || dim <= 0 || len(f.Atlas) < dim*dim {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > dim {
		x1 = dim
	}
	if y1 > dim {
		y1 = dim
	}
	w, rows := x1-x0, y1-y0
	if w <= 0 || rows <= 0 {
		return
	}

	glBindTexture(GL_TEXTURE_2D, t.ID)
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)

	// 整行范围：内存连续，直接上传
	if x0 == 0 && x1 == dim {
		off := y0 * dim
		glTexSubImage2D(GL_TEXTURE_2D, 0, 0, int32(y0), int32(dim), int32(rows), GL_RED, GL_UNSIGNED_BYTE, f.Atlas[off:])
		return
	}

	n := rows * w
	if cap(r.atlasBuf) < n {
		r.atlasBuf = make([]byte, n)
	}
	buf := r.atlasBuf[:n]
	for i := 0; i < rows; i++ {
		src := (y0+i)*dim + x0
		copy(buf[i*w:(i+1)*w], f.Atlas[src:src+w])
	}
	glTexSubImage2D(GL_TEXTURE_2D, 0, int32(x0), int32(y0), int32(w), int32(rows), GL_RED, GL_UNSIGNED_BYTE, buf)
}

// uploadColorRegion 只上传彩色图集的脏矩形（RGBA8，行跨度 dim*4）。
func (r *Renderer) uploadColorRegion(f *text.Font, t *Texture, x0, y0, x1, y1 int) {
	dim := f.AtlasColorDimNow()
	if t == nil || t.ID == 0 || dim <= 0 || len(f.AtlasColor) < dim*dim*4 {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > dim {
		x1 = dim
	}
	if y1 > dim {
		y1 = dim
	}
	w, rows := x1-x0, y1-y0
	if w <= 0 || rows <= 0 {
		return
	}
	glBindTexture(GL_TEXTURE_2D, t.ID)
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)
	if x0 == 0 && x1 == dim {
		off := y0 * dim * 4
		glTexSubImage2D(GL_TEXTURE_2D, 0, 0, int32(y0), int32(dim), int32(rows), GL_RGBA, GL_UNSIGNED_BYTE, f.AtlasColor[off:])
		return
	}
	n := rows * w * 4
	buf := make([]byte, n)
	stride, rowBytes := dim*4, w*4
	for i := 0; i < rows; i++ {
		src := (y0+i)*stride + x0*4
		copy(buf[i*rowBytes:(i+1)*rowBytes], f.AtlasColor[src:src+rowBytes])
	}
	glTexSubImage2D(GL_TEXTURE_2D, 0, int32(x0), int32(y0), int32(w), int32(rows), GL_RGBA, GL_UNSIGNED_BYTE, buf)
}

// UpdateTextureRGBA 局部更新 RGBA 纹理。
func (r *Renderer) UpdateTextureRGBA(t *Texture, x, y, w, h int, data []byte) {
	if t == nil || len(data) == 0 {
		return
	}
	glBindTexture(GL_TEXTURE_2D, t.ID)
	r.curTexID = 0
	glPixelStorei(GL_UNPACK_ALIGNMENT, 1)
	glTexSubImage2D(GL_TEXTURE_2D, 0, int32(x), int32(y), int32(w), int32(h), GL_RGBA, GL_UNSIGNED_BYTE, data)
}

// DeleteTexture 释放一张纹理（画面尺寸变化、片源切换时调用）。
func (r *Renderer) DeleteTexture(t *Texture) {
	if t == nil || t.ID == 0 {
		return
	}
	id := t.ID
	glDeleteTextures(1, &id)
	t.ID = 0
	// 删除后单元 0 上可能仍绑着旧名字，缓存置无效
	r.curTexID = 0
}

// ReleaseFont 释放字体对应的图集纹理（字体实例被缓存回收时调用）。
func (r *Renderer) ReleaseFont(f *text.Font) {
	if f == nil {
		return
	}
	if fa, ok := r.fonts[f]; ok {
		if fa.mono != nil {
			r.DeleteTexture(fa.mono)
		}
		if fa.color != nil {
			r.DeleteTexture(fa.color)
		}
		delete(r.fonts, f)
	}
}

func (r *Renderer) setClip(c geom.Rect) {
	// 同一矩形重复设置（嵌套滚动容器常见）不打断批次
	if r.clipOn && r.clipCur == c {
		return
	}
	r.flush()
	s := r.scale
	x := int32(c.X * s)
	y := int32(float32(r.ph) - c.Bottom()*s)
	w := int32(c.W * s)
	h := int32(c.H * s)
	glEnable(GL_SCISSOR_TEST)
	glScissor(x, y, w, h)
	r.clipOn, r.clipCur = true, c
}

func (r *Renderer) clearClip() {
	if !r.clipOn {
		return
	}
	r.flush()
	glDisable(GL_SCISSOR_TEST)
	r.clipOn = false
}

// Rect 立即绘制一个 Shape。
func (r *Renderer) Rect(sh Shape) { r.pushShape(sh) }

// TextRun 立即绘制一段文字。
func (r *Renderer) TextRun(t TextRun) { r.pushText(t) }

func (r *Renderer) pushShape(sh Shape) {
	rs := sh.Rect
	if rs.W <= 0 || rs.H <= 0 {
		return
	}
	r.ensureKind(batchShape, nil)
	s := r.scale

	if sh.ShadowColor.A > 0 {
		sp := sh.ShadowSpread
		blur := sh.ShadowBlur
		if blur <= 0 {
			blur = 1
		}
		base := rs.Offset(sh.ShadowOffsetX, sh.ShadowOffsetY)
		outer := geom.Rect{base.X - sp - blur, base.Y - sp - blur, base.W + (sp+blur)*2, base.H + (sp+blur)*2}
		var rad [4]float32
		for i := 0; i < 4; i++ {
			rad[i] = sh.Radius[i] + sp
		}
		r.appendQuad(outer, rad, Paint{}, Paint{A: -1}, blur*1.6, 2, 0, 0)
	}

	if sh.Grad.On {
		r.appendQuad(rs, sh.Radius, sh.Grad.From, sh.Grad.To, 0.7, 0, 0, sh.Grad.Angle)
	} else if sh.Fill.A > 0 {
		r.appendQuad(rs, sh.Radius, sh.Fill, Paint{A: -1}, 0.7, 0, 0, 0)
	}

	if sh.Stroke > 0 && sh.StrokeColor.A > 0 {
		r.appendQuad(rs.InsetAll(sh.Stroke*0.5), sh.Radius, sh.StrokeColor, Paint{A: -1}, 0.7, 1, sh.Stroke, 0)
	}
	_ = s
}

// growFloats 把顶点切片扩展到 n 个 float，容量不足时按 1.5~2 倍扩容。
//
// 不能直接写 buf[:n]：切片表达式只能在上限不超过 cap(buf) 时使用，超出会直接 panic
// （长文本页曾出现 slice bounds out of range [:55350] with capacity 55296 —— 初始容量
// 只按典型一屏预留，而单个文本批次超过 1024 个字形就把初始容量撑破了）。
// 这里的扩容上限仍由调用方的 maxVerts 批次切分保证。
func growFloats(buf []float32, n int) []float32 {
	if n <= cap(buf) {
		return buf[:n]
	}
	nc := cap(buf) * 2
	if nc < n {
		nc = n
	}
	nb := make([]float32, n, nc)
	copy(nb, buf)
	return nb
}

// appendQuad 追加一个矩形（6 顶点）。渐变方向（CSS 角度：0=向上 / 90=向右 /
// 180=向下）通过把两色混合结果烘焙到四个角的顶点色实现，由 GPU 在三角形内插值，
// 不需要新增顶点属性，着色器保持原样（用不到两色 mix 时把 vColor2.A 置 -1）。
func (r *Renderer) appendQuad(rs geom.Rect, rad [4]float32, c1, c2 Paint, feather, mode, stroke, angle float32) {
	// 填充/描边的 SDF 边缘过渡带宽度按「设备像素」恒定：超采样时画布分辨率是 ss 倍，
	// 若仍按逻辑像素给 0.7，过渡带在设备空间就变成 0.7*ss 像素，降采样后边缘反而更软，
	// 白白吃掉超采样带来的锐度。这里除以 ss，让 1.25×/1.5×/2× 的边缘都比 1:1 更干净。
	if r.ssaa > 1.01 && mode < 1.5 && feather > 0 {
		feather /= r.ssaa
	}
	if r.shapeN+6 > maxVerts {
		r.flush()
		r.cur = batchShape
	}
	hx, hy := rs.W/2, rs.H/2
	cx, cy := rs.X+hx, rs.Y+hy
	s := r.scale

	grad := c2.A >= 0
	var dx, dy, den float32
	if grad {
		a := float64(angle) * math.Pi / 180
		dx, dy = float32(math.Sin(a)), -float32(math.Cos(a))
		den = float32(math.Abs(float64(dx)))*hx + float32(math.Abs(float64(dy)))*hy
		if den <= 0 {
			grad = false
		}
	}
	corner := func(lx, ly float32) Paint {
		if !grad {
			return c1
		}
		t := (lx*dx+ly*dy)/den*0.5 + 0.5
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		return Paint{
			R: c1.R + (c2.R-c1.R)*t,
			G: c1.G + (c2.G-c1.G)*t,
			B: c1.B + (c2.B-c1.B)*t,
			A: c1.A + (c2.A-c1.A)*t,
		}
	}
	csx := [6]float32{-1, 1, 1, -1, 1, -1}
	csy := [6]float32{-1, -1, 1, -1, 1, 1}
	// 一次扩容直写偏移，避免每个顶点都走一遍 append 的 len/cap 检查与扩容分支
	n := len(r.shapeData)
	r.shapeData = growFloats(r.shapeData, n+6*floatsShapeVert)
	d := r.shapeData[n:]
	for k := 0; k < 6; k++ {
		lx, ly := csx[k]*hx, csy[k]*hy
		v := corner(lx, ly)
		o := k * floatsShapeVert
		d[o+0], d[o+1] = cx+lx, cy+ly
		d[o+2], d[o+3] = lx, ly
		d[o+4], d[o+5], d[o+6], d[o+7] = v.R, v.G, v.B, v.A
		// 顶点色 2 的 alpha 固定 -1：告诉着色器“没有第二颜色”，不走两色 mix
		d[o+8], d[o+9], d[o+10], d[o+11] = 0, 0, 0, -1
		d[o+12], d[o+13] = hx, hy
		d[o+14], d[o+15], d[o+16], d[o+17] = rad[0], rad[1], rad[2], rad[3]
		d[o+18], d[o+19], d[o+20], d[o+21] = feather, mode, stroke, 0
	}
	r.shapeN += 6
	_ = s
}

func (r *Renderer) pushText(t TextRun) {
	f := t.Font
	if f == nil || t.Text == "" {
		return
	}

	sc := f.Scale()
	penX := t.X * sc
	baseY := t.Y*sc + f.Ascent*sc
	c := t.Color
	// 字形序列按字符串缓存（见 text.Font.GlyphsFor）：
	// 旧实现对每个字符调 GlyphFor，一次 RWMutex + 一次 map 哈希，
	// 一屏 2000 字就是每帧几千次加锁，文本页滚动时是纯浪费。
	glyphs := f.GlyphsFor(t.Text)
	inv := float32(1)
	if sc != 0 {
		inv = 1 / sc
	}

	for i := range glyphs {
		g := &glyphs[i]
		if !g.Valid {
			penX += g.Advance
			continue
		}
		// 一个批次只能绑一张图集：Emoji（彩色图集）与普通字形（R8 图集）
		// 交替出现时会自动切批，不再需要把彩色像素塞进单色图集。
		r.ensureText(f, g.Color)
		x0 := float32(math.Round(float64(penX + g.Left)))
		y0 := float32(math.Round(float64(baseY + g.Top)))
		x1 := x0 + g.W
		y1 := y0 + g.H

		if r.textN+6 > maxVerts {
			r.flush()
			r.cur = batchText
			r.curFont = f
			r.curFontColor = g.Color
		}
		mode := float32(0)
		if g.Color {
			mode = 1
		}
		// 顶点一次扩容直写（每个顶点 9 个 float）
		n := len(r.textData)
		r.textData = growFloats(r.textData, n+6*floatsTextVert)
		d := r.textData[n:]
		vxs := [6]float32{x0, x1, x1, x0, x1, x0}
		vys := [6]float32{y0, y0, y1, y0, y1, y1}
		us := [6]float32{g.U0, g.U1, g.U1, g.U0, g.U1, g.U0}
		vs := [6]float32{g.V1, g.V1, g.V0, g.V1, g.V0, g.V0}
		for k := 0; k < 6; k++ {
			o := k * floatsTextVert
			d[o+0], d[o+1] = vxs[k]*inv, vys[k]*inv
			d[o+2], d[o+3] = us[k], vs[k]
			d[o+4], d[o+5], d[o+6], d[o+7] = c.R, c.G, c.B, c.A
			d[o+8] = mode
		}
		r.textN += 6
		penX += g.Advance
	}
}

// Draw 实现 Backend 接口。
func (r *Renderer) Draw(instrs []Instr) {
	for _, in := range instrs {
		switch in.Kind {
		case InstrShape:
			r.pushShape(in.Shape)
		case InstrText:
			r.pushText(in.Text)
		case InstrClipPush:
			r.clipSaved = append(r.clipSaved, in.Clip)
			r.setClip(in.Clip)
		case InstrClipPop:
			if n := len(r.clipSaved); n > 0 {
				r.clipSaved = r.clipSaved[:n-1]
			}
			if n := len(r.clipSaved); n > 0 {
				r.setClip(r.clipSaved[n-1])
			} else {
				r.clearClip()
			}
		}
	}
}
