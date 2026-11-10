// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package text 提供字形光栅化与测量。
//
// 光栅化方式参考 gio：不使用 GDI 的提示化位图，而是用 golang.org/x/image/font/opentype
// 直接把 TrueType 轮廓按目标像素尺寸做覆盖率抗锯齿（HintingNone），
// 笔画粗细均匀、边缘平滑，观感与浏览器（DirectWrite 灰度抗锯齿）一致。
//
// 图集为 RGBA8：普通字形写入 (v,v,v,255)，彩色 Emoji 写入真实颜色，
// 渲染时由着色器按 mode 区分（见 gpu/pushText）。
package text

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Glyph 字形在图集中的位置（位图尺寸/偏移均为物理像素）。
type Glyph struct {
	U0, V0, U1, V1 float32
	W, H           float32
	Left, Top      float32
	Advance        float32
	Valid          bool
	// Color 为 true 时图集里存的是彩色像素（Emoji），渲染时不做颜色乘算。
	Color bool
}

// Font 基于 opentype 矢量光栅化的字体。
type Font struct {
	Face   string
	Size   float32 // 逻辑字号
	Weight int32

	scale float32

	// gen 字体代次（进程内唯一，创建时分配）。
	// 字体被回收后指针地址可能被新字体复用，折行缓存用「代次」而不是指针区分，
	// 否则会命中上一代字体留下的行宽结果（宽度与字形完全不同）。
	gen atomic.Uint32
	// advASCII ASCII 字符的推进量快照（逻辑像素，0 = 未测量）。
	// 折行每字符都要取一次推进量，旧实现走 GlyphFor（读锁 + map）；
	// 代码/英文/数字占比高的页面靠这张无锁表直接省掉加锁与哈希。
	advASCII atomic.Pointer[[128]float32]
	// advSnap 非 ASCII 推进量的不可变快照（读路径无锁）；advBuild 为写侧累积表，
	// 每攒够一批新字符再整体复制成新快照，避免「每个新字都复制一次 map」。
	advSnap  atomic.Pointer[map[rune]float32]
	advMu    sync.Mutex
	advBuild map[rune]float32
	advDirty int

	Ascent, Descent, LineHeight float32 // 逻辑像素

	mu    sync.RWMutex
	cache map[glyphKey]*Glyph
	// measure 行宽缓存（逻辑像素）：UI 每帧都会重复测量同一批字符串，
	// 逐字查字形 + 反复加锁在长列表/富文本页会吃掉大量时间，命中后只剩一次哈希查找。
	measure map[string]float32
	// runs 字符串 → 字形序列缓存（绘制热路径，见 GlyphsFor）。
	// 图集重建 / 缩放变化 / 写满重用都会使 UV 失效，因此这些地方必须一并清空。
	runs map[string][]Glyph
	// runGlyphs 缓存里的字形总数（内存预算，防止多字体时膨胀）
	runGlyphs int
	// runBytes / measureBytes 两张文本缓存的**字节**记账（估算值）。
	// 只按条数限流时，长字符串界面（聊天/日志）能在条数远未达标的情况下
	// 吃掉几 MB，因此这里按字节预算淘汰（见 measureByteBudget）。
	runBytes     int
	measureBytes int
	// atlasGen 图集版本号：扩容 / 重建 / 写满重用都会自增。
	// 构建字形序列时若中间发生扩容，已取得的 UV 就失效了（见 GlyphsFor 的重试）。
	atlasGen uint64
	face     font.Face
	// fallback 主字体缺字形时的后备字体（Symbol / Emoji 单色轮廓）
	fallback     font.Face
	fallbackName string
	// cjk / cjkTried 主字体与符号后备都缺字形时的中文后备（懒加载，见 rasterizeCJK）。
	// 某些机型的「粗体」文件只有拉丁字形（如 Roboto-Bold），选中后汉字会变成
	// 只有宽度没有笔画；这里兜一层系统默认字体，保证汉字永远画得出来。
	cjk      font.Face
	cjkTried bool
	// emoji 彩色光栅器（GDI，懒创建）
	emoji *gdiFont

	// 图集（单色字形：每像素 1 字节覆盖率，由渲染器上传为 R8 纹理）。
	// 上传采用「脏矩形」策略：只把本次新光栅化的字形区域传给 GL，
	// 而不是每新增一个字形就把整张图集重传一遍。
	//
	// 为什么不直接存 RGBA8：普通字形只需要一字节覆盖率，Emoji 才有颜色。
	// 混在一张 RGBA8 图集里等于白占 4 倍内存（24 个字体实例实测 8MB）；
	// 现在单色走 R8、彩色单独一张（见 AtlasColor），只在真遇到 Emoji 时才分配。
	Atlas     []byte
	AtlasSize int // 当前图集边长（0 = 尚未分配）
	// AtlasColor 彩色字形（Emoji）图集（RGBA8）。
	AtlasColor []byte
	ColorSize  int // 彩色图集边长（0 = 尚未分配）
	ColorDirty bool
	// ColorReset 彩色图集被整体重建/清空/扩容，渲染器需重建对应纹理。
	ColorReset     bool
	colorDirtyX0   int
	colorDirtyY0   int
	colorDirtyX1   int
	colorDirtyY1   int
	colorGridX     int
	colorGridY     int
	colorRowHeight int
	// AtlasDirty 与旧版兼容：有任何未上传区域时为 true。
	AtlasDirty bool
	// AtlasReset 图集被整体重建/清空/扩容（SetScale、写满），渲染器需重建纹理。
	AtlasReset bool
	// 脏矩形（纹理坐标：x 向右、y 自底向上，x1/y1 为开区间）
	dirtyX0, dirtyY0, dirtyX1, dirtyY1 int
	gridX                              int
	gridY                              int
	rowHeight                          int
}

type glyphKey struct {
	r  rune
	px int32
}

// 字形图集尺寸：从 minAtlasDim 起步，装不下时逐级翻倍，最大 maxAtlasDim。
// 单字体图集 = dim² × 4 字节（256→256KB、512→1MB、1024→4MB）。
// 以前每个字体一律预分配 2048²（16MB），一个页面用到十几种字号/字重就直接吃掉上百 MB。
const (
	minAtlasDim = 256
	maxAtlasDim = 1024
	// maxColorAtlasDim 彩色（Emoji）图集上限：RGBA8 512² = 1MB。
	maxColorAtlasDim = 512
	// AtlasDim 向后兼容：图集最大边长。
	AtlasDim = maxAtlasDim
)

// glyphPad 字形四周留白（纹理像素），供双线性采样使用，避免渗入相邻字形。
const glyphPad = 1

// fontSeq 字体代次计数器（进程内唯一，见 Font.gen）。
var fontSeq atomic.Uint32

// NewFont 创建字体（纯 Go 光栅化 + GDI Emoji 彩色）。
func NewFont(faceName string, size float32, weight int32, scale float32) *Font {
	if size <= 0 {
		size = 14
	}
	if scale <= 0 {
		scale = 1
	}
	if weight == 0 {
		weight = 400
	}
	f := &Font{
		Face:      faceName,
		Size:      size,
		Weight:    weight,
		scale:     scale,
		cache:     make(map[glyphKey]*Glyph, 512),
		measure:   make(map[string]float32, 256),
		runs:      make(map[string][]Glyph, 64),
		AtlasSize: 0, // 懒分配：真正光栅化时才建图集
	}
	f.buildFace()
	return f
}

// Gen 返回字体代次（进程内唯一；字体对象被回收后地址可能复用，
// 折行缓存必须用代次而不是指针做键的一部分）。
func (f *Font) Gen() uint32 { return f.gen.Load() }

// Advance 返回字符的推进量（逻辑像素，已除缩放）。
//
// 折行热路径对每个字符都要取一次推进量，旧实现每次走 GlyphFor（读锁 + map）：
// 一段 500 字的段落就是 500 次加锁 + 哈希。这里 ASCII 走无锁数组、
// 非 ASCII 走不可变快照 map（读路径完全不加锁），只有遇到新字符才回到
// GlyphFor 光栅化并重建一次快照。
func (f *Font) Advance(r rune) float32 {
	if r < 128 {
		if a := f.advASCII.Load(); a != nil {
			if v := a[r]; v != 0 {
				return v
			}
		}
	} else if snap := f.advSnap.Load(); snap != nil {
		if v, ok := (*snap)[r]; ok {
			return v
		}
	}
	sc := f.scale
	if sc <= 0 {
		sc = 1
	}
	v := f.GlyphFor(r).Advance / sc

	f.advMu.Lock()
	if f.advBuild == nil {
		f.advBuild = make(map[rune]float32, 256)
	}
	if old, ok := f.advBuild[r]; !ok || old != v {
		f.advBuild[r] = v
		f.advDirty++
	}
	if r < 128 {
		cur := f.advASCII.Load()
		na := new([128]float32)
		if cur != nil {
			*na = *cur
		}
		na[r] = v
		f.advASCII.Store(na)
	} else if f.advDirty >= 64 || f.advSnap.Load() == nil {
		// 攒一批再整体快照：每个新汉字都复制整表的话，冷启动会 O(n²)。
		cp := make(map[rune]float32, len(f.advBuild))
		for k, val := range f.advBuild {
			cp[k] = val
		}
		f.advSnap.Store(&cp)
		f.advDirty = 0
	}
	f.advMu.Unlock()
	return v
}

// resetAdvance 清空推进量缓存（face 重建 / 缩放变化 / 文字锐度切换后调用）。
func (f *Font) resetAdvance() {
	f.advASCII.Store(nil)
	f.advMu.Lock()
	f.advBuild = nil
	f.advDirty = 0
	f.advSnap.Store(nil)
	f.advMu.Unlock()
}

// atlasDim 返回当前图集边长（未分配时为最小尺寸）。
func (f *Font) atlasDim() int {
	if f.AtlasSize <= 0 {
		return minAtlasDim
	}
	return f.AtlasSize
}

// AtlasDimNow 返回当前图集边长（供渲染器创建/更新纹理）。
func (f *Font) AtlasDimNow() int { return f.atlasDim() }

// growAtlas 扩容并清空图集（已有字形需重新光栅化，UV 会随之重算）。
// 调用方需持有 f.mu。
func (f *Font) growAtlas(dim int) {
	f.AtlasSize = dim
	f.Atlas = make([]byte, dim*dim)
	f.cache = make(map[glyphKey]*Glyph, 1024)
	f.dropRunsLocked()
	f.gridX, f.gridY, f.rowHeight = 0, 0, 0
	f.AtlasDirty = false
	f.AtlasReset = true
	f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = 0, 0, 0, 0
	dropColorAtlas(f)
}

// dropColorAtlas 释放彩色图集（单色图集重建时旧的彩色 UV 同样失效）。
func dropColorAtlas(f *Font) {
	f.AtlasColor = nil
	f.ColorSize = 0
	f.ColorDirty = false
	f.ColorReset = true
	f.colorDirtyX0, f.colorDirtyY0, f.colorDirtyX1, f.colorDirtyY1 = 0, 0, 0, 0
	f.colorGridX, f.colorGridY, f.colorRowHeight = 0, 0, 0
}

// warnCJKOnce 只提示一次：字体加载路径每帧都可能触发，日志不能刷屏。
var warnCJKOnce sync.Once

// warnMissingCJK 主字体不含中日韩字形、且中文后备字体也拿不到时提示一次。
//
// 典型场景：容器 / WSL 的 Ubuntu 默认不装 CJK 字体，表现为「英文正常、中文全空白」。
// 仅在**中文真的会显示空白**时提示：
//   - 显式选了 Consolas / Segoe UI 这类拉丁字体是业务的本意，中文由后备字体渲染，
//     不应该在启动日志里吓人（以前就是这么误报的）；
//   - 只要 cjkFontFace() 能拿到中文后备字体，中文就不会空白。
func warnMissingCJK(face string, sf *sfnt.Font) {
	fam := ""
	if sf != nil {
		var buf sfnt.Buffer
		if n, err := sf.Name(&buf, sfnt.NameIDFamily); err == nil {
			fam = n
		}
	}
	warnCJKOnce.Do(func() {
		fmt.Fprintf(os.Stderr,
			"[guis] 警告：界面主字体（%s）缺少中日韩字形，也未找到中文后备字体，中文会显示为空白。\n"+
				"  修复：安装中文字体（Debian/Ubuntu: apt install fonts-noto-cjk），\n"+
				"        或用环境变量指定字体文件：GUIS_FONT_FILE=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc\n",
			fam)
	})
}

// fontHasCJK 字体是否含「中」的字形（判断主字体能否直接渲染中文）。
func fontHasCJK(sf *sfnt.Font) bool {
	if sf == nil {
		return false
	}
	var buf sfnt.Buffer
	idx, err := sf.GlyphIndex(&buf, '中')
	return err == nil && idx != 0
}

// isPrimaryUIFamily 是否是「界面主字体」家族（空串 = 平台默认）。
//
// 显式指定 Consolas / Segoe UI 这类拉丁字体不算主字体，缺 CJK 属于正常用法。
func isPrimaryUIFamily(face string) bool {
	if strings.TrimSpace(face) == "" {
		return true
	}
	return containsAny(face, "雅黑", "YaHei", "Noto Sans CJK", "Noto Sans SC", "PingFang",
		"Source Han", "SimSun", "宋体", "Microsoft JhengHei", "微軟正黑", "微软正黑")
}

// ---- 文字锐度档位 ----
//
// 档位影响两件事：
//  1. 字形光栅化是否做 hinting（TrueType 轮廓按像素网格对齐笔画）——
//     小字号中文（13~18px）在 HintingNone 下横竖笔被覆盖率抗锯齿抹平，
//     落到整数像素网格后明显更「锐」；
//  2. 覆盖率 gamma（渲染器按同一档位取值，见 gpu.defaultTextGamma）。
//
// 图集里的字形按档位光栅化，切档必须重建 face 并清空图集（见 Font.RebuildSharp）。
const (
	SharpStandard = 0 // 标准：不做 hinting，笔画最平滑（旧行为）
	SharpClear    = 1 // 清晰：小字号全 hinting，笔画对齐像素
	SharpMax      = 2 // 极致：连大字号也 hinting，配合更实的覆盖率
)

// sharpLevel 当前文字锐度档位（0/1/2）。
var sharpLevel atomic.Int32

// sharpHintingMaxPx 清晰档超过该物理字号就不再 hinting：
// 大字号轮廓本身清晰，hinting 反而让同一行里的笔画宽度跳变。
const sharpHintingMaxPx = 32

// SetSharp 设置文字锐度档位（夹取到 0~2）。已在使用的字体需自行 RebuildSharp。
func SetSharp(level int) {
	if level < SharpStandard {
		level = SharpStandard
	}
	if level > SharpMax {
		level = SharpMax
	}
	sharpLevel.Store(int32(level))
}

// Sharp 当前文字锐度档位。
func Sharp() int { return int(sharpLevel.Load()) }

// hintingFor 按物理字号与当前档位选择 hinting 模式。
//
// 只有 TrueType（glyf）轮廓会真正生效；CFF/OTF 字形自动退化为无 hinting，无副作用。
func hintingFor(px float32) font.Hinting {
	if Sharp() == SharpStandard || px <= 0 {
		return font.HintingNone
	}
	if px > sharpHintingMaxPx && Sharp() < SharpMax {
		return font.HintingNone
	}
	return font.HintingFull
}

// buildFace 按当前字号与 DPI 重建 opentype face。
func (f *Font) buildFace() {
	// face 变了（字号 / 缩放 / 锐度 / 家族）：按字符缓存的推进量全部失效，
	// 同时换一个代次，让以代次为键的折行缓存自动失效（不必清全局缓存）。
	f.resetAdvance()
	f.gen.Store(fontSeq.Add(1))
	// 字号 / 缩放变了，中文后备的 face 也要按新的像素尺寸重建
	f.cjk, f.cjkTried = nil, false
	px := f.Size * f.scale
	if px < 1 {
		px = 1
	}
	if sf, err := loadSystemFont(f.Face, f.Weight); err == nil {
		if os.Getenv("GUIS_FONT_DEBUG") != "" {
			var dbgBuf sfnt.Buffer
			fam, _ := sf.Name(&dbgBuf, sfnt.NameIDFamily)
			idx, _ := sf.GlyphIndex(&dbgBuf, '中')
			fmt.Fprintf(os.Stderr, "[dbg] buildFace face=%q resolved=%q cjkIdx=%d\n", f.Face, fam, idx)
		}
		// 只在「主字体缺 CJK 且中文后备也拿不到」时提示，避免为 Consolas / Segoe UI
		// 这类拉丁字体误报「中文会显示为空白」。
		if isPrimaryUIFamily(f.Face) && !fontHasCJK(sf) && f.cjkFontFace() == nil {
			warnMissingCJK(f.Face, sf)
		}
		if face, err := opentype.NewFace(sf, &opentype.FaceOptions{
			Size:    float64(px),
			DPI:     72,
			Hinting: hintingFor(px),
		}); err == nil {
			f.face = face
			m := face.Metrics()
			f.Ascent = float32(m.Ascent) / 64 / f.scale
			f.Descent = float32(m.Descent) / 64 / f.scale
			f.LineHeight = float32(m.Height) / 64 / f.scale
		}
	}
	if f.face == nil {
		f.face = nil
		f.Ascent = f.Size * 0.8
		f.Descent = f.Size * 0.22
		f.LineHeight = f.Size * 1.4
	}

	// 后备字体：平台符号字体（覆盖符号/箭头/部分 emoji 轮廓）。
	// Windows 是 Segoe UI Symbol，Linux 是 DejaVu Sans，macOS 是 Apple Symbols——
	// 以前硬编码成 "Segoe UI Symbol"，非 Windows 平台永远加载失败，后备字体形同虚设。
	if f.fallbackName == "" {
		f.fallbackName = platformSymbolFamily()
	}
	if f.fallbackName != "" {
		if sf, err := loadSystemFont(f.fallbackName, f.Weight); err == nil {
			if face, err := opentype.NewFace(sf, &opentype.FaceOptions{
				Size:    float64(px),
				DPI:     72,
				Hinting: hintingFor(px),
			}); err == nil {
				f.fallback = face
			}
		}
	}
	if f.emoji != nil {
		f.emoji.ensureFont(int32(px + 0.5))
	}
}

// SetScale 在 DPI 变化时重建（图集清空）。
func (f *Font) SetScale(scale float32) {
	if scale <= 0 || scale == f.scale {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scale = scale
	f.buildFace()
	f.cache = make(map[glyphKey]*Glyph, 512)
	// 字号变了，行宽缓存与字形序列全部失效
	f.measure = make(map[string]float32, 256)
	f.measureBytes = 0
	f.dropRunsLocked()
	// 释放旧图集（懒分配：下次光栅化时按最小尺寸重建），并标记纹理需重建
	f.Atlas = nil
	f.AtlasSize = 0
	f.AtlasDirty = false
	f.AtlasReset = true
	f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = 0, 0, 0, 0
	f.gridX, f.gridY, f.rowHeight = 0, 0, 0
	dropColorAtlas(f)
	f.atlasGen++
}

// RebuildSharp 文字锐度档位变化后重建 face 并清空字形缓存与图集。
//
// 与 SetScale 的区别：字号不变，但字形要按新的 hinting 重新光栅化，
// 旧的覆盖率位图与 UV 全部作废（文本量缓存也随之失效）。
func (f *Font) RebuildSharp() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buildFace()
	f.cache = make(map[glyphKey]*Glyph, 512)
	f.measure = make(map[string]float32, 256)
	f.measureBytes = 0
	f.dropRunsLocked()
	// 释放旧图集（懒分配：下次光栅化时按最小尺寸重建），并标记纹理需重建
	f.Atlas = nil
	f.AtlasSize = 0
	f.AtlasDirty = false
	f.AtlasReset = true
	f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = 0, 0, 0, 0
	f.gridX, f.gridY, f.rowHeight = 0, 0, 0
	dropColorAtlas(f)
	f.atlasGen++
}

// AtlasBytes 返回当前字形图集占用的字节数（懒分配，未使用时为 0）。
func (f *Font) AtlasBytes() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.Atlas) + len(f.AtlasColor)
}

// AtlasColorDimNow 返回彩色图集边长（0 = 尚未分配）。
func (f *Font) AtlasColorDimNow() int { return f.ColorSize }

// markDirty 记录一个刚写入图集的字形矩形（纹理坐标，y 自底向上）。
// 调用方需持有 f.mu。
func (f *Font) markDirty(ox, oy, bw, bh int) {
	dim := f.atlasDim()
	y0 := dim - 1 - (oy + bh - 1)
	y1 := dim - oy
	x0, x1 := ox, ox+bw
	if y0 < 0 {
		y0 = 0
	}
	if y1 > dim {
		y1 = dim
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 > dim {
		x1 = dim
	}
	if y0 >= y1 || x0 >= x1 {
		return
	}
	if !f.AtlasDirty {
		f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = x0, y0, x1, y1
		f.AtlasDirty = true
		return
	}
	if x0 < f.dirtyX0 {
		f.dirtyX0 = x0
	}
	if y0 < f.dirtyY0 {
		f.dirtyY0 = y0
	}
	if x1 > f.dirtyX1 {
		f.dirtyX1 = x1
	}
	if y1 > f.dirtyY1 {
		f.dirtyY1 = y1
	}
}

// TakeDirty 取出并清空脏矩形（供渲染器做局部 glTexSubImage2D）。
func (f *Font) TakeDirty() (x0, y0, x1, y1 int, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.AtlasDirty {
		return 0, 0, 0, 0, false
	}
	x0, y0, x1, y1 = f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1
	f.AtlasDirty = false
	f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = 0, 0, 0, 0
	return x0, y0, x1, y1, true
}

// TakeReset 取出并清空「图集已整体重建」标记。
func (f *Font) TakeReset() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.AtlasReset {
		return false
	}
	f.AtlasReset = false
	return true
}

// markColorDirty 记录彩色图集的脏矩形（与 markDirty 同构，尺寸用 ColorSize）。
// 调用方需持有 f.mu。
func (f *Font) markColorDirty(ox, oy, bw, bh int) {
	dim := f.ColorSize
	if dim <= 0 {
		return
	}
	y0 := dim - 1 - (oy + bh - 1)
	y1 := dim - oy
	x0, x1 := ox, ox+bw
	if y0 < 0 {
		y0 = 0
	}
	if y1 > dim {
		y1 = dim
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 > dim {
		x1 = dim
	}
	if y0 >= y1 || x0 >= x1 {
		return
	}
	if !f.ColorDirty {
		f.colorDirtyX0, f.colorDirtyY0, f.colorDirtyX1, f.colorDirtyY1 = x0, y0, x1, y1
		f.ColorDirty = true
		return
	}
	if x0 < f.colorDirtyX0 {
		f.colorDirtyX0 = x0
	}
	if y0 < f.colorDirtyY0 {
		f.colorDirtyY0 = y0
	}
	if x1 > f.colorDirtyX1 {
		f.colorDirtyX1 = x1
	}
	if y1 > f.colorDirtyY1 {
		f.colorDirtyY1 = y1
	}
}

// TakeColorDirty 取出并清空彩色图集的脏矩形。
func (f *Font) TakeColorDirty() (x0, y0, x1, y1 int, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ColorDirty {
		return 0, 0, 0, 0, false
	}
	x0, y0, x1, y1 = f.colorDirtyX0, f.colorDirtyY0, f.colorDirtyX1, f.colorDirtyY1
	f.ColorDirty = false
	f.colorDirtyX0, f.colorDirtyY0, f.colorDirtyX1, f.colorDirtyY1 = 0, 0, 0, 0
	return x0, y0, x1, y1, true
}

// TakeColorReset 取出并清空「彩色图集已重建」标记。
func (f *Font) TakeColorReset() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.ColorReset {
		return false
	}
	f.ColorReset = false
	return true
}

// GlyphFor 取（必要时光栅化）字形。
//
// 热路径：命中的字形只在读锁下查一次 map（旧实现每字符都抢一次互斥锁，
// 一屏文本每帧上千次加锁/解锁，是绘制阶段最贵的开销之一）。
// 未命中才升级为写锁光栅化，并在写锁内复查一次避免重复光栅化。
func (f *Font) GlyphFor(r rune) *Glyph {
	f.mu.RLock()
	px := int32(f.Size*f.scale + 0.5)
	if px < 1 {
		px = 1
	}
	key := glyphKey{r, px}
	if g, ok := f.cache[key]; ok {
		f.mu.RUnlock()
		return g
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	if g, ok := f.cache[key]; ok {
		return g
	}
	g := f.rasterize(r)
	f.cache[key] = g
	return g
}

func (f *Font) rasterize(r rune) *Glyph {
	// Emoji 优先走彩色位图路径
	if isEmojiRune(r) {
		if g, ok := f.rasterizeEmoji(r); ok {
			return g
		}
	}
	if g, ok := f.rasterizeFace(f.face, r); ok {
		return g
	}
	// 主字体缺字形：尝试后备字体（符号/箭头），再试系统默认字体（中日韩 /
	// 阿拉伯 / 西里尔 / 泰文等多语言字形），最后 Emoji 单色轮廓
	if g, ok := f.rasterizeFace(f.fallback, r); ok {
		return g
	}
	if g, ok := f.rasterizeSystem(r); ok {
		return g
	}
	if g, ok := f.rasterizeEmoji(r); ok {
		return g
	}
	g := &Glyph{}
	if unicode.IsSpace(r) {
		g.Advance = f.Size * f.scale * 0.5
	} else {
		g.Advance = f.Size * f.scale * 0.9
	}
	return g
}

// rasterizeSystem 主字体与符号后备都缺字形时，用系统默认字体补一层后备。
//
// 懒加载：只有真的遇到缺字形才解析字体，常规路径没有额外开销。
//
// 覆盖面：不只是中日韩——阿拉伯语（RTL）、西里尔、泰文、天城文等都会走到这里。
// 只有 ASCII（0x20~0x7e）不尝试：主字体连拉丁字母都没有时，系统字体通常也一样，
// 白白多加载一张 face。
func (f *Font) rasterizeSystem(r rune) (*Glyph, bool) {
	if r < 0x80 {
		return nil, false
	}
	return f.rasterizeFace(f.cjkFontFace(), r)
}

// cjkFontFace 懒加载中文后备字体（系统默认字族；Android 上是 Noto Sans CJK）。
func (f *Font) cjkFontFace() font.Face {
	if f.cjkTried {
		return f.cjk
	}
	f.cjkTried = true
	sf, err := loadSystemFont("", f.Weight)
	if err != nil || sf == nil {
		return nil
	}
	px := f.Size * f.scale
	if px < 1 {
		px = 1
	}
	face, err := opentype.NewFace(sf, &opentype.FaceOptions{
		Size:    float64(px),
		DPI:     72,
		Hinting: hintingFor(px),
	})
	if err != nil {
		return nil
	}
	f.cjk = face
	return f.cjk
}

// ---- 覆盖率锐化（文字锐度的实际来源）----
//
// go 的 TrueType hinter 只做推进量取整，对轮廓的像素对齐作用很弱（实测 14px 汉字的
// 实心/过渡像素数几乎不变），真正让笔画「更锐」的手段是把覆盖率往两端推：
// 抗锯齿产生的 30%~70% 灰边被压暗/提亮，过渡带变窄，而 0 与 255 保持不变
// （实心面积不膨胀，不会变粗）。用预计算 LUT，逐像素只剩一次查表。
var sharpLUT [3][256]uint8

func init() {
	ks := [3]float32{0, 0.55, 0.85} // 标准 / 清晰 / 极致 的 S 曲线混合系数
	for i := 0; i < 256; i++ {
		t := float32(i) / 255
		s := t * t * (3 - 2*t) // smoothstep：中灰不变，两端加速
		for level := 0; level < 3; level++ {
			out := t + (s-t)*ks[level]
			if out < 0 {
				out = 0
			} else if out > 1 {
				out = 1
			}
			sharpLUT[level][i] = uint8(out*255 + 0.5)
		}
	}
}

// sharpCoverage 按当前锐度档位重映射一个覆盖率采样值。
func sharpCoverage(level int, a uint8) uint8 {
	if level < 0 || level > 2 {
		return a
	}
	return sharpLUT[level][a]
}

// rasterizeFace 用 opentype face 光栅化灰度字形并写入 RGBA 图集。
func (f *Font) rasterizeFace(face font.Face, r rune) (*Glyph, bool) {
	if face == nil {
		return nil, false
	}
	dr, mask, maskp, advance, ok := face.Glyph(fixed.Point26_6{}, r)
	if !ok || dr.Empty() {
		return nil, false
	}
	bw, bh := dr.Dx(), dr.Dy()
	if bw <= 0 || bh <= 0 {
		return nil, false
	}
	ox, oy, ok := f.allocCell(bw, bh)
	if !ok {
		return nil, false
	}
	gray := color.GrayModel
	dim := f.AtlasSize
	sharp := Sharp()
	for y := 0; y < bh; y++ {
		// GL 纹理第 0 行对应 v=0（底部），位图首行是顶部，写入时垂直翻转
		dstRow := dim - 1 - (oy + y)
		if dstRow < 0 || dstRow >= dim {
			continue
		}
		dst := f.Atlas[dstRow*dim+ox:]
		srcY := maskp.Y + y
		for x := 0; x < bw; x++ {
			dst[x] = sharpCoverage(sharp, gray.Convert(mask.At(maskp.X+x, srcY)).(color.Gray).Y)
		}
	}
	f.markDirty(ox, oy, bw, bh)
	g := f.finishGlyph(dim, false, ox, oy, bw, bh, advance, dr.Min.X, dr.Min.Y)
	return g, true
}

// rasterizeEmoji 用 GDI 渲染彩色 Emoji 位图。
func (f *Font) rasterizeEmoji(r rune) (*Glyph, bool) {
	if f.emoji == nil {
		f.emoji = newGdiFont("Segoe UI Emoji")
	}
	px := int32(f.Size*f.scale + 0.5)
	if px < 8 {
		return nil, false
	}
	f.emoji.ensureFont(px)
	img, adv, ok := f.emoji.render(string(r), px)
	if !ok || img == nil {
		return nil, false
	}
	bw, bh := img.W, img.H
	ox, oy, ok2 := f.allocCellColor(bw, bh)
	if !ok2 {
		return nil, false
	}
	dim := f.ColorSize
	for y := 0; y < bh; y++ {
		dstRow := dim - 1 - (oy + y)
		if dstRow < 0 || dstRow >= dim {
			continue
		}
		dst := f.AtlasColor[dstRow*dim*4+ox*4:]
		src := img.Pix[y*img.W*4:]
		copy(dst[:bw*4], src[:bw*4])
	}
	f.markColorDirty(ox, oy, bw, bh)
	g := f.finishGlyph(dim, true, ox, oy, bw, bh, fixed.Int26_6(adv*64), img.Left, img.Top)
	g.Color = true
	return g, true
}

// allocCell 在图集中分配一块 bw×bh 区域（含 padding）。
// 图集从 256² 起步，装不下逐级翻倍到 1024²；到顶后写满才清空重用。
func (f *Font) allocCell(bw, bh int) (int, int, bool) {
	if f.Atlas == nil {
		f.growAtlas(minAtlasDim)
	}
	need := bw + glyphPad*2
	dim := f.AtlasSize
	if f.gridX+need+1 > dim {
		f.gridX = 0
		f.gridY += f.rowHeight + 1
		f.rowHeight = 0
	}
	if f.gridY+bh+glyphPad*2+1 > dim {
		// 装不下：先尝试扩容（已有字形会重新光栅化），已是最大尺寸才清空重用
		if !f.growOneStep(need, bh+glyphPad*2+2) {
			f.gridX, f.gridY, f.rowHeight = 0, 0, 0
			f.cache = map[glyphKey]*Glyph{}
			f.dropRunsLocked()
			f.atlasGen++
			for i := range f.Atlas {
				f.Atlas[i] = 0
			}
			// 显存里的旧字形必须一并清除，交给渲染器重建纹理
			f.AtlasReset = true
			f.AtlasDirty = false
			f.dirtyX0, f.dirtyY0, f.dirtyX1, f.dirtyY1 = 0, 0, 0, 0
		}
	}
	ox, oy := f.gridX+glyphPad, f.gridY+glyphPad
	f.gridX += need + 1
	if bh+glyphPad*2 > f.rowHeight {
		f.rowHeight = bh + glyphPad*2
	}
	// 注意：这里不能再置 AtlasDirty——脏区统一由 markDirty 在写入像素后叠加，
	// 否则会退化成「整张图集」级别的脏区（=每次新字形仍全量上传）。
	return ox, oy, true
}

// maxDim 图集最大边长：超高清渲染（scale ≥ 1.75）时字形位图成倍变大，1024²
// 很容易被写满而触发「清空重用」（会让同帧已生成的 UV 失效、出现缺字），故放宽到 2048²。
// 单色图集是 R8（1 字节/像素），1024² 只占 1MB、2048² 占 4MB，比旧 RGBA8 低 4 倍。
func (f *Font) maxDim() int {
	if f.scale >= 1.75 {
		return 2048
	}
	return maxAtlasDim
}

// growOneStep 把图集放大到能容纳 needW×needH 的下一档；已最大或仍放不下返回 false。
func (f *Font) growOneStep(needW, needH int) bool {
	maxDim := f.maxDim()
	want := f.AtlasSize * 2
	if want < minAtlasDim*2 {
		want = minAtlasDim * 2
	}
	for want < needW || want < needH {
		if want >= maxDim {
			return false
		}
		want *= 2
	}
	if want > maxDim {
		want = maxDim
	}
	if want <= f.AtlasSize {
		return false
	}
	f.growAtlas(want)
	return true
}

// allocCellColor 在彩色图集中分配 bw×bh。
//
// Emoji 数量极少（一屏常为 0~3 个），且位图本身就小，因此图集封顶 512²
// （RGBA8 共 1MB）；写满就整体清空重用（重光栅代价可忽略）。
func (f *Font) allocCellColor(bw, bh int) (int, int, bool) {
	if f.AtlasColor == nil {
		f.ColorSize = minAtlasDim
		f.AtlasColor = make([]byte, f.ColorSize*f.ColorSize*4)
		f.ColorReset = true
	}
	need := bw + glyphPad*2
	dim := f.ColorSize
	if f.colorGridX+need+1 > dim {
		f.colorGridX = 0
		f.colorGridY += f.colorRowHeight + 1
		f.colorRowHeight = 0
	}
	if f.colorGridY+bh+glyphPad*2+1 > dim {
		// 装不下：能扩容就扩容（等价于重建，旧字形需重新光栅化），
		// 已经是上限就清空重用
		if dim < maxColorAtlasDim {
			f.ColorSize = dim * 2
			f.AtlasColor = make([]byte, f.ColorSize*f.ColorSize*4)
			f.colorGridX, f.colorGridY, f.colorRowHeight = 0, 0, 0
			f.ColorReset = true
			f.ColorDirty = false
			dim = f.ColorSize
			if dim < need+1 || dim < bh+glyphPad*2+2 {
				return 0, 0, false
			}
		} else {
			f.colorGridX, f.colorGridY, f.colorRowHeight = 0, 0, 0
			for i := range f.AtlasColor {
				f.AtlasColor[i] = 0
			}
			// 彩色字形的 UV 全部失效，连同字形缓存一起丢弃
			f.cache = map[glyphKey]*Glyph{}
			f.dropRunsLocked()
			f.atlasGen++
			f.ColorReset = true
			f.ColorDirty = false
			if dim < need+1 || dim < bh+glyphPad*2+2 {
				return 0, 0, false
			}
		}
	}
	ox, oy := f.colorGridX+glyphPad, f.colorGridY+glyphPad
	f.colorGridX += need + 1
	if bh+glyphPad*2 > f.colorRowHeight {
		f.colorRowHeight = bh + glyphPad*2
	}
	return ox, oy, true
}

// finishGlyph 生成 UV（纹素边界）与度量。
//
// UV 必须取「纹素边界」而不是「纹素中心」：绘制时 quad 的宽高就是 bw×bh 个物理像素，
// 且左上角已对齐到整数物理像素（见 gpu.pushText 的 math.Round）。按边界取值时，
// 第 k 个 fragment（中心 x0+k+0.5）映射到的纹素坐标恰好是 ox+k+0.5，正落在纹素中心，
// 双线性采样权重为 1，等价于 1:1 直接拷贝，字形保持光栅化时的锐度。
//
// 旧实现用 (ox+0.5)…(ox+bw-0.5) 的纹素中心对齐，但 quad 并没有内缩半像素，
// 相当于把整块字形横向/纵向各压缩 1 个纹素；每个 fragment 都落在纹素之间，
// 双线性把相邻纹素一混，笔画就被重采样糊掉——这正是「GPU 文字发虚」的主因。
func (f *Font) finishGlyph(dim int, colorAtlas bool, ox, oy, bw, bh int, advance fixed.Int26_6, left, top int) *Glyph {
	d := float32(dim)
	inv := 1.0 / d
	g := &Glyph{}
	g.U0 = float32(ox) * inv
	g.U1 = float32(ox+bw) * inv
	// 位图首行写在图集顶部：oy 向下增长，纹理 v 轴向上增长，故上下边界要倒过来算。
	g.V1 = float32(dim-oy) * inv
	g.V0 = float32(dim-oy-bh) * inv
	g.W = float32(bw)
	g.H = float32(bh)
	g.Left = float32(left)
	g.Top = float32(top)
	g.Advance = float32(advance) / 64
	if g.Advance == 0 {
		g.Advance = float32(bw)
	}
	g.Valid = true
	return g
}

// measureCacheMax 行宽缓存条数上限（兜底）。
//
// 真正起作用的是下面的字节预算：条数只能防「条目爆炸」防不住「每条很长」，
// 而聊天/日志类界面每条都是唯一长字符串（300 字 ≈ 900B 键），
// 8192 条就是 7MB/字体，乘上 3~5 个字号实例后非常可观。
const measureCacheMax = 8192

// 文本缓存的字节预算（单字体实例）与淘汰比例。
//
// 超出预算时淘汰约 1/8（map 迭代顺序随机，近似均匀采样，不需要 LRU）：
// 旧实现是「满了整体清空」，一次清空会让随后几帧全部未命中、
// 长文档要按段落重新测量与重折行——这曾是「字多就卡」的主因之一。
const (
	// measureByteBudget 行宽缓存：512KB/字体（约 1500 条 300 字长串）。
	measureByteBudget = 512 << 10
	// runByteBudget 字形序列缓存：1MB/字体（每字形 48B + 键字符串）。
	runByteBudget = 1 << 20
	// cacheEvictPart 超出预算时淘汰的比例分母（1/8）。
	cacheEvictPart = 8
)

// measureEntryBytes 估算一条行宽缓存的堆占用（键字符串 + float32 + map 槽位）。
func measureEntryBytes(s string) int { return len(s) + 56 }

// runEntryBytes 估算一条字形序列缓存的堆占用（键字符串 + Glyph 切片 + 槽位）。
// 每字形 64B（10 个 float32 + 2 个 bool + 对齐，NewFont 侧不会再分配）。
func runEntryBytes(s string, n int) int { return len(s) + n*64 + 64 }

// evictMeasureLocked 淘汰约 1/cacheEvictPart 的行宽缓存。调用方需持有 f.mu。
func (f *Font) evictMeasureLocked() {
	drop := len(f.measure) / cacheEvictPart
	if drop < 1 {
		drop = 1
	}
	for k := range f.measure {
		delete(f.measure, k)
		f.measureBytes -= measureEntryBytes(k)
		if drop--; drop <= 0 {
			break
		}
	}
	if f.measureBytes < 0 {
		f.measureBytes = 0
	}
}

// evictRunsLocked 淘汰约 1/cacheEvictPart 的字形序列缓存。调用方需持有 f.mu。
func (f *Font) evictRunsLocked() {
	drop := len(f.runs) / cacheEvictPart
	if drop < 1 {
		drop = 1
	}
	for k, g := range f.runs {
		delete(f.runs, k)
		f.runGlyphs -= len(g)
		f.runBytes -= runEntryBytes(k, len(g))
		if drop--; drop <= 0 {
			break
		}
	}
	if f.runGlyphs < 0 {
		f.runGlyphs = 0
	}
	if f.runBytes < 0 {
		f.runBytes = 0
	}
}

// dropRunsLocked 清空字形序列缓存与记账（图集重建 / 缩放变化时 UV 全失效）。
func (f *Font) dropRunsLocked() {
	f.runs = make(map[string][]Glyph, 64)
	f.runGlyphs = 0
	f.runBytes = 0
}

// Measure 计算字符串行宽（逻辑像素）。
//
// 布局阶段（measureNode）与绘制阶段（paintText）每帧都会测量同一批字符串，
// 这里按字符串整体缓存结果：命中即返回，避免逐字 GlyphFor + 除法。
func (f *Font) Measure(s string) float32 {
	if s == "" {
		return 0
	}
	f.mu.RLock()
	if w, ok := f.measure[s]; ok {
		f.mu.RUnlock()
		return w
	}
	f.mu.RUnlock()

	var w float32
	for _, r := range s {
		w += f.Advance(r)
	}
	out := w

	f.mu.Lock()
	if f.measure == nil {
		f.measure = make(map[string]float32, 256)
		f.measureBytes = 0
	}
	// 并发下可能已被另一个 goroutine 填过：保留已有值，不重复记账。
	if _, ok := f.measure[s]; !ok {
		f.measure[s] = out
		f.measureBytes += measureEntryBytes(s)
	}
	if len(f.measure) > measureCacheMax || f.measureBytes > measureByteBudget {
		f.evictMeasureLocked()
	}
	f.mu.Unlock()
	return out
}

// runCacheMax 字形序列缓存条数上限（兜底；主限制是 runByteBudget）。
const (
	runCacheMax = 1024
	// runGlyphMax 单字体缓存的字形总数上限（兜底）。
	// 条数上限不够：一行 40 字 × 1024 条就是 4 万字，按字体实例坪会到 MB 级。
	runGlyphMax = 8192
	// runStringMax 单条文本超过该字数就不入缓存：
	// 一整篇小说/长日志作为一条会占住预算，还会在每次变更时反复重建。
	runStringMax = 512
)

// GlyphsFor 返回字符串的字形序列（带缓存，绘制热路径专用）。
//
// gpu.pushText 原来对每个字符调用 GlyphFor：一次 RWMutex + 一次 map 哈希，
// 按 40ns/字形算，一屏 2000 个字就是每帧 ~80µs 的纯开销，而内容其实逐帧不变。
// 这里按字符串整体缓存一次，之后每帧只剩一次 map 查找 + 顺序读结构体。
//
// 返回的切片为缓存共享数据，调用方只读，禁止修改元素或 append。
func (f *Font) GlyphsFor(s string) []Glyph {
	if s == "" {
		return nil
	}
	f.mu.RLock()
	if g, ok := f.runs[s]; ok {
		f.mu.RUnlock()
		return g
	}
	f.mu.RUnlock()

	out := make([]Glyph, 0, utf8.RuneCountInString(s))
	f.mu.RLock()
	gen := f.atlasGen
	f.mu.RUnlock()
	for attempt := 0; attempt < 3; attempt++ {
		out = out[:0]
		for _, r := range s {
			out = append(out, *f.GlyphFor(r))
		}
		// 构建期间如果图集被扩容 / 重建，前面取得的 UV 已经失效：重新取一次。
		// （长中文行的首个字形故意不预留空间时确实会中途扩容，不复检就会出现错字。）
		f.mu.RLock()
		stable := f.atlasGen == gen
		gen = f.atlasGen
		f.mu.RUnlock()
		if stable {
			break
		}
	}
	// 超长文本直接返回不入缓存（避免单条目占满预算 + 反复重建）
	if len(out) > runStringMax {
		return out
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.runs == nil {
		f.runs = make(map[string][]Glyph, 64)
		f.runGlyphs = 0
		f.runBytes = 0
	}
	if _, ok := f.runs[s]; !ok {
		f.runs[s] = out
		f.runGlyphs += len(out)
		f.runBytes += runEntryBytes(s, len(out))
	}
	// 条数 / 字形数 / 字节三个维度任一超额就淘汰 1/8（不再整体清空）。
	if len(f.runs) > runCacheMax || f.runGlyphs > runGlyphMax || f.runBytes > runByteBudget {
		f.evictRunsLocked()
	}
	return out
}

// MeasureRunes 仅测量部分。
func (f *Font) MeasureRunes(rs []rune) float32 {
	var w float32
	for _, r := range rs {
		w += f.Advance(r)
	}
	return w
}

// Truncate 按宽度截断并加省略号（fit 为逻辑像素）。
func (f *Font) Truncate(s string, fit float32) string {
	if f.Measure(s) <= fit || fit <= 0 {
		return s
	}
	rs := []rune(s)
	ell := f.Measure("…")
	limit := fit - ell
	var w float32
	n := 0
	for _, r := range rs {
		w += f.Advance(r)
		if w > limit {
			break
		}
		n++
	}
	if n <= 0 {
		return "…"
	}
	return string(rs[:n]) + "…"
}

// Scale 返回当前 DPI 缩放。
func (f *Font) Scale() float32 { return f.scale }

// Close 释放资源（opentype face 无需显式释放）。
func (f *Font) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.face = nil
	f.fallback = nil
	f.cache = map[glyphKey]*Glyph{}
	f.measure = nil
	f.measureBytes = 0
	f.runs = nil
	f.runGlyphs = 0
	f.runBytes = 0
	// 释放字形图集（单色 1MB / 彩色最多 1MB）：字体被缓存回收后不应继续占内存
	f.Atlas = nil
	f.AtlasDirty = false
	f.AtlasReset = true
	f.AtlasSize = 0
	f.gridX, f.gridY, f.rowHeight = 0, 0, 0
	dropColorAtlas(f)
	if f.emoji != nil {
		f.emoji.close()
		f.emoji = nil
	}
}

// isEmojiRune 判断是否彩色 Emoji（常见区段）。
func isEmojiRune(r rune) bool {
	switch {
	case r == 0x200D || r == 0xFE0F || r == 0x20E3: // ZWJ / VS16 / 键帽
		return false
	case r >= 0x1F000 && r <= 0x1FAFF: // 表情/交通/补充符号
		return true
	case r >= 0x2600 && r <= 0x27BF: // 杂项符号与装饰
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF: // 国旗区域指示符
		return true
	}
	return false
}

// ---- 系统字体解析（按家族名映射到字体文件）----

var (
	sysFontsMu sync.Mutex
	sysFonts   = map[string]*sfnt.Font{}
)

// fontCandidates 返回候选字体文件（路径有序，前面的优先）。
//
// GUIS_FONT_FILE 可直接指定主字体文件，优先级最高：
// 容器/WSL 里系统没装中文字体时（表现为「英文正常、中文全空白」），
// 用它指向任意 ttc/ttf（如 WSLg 下的 /mnt/c/Windows/Fonts/msyh.ttc）即可，无需改系统。
//
// 平台差异全部收敛在 platformFontCandidates / platformDefaultFontCandidates 里：
//   - Windows：按文件名猜 %WINDIR%\Fonts 下的字体；
//   - Linux/macOS/Android/iOS：扫描系统字体目录并按关键词匹配；
//   - 浏览器：查 Web 运行时注册进来的字体表。
func fontCandidates(faceName string, weight int32) []string {
	if p := strings.TrimSpace(os.Getenv("GUIS_FONT_FILE")); p != "" {
		return []string{p}
	}
	if p := platformFontCandidates(faceName, weight); len(p) > 0 {
		return p
	}
	return platformDefaultFontCandidates(weight)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 || m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// mapFontFile 读取字体文件：桌面端走内存映射，浏览器端查已注册字体表。
func mapFontFile(path string) ([]byte, error) { return platformMapFontFile(path) }

// FontDebugInfo 返回名字对应的字体数据长度与**实际解析出来**的字形数。
//
// 只给诊断/自动化冒烟用（window.__guisFontInfo）：网页端热注入全量字体后，
// 数据长度与字形数都应该变大；若长度变了而字形数没变，说明解析缓存没失效。
func FontDebugInfo(name string) (dataLen, glyphs int) {
	if b, err := mapFontFile(name); err == nil {
		dataLen = len(b)
	}
	if f, err := loadSystemFont(name, 400); err == nil && f != nil {
		glyphs = f.NumGlyphs()
	}
	return
}

// loadSystemFont 解析系统字体文件，优先返回文件名向量中的第一个可用字体。
func loadSystemFont(faceName string, weight int32) (*sfnt.Font, error) {
	paths := fontCandidates(faceName, weight)
	for _, p := range paths {
		key := p
		sysFontsMu.Lock()
		if f, ok := sysFonts[key]; ok {
			sysFontsMu.Unlock()
			if f != nil {
				return f, nil
			}
			continue
		}
		data, err := mapFontFile(p)
		var picked *sfnt.Font
		if err == nil {
			if col, cerr := opentype.ParseCollection(data); cerr == nil && col.NumFonts() > 0 {
				// 优先取族名匹配 "UI" 的那个（Microsoft YaHei UI）
				var best *sfnt.Font
				for i := 0; i < col.NumFonts(); i++ {
					if fnt, ferr := col.Font(i); ferr == nil {
						if best == nil {
							best = fnt
						}
						var buf sfnt.Buffer
						if fam, ferr := fnt.Name(&buf, sfnt.NameIDFamily); ferr == nil && containsAny(fam, "UI") {
							best = fnt
							break
						}
					}
				}
				picked = best
			} else if fnt, ferr := opentype.Parse(data); ferr == nil {
				picked = fnt
			}
		}
		sysFonts[key] = picked
		sysFontsMu.Unlock()
		if picked != nil {
			return picked, nil
		}
	}
	return nil, os.ErrNotExist
}
