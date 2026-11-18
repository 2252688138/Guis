// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import "math"

// 组件动画：绘制期顶点变换。
//
// 背景：Renderer 没有矩阵 / 变换栈，所有顶点位置都按 UI 逻辑坐标直接写进批次，
// 所以 ui 层的动画只能靠改布局（Frame / Style.DrawX）实现，缩放与整体淡出做不到。
// 这里补一个「顶点级」的变换栈：位置按轴心缩放 + 旋转 + 平移，顶点色 alpha 乘一个系数。
// 因为文字字形、图标、图片、圆角矩形最终都是顶点，所以压栈之后整棵子树
// （含文字）会一起平移 / 缩放 / 旋转 / 淡入淡出，调用方不需要改任何绘制代码。
//
// 不参与变换的部分：裁剪矩形（scissor，仍按原矩形）、毛玻璃、阴影 —— 它们
// 依赖屏幕矩形而不是顶点，动画期间按原位置计算，观感上等价于「内容在动、底板不动」。

// AnimTransform 一层动画变换：以 (PivotX, PivotY) 为轴心，先缩放再旋转，最后平移
// (DX, DY)，并把顶点色 alpha 乘上 Alpha。Scale 为 0 表示 1（不缩放），
// Rotate 为弧度（屏幕 y 向下，正值顺时针）。
type AnimTransform struct {
	PivotX, PivotY float32
	ScaleX, ScaleY float32
	Rotate         float32
	DX, DY         float32
	Alpha          float32
}

// animStack 当前生效的动画变换（已与父层复合）。
//
// 用仿射表示：x' = m00*x + m01*y + tx，y' = m10*x + m11*y + ty，
// 嵌套时线性部分相乘、平移项跟着父层走，因此子节点的动画会自然叠加在父节点的
// 动画之上（父淡入、旋转时子节点一起变）。
// 纯缩放是它的特例（m01 = m10 = 0），旋转才会出现非对角项。
type animStack struct {
	on                 bool
	m00, m01, m10, m11 float32 // 线性部分
	tx, ty, a          float32 // 平移与透明度
	saved              []float32
}

// animSaveStride 每层保存的 float 数：m00, m01, m10, m11, tx, ty, a。
const animSaveStride = 7

func (s *animStack) reset() {
	s.on = false
	s.m00, s.m01, s.m10, s.m11 = 1, 0, 0, 1
	s.tx, s.ty = 0, 0
	s.a = 1
	s.saved = s.saved[:0]
}

// PushAnim 压入一层动画变换（与当前层复合），必须与 PopAnim 成对调用。
func (r *Renderer) PushAnim(t AnimTransform) {
	s := &r.anim
	sx, sy := t.ScaleX, t.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	alpha := t.Alpha
	if alpha < 0 {
		alpha = 0
	}
	// 本层线性部分 = R(θ)·S(sx, sy)
	var m00, m01, m10, m11 float32
	if t.Rotate == 0 {
		m00, m01, m10, m11 = sx, 0, 0, sy
	} else {
		c := float32(math.Cos(float64(t.Rotate)))
		si := float32(math.Sin(float64(t.Rotate)))
		m00, m01 = c*sx, -si*sy
		m10, m11 = si*sx, c*sy
	}
	// 平移：p → L*(p-pivot) + pivot + d  ⟺  L*p + (pivot - L*pivot + d)
	ctx := t.PivotX - (m00*t.PivotX + m01*t.PivotY) + t.DX
	cty := t.PivotY - (m10*t.PivotX + m11*t.PivotY) + t.DY

	// 先存旧状态（首次压栈时旧状态是恒等，复位后就自然回到「无动画」）
	s.saved = append(s.saved, s.m00, s.m01, s.m10, s.m11, s.tx, s.ty, s.a)
	if !s.on {
		s.m00, s.m01, s.m10, s.m11 = m00, m01, m10, m11
		s.tx, s.ty = ctx, cty
		s.a = alpha
	} else {
		pm00, pm01, pm10, pm11 := s.m00, s.m01, s.m10, s.m11
		ptx, pty, pa := s.tx, s.ty, s.a
		s.m00 = pm00*m00 + pm01*m10
		s.m01 = pm00*m01 + pm01*m11
		s.m10 = pm10*m00 + pm11*m10
		s.m11 = pm10*m01 + pm11*m11
		s.tx = pm00*ctx + pm01*cty + ptx
		s.ty = pm10*ctx + pm11*cty + pty
		s.a = pa * alpha
	}
	s.on = true
}

// PopAnim 弹出一层动画变换。
func (r *Renderer) PopAnim() {
	s := &r.anim
	n := len(s.saved)
	if n < animSaveStride {
		s.reset()
		return
	}
	s.m00, s.m01 = s.saved[n-7], s.saved[n-6]
	s.m10, s.m11 = s.saved[n-5], s.saved[n-4]
	s.tx, s.ty = s.saved[n-3], s.saved[n-2]
	s.a = s.saved[n-1]
	s.saved = s.saved[:n-animSaveStride]
	s.on = len(s.saved) > 0
}

// apply 把当前变换作用到一个点上（未开启动画时原值返回）。
func (s *animStack) apply(x, y float32) (float32, float32) {
	if !s.on {
		return x, y
	}
	return s.m00*x + s.m01*y + s.tx, s.m10*x + s.m11*y + s.ty
}

// applyLinear 只作用线性部分（缩放 + 旋转，不含平移），
// 用于四边形顶点：顶点 = 变换后的中心 + applyLinear(局部偏移)。
func (s *animStack) applyLinear(x, y float32) (float32, float32) {
	if !s.on {
		return x, y
	}
	return s.m00*x + s.m01*y, s.m10*x + s.m11*y
}

// rotating 当前变换是否含旋转（含旋转时纯缩放的快捷算式不再成立）。
func (s *animStack) rotating() bool {
	return s.on && (s.m01 != 0 || s.m10 != 0)
}

// animScale 当前缩放系数（未开启动画时恒为 1；含旋转时按两条基向量长度折算）。
func (r *Renderer) animScale() (float32, float32) {
	if !r.anim.on {
		return 1, 1
	}
	if !r.anim.rotating() {
		return r.anim.m00, r.anim.m11
	}
	sx := float32(math.Hypot(float64(r.anim.m00), float64(r.anim.m10)))
	sy := float32(math.Hypot(float64(r.anim.m01), float64(r.anim.m11)))
	return sx, sy
}

// animOffset 当前位移（未开启动画时恒为 0）。
func (r *Renderer) animOffset() (float32, float32) {
	if !r.anim.on {
		return 0, 0
	}
	return r.anim.tx, r.anim.ty
}

// animAlpha 当前不透明度乘子（未开启动画时恒为 1）。
func (r *Renderer) animAlpha() float32 {
	if !r.anim.on {
		return 1
	}
	return r.anim.a
}

// AnimActive 是否有动画变换正在生效（ui 层据此决定是否持续重绘）。
func (r *Renderer) AnimActive() bool { return r.anim.on }
