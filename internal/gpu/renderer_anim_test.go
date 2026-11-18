// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"guis/internal/geom"
	"math"
	"testing"
)

// 嵌套压栈应把父子两层的缩放 / 位移 / 透明度复合起来，出栈逐个还原。
func TestAnimTransformCompose(t *testing.T) {
	r := &Renderer{}
	r.anim.reset()
	if r.AnimActive() {
		t.Fatal("初始不应有动画")
	}

	r.PushAnim(AnimTransform{PivotX: 0, PivotY: 0, ScaleX: 2, ScaleY: 2, Alpha: 0.5})
	if sx, sy := r.animScale(); sx != 2 || sy != 2 {
		t.Fatalf("外层缩放应为 2，得到 %v/%v", sx, sy)
	}
	if a := r.animAlpha(); a != 0.5 {
		t.Fatalf("外层透明度应为 0.5，得到 %v", a)
	}

	// 内层：以 (100,0) 为轴心缩到 0.5 再右移 10 ⟹ x' = 0.5x + 60
	r.PushAnim(AnimTransform{PivotX: 100, ScaleX: 0.5, ScaleY: 0.5, DX: 10, Alpha: 0.5})
	if sx, sy := r.animScale(); sx != 1 || sy != 1 {
		t.Fatalf("复合缩放应为 1，得到 %v/%v", sx, sy)
	}
	if tx, ty := r.animOffset(); tx != 120 || ty != 0 {
		t.Fatalf("复合位移应为 (120,0)，得到 (%v,%v)", tx, ty)
	}
	if a := r.animAlpha(); a != 0.25 {
		t.Fatalf("复合透明度应为 0.25，得到 %v", a)
	}

	r.PopAnim()
	if tx, _ := r.animOffset(); tx != 0 {
		t.Fatalf("出栈后位移应回到 0，得到 %v", tx)
	}
	r.PopAnim()
	if r.AnimActive() {
		t.Fatal("全部出栈后应回到无动画状态")
	}
	if sx, _ := r.animScale(); sx != 1 {
		t.Fatalf("全部出栈后缩放应为 1，得到 %v", sx)
	}
	if a := r.animAlpha(); a != 1 {
		t.Fatalf("全部出栈后透明度应为 1，得到 %v", a)
	}
}

// 缩放为 0 视为「不缩放」，避免业务传零值把内容画成一点。
func TestAnimTransformZeroScale(t *testing.T) {
	r := &Renderer{}
	r.anim.reset()
	r.PushAnim(AnimTransform{PivotX: 0, PivotY: 0, ScaleX: 0, ScaleY: 0, Alpha: 1})
	if sx, sy := r.animScale(); sx != 1 || sy != 1 {
		t.Fatalf("0 缩放应按 1 处理，得到 %v/%v", sx, sy)
	}
	r.PopAnim()
}

// 变换必须真的落到顶点上：压栈后顶点位置按轴心缩放，半尺寸同比放大，
// 顶点色 alpha 乘上不透明度（绘制代码完全不用改）。
func TestAnimTransformReachesVertices(t *testing.T) {
	r := &Renderer{scale: 1}
	r.anim.reset()
	r.PushAnim(AnimTransform{PivotX: 50, PivotY: 50, ScaleX: 2, ScaleY: 2, Alpha: 0.5})
	r.appendQuad(geom.Rect{X: 40, Y: 40, W: 20, H: 20}, [4]float32{},
		Paint{R: 1, G: 1, B: 1, A: 1}, Paint{A: -1}, 0.7, 0, 0, 0)
	if r.shapeN != 6 {
		t.Fatalf("应写入 6 个顶点，得到 %d", r.shapeN)
	}
	d := r.shapeData
	// 顶点 0：原矩形左上角 (40,40)，轴心 (50,50) 放大 2 倍 → (30,30)
	if d[0] != 30 || d[1] != 30 {
		t.Fatalf("顶点位置应为 (30,30)，得到 (%v,%v)", d[0], d[1])
	}
	// 局部半尺寸保持原值（SDF 在未变换的局部空间里算），位置靠顶点仿射放大
	if d[12] != 10 || d[13] != 10 {
		t.Fatalf("半尺寸应保持 (10,10)，得到 (%v,%v)", d[12], d[13])
	}
	// 颜色 alpha 1 * 0.5
	if d[7] != 0.5 {
		t.Fatalf("顶点 alpha 应为 0.5，得到 %v", d[7])
	}
	r.PopAnim()
	// 出栈后同样的矩形应绘制在原地
	r.shapeData = r.shapeData[:0]
	r.shapeN = 0
	r.appendQuad(geom.Rect{X: 40, Y: 40, W: 20, H: 20}, [4]float32{},
		Paint{R: 1, G: 1, B: 1, A: 1}, Paint{A: -1}, 0.7, 0, 0, 0)
	if r.shapeData[0] != 40 || r.shapeData[7] != 1 {
		t.Fatalf("出栈后应恢复原值，得到位置 %v alpha %v", r.shapeData[0], r.shapeData[7])
	}
}

// 旋转：rect 以自身中心为轴心转 90°（屏幕 y 向下，正角顺时针），
// 左上角 (-10,-10) 的局部偏移应落到 (+10,-10)，即屏幕 (60,40)。
func TestAnimTransformRotate(t *testing.T) {
	r := &Renderer{scale: 1}
	r.anim.reset()
	r.PushAnim(AnimTransform{PivotX: 50, PivotY: 50, ScaleX: 1, ScaleY: 1, Rotate: float32(math.Pi / 2), Alpha: 1})
	r.appendQuad(geom.Rect{X: 40, Y: 40, W: 20, H: 20}, [4]float32{},
		Paint{R: 1, G: 1, B: 1, A: 1}, Paint{A: -1}, 0.7, 0, 0, 0)
	d := r.shapeData
	if math.Abs(float64(d[0]-60)) > 0.01 || math.Abs(float64(d[1]-40)) > 0.01 {
		t.Fatalf("旋转 90° 后左上角应在 (60,40)，得到 (%v,%v)", d[0], d[1])
	}
	if !r.anim.rotating() {
		t.Fatal("当前变换应被识别为含旋转")
	}
	if sx, sy := r.animScale(); math.Abs(float64(sx-1)) > 0.01 || math.Abs(float64(sy-1)) > 0.01 {
		t.Fatalf("纯旋转的等效缩放应为 1，得到 %v/%v", sx, sy)
	}
	r.PopAnim()
	if r.anim.rotating() || r.AnimActive() {
		t.Fatal("出栈后应回到无变换")
	}
}

// 嵌套的旋转 + 缩放要按矩阵复合：父层转 90°、子层放大 2 倍，
// 子层的轴心缩放先发生，再被父层旋转带走。
func TestAnimTransformRotateCompose(t *testing.T) {
	r := &Renderer{scale: 1}
	r.anim.reset()
	r.PushAnim(AnimTransform{PivotX: 0, PivotY: 0, ScaleX: 1, ScaleY: 1, Rotate: float32(math.Pi / 2), Alpha: 1})
	r.PushAnim(AnimTransform{PivotX: 0, PivotY: 0, ScaleX: 2, ScaleY: 2, Alpha: 1})
	// 线性部分 = R(90°)·S(2) → x 轴基向量 (2,0) 转成 (0,2)
	if gx, gy := r.anim.applyLinear(1, 0); math.Abs(float64(gx)) > 0.01 || math.Abs(float64(gy-2)) > 0.01 {
		t.Fatalf("(1,0) 应被映射到 (0,2)，得到 (%v,%v)", gx, gy)
	}
	if gx, gy := r.anim.applyLinear(0, 1); math.Abs(float64(gx+2)) > 0.01 || math.Abs(float64(gy)) > 0.01 {
		t.Fatalf("(0,1) 应被映射到 (-2,0)，得到 (%v,%v)", gx, gy)
	}
	r.PopAnim()
	r.PopAnim()
	if r.AnimActive() {
		t.Fatal("两层都出栈后应无动画")
	}
}
