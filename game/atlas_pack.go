// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"sort"
)

// PackImage 待打包的一张图（Name 是回读时用的键）。
type PackImage struct {
	Name  string
	Image image.Image
}

// PackOptions 图集打包参数。
type PackOptions struct {
	// Padding 图与图之间、图与边界的留白（像素，默认 1）。
	Padding int
	// MaxSize 图集最大边长（默认 4096）。
	MaxSize int
	// PowerOfTwo 边长是否向上取到 2 的幂（部分老 GPU 需要）。
	PowerOfTwo bool
}

// PackRect 图集内一张图的位置（像素）。
type PackRect struct {
	Name string
	X, Y int
	W, H int
}

// PackResult 打包结果。
type PackResult struct {
	Image *image.RGBA
	Rects []PackRect
	W, H  int
	// Used 面积利用率（0~1）。
	Used float64
}

// Rect2 把打包位置转为 Sprite 用的源矩形。
func (p PackRect) Rect2() Rect2 {
	return Rect2{X: float32(p.X), Y: float32(p.Y), W: float32(p.W), H: float32(p.H)}
}

type packRect struct{ x, y, w, h int }

type packItem struct {
	name string
	img  image.Image
	w, h int
}

// PackAtlas 用 MaxRects（BSSF）把多张图打包进一张图集。
//
// 返回的图集是 RGBA，可直接 TextureFromImage 或编码为 PNG 保存；
// Rects 顺序与输入无关，按 Name 查找。空输入返回 1×1 空图而非错误。
func PackAtlas(imgs []PackImage, opts PackOptions) (*PackResult, error) {
	pad := opts.Padding
	if pad < 0 {
		pad = 0
	}
	maxSize := opts.MaxSize
	if maxSize <= 0 {
		maxSize = 4096
	}
	items := make([]packItem, 0, len(imgs))
	area := 0
	for _, in := range imgs {
		if in.Image == nil {
			return nil, fmt.Errorf("game: 图集打包收到空图像 %q", in.Name)
		}
		b := in.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		if w <= 0 || h <= 0 {
			continue
		}
		items = append(items, packItem{name: in.Name, img: in.Image, w: w, h: h})
		area += (w + pad) * (h + pad)
	}
	if len(items) == 0 {
		return &PackResult{Image: image.NewRGBA(image.Rect(0, 0, 1, 1)), W: 1, H: 1}, nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.w*a.h != b.w*b.h {
			return a.w*a.h > b.w*b.h
		}
		return a.w > b.w
	})

	size := int(math.Ceil(math.Sqrt(float64(area))))
	if opts.PowerOfTwo {
		size = nextPow2(size)
	}
	if size < 1 {
		size = 1
	}
	var placed []PackRect
	var free []packRect
	for {
		if size > maxSize {
			return nil, fmt.Errorf("game: 图集超出最大尺寸 %d（约需面积 %d）", maxSize, area)
		}
		free = []packRect{{0, 0, size, size}}
		placed = placed[:0]
		ok := true
		for _, it := range items {
			r, rest, fit := packPlace(free, it.w, it.h, pad)
			if !fit {
				ok = false
				break
			}
			placed = append(placed, PackRect{Name: it.name, X: r.x, Y: r.y, W: it.w, H: it.h})
			free = rest
		}
		if ok {
			break
		}
		if opts.PowerOfTwo {
			size *= 2
		} else {
			step := size / 8
			if step < 16 {
				step = 16
			}
			size = (size + step + 15) / 16 * 16
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for i, it := range items {
		r := placed[i]
		draw.Draw(dst, image.Rect(r.X, r.Y, r.X+r.W, r.Y+r.H), it.img, it.img.Bounds().Min, draw.Src)
	}
	used := 0
	for _, r := range placed {
		used += r.W * r.H
	}
	return &PackResult{
		Image: dst, Rects: placed, W: size, H: size,
		Used: float64(used) / float64(size*size),
	}, nil
}

// packPlace 在 free 中找最佳位置（Best Short Side Fit），返回矩形与新的 free 列表。
func packPlace(free []packRect, w, h, pad int) (packRect, []packRect, bool) {
	best := -1
	bestShort, bestLong := math.MaxInt32, math.MaxInt32
	for i, f := range free {
		if w > f.w || h > f.h {
			continue
		}
		short := minInt(f.w-w, f.h-h)
		long := maxInt(f.w-w, f.h-h)
		if short < bestShort || (short == bestShort && long < bestLong) {
			best, bestShort, bestLong = i, short, long
		}
	}
	if best < 0 {
		return packRect{}, free, false
	}
	f := free[best]
	used := packRect{x: f.x, y: f.y, w: w + pad, h: h + pad}
	out := free[:0:0]
	for i, fr := range free {
		if i == best {
			continue
		}
		out = append(out, fr)
	}
	out = append(out, splitFree(f, used)...)
	free = pruneFree(out)
	return packRect{x: f.x, y: f.y, w: w, h: h}, free, true
}

func splitFree(f, used packRect) []packRect {
	if used.x >= f.x+f.w || used.y >= f.y+f.h || used.x+used.w <= f.x || used.y+used.h <= f.y {
		return []packRect{f}
	}
	var out []packRect
	if used.x > f.x {
		out = append(out, packRect{x: f.x, y: f.y, w: used.x - f.x, h: f.h})
	}
	if used.x+used.w < f.x+f.w {
		out = append(out, packRect{x: used.x + used.w, y: f.y, w: f.x + f.w - (used.x + used.w), h: f.h})
	}
	if used.y > f.y {
		out = append(out, packRect{x: f.x, y: f.y, w: f.w, h: used.y - f.y})
	}
	if used.y+used.h < f.y+f.h {
		out = append(out, packRect{x: f.x, y: used.y + used.h, w: f.w, h: f.y + f.h - (used.y + used.h)})
	}
	return out
}

func pruneFree(in []packRect) []packRect {
	out := make([]packRect, 0, len(in))
	for i, a := range in {
		if a.w <= 0 || a.h <= 0 {
			continue
		}
		contained := false
		for j, b := range in {
			if i == j || b.w <= 0 || b.h <= 0 {
				continue
			}
			if a.x >= b.x && a.y >= b.y && a.x+a.w <= b.x+b.w && a.y+a.h <= b.y+b.h {
				// 完全相同的矩形只保留下标小的那个
				if a.x == b.x && a.y == b.y && a.w == b.w && a.h == b.h && j > i {
					continue
				}
				contained = true
				break
			}
		}
		if !contained {
			out = append(out, a)
		}
	}
	return out
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
