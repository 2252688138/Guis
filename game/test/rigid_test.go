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

// rigidFinite 判断向量各分量是否有限。
func rigidFinite(v game.Vec3) bool {
	return !math.IsNaN(float64(v.X)) && !math.IsInf(float64(v.X), 0) &&
		!math.IsNaN(float64(v.Y)) && !math.IsInf(float64(v.Y), 0) &&
		!math.IsNaN(float64(v.Z)) && !math.IsInf(float64(v.Z), 0)
}

// TestRigidFreeFall 自由落体落地后静止。
func TestRigidFreeFall(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, -9.81, 0))
	ground := game.NewRigidBody(game.V3(0, 0, 0), 0, game.PlaneShape(game.V3(0, 1, 0)))
	ground.Friction = 0.6
	w.Add(ground)

	ball := game.NewRigidBody(game.V3(0, 3, 0), 1, game.SphereShape(0.5))
	ball.Restitution = 0
	ball.Friction = 0.5
	w.Add(ball)

	for i := 0; i < 180; i++ { // 3 秒
		w.Step(1.0 / 60)
	}
	if !rigidFinite(ball.Pos) || !rigidFinite(ball.Vel) {
		t.Fatalf("自由落体出现 NaN/Inf: pos=%v vel=%v", ball.Pos, ball.Vel)
	}
	if math.Abs(float64(ball.Pos.Y-0.5)) > 0.02 {
		t.Fatalf("落地高度错误: y=%v 期望 0.5", ball.Pos.Y)
	}
	if ball.Vel.Len() > 0.05 {
		t.Fatalf("落地后未静止: |v|=%v", ball.Vel.Len())
	}
}

// TestRigidAngularMomentum 无重力下角速度保持，2 秒朝向变化正确。
func TestRigidAngularMomentum(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, 0, 0))
	b := game.NewRigidBody(game.V3(0, 0, 0), 1, game.BoxShape(0.5, 1, 0.25))
	b.LinearDamping = 0
	b.AngularDamping = 0
	b.AngVel = game.V3(0, 2, 0)
	w.Add(b)

	for i := 0; i < 120; i++ { // 2 秒
		w.Step(1.0 / 60)
	}
	if !rigidFinite(b.AngVel) || !rigidFinite(b.Pos) {
		t.Fatalf("角动量测试出现 NaN/Inf")
	}
	if math.Abs(float64(b.AngVel.Len()-2)) > 0.01 {
		t.Fatalf("角速度未守恒: |w|=%v 期望 2", b.AngVel.Len())
	}
	if math.Abs(float64(b.AngVel.Y-2)) > 0.01 {
		t.Fatalf("角速度方向漂移: w=%v", b.AngVel)
	}
	want := game.QuatAxisAngle(game.V3(0, 1, 0), 4).Rotate(game.V3(1, 0, 0))
	got := b.Rot.Rotate(game.V3(1, 0, 0))
	if got.Dist(want) > 0.02 {
		t.Fatalf("2 秒后朝向错误: got=%v want=%v", got, want)
	}
}

// TestRigidBoxStack 100 个盒子堆叠 3 秒不发散。
func TestRigidBoxStack(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, -9.81, 0))
	ground := game.NewRigidBody(game.V3(0, 0, 0), 0, game.PlaneShape(game.V3(0, 1, 0)))
	ground.Friction = 0.8
	w.Add(ground)

	const half = 0.5
	const side = 5
	const layers = 4
	boxes := make([]*game.RigidBody, 0, side*side*layers)
	start := make([]game.Vec3, 0, side*side*layers)
	for l := 0; l < layers; l++ {
		for i := 0; i < side; i++ {
			for k := 0; k < side; k++ {
				p := game.V3(float32(i-side/2)*1.02, half+float32(l)*1.0, float32(k-side/2)*1.02)
				b := game.NewRigidBody(p, 1, game.BoxShape(half, half, half))
				b.Friction = 0.7
				b.Restitution = 0
				w.Add(b)
				boxes = append(boxes, b)
				start = append(start, b.Pos)
			}
		}
	}
	if len(boxes) != 100 {
		t.Fatalf("盒子数量错误: %d", len(boxes))
	}
	for i := 0; i < 180; i++ { // 3 秒
		w.Step(1.0 / 60)
	}
	st := w.Stats()
	if st.Bodies != 101 {
		t.Fatalf("统计刚体数错误: %d", st.Bodies)
	}
	maxErr := float32(0)
	for i, b := range boxes {
		if !rigidFinite(b.Pos) || !rigidFinite(b.Vel) || !rigidFinite(b.AngVel) {
			t.Fatalf("堆叠出现 NaN/Inf: body=%d pos=%v", i, b.Pos)
		}
		if d := b.Pos.Dist(start[i]); d > maxErr {
			maxErr = d
		}
	}
	if maxErr > 0.05 {
		t.Fatalf("堆叠漂移过大: maxErr=%v (上限 0.05)", maxErr)
	}
}

// TestRigidHingePendulum 铰链摆锤周期接近解析值。
func TestRigidHingePendulum(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, -9.81, 0))
	pivot := game.V3(0, 2, 0)
	anchor := game.NewRigidBody(pivot, 0, game.SphereShape(0.05))
	w.Add(anchor)

	const hy = 1.0 // 摆杆半长
	rod := game.NewRigidBody(game.V3(0, 0, 0), 1, game.BoxShape(0.05, hy, 0.05))
	rod.LinearDamping = 0
	rod.AngularDamping = 0
	const amp = 0.2
	rot := game.QuatAxisAngle(game.V3(0, 0, 1), amp)
	rod.Rot = rot
	rod.Pos = pivot.Add(rot.Rotate(game.V3(0, -hy, 0)))
	w.Add(rod)
	w.AddJoint(game.NewHingeJoint(anchor, rod, pivot, game.V3(0, 0, 1)))

	dt := float32(1.0 / 60.0)
	cross := make([]float32, 0, 8)
	prev := rod.Pos.Sub(pivot).X
	tm := float32(0)
	for i := 0; i < 360; i++ { // 6 秒
		w.Step(dt)
		tm += dt
		if !rigidFinite(rod.Pos) {
			t.Fatalf("摆锤出现 NaN/Inf: pos=%v", rod.Pos)
		}
		cur := rod.Pos.Sub(pivot).X
		if (prev > 0 && cur <= 0) || (prev < 0 && cur >= 0) {
			cross = append(cross, tm)
		}
		prev = cur
	}
	if len(cross) < 3 {
		t.Fatalf("摆锤没有正常摆动: 过零次数=%d", len(cross))
	}
	halfPeriod := float32(0)
	n := 0
	for i := 1; i < len(cross); i++ {
		halfPeriod += cross[i] - cross[i-1]
		n++
	}
	halfPeriod /= float32(n)
	period := halfPeriod * 2
	// 细杆绕端点摆动：T = 2π√(4L/(3g)), L = hy
	want := float32(2 * math.Pi * math.Sqrt(4*float64(hy)/(3*9.81)))
	if rel := math.Abs(float64(period-want)) / float64(want); rel > 0.08 {
		t.Fatalf("摆锤周期偏差过大: got=%v want=%v 相对误差=%.3f", period, want, rel)
	}
}

// TestRigidDistanceJoint 距离约束保持长度。
func TestRigidDistanceJoint(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, -9.81, 0))
	anchor := game.NewRigidBody(game.V3(0, 5, 0), 0, game.SphereShape(0.1))
	w.Add(anchor)
	bob := game.NewRigidBody(game.V3(2, 5, 0), 1, game.SphereShape(0.2))
	w.Add(bob)
	w.AddJoint(game.NewDistanceJoint(anchor, bob, 1.5))

	for i := 0; i < 300; i++ {
		w.Step(1.0 / 60)
	}
	d := anchor.Pos.Dist(bob.Pos)
	if !rigidFinite(bob.Pos) {
		t.Fatalf("距离关节出现 NaN/Inf")
	}
	if math.Abs(float64(d-1.5)) > 0.02 {
		t.Fatalf("距离约束失效: d=%v 期望 1.5", d)
	}
}

// TestRigidSpring 弹簧收敛到静止长度。
func TestRigidSpring(t *testing.T) {
	w := game.NewRigidWorld(game.V3(0, 0, 0))
	anchor := game.NewRigidBody(game.V3(0, 0, 0), 0, game.SphereShape(0.1))
	w.Add(anchor)
	bob := game.NewRigidBody(game.V3(2.5, 0, 0), 1, game.SphereShape(0.2))
	bob.LinearDamping = 0
	w.Add(bob)
	w.AddJoint(game.NewSpringJoint(anchor, bob, 1.0, 60, 8))

	for i := 0; i < 600; i++ { // 10 秒
		w.Step(1.0 / 60)
	}
	d := anchor.Pos.Dist(bob.Pos)
	if !rigidFinite(bob.Pos) || !rigidFinite(bob.Vel) {
		t.Fatalf("弹簧出现 NaN/Inf")
	}
	if math.Abs(float64(d-1.0)) > 0.05 {
		t.Fatalf("弹簧未收敛: d=%v 期望 1.0", d)
	}
	if bob.Vel.Len() > 0.05 {
		t.Fatalf("弹簧未静止: |v|=%v", bob.Vel.Len())
	}
}
