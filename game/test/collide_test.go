// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"math"
	"testing"

	"guis/game"
)

func TestRayTriangleHit(t *testing.T) {
	a := game.Vec3{X: -1, Z: -3}
	b := game.Vec3{X: 1, Z: -3}
	c := game.Vec3{X: 0, Y: 1, Z: -3}

	tt, ok := game.RayTriangle(game.Vec3{}, game.Vec3{Z: -1}, a, b, c)
	if !ok || math.Abs(float64(tt-3)) > 1e-4 {
		t.Fatalf("命中三角形 t = %v ok=%v，期望 3", tt, ok)
	}
	// 偏到三角形外 → 不命中
	if _, ok := game.RayTriangle(game.Vec3{X: 5}, game.Vec3{Z: -1}, a, b, c); ok {
		t.Fatal("三角形外不应命中")
	}
	// 背向 → 不命中
	if _, ok := game.RayTriangle(game.Vec3{}, game.Vec3{Z: 1}, a, b, c); ok {
		t.Fatal("背离方向不应命中")
	}
}

func TestOBBCollide(t *testing.T) {
	o := game.AABBOBB(game.AABB{Min: game.Vec3{X: -1, Y: -1, Z: -5}, Max: game.Vec3{X: 1, Y: 1, Z: -3}})
	if !o.Contains(game.Vec3{Z: -4}) {
		t.Fatal("盒内点应 Contains")
	}
	if o.Contains(game.Vec3{X: 3, Z: -4}) {
		t.Fatal("盒外点不应 Contains")
	}
	if cp := o.ClosestPoint(game.Vec3{X: 5, Z: -4}); math.Abs(float64(cp.X-1)) > 1e-4 {
		t.Fatalf("最近点 X = %v，期望 1", cp.X)
	}
	tt, ok := o.Ray(game.Vec3{}, game.Vec3{Z: -1})
	if !ok || math.Abs(float64(tt-3)) > 1e-4 {
		t.Fatalf("OBB 射线 t = %v ok=%v，期望 3", tt, ok)
	}
	if _, ok := o.Ray(game.Vec3{X: 10}, game.Vec3{Z: -1}); ok {
		t.Fatal("偏开的射线不应命中 OBB")
	}

	// 旋转 45° 的盒子：轴向换成斜 45°，原本擦边的射线应该能命中
	c45 := float32(math.Cos(math.Pi / 4))
	s45 := float32(math.Sin(math.Pi / 4))
	rot := game.NewOBB(game.Vec3{Z: -4}, game.Vec3{X: 1, Y: 1, Z: 1},
		[3]game.Vec3{{X: c45, Y: s45}, {X: -s45, Y: c45}, {Z: 1}})
	if !rot.Contains(game.Vec3{Z: -4}) {
		t.Fatal("旋转盒子应包含中心")
	}
	// 世界 X=1.3 在旋转后的盒子内部（对角线方向扩展了半宽）
	if !rot.Contains(game.Vec3{X: 1.3, Z: -4}) {
		t.Fatal("旋转 45° 后 X=1.3 应在盒内")
	}
}

func TestCapsuleCollide(t *testing.T) {
	c := game.Capsule{A: game.Vec3{}, B: game.Vec3{Y: 2}, Radius: 0.5}
	if !c.Contains(game.Vec3{Y: 1, X: 0.4}) {
		t.Fatal("胶囊内点应 Contains")
	}
	if c.Contains(game.Vec3{Y: 1, X: 0.6}) {
		t.Fatal("胶囊外点不应 Contains")
	}
	if cp := c.ClosestPoint(game.Vec3{Y: 5, X: 3}); math.Abs(float64(cp.Y-2)) > 1e-4 {
		t.Fatalf("最近点 Y = %v，期望 2", cp.Y)
	}
	tt, ok := c.Ray(game.Vec3{X: -5, Y: 1}, game.Vec3{X: 1})
	if !ok || math.Abs(float64(tt-4.5)) > 1e-3 {
		t.Fatalf("胶囊射线 t = %v ok=%v，期望 4.5", tt, ok)
	}
	if _, ok := c.Ray(game.Vec3{X: -5, Y: 10}, game.Vec3{X: 1}); ok {
		t.Fatal("高处平行的射线不应命中胶囊")
	}
}

func TestRayMeshTriangle(t *testing.T) {
	m := game.NewMesh([]game.Vec3{
		{X: -1, Z: -5}, {X: 1, Z: -5}, {X: 0, Y: 1, Z: -5},
	}, []uint32{0, 1, 2})
	tt, tri, ok := game.RayMesh(game.Vec3{}, game.Vec3{Z: -1}, m, game.Identity())
	if !ok || tri != 0 || math.Abs(float64(tt-5)) > 1e-3 {
		t.Fatalf("RayMesh t = %v tri=%d ok=%v，期望 t≈5 tri=0", tt, tri, ok)
	}
	// 平移 +4：同一射线应得到 t ≈ 1
	tt, _, ok = game.RayMesh(game.Vec3{}, game.Vec3{Z: -1}, m, game.Translate(game.Vec3{Z: 4}))
	if !ok || math.Abs(float64(tt-1)) > 1e-3 {
		t.Fatalf("平移后 RayMesh t = %v ok=%v，期望 ≈1", tt, ok)
	}
}

func TestBodyStepMove(t *testing.T) {
	b := game.NewBody(game.Vec3{})
	// 台阶高度：x<2 为 0，x≥2 为 0.3
	ground := func(x, z float32) float32 {
		if x >= 2 {
			return 0.3
		}
		return 0
	}
	if !b.StepMove(game.Vec3{X: 1}, ground, 0.35) {
		t.Fatal("低台阶应该能走上去")
	}
	if math.Abs(float64(b.Pos.X-1)) > 1e-5 {
		t.Fatalf("移动后 X = %v，期望 1", b.Pos.X)
	}
	// 跨到 x=3：地面抬高 0.3，应自动抬升
	if !b.StepMove(game.Vec3{X: 2}, ground, 0.35) {
		t.Fatal("低台阶应可移动")
	}
	if math.Abs(float64(b.Pos.Y-0.3)) > 1e-5 {
		t.Fatalf("上台阶后 Y = %v，期望 0.3", b.Pos.Y)
	}
	// 高台阶：地面 1.5，应被挡住
	high := func(x, z float32) float32 {
		if x >= 6 {
			return 1.5
		}
		return 0.3
	}
	if b.StepMove(game.Vec3{X: 5}, high, 0.35) {
		t.Fatal("过高台阶应该挡住")
	}
	if math.Abs(float64(b.Pos.X-3)) > 1e-5 {
		t.Fatalf("被挡住后 X 应保持 3，实际 %v", b.Pos.X)
	}
}
