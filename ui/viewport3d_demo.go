// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// DemoViewportSource 程序化的 3D 视口参考源：渐变背景 + 旋转方块。
//
// 不依赖任何外部引擎，用来验证「ViewportSource → 纹理 → 合成」整条链路，
// 也可以作为宿主自己写渲染源时的最小示例：
//
//	src := ui.NewDemoViewportSource()
//	ui.Viewport3D(&ui.Viewport3DOpts{Src: src, Key: "demo"})

import "math"

// DemoViewportSource 渐变 + 旋转方块的视口源。
type DemoViewportSource struct {
	t float32 // 累计时间（秒）
}

// NewDemoViewportSource 创建演示视口源。
func NewDemoViewportSource() *DemoViewportSource {
	return &DemoViewportSource{}
}

// Time 累计时间（诊断 / 测试用）。
func (s *DemoViewportSource) Time() float32 { return s.t }

// RenderViewport 渲染一帧（RGBA，直通道 alpha）。
func (s *DemoViewportSource) RenderViewport(w, h int, dt float32) ([]byte, int, int) {
	if w <= 0 || h <= 0 {
		return nil, 0, 0
	}
	if dt > 0 {
		s.t += dt
	}
	pix := make([]byte, w*h*4)
	// 背景：左上到右下的渐变
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			pix[i] = uint8(60 + 120*x/maxInt(1, w-1))
			pix[i+1] = uint8(30 + 100*y/maxInt(1, h-1))
			pix[i+2] = 70
			pix[i+3] = 255
		}
	}
	// 旋转方块
	cx, cy := float32(w)/2, float32(h)/2
	size := float32(minInt(w, h)) / 4
	ang := s.t * 1.6
	pts := make([][2]float32, 4)
	for i := 0; i < 4; i++ {
		a := ang + float32(i)*float32(math.Pi)/2 + float32(math.Pi)/4
		pts[i] = [2]float32{cx + size*float32(math.Cos(float64(a))), cy + size*float32(math.Sin(float64(a)))}
	}
	minX, maxX := pts[0][0], pts[0][0]
	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts {
		minX = minF32(minX, p[0])
		maxX = maxF32(maxX, p[0])
		minY = minF32(minY, p[1])
		maxY = maxF32(maxY, p[1])
	}
	x0, x1 := int(maxF32(0, minX)), int(minF32(float32(w)-1, maxX))
	y0, y1 := int(maxF32(0, minY)), int(minF32(float32(h)-1, maxY))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !pointInQuad(float32(x)+0.5, float32(y)+0.5, pts) {
				continue
			}
			i := (y*w + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = 240, 245, 255, 255
		}
	}
	return pix, w, h
}

// pointInQuad 四边形内部判定（凸四边形，叉积同号）。
func pointInQuad(px, py float32, pts [][2]float32) bool {
	pos, neg := false, false
	n := len(pts)
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%n]
		cross := (b[0]-a[0])*(py-a[1]) - (b[1]-a[1])*(px-a[0])
		if cross > 0 {
			pos = true
		} else if cross < 0 {
			neg = true
		}
		if pos && neg {
			return false
		}
	}
	return true
}
