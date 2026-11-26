// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "math"

// Vec2 二维向量（屏幕坐标 / UV）。
type Vec2 struct {
	X, Y float32
}

// V2 构造二维向量。
func V2(x, y float32) Vec2 { return Vec2{X: x, Y: y} }

// Add 分量相加。
func (v Vec2) Add(o Vec2) Vec2 { return Vec2{v.X + o.X, v.Y + o.Y} }

// Sub 分量相减。
func (v Vec2) Sub(o Vec2) Vec2 { return Vec2{v.X - o.X, v.Y - o.Y} }

// Scale 按标量缩放。
func (v Vec2) Scale(s float32) Vec2 { return Vec2{v.X * s, v.Y * s} }

// Dot 点积。
func (v Vec2) Dot(o Vec2) float32 { return v.X*o.X + v.Y*o.Y }

// Len 长度。
func (v Vec2) Len() float32 { return float32(math.Hypot(float64(v.X), float64(v.Y))) }

// Normalize 归一化（零向量返回零）。
func (v Vec2) Normalize() Vec2 {
	l := v.Len()
	if l == 0 {
		return Vec2{}
	}
	return Vec2{v.X / l, v.Y / l}
}

// Vec3 三维向量（世界/模型空间）。
type Vec3 struct {
	X, Y, Z float32
}

// V3 构造三维向量。
func V3(x, y, z float32) Vec3 { return Vec3{X: x, Y: y, Z: z} }

// Add 分量相加。
func (v Vec3) Add(o Vec3) Vec3 { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }

// Sub 分量相减。
func (v Vec3) Sub(o Vec3) Vec3 { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }

// Mul 分量相乘。
func (v Vec3) Mul(o Vec3) Vec3 { return Vec3{v.X * o.X, v.Y * o.Y, v.Z * o.Z} }

// Scale 按标量缩放。
func (v Vec3) Scale(s float32) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Neg 取反。
func (v Vec3) Neg() Vec3 { return Vec3{-v.X, -v.Y, -v.Z} }

// Dot 点积。
func (v Vec3) Dot(o Vec3) float32 { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }

// Cross 叉积。
func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		v.Y*o.Z - v.Z*o.Y,
		v.Z*o.X - v.X*o.Z,
		v.X*o.Y - v.Y*o.X,
	}
}

// Len 长度。
func (v Vec3) Len() float32 { return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z))) }

// Dist 两点的距离。
func (v Vec3) Dist(o Vec3) float32 { return v.Sub(o).Len() }

// Normalize 归一化（零向量返回零）。
func (v Vec3) Normalize() Vec3 {
	l := v.Len()
	if l == 0 {
		return Vec3{}
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}
}

// Lerp 线性插值。
func (v Vec3) Lerp(o Vec3, t float32) Vec3 {
	return Vec3{v.X + (o.X-v.X)*t, v.Y + (o.Y-v.Y)*t, v.Z + (o.Z-v.Z)*t}
}

// Equal 分量全等（用于测试与断言）。
func (v Vec3) Equal(o Vec3) bool { return v.X == o.X && v.Y == o.Y && v.Z == o.Z }

// Min 分量最小值。
func (v Vec3) Min(o Vec3) Vec3 {
	return Vec3{minf(v.X, o.X), minf(v.Y, o.Y), minf(v.Z, o.Z)}
}

// Max 分量最大值。
func (v Vec3) Max(o Vec3) Vec3 {
	return Vec3{maxf(v.X, o.X), maxf(v.Y, o.Y), maxf(v.Z, o.Z)}
}

// Vec4 齐次坐标 / 颜色。
type Vec4 struct {
	X, Y, Z, W float32
}

// V4 构造四维向量。
func V4(x, y, z, w float32) Vec4 { return Vec4{X: x, Y: y, Z: z, W: w} }

// Vec3 取 xyz 分量。
func (v Vec4) Vec3() Vec3 { return Vec3{v.X, v.Y, v.Z} }

// Quat 四元数（单位四元数表示旋转）。
type Quat struct {
	X, Y, Z, W float32
}

// QuatIdentity 单位四元数（无旋转）。
func QuatIdentity() Quat { return Quat{0, 0, 0, 1} }

// QuatAxisAngle 绕轴旋转 rad 弧度。
func QuatAxisAngle(axis Vec3, rad float32) Quat {
	a := axis.Normalize()
	s := float32(math.Sin(float64(rad) / 2))
	return Quat{a.X * s, a.Y * s, a.Z * s, float32(math.Cos(float64(rad) / 2))}
}

// Mul 四元数相乘（q*p 表示先 p 后 q）。
func (q Quat) Mul(p Quat) Quat {
	return Quat{
		q.W*p.X + q.X*p.W + q.Y*p.Z - q.Z*p.Y,
		q.W*p.Y - q.X*p.Z + q.Y*p.W + q.Z*p.X,
		q.W*p.Z + q.X*p.Y - q.Y*p.X + q.Z*p.W,
		q.W*p.W - q.X*p.X - q.Y*p.Y - q.Z*p.Z,
	}
}

// Normalize 归一化（零四元数返回单位四元数）。
func (q Quat) Normalize() Quat {
	l := float32(math.Sqrt(float64(q.X*q.X + q.Y*q.Y + q.Z*q.Z + q.W*q.W)))
	if l == 0 {
		return QuatIdentity()
	}
	return Quat{q.X / l, q.Y / l, q.Z / l, q.W / l}
}

// Rotate 用四元数旋转向量。
func (q Quat) Rotate(v Vec3) Vec3 {
	q = q.Normalize()
	u := Vec3{q.X, q.Y, q.Z}
	s := q.W
	// v' = 2(u·v)u + (s²-u·u)v + 2s(u×v)
	uv := u.Dot(v)
	uu := u.Dot(u)
	return u.Scale(2 * uv).Add(v.Scale(s*s - uu)).Add(u.Cross(v).Scale(2 * s))
}

// Mat4 4x4 矩阵，行主序：m[行*4+列]，作用于列向量。
type Mat4 [16]float32

// Identity 返回单位矩阵。
func Identity() Mat4 {
	return Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// At 取第 r 行第 c 列（0 起）。
func (m Mat4) At(r, c int) float32 { return m[r*4+c] }

// Mul 矩阵相乘（m*n 表示先应用 n 再应用 m）。
func (m Mat4) Mul(n Mat4) Mat4 {
	var o Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			o[r*4+c] = m[r*4]*n[c] + m[r*4+1]*n[4+c] + m[r*4+2]*n[8+c] + m[r*4+3]*n[12+c]
		}
	}
	return o
}

// Translate 平移矩阵。
func Translate(v Vec3) Mat4 {
	return Mat4{1, 0, 0, v.X, 0, 1, 0, v.Y, 0, 0, 1, v.Z, 0, 0, 0, 1}
}

// Scale 缩放矩阵。
func Scale(v Vec3) Mat4 {
	return Mat4{v.X, 0, 0, 0, 0, v.Y, 0, 0, 0, 0, v.Z, 0, 0, 0, 0, 1}
}

// Rotate 绕任意轴旋转 rad 弧度。
func Rotate(axis Vec3, rad float32) Mat4 { return QuatAxisAngle(axis, rad).Mat4() }

// RotateEuler 欧拉角旋转（弧度，先 Z(roll)、再 X(pitch)、最后 Y(yaw)）。
func RotateEuler(pitch, yaw, roll float32) Mat4 {
	q := QuatAxisAngle(V3(0, 1, 0), yaw).Mul(
		QuatAxisAngle(V3(1, 0, 0), pitch)).Mul(
		QuatAxisAngle(V3(0, 0, 1), roll))
	return q.Mat4()
}

// Mat4 转旋转矩阵（行主序 3x3 嵌入 4x4）。
func (q Quat) Mat4() Mat4 {
	q = q.Normalize()
	x, y, z, w := q.X, q.Y, q.Z, q.W
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z
	return Mat4{
		1 - 2*(yy+zz), 2 * (xy - wz), 2 * (xz + wy), 0,
		2 * (xy + wz), 1 - 2*(xx+zz), 2 * (yz - wx), 0,
		2 * (xz - wy), 2 * (yz + wx), 1 - 2*(xx+yy), 0,
		0, 0, 0, 1,
	}
}

// Euler 从旋转矩阵反解欧拉角（pitch/yaw/roll，单位为弧度）。
func (m Mat4) Euler() (pitch, yaw, roll float32) {
	sy := float64(-m[8])
	if sy > 0.9999 {
		sy = 0.9999
	} else if sy < -0.9999 {
		sy = -0.9999
	}
	pitch = float32(math.Atan2(float64(m[9]), float64(m[10])))
	yaw = float32(math.Asin(sy))
	roll = float32(math.Atan2(float64(m[4]), float64(m[0])))
	return
}

// Perspective 右手系透视投影（相机看向 -Z，可见 z ∈ [-far, -near]）。
func Perspective(fovY, aspect, near, far float32) Mat4 {
	if aspect <= 0 {
		aspect = 1
	}
	if near <= 0 {
		near = 0.001
	}
	if far <= near {
		far = near + 1
	}
	f := 1 / float32(math.Tan(float64(fovY)/2))
	nf := 1 / (near - far)
	return Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) * nf, 2 * far * near * nf,
		0, 0, -1, 0,
	}
}

// Ortho 正交投影（相机看向 -Z）。
func Ortho(left, right, bottom, top, near, far float32) Mat4 {
	rw := right - left
	rh := top - bottom
	rd := far - near
	if rw == 0 {
		rw = 1
	}
	if rh == 0 {
		rh = 1
	}
	if rd == 0 {
		rd = 1
	}
	return Mat4{
		2 / rw, 0, 0, -(right + left) / rw,
		0, 2 / rh, 0, -(top + bottom) / rh,
		0, 0, -2 / rd, -(far + near) / rd,
		0, 0, 0, 1,
	}
}

// LookAt 视图矩阵（eye 看向 center，up 为上方向）。
func LookAt(eye, center, up Vec3) Mat4 {
	f := center.Sub(eye).Normalize()
	if f.Len() == 0 {
		f = V3(0, 0, -1)
	}
	s := f.Cross(up).Normalize()
	if s.Len() == 0 {
		// up 与视线平行时换一个参考轴，避免退化。
		alt := V3(0, 0, 1)
		if math.Abs(float64(f.Z)) > 0.9 {
			alt = V3(1, 0, 0)
		}
		s = f.Cross(alt).Normalize()
	}
	u := s.Cross(f)
	return Mat4{
		s.X, s.Y, s.Z, -s.Dot(eye),
		u.X, u.Y, u.Z, -u.Dot(eye),
		-f.X, -f.Y, -f.Z, f.Dot(eye),
		0, 0, 0, 1,
	}
}

// Invert 求逆（高斯-约当消元，带部分主元），奇异矩阵返回 ok=false。
func (m Mat4) Invert() (Mat4, bool) {
	var a [4][8]float64
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			a[r][c] = float64(m[r*4+c])
		}
		a[r][4+r] = 1
	}
	for col := 0; col < 4; col++ {
		p := col
		for r := col + 1; r < 4; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[p][col]) {
				p = r
			}
		}
		if math.Abs(a[p][col]) < 1e-12 {
			return Mat4{}, false
		}
		if p != col {
			a[p], a[col] = a[col], a[p]
		}
		piv := a[col][col]
		for c := 0; c < 8; c++ {
			a[col][c] /= piv
		}
		for r := 0; r < 4; r++ {
			if r == col {
				continue
			}
			f := a[r][col]
			if f == 0 {
				continue
			}
			for c := 0; c < 8; c++ {
				a[r][c] -= f * a[col][c]
			}
		}
	}
	var o Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			o[r*4+c] = float32(a[r][4+c])
		}
	}
	return o, true
}

// Transpose 转置。
func (m Mat4) Transpose() Mat4 {
	var o Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			o[r*4+c] = m[c*4+r]
		}
	}
	return o
}

// NormalMatrix 法线矩阵（左上 3x3 的逆转置），用于非等比缩放下的法线变换。
func (m Mat4) NormalMatrix() Mat4 {
	m3 := Mat4{
		m[0], m[1], m[2], 0,
		m[4], m[5], m[6], 0,
		m[8], m[9], m[10], 0,
		0, 0, 0, 1,
	}
	inv, ok := m3.Invert()
	if !ok {
		return Identity()
	}
	var o Mat4
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			o[r*4+c] = inv[c*4+r]
		}
	}
	o[15] = 1
	return o
}

// TransformVec4 作用于齐次向量（不除法）。
func (m Mat4) TransformVec4(v Vec4) Vec4 {
	return Vec4{
		m[0]*v.X + m[1]*v.Y + m[2]*v.Z + m[3]*v.W,
		m[4]*v.X + m[5]*v.Y + m[6]*v.Z + m[7]*v.W,
		m[8]*v.X + m[9]*v.Y + m[10]*v.Z + m[11]*v.W,
		m[12]*v.X + m[13]*v.Y + m[14]*v.Z + m[15]*v.W,
	}
}

// TransformPoint 变换点（含透视除法；w≈0 时不做除法）。
func (m Mat4) TransformPoint(v Vec3) Vec3 {
	r := m.TransformVec4(V4(v.X, v.Y, v.Z, 1))
	if r.W == 0 {
		return Vec3{r.X, r.Y, r.Z}
	}
	return Vec3{r.X / r.W, r.Y / r.W, r.Z / r.W}
}

// TransformDir 变换方向（忽略平移）。
func (m Mat4) TransformDir(v Vec3) Vec3 {
	return Vec3{
		m[0]*v.X + m[1]*v.Y + m[2]*v.Z,
		m[4]*v.X + m[5]*v.Y + m[6]*v.Z,
		m[8]*v.X + m[9]*v.Y + m[10]*v.Z,
	}
}

// TRS 组合平移 / 旋转（欧拉角，弧度）/ 缩放。
func TRS(pos, euler, scl Vec3) Mat4 {
	return Translate(pos).Mul(RotateEuler(euler.X, euler.Y, euler.Z)).Mul(Scale(scl))
}

// Clamp 把 v 限制在 [lo, hi]。
func Clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Lerp 标量线性插值。
func Lerp(a, b, t float32) float32 { return a + (b-a)*t }

// SmoothStep 平滑插值（t 会在 [0,1] 内截断）。
func SmoothStep(t float32) float32 {
	t = Clamp(t, 0, 1)
	return t * t * (3 - 2*t)
}

// Radians 角度转弧度。
func Radians(deg float32) float32 { return deg * float32(math.Pi) / 180 }

// Degrees 弧度转角度。
func Degrees(rad float32) float32 { return rad * 180 / float32(math.Pi) }

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
