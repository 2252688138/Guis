// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// SkyMode 天空模式。
type SkyMode uint8

const (
	// SkyOff 不绘制天空（用 Scene.Background / Renderer.BgColor 纯色背景）。
	SkyOff SkyMode = iota
	// SkyGradient 三色垂直渐变天空（最便宜，适合卡通 / 像素风）。
	SkyGradient
	// SkyCube 立方体天空盒（4×3 交叉展开图，6 面采样）。
	SkyCube
)

// Sky 天空盒 / 渐变天空（挂在 Scene.Sky 上）。
//
// 透视相机下逐像素按视线方向着色，视野旋转、抬头低头都能正确响应；
// 只在背景像素上执行（先于场景光栅化，场景覆盖的自然被遮挡）。
//
//	sc.Sky = game.GradientSky(game.RGB(60, 120, 200), game.RGB(170, 200, 235), game.RGB(40, 45, 60))
//
// 立方体天空盒使用 4×3 交叉布局（FaceSize = 图宽/4）：
//
//	[   ] [ +Y ] [   ] [   ]
//	[ -X] [ +Z ] [+X ] [ -Z]
//	[   ] [ -Y ] [   ] [   ]
type Sky struct {
	// Mode 天空模式。
	Mode SkyMode
	// Top, Horizon, Bottom 渐变天空的天顶色 / 地平线色 / 天底（下方）色。
	Top, Horizon, Bottom RGBA
	// Cube 立方体天空盒贴图（4×3 交叉展开布局）。
	Cube *Texture
	// FaceSize 立方体单面边长（0 = Cube.W/4）。
	FaceSize int
	// Intensity 亮度系数（默认 1）。
	Intensity float32
	// Rotation 绕 Y 轴旋转（弧度，用于调整天空盒朝向）。
	Rotation float32
}

// GradientSky 构造三色渐变天空。
func GradientSky(top, horizon, bottom RGBA) Sky {
	return Sky{Mode: SkyGradient, Top: top, Horizon: horizon, Bottom: bottom, Intensity: 1}
}

// CubeSky 构造立方体天空盒（交叉展开图，FaceSize 为单面边长，0 = 图宽/4）。
func CubeSky(cube *Texture, faceSize int) Sky {
	s := Sky{Mode: SkyCube, Cube: cube, FaceSize: faceSize, Intensity: 1}
	if cube != nil && faceSize <= 0 {
		s.FaceSize = cube.W / 4
	}
	return s
}

// Enabled 是否启用天空绘制。
func (s Sky) Enabled() bool {
	if s.Mode == SkyOff {
		return false
	}
	if s.Mode == SkyCube && (s.Cube == nil || s.FaceSize <= 0) {
		return false
	}
	return true
}

// renderSky 把天空画进帧缓冲背景（在 clearFrame 之后、光栅化之前调用）。
func renderSky(fb *frameBuf, cam *Camera, aspect float32, sky Sky) {
	if fb == nil || cam == nil || !sky.Enabled() || fb.w <= 0 || fb.h <= 0 {
		return
	}
	view := cam.View()
	// LookAt 生成的视图矩阵前三行就是相机的右 / 上 / 后向量
	right := V3(view.At(0, 0), view.At(0, 1), view.At(0, 2))
	up := V3(view.At(1, 0), view.At(1, 1), view.At(1, 2))
	fwd := V3(-view.At(2, 0), -view.At(2, 1), -view.At(2, 2))

	intensity := sky.Intensity
	if intensity <= 0 {
		intensity = 1
	}
	rot := sky.Rotation
	cosR, sinR := float32(math.Cos(float64(-rot))), float32(math.Sin(float64(-rot)))

	// 透视：每像素方向不同；正交：全屏同一方向
	tanHalf := float32(math.Tan(float64(cam.FOVY) * math.Pi / 360))
	if tanHalf <= 0 {
		tanHalf = float32(math.Tan(30 * math.Pi / 180))
	}
	persp := !cam.Ortho

	for y := 0; y < fb.h; y++ {
		row := y * fb.w
		var dirY Vec3
		if persp {
			ndcY := (1 - 2*(float32(y)+0.5)/float32(fb.h)) * tanHalf
			dirY = fwd.Add(up.Scale(ndcY))
		}
		for x := 0; x < fb.w; x++ {
			var d Vec3
			if persp {
				ndcX := (2*(float32(x)+0.5)/float32(fb.w) - 1) * tanHalf * aspect
				d = dirY.Add(right.Scale(ndcX)).Normalize()
			} else {
				d = fwd
			}
			if rot != 0 {
				x0, z0 := d.X, d.Z
				d.X = x0*cosR + z0*sinR
				d.Z = -x0*sinR + z0*cosR
			}
			var c RGBA
			if sky.Mode == SkyCube {
				c = sampleCube(sky.Cube, sky.FaceSize, d)
			} else {
				c = gradientColor(sky, d.Y)
			}
			if intensity != 1 {
				c = c.Scale(intensity)
			}
			fb.color[row+x] = packColor(c)
		}
	}
}

// gradientColor 按方向 Y 分量在两段渐变中取色。
func gradientColor(sky Sky, dy float32) RGBA {
	hor := sky.Horizon
	if dy >= 0 {
		t := float32(math.Sqrt(float64(Clamp(dy, 0, 1))))
		return hor.Lerp(sky.Top, t)
	}
	t := float32(math.Sqrt(float64(Clamp(-dy, 0, 1))))
	return hor.Lerp(sky.Bottom, t)
}

// cubeFaceUV 把方向向量转成面索引与面内 UV。
// 面序：0=+X 1=-X 2=+Y 3=-Y 4=+Z 5=-Z。
func cubeFaceUV(d Vec3) (face int, u, v float32) {
	ax, ay, az := float32(math.Abs(float64(d.X))), float32(math.Abs(float64(d.Y))), float32(math.Abs(float64(d.Z)))
	switch {
	case ax >= ay && ax >= az:
		if d.X > 0 {
			return 0, (-d.Z/ax + 1) * 0.5, (1 - d.Y/ax) * 0.5
		}
		return 1, (d.Z/ax + 1) * 0.5, (1 - d.Y/ax) * 0.5
	case ay >= az:
		if d.Y > 0 {
			return 2, (d.X/ay + 1) * 0.5, (-d.Z/ay + 1) * 0.5
		}
		return 3, (d.X/ay + 1) * 0.5, (d.Z/ay + 1) * 0.5
	default:
		if d.Z > 0 {
			return 4, (d.X/az + 1) * 0.5, (1 - d.Y/az) * 0.5
		}
		return 5, (-d.X/az + 1) * 0.5, (1 - d.Y/az) * 0.5
	}
}

// cubeFaceTile 面索引 → 交叉展开图中的列 / 行。
func cubeFaceTile(face int) (col, row int) {
	switch face {
	case 0:
		return 2, 1
	case 1:
		return 0, 1
	case 2:
		return 1, 0
	case 3:
		return 1, 2
	case 4:
		return 1, 1
	default:
		return 3, 1
	}
}

// sampleCube 按方向采样立方体天空盒。
func sampleCube(t *Texture, faceSize int, d Vec3) RGBA {
	if t == nil || faceSize <= 0 {
		return RGB(0, 0, 0)
	}
	face, u, v := cubeFaceUV(d)
	col, row := cubeFaceTile(face)
	x := col*faceSize + int(u*float32(faceSize))
	y := row*faceSize + int(v*float32(faceSize))
	if x < 0 || y < 0 || x >= t.W || y >= t.H {
		return RGB(0, 0, 0)
	}
	return t.Pix[y*t.W+x]
}

// skyColor 按世界方向采样天空颜色（PBR 环境光 / IBL 近似用）。
//
// 只做「方向 → 颜色」的一次查询：渐变天空按 Y 插值，立方体天空按面采样。
// 不做预滤波卷积，粗糙度由调用方用「与漫反射平均混合」的方式近似。
func skyColor(sky Sky, d Vec3) RGBA {
	if !sky.Enabled() {
		return RGBA{}
	}
	if sky.Rotation != 0 {
		c, s := float32(math.Cos(float64(-sky.Rotation))), float32(math.Sin(float64(-sky.Rotation)))
		d = V3(d.X*c-d.Z*s, d.Y, d.X*s+d.Z*c)
	}
	if sky.Mode == SkyCube && sky.Cube != nil {
		fs := sky.FaceSize
		if fs <= 0 {
			fs = sky.Cube.W / 4
		}
		return sampleCube(sky.Cube, fs, d)
	}
	return gradientColor(sky, d.Y)
}
