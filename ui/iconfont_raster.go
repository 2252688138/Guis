// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"strconv"
)

// Iconfont 矢量光栅化内核。
//
// 图标用 24×24 设计网格上的 SVG path 描述（M/L/H/V/C/S/Q/T/A/Z，绝对与相对命令都支持），
// 这里把曲线展平成折线，再算出每个像素的覆盖率（白色 + alpha），
// 绘制时用 gpu.Image.Tint 着色——同一张纹理可以任意换色，不必为每种颜色再建一份。
//
// 不走系统字体的符号字形：符号随字体漂移、缺字形会画成方块；
// 也不引入 .ttf 图标字体：一份路径数据就能任意尺寸清晰渲染，还能运行时注册自定义图标。

// iconGridSize 设计网格边长（与主流图标库一致：24×24）。
const iconGridSize = 24.0

// iconRasterSS 填充光栅的每像素超采样倍数（4×4 = 16 个样本）。
const iconRasterSS = 4

// iconPt 设计网格坐标。
type iconPt struct{ X, Y float64 }

// iconSubPath 一条展平后的子路径。
type iconSubPath struct {
	Pts    []iconPt
	Closed bool
}

// iconSeg 折线中的一条线段（含零长度线段：它会以圆端点画成一个圆点）。
type iconSeg struct {
	AX, AY, BX, BY float64
}

// iconFlattenPath 解析 SVG path 数据并展平成折线（坐标单位 = 设计网格）。
// 无法识别的字符会被跳过，保证脏数据不会 panic、也不会死循环。
func iconFlattenPath(d string) []iconSubPath {
	p := &iconPathParser{s: d}
	p.run()
	return p.subs
}

// iconPathParser SVG path 解析器。
type iconPathParser struct {
	s    string
	i    int
	subs []iconSubPath
	cur  []iconPt
	// 当前点 / 子路径起点
	x, y float64
	sx, sy float64
	// 上一段曲线（三次/二次）的末控制点，S/T 的反射点从这里来
	cx, cy   float64
	ctrlKind byte
	// 当前命令（M 之后的坐标对按隐式 L/l 处理）
	cmd byte
}

// isIconCmd 是否是 path 命令字符。
func isIconCmd(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's',
		'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}

func (p *iconPathParser) run() {
	for {
		p.skipSep()
		if p.i >= len(p.s) {
			break
		}
		c := p.s[p.i]
		if isIconCmd(c) {
			p.i++
			if c == 'Z' || c == 'z' {
				p.closeSub()
				p.cmd = 0
				continue
			}
			p.cmd = c
			continue
		}
		if p.cmd == 0 || !p.exec(p.cmd) {
			// 无法继续解析：跳过一个字符，避免卡在同一处
			p.i++
		}
	}
	p.closeSub()
}

// skipSep 跳过分隔符（空白 / 逗号）。
func (p *iconPathParser) skipSep() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\r', '\n', ',':
			p.i++
		default:
			return
		}
	}
}

// readN 读一个数字；读不到时返回 false（调用方据此跳过该字符）。
func (p *iconPathParser) readN() (float64, bool) {
	p.skipSep()
	start := p.i
	i := p.i
	if i < len(p.s) && (p.s[i] == '-' || p.s[i] == '+') {
		i++
	}
	digits := false
	for i < len(p.s) && p.s[i] >= '0' && p.s[i] <= '9' {
		i++
		digits = true
	}
	if i < len(p.s) && p.s[i] == '.' {
		i++
		for i < len(p.s) && p.s[i] >= '0' && p.s[i] <= '9' {
			i++
			digits = true
		}
	}
	if !digits {
		return 0, false
	}
	if i < len(p.s) && (p.s[i] == 'e' || p.s[i] == 'E') {
		j := i + 1
		if j < len(p.s) && (p.s[j] == '-' || p.s[j] == '+') {
			j++
		}
		k := j
		for k < len(p.s) && p.s[k] >= '0' && p.s[k] <= '9' {
			k++
		}
		if k > j {
			i = k
		}
	}
	v, err := strconv.ParseFloat(p.s[start:i], 64)
	if err != nil {
		return 0, false
	}
	p.i = i
	return v, true
}

// exec 执行一组命令参数。返回值表示是否成功消费。
func (p *iconPathParser) exec(c byte) bool {
	switch c {
	case 'M', 'm':
		x, ok1 := p.readN()
		y, ok2 := p.readN()
		if !ok1 || !ok2 {
			return false
		}
		if c == 'm' {
			x += p.x
			y += p.y
		}
		p.closeSub()
		p.x, p.y = x, y
		p.sx, p.sy = x, y
		p.cur = []iconPt{{x, y}}
		p.ctrlKind = 0
		// 首点之后的坐标对是隐式 lineto
		if c == 'M' {
			p.cmd = 'L'
		} else {
			p.cmd = 'l'
		}
		return true

	case 'L', 'l':
		x, ok1 := p.readN()
		y, ok2 := p.readN()
		if !ok1 || !ok2 {
			return false
		}
		if c == 'l' {
			x += p.x
			y += p.y
		}
		p.lineTo(x, y)
		return true

	case 'H', 'h':
		x, ok := p.readN()
		if !ok {
			return false
		}
		if c == 'h' {
			x += p.x
		}
		p.lineTo(x, p.y)
		return true

	case 'V', 'v':
		y, ok := p.readN()
		if !ok {
			return false
		}
		if c == 'v' {
			y += p.y
		}
		p.lineTo(p.x, y)
		return true

	case 'C', 'c':
		x1, ok1 := p.readN()
		y1, ok2 := p.readN()
		x2, ok3 := p.readN()
		y2, ok4 := p.readN()
		x, ok5 := p.readN()
		y, ok6 := p.readN()
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			return false
		}
		if c == 'c' {
			x1, y1 = x1+p.x, y1+p.y
			x2, y2 = x2+p.x, y2+p.y
			x, y = x+p.x, y+p.y
		}
		p.cubicTo(x1, y1, x2, y2, x, y)
		return true

	case 'S', 's':
		x2, ok1 := p.readN()
		y2, ok2 := p.readN()
		x, ok3 := p.readN()
		y, ok4 := p.readN()
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return false
		}
		if c == 's' {
			x2, y2 = x2+p.x, y2+p.y
			x, y = x+p.x, y+p.y
		}
		x1, y1 := p.x, p.y
		if p.ctrlKind == 'C' {
			x1 = 2*p.x - p.cx
			y1 = 2*p.y - p.cy
		}
		p.cubicTo(x1, y1, x2, y2, x, y)
		return true

	case 'Q', 'q':
		x1, ok1 := p.readN()
		y1, ok2 := p.readN()
		x, ok3 := p.readN()
		y, ok4 := p.readN()
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return false
		}
		if c == 'q' {
			x1, y1 = x1+p.x, y1+p.y
			x, y = x+p.x, y+p.y
		}
		p.quadTo(x1, y1, x, y)
		return true

	case 'T', 't':
		x, ok1 := p.readN()
		y, ok2 := p.readN()
		if !ok1 || !ok2 {
			return false
		}
		if c == 't' {
			x += p.x
			y += p.y
		}
		x1, y1 := p.x, p.y
		if p.ctrlKind == 'Q' {
			x1 = 2*p.x - p.cx
			y1 = 2*p.y - p.cy
		}
		p.quadTo(x1, y1, x, y)
		return true

	case 'A', 'a':
		rx, ok1 := p.readN()
		ry, ok2 := p.readN()
		rot, ok3 := p.readN()
		laf, ok4 := p.readN()
		sf, ok5 := p.readN()
		x, ok6 := p.readN()
		y, ok7 := p.readN()
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
			return false
		}
		if c == 'a' {
			x += p.x
			y += p.y
		}
		p.arcTo(rx, ry, rot, laf != 0, sf != 0, x, y)
		return true
	}
	return false
}

// lineTo 追加一条直线。
func (p *iconPathParser) lineTo(x, y float64) {
	p.cur = append(p.cur, iconPt{x, y})
	p.x, p.y = x, y
	p.ctrlKind = 0
}

// cubicTo 追加三次贝塞尔曲线（按控制多边形长度自适应细分）。
func (p *iconPathParser) cubicTo(x1, y1, x2, y2, x, y float64) {
	p0 := iconPt{p.x, p.y}
	c1 := iconPt{x1, y1}
	c2 := iconPt{x2, y2}
	p1 := iconPt{x, y}
	n := iconCurveSteps(iconDist(p0, c1) + iconDist(c1, c2) + iconDist(c2, p1))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		p.cur = append(p.cur, iconPt{
			X: u*u*u*p0.X + 3*u*u*t*c1.X + 3*u*t*t*c2.X + t*t*t*p1.X,
			Y: u*u*u*p0.Y + 3*u*u*t*c1.Y + 3*u*t*t*c2.Y + t*t*t*p1.Y,
		})
	}
	p.x, p.y = x, y
	p.cx, p.cy, p.ctrlKind = x2, y2, 'C'
}

// quadTo 追加二次贝塞尔曲线。
func (p *iconPathParser) quadTo(x1, y1, x, y float64) {
	p0 := iconPt{p.x, p.y}
	c := iconPt{x1, y1}
	p1 := iconPt{x, y}
	n := iconCurveSteps(iconDist(p0, c) + iconDist(c, p1))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		p.cur = append(p.cur, iconPt{
			X: u*u*p0.X + 2*u*t*c.X + t*t*p1.X,
			Y: u*u*p0.Y + 2*u*t*c.Y + t*t*p1.Y,
		})
	}
	p.x, p.y = x, y
	p.cx, p.cy, p.ctrlKind = x1, y1, 'Q'
}

// arcTo 追加椭圆弧（SVG 端点参数 → 圆心参数后按角度细分）。
func (p *iconPathParser) arcTo(rx, ry, rot float64, large, sweep bool, x, y float64) {
	x0, y0 := p.x, p.y
	if rx == 0 || ry == 0 || (x0 == x && y0 == y) {
		p.lineTo(x, y)
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	phi := rot * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := (x0-x)/2, (y0-y)/2
	x1p := cos*dx + sin*dy
	y1p := -sin*dx + cos*dy
	// 半径过小：按比例放大到刚好够用
	lam := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lam > 1 {
		s := math.Sqrt(lam)
		rx *= s
		ry *= s
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	co := 0.0
	if den > 0 {
		co = math.Sqrt(math.Max(0, num/den))
	}
	if large == sweep {
		co = -co
	}
	cxp := co * rx * y1p / ry
	cyp := -co * ry * x1p / rx
	cx := cos*cxp - sin*cyp + (x0+x)/2
	cy := sin*cxp + cos*cyp + (y0+y)/2

	ux, uy := (x1p-cxp)/rx, (y1p-cyp)/ry
	vx, vy := (-x1p-cxp)/rx, (-y1p-cyp)/ry
	t1 := math.Atan2(uy, ux)
	dt := math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	}
	if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(dt) / 0.12))
	if n < 4 {
		n = 4
	}
	if n > 64 {
		n = 64
	}
	for i := 1; i <= n; i++ {
		t := t1 + dt*float64(i)/float64(n)
		ct, st := math.Cos(t), math.Sin(t)
		p.cur = append(p.cur, iconPt{
			X: cx + cos*rx*ct - sin*ry*st,
			Y: cy + sin*rx*ct + cos*ry*st,
		})
	}
	p.x, p.y = x, y
	p.ctrlKind = 0
}

// closeSub 结束当前子路径（只有一个点时丢弃）。
func (p *iconPathParser) closeSub() {
	if len(p.cur) >= 2 {
		p.subs = append(p.subs, iconSubPath{Pts: p.cur, Closed: true})
	}
	p.cur = nil
}

// iconCurveSteps 曲线细分步数：按控制多边形长度估算，0.7 网格单位一步。
func iconCurveSteps(d float64) int {
	n := int(math.Ceil(d / 0.7))
	if n < 3 {
		n = 3
	}
	if n > 48 {
		n = 48
	}
	return n
}

func iconDist(a, b iconPt) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// iconSegments 把子路径展开成线段集合（闭合路径补上收尾边）。
func iconSegments(subs []iconSubPath) []iconSeg {
	var out []iconSeg
	for _, sp := range subs {
		n := len(sp.Pts)
		if n < 2 {
			continue
		}
		for i := 1; i < n; i++ {
			out = append(out, iconSeg{
				AX: sp.Pts[i-1].X, AY: sp.Pts[i-1].Y,
				BX: sp.Pts[i].X, BY: sp.Pts[i].Y,
			})
		}
		if sp.Closed {
			out = append(out, iconSeg{
				AX: sp.Pts[n-1].X, AY: sp.Pts[n-1].Y,
				BX: sp.Pts[0].X, BY: sp.Pts[0].Y,
			})
		}
	}
	return out
}

// iconRaster 把折线光栅化成 size×size 的 RGBA 图像（白色 + alpha 覆盖率）。
//
// fill=true 走奇偶填充（4×4 超采样），否则按 stroke 描边宽度做解析式抗锯齿：
// 每个像素中心到最近线段的距离换算成覆盖率，边缘过渡刚好一像素宽。
func iconRaster(subs []iconSubPath, fill bool, stroke float64, size int) []byte {
	if size < 4 {
		size = 4
	}
	px := make([]byte, size*size*4)
	if len(subs) == 0 {
		return px
	}
	scale := float64(size) / iconGridSize
	if fill {
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				hit := 0
				for sy := 0; sy < iconRasterSS; sy++ {
					for sx := 0; sx < iconRasterSS; sx++ {
						dx := (float64(x) + (float64(sx)+0.5)/iconRasterSS) / scale
						dy := (float64(y) + (float64(sy)+0.5)/iconRasterSS) / scale
						if iconInside(subs, dx, dy) {
							hit++
						}
					}
				}
				if hit == 0 {
					continue
				}
				i := (y*size + x) * 4
				px[i], px[i+1], px[i+2] = 255, 255, 255
				px[i+3] = uint8((255*hit + iconRasterSS*iconRasterSS/2) / (iconRasterSS * iconRasterSS))
			}
		}
		return px
	}

	segs := iconSegments(subs)
	if len(segs) == 0 {
		return px
	}
	half := stroke / 2
	if half < 0.25 {
		half = 0.25
	}
	for y := 0; y < size; y++ {
		sy := (float64(y) + 0.5) / scale
		for x := 0; x < size; x++ {
			sx := (float64(x) + 0.5) / scale
			best := math.MaxFloat64
			for _, s := range segs {
				if d := iconSegDist2(sx, sy, s); d < best {
					best = d
					if best == 0 {
						break
					}
				}
			}
			// 设计网格距离 → 设备像素距离，再换成覆盖率
			dpx := math.Sqrt(best) * scale
			cov := (half*scale + 0.5) - dpx
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			i := (y*size + x) * 4
			px[i], px[i+1], px[i+2] = 255, 255, 255
			px[i+3] = uint8(255*cov + 0.5)
		}
	}
	return px
}

// iconSegDist2 点到线段的距离平方（零长度线段退化成点到点）。
func iconSegDist2(px, py float64, s iconSeg) float64 {
	dx, dy := s.BX-s.AX, s.BY-s.AY
	l2 := dx*dx + dy*dy
	if l2 <= 1e-12 {
		ax, ay := px-s.AX, py-s.AY
		return ax*ax + ay*ay
	}
	t := ((px-s.AX)*dx + (py-s.AY)*dy) / l2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	ax, ay := px-(s.AX+t*dx), py-(s.AY+t*dy)
	return ax*ax + ay*ay
}

// iconInside 奇偶规则判断点是否在闭合路径内。
func iconInside(subs []iconSubPath, x, y float64) bool {
	inside := false
	for _, sp := range subs {
		n := len(sp.Pts)
		if n < 2 {
			continue
		}
		for i := 0; i < n; i++ {
			j := i + 1
			if j == n {
				if !sp.Closed {
					break
				}
				j = 0
			}
			a, b := sp.Pts[i], sp.Pts[j]
			if (a.Y > y) == (b.Y > y) {
				continue
			}
			xint := a.X + (y-a.Y)/(b.Y-a.Y)*(b.X-a.X)
			if xint > x {
				inside = !inside
			}
		}
	}
	return inside
}
