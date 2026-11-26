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

func TestClipNearSplitsPolygon(t *testing.T) {
	// 一个顶点在相机后方、两个在前方的三角形：近平面裁剪后应仍画出像素，
	// 而不是整片丢弃，或写入越界/非法像素。
	eng := game.NewEngine(64, 64)
	eng.Scene.SetAmbientIntensity(1)
	frame, w, h := eng.RenderViewport(64, 64, 1.0/60.0)
	// RenderViewport 返回渲染器内部缓冲，多次调用共用同一底层数组，必须先拷贝。
	base := append([]byte(nil), frame...)

	m := &game.Mesh{
		// 相机默认位于 (0,0,5) 看向原点，Near = 0.1：p0 因此落在相机后方。
		Positions: []game.Vec3{game.V3(0, -1, 6), game.V3(-2, 0.5, 0), game.V3(2, 0.5, 0)},
		Indices:   []uint32{0, 1, 2},
	}
	m.RecomputeNormals()
	m.RecomputeBounds()
	mat := game.DefaultMaterial()
	mat.Shade = game.ShadeUnlit
	mat.DoubleSided = true
	eng.Scene.Add(game.NewMeshNode("tri", m, mat))

	pix, _, _ := eng.RenderViewport(64, 64, 1.0/60.0)
	if len(pix) != w*h*4 {
		t.Fatalf("像素缓冲长度 = %d，期望 %d", len(pix), w*h*4)
	}
	diff := 0
	for i := 0; i+3 < len(pix); i += 4 {
		if pix[i] != base[i] || pix[i+1] != base[i+1] || pix[i+2] != base[i+2] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatalf("跨近平面的三角形应被绘制，实际 %dx%d 画面无变化", w, h)
	}
}

func TestAnimatorTracks(t *testing.T) {
	n := game.NewNode("n")
	a := game.NewAnimator()
	a.MoveTo(n, game.V3(2, 0, 0), 1)
	a.Spin(n, game.V3(0, 1, 0), 1)
	if a.Count() != 2 {
		t.Fatalf("轨道数 = %d，期望 2", a.Count())
	}
	a.Update(0.5)
	if n.Pos.X <= 0 || n.Pos.X >= 2 {
		t.Fatalf("插值中途 x = %v，应在 (0,2) 之间", n.Pos.X)
	}
	if math.Abs(float64(n.Rot.Y-0.5)) > 1e-5 {
		t.Fatalf("绕 Y 旋转 = %v，期望 ≈0.5", n.Rot.Y)
	}
	a.Update(0.6) // 位移轨道结束，旋转轨道保留
	if n.Pos.X != 2 {
		t.Fatalf("轨道结束后 x = %v，期望 2", n.Pos.X)
	}
	if a.Count() != 1 {
		t.Fatalf("结束的位移轨道应被移除，剩 %d", a.Count())
	}
	a.Stop(n)
	if a.Count() != 0 {
		t.Fatalf("Stop 后应清空，剩 %d", a.Count())
	}
	a.Clear()
}

func TestEngineRenderViewport(t *testing.T) {
	e := game.NewEngine(96, 54)
	e.Scene.SetAmbientIntensity(0.4)
	box := game.NewMeshNode("box", game.NewBox(1, 1, 1), game.DefaultMaterial())
	e.Scene.Add(box)
	pix, w, h := e.RenderViewport(192, 108, 1.0/60.0)
	if w != 96 {
		t.Fatalf("内部渲染宽度 = %d，期望 96", w)
	}
	if cw, ch := e.ClientSize(); cw != 192 || ch != 108 {
		t.Fatalf("ClientSize = %d×%d，期望 192×108", cw, ch)
	}
	if hit, _, ok := e.PickClient(96, 54); !ok || hit != box {
		t.Fatalf("PickClient 应命中盒子，实际 hit=%v ok=%v", hit, ok)
	}
	if len(pix) != w*h*4 {
		t.Fatalf("像素长度 = %d，期望 %d", len(pix), w*h*4)
	}
	nonBg := 0
	for i := 3; i < len(pix); i += 4 {
		nonBg += int(pix[i])
	}
	if nonBg == 0 {
		t.Fatal("RenderViewport 未输出任何不透明像素")
	}
	sw, sh := e.Size()
	if sw != w || sh != h {
		t.Fatalf("Size = (%d,%d)，期望 (%d,%d)", sw, sh, w, h)
	}
}

func TestEngineHandleInput(t *testing.T) {
	e := game.NewEngine(64, 36)
	e.HandleKey("w", true)
	if !e.Input.KeyDown("w") {
		t.Fatal("HandleKey 未写入 Input")
	}
	e.HandleMouse(3, 4, 0, true)
	if !e.Input.Buttons[0] {
		t.Fatal("HandleMouse 未写入按键状态")
	}
	e.HandleWheel(1)
	if e.Input.Wheel != 1 {
		t.Fatalf("Wheel = %v", e.Input.Wheel)
	}
	e.Tick(1.0 / 60.0)
	if e.Input.Wheel != 0 {
		t.Fatal("Tick 后 AutoClearInput 应清空 Wheel")
	}
}

func TestCameraRay(t *testing.T) {
	c := game.NewCamera()
	origin, dir := c.Ray(50, 50, 100, 100)
	if !near(float64(origin.Z), 5, 1e-5) {
		t.Fatalf("射线起点 = %v，期望相机位置", origin)
	}
	if !near(float64(dir.Z), -1, 1e-4) {
		t.Fatalf("屏幕中心射线方向 = %v，期望沿着 -Z", dir)
	}
	c.Ortho = true
	_, dir2 := c.Ray(0, 0, 100, 100)
	if !near(float64(dir2.Z), -1, 1e-4) {
		t.Fatalf("正交射线方向 = %v", dir2)
	}
}

func TestLoadOBJ(t *testing.T) {
	src := `# 单三角形
group demo
v 0 0 0
v 1 0 0
v 0 1 0
vt 0 0
vt 1 0
vt 0 1
vn 0 0 1
f 1/1/1 2/2/1 3/3/1
`
	m, err := game.LoadOBJBytes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.TriCount() != 1 {
		t.Fatalf("三角数 = %d，期望 1", m.TriCount())
	}
	if len(m.Positions) != 3 || len(m.Normals) != 3 || len(m.UVs) != 3 {
		t.Fatalf("顶点/法线/UV 数量 = %d/%d/%d", len(m.Positions), len(m.Normals), len(m.UVs))
	}
	if m.Bounds.Max.X != 1 {
		t.Fatalf("包围盒 = %v", m.Bounds)
	}

	// 四边形应三角化成 2 个三角形
	quad := "v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nf 1 2 3 4\n"
	qm, err := game.LoadOBJBytes([]byte(quad))
	if err != nil {
		t.Fatal(err)
	}
	if qm.TriCount() != 2 {
		t.Fatalf("四边形三角化后三角数 = %d，期望 2", qm.TriCount())
	}
	if _, err := game.LoadOBJBytes([]byte("# empty\n")); err == nil {
		t.Fatal("空 OBJ 应返回错误")
	}
}

func TestGroundPlaneRenders(t *testing.T) {
	sc := game.NewScene()
	sc.SetAmbientIntensity(0.6)
	sc.AddSun(game.V3(-0.45, -1, -0.35), 1)
	sc.Add(game.NewMeshNode("ground", game.NewPlane(24, 24, 1),
		game.DefaultMaterial().WithColor(game.RGB(140, 150, 170)).WithShade(game.ShadeFlat)))
	sc.Camera.Pos = game.V3(0, 4, 6)
	sc.Camera.Target = game.V3(0, 0, 0)

	r := newTestRenderer(64, 64)
	img := r.Frame(sc)

	lit := 0
	for y := 32; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if c := px(img, x, y); c.R > 20 || c.G > 20 || c.B > 20 {
				lit++
			}
		}
	}
	if lit < 100 {
		t.Fatalf("俯视地面应有大量像素，实际下半屏亮像素仅 %d（Culled=%d, Tris=%d）",
			lit, r.Stats().Culled, r.Stats().Triangles)
	}
	// 回归：大型物体与视锥相交时不得被误剔除（世界包围盒 8 角需覆盖 min/max）
	if c := r.Stats().Culled; c != 0 {
		t.Fatalf("大地面不应被视锥剔除，Culled=%d", c)
	}
	if tr := r.Stats().Triangles; tr < 2 {
		t.Fatalf("地面至少应有 2 个三角形参与光栅化（裁剪可能拆出更多），实际 %d", tr)
	}
}

func TestColorHelpers(t *testing.T) {
	a, b := game.RGB(0, 0, 0), game.RGB(255, 255, 255)
	mid := a.Lerp(b, 0.5)
	if mid.R < 127 || mid.R > 128 {
		t.Fatalf("Lerp 中值 = %v", mid)
	}
	if c := game.RGB(255, 0, 0).Mul(game.RGB(128, 255, 255)); c.R != 128 {
		t.Fatalf("Mul = %v", c)
	}
	if c := game.RGB(100, 100, 100).Scale(0.5); c.R != 50 {
		t.Fatalf("Scale = %v", c)
	}
	if !game.RGB(1, 2, 3).Opaque() {
		t.Fatal("RGB 应不透明")
	}
	if game.RGB(1, 2, 3).WithAlpha(0).Opaque() {
		t.Fatal("alpha=0 不应被视为不透明")
	}
}

func TestMeshGenerators(t *testing.T) {
	for name, m := range map[string]*game.Mesh{
		"box":    game.NewBox(1, 2, 3),
		"sphere": game.NewSphere(1, 12, 8),
		"plane":  game.NewPlane(2, 2, 2),
		"cyl":    game.NewCylinder(1, 2, 12),
		"torus":  game.NewTorus(1, 0.3, 12, 8),
	} {
		if m.TriCount() == 0 {
			t.Fatalf("%s 没有三角形", name)
		}
		if len(m.Normals) != len(m.Positions) {
			t.Fatalf("%s 法线数 %d != 顶点数 %d", name, len(m.Normals), len(m.Positions))
		}
		if m.Bounds.Size().Len() == 0 {
			t.Fatalf("%s 包围盒为空", name)
		}
	}
}

func TestMaterialCloneAndSceneHelpers(t *testing.T) {
	m := game.DefaultMaterial().WithColor(game.RGB(1, 2, 3)).WithShade(game.ShadeToon)
	c := m.Clone()
	if c.BaseColor != m.BaseColor || c.Shade != m.Shade {
		t.Fatal("Clone 应复制字段")
	}
	c.BaseColor = game.RGB(9, 9, 9)
	if m.BaseColor == c.BaseColor {
		t.Fatal("Clone 应为独立副本")
	}

	sc := game.NewScene()
	sc.AddSun(game.V3(0, -1, 0), 1)
	if len(sc.Lights) != 1 || sc.Lights[0].Kind != game.LightDirectional {
		t.Fatalf("AddSun 后光源 = %v", sc.Lights)
	}
	sc.AddPointLight(game.V3(1, 1, 1), 5, 1)
	if len(sc.Lights) != 2 {
		t.Fatalf("点光源数量 = %d", len(sc.Lights))
	}
	sc.SetAmbientIntensity(1)
	if sc.Ambient.R != 255 {
		t.Fatalf("环境光 = %v", sc.Ambient)
	}
}

func TestSceneEachAndNodeHelpers(t *testing.T) {
	sc := game.NewScene()
	a := game.NewNode("a")
	b := game.NewNode("b")
	a.Add(b)
	sc.Add(a)
	n := 0
	sc.Each(func(*game.Node) { n++ })
	if n < 3 {
		t.Fatalf("Each 遍历节点数 = %d", n)
	}
	if !a.Visible || !a.Scl.Equal(game.V3(1, 1, 1)) {
		t.Fatalf("NewNode 默认值不正确: visible=%v scl=%v", a.Visible, a.Scl)
	}
}
