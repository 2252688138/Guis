// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bytes"
	"image"
	"image/gif"
	"image/png"
	"image/color/palette"
	"os"
	"time"
)

// Snapshot 拷贝渲染器当前帧（与内部缓冲解耦，可在其他 goroutine 使用）。
func Snapshot(r *Renderer) *image.RGBA {
	if r == nil {
		return nil
	}
	src := r.Image()
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(dst.Pix, src.Pix)
	return dst
}

// SnapshotPNGBytes 当前帧编码为 PNG 字节。
func SnapshotPNGBytes(r *Renderer) ([]byte, error) {
	img := Snapshot(r)
	if img == nil {
		return nil, os.ErrInvalid
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SnapshotPNG 把当前帧保存为 PNG 文件（目录自动创建）。
func SnapshotPNG(r *Renderer, path string) error {
	b, err := SnapshotPNGBytes(r)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

// GIFRecorder 把多帧录成 GIF（帧数上限防止内存暴涨）。
type GIFRecorder struct {
	// MaxFrames 最多保留帧数（≤0 按 120）。
	MaxFrames int
	// Delay 帧间隔（百分之一秒，GIF 最小 2）。
	Delay int

	frames []*image.RGBA
}

// NewGIFRecorder 创建 GIF 录制器。
func NewGIFRecorder(maxFrames, delayCs int) *GIFRecorder {
	if maxFrames <= 0 {
		maxFrames = 120
	}
	if delayCs < 2 {
		delayCs = 2
	}
	return &GIFRecorder{MaxFrames: maxFrames, Delay: delayCs}
}

// Add 追加一帧（超过上限时丢弃最早的帧）。
func (g *GIFRecorder) Add(img *image.RGBA) {
	if g == nil || img == nil {
		return
	}
	cp := image.NewRGBA(img.Rect)
	copy(cp.Pix, img.Pix)
	g.frames = append(g.frames, cp)
	if len(g.frames) > g.MaxFrames {
		g.frames = g.frames[len(g.frames)-g.MaxFrames:]
	}
}

// Len 已录制帧数。
func (g *GIFRecorder) Len() int {
	if g == nil {
		return 0
	}
	return len(g.frames)
}

// Clear 清空帧。
func (g *GIFRecorder) Clear() {
	if g == nil {
		return
	}
	g.frames = nil
}

// Save 编码并保存 GIF（少于 2 帧时报错）。
func (g *GIFRecorder) Save(path string) error {
	if g == nil || len(g.frames) < 2 {
		return os.ErrInvalid
	}
	out := &gif.GIF{}
	for _, f := range g.frames {
		out.Image = append(out.Image, toPaletted(f))
		out.Delay = append(out.Delay, g.Delay)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, out); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes())
}

// toPaletted RGBA → 调色板图像（GIF 帧格式）。
func toPaletted(src *image.RGBA) *image.Paletted {
	b := src.Bounds()
	dst := image.NewPaletted(b, palette.Plan9)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

// FrameLimiter 帧率限制器（软件渲染下避免空转吃满 CPU）。
type FrameLimiter struct {
	// TargetFPS 目标帧率（≤0 不限帧）。
	TargetFPS float64
	last      time.Time
	interval  time.Duration
	fps       float64
}

// NewFrameLimiter 创建帧率限制器。
func NewFrameLimiter(fps int) *FrameLimiter {
	f := &FrameLimiter{TargetFPS: float64(fps)}
	if fps > 0 {
		f.interval = time.Second / time.Duration(fps)
	}
	return f
}

// Wait 睡到下一帧到点，返回本次实际等待时长。
func (f *FrameLimiter) Wait() time.Duration {
	if f == nil || f.interval <= 0 {
		return 0
	}
	now := time.Now()
	if f.last.IsZero() {
		f.last = now
		f.fps = f.TargetFPS
		return 0
	}
	deadline := f.last.Add(f.interval)
	if d := time.Until(deadline); d > 0 {
		time.Sleep(d)
	}
	elapsed := time.Since(f.last)
	f.last = time.Now()
	if elapsed > 0 {
		f.fps = 1 / elapsed.Seconds()
	}
	return elapsed
}

// FPS 最近一帧实测帧率。
func (f *FrameLimiter) FPS() float64 {
	if f == nil {
		return 0
	}
	return f.fps
}

// Reset 重置计时（切场景/暂停恢复时用）。
func (f *FrameLimiter) Reset() {
	if f == nil {
		return
	}
	f.last = time.Time{}
	f.fps = 0
}
