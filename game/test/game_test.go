// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"guis/game"
)

// ---- 小工具 ----

func px(img *image.RGBA, x, y int) game.RGBA {
	j := y*img.Stride + x*4
	return game.RGBA{R: img.Pix[j], G: img.Pix[j+1], B: img.Pix[j+2], A: img.Pix[j+3]}
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func triMesh() *game.Mesh {
	m := &game.Mesh{
		Positions: []game.Vec3{game.V3(-1, -1, 0), game.V3(1, -1, 0), game.V3(0, 1, 0)},
		Indices:   []uint32{0, 1, 2},
	}
	m.RecomputeNormals()
	m.RecomputeBounds()
	return m
}

func simpleScene(mat *game.Material, mesh *game.Mesh) *game.Scene {
	sc := game.NewScene()
	sc.SetAmbientIntensity(0)
	sc.Lights = nil
	sc.Background = game.RGB(0, 0, 0)
	if mesh == nil {
		mesh = triMesh()
	}
	sc.Add(game.NewMeshNode("tri", mesh, mat))
	return sc
}

func newTestRenderer(w, h int) *game.Renderer {
	r := game.NewRenderer(w, h)
	r.BgColor = game.RGB(0, 0, 0)
	r.Threads = 1
	return r
}

// ---- B2 数学 ----

func TestMat4Invert(t *testing.T) {
	m := game.Translate(game.V3(1, 2, 3)).Mul(game.RotateEuler(0.3, 0.7, 0.2)).Mul(game.Scale(game.V3(2, 3, 0.5)))
	inv, ok := m.Invert()
	if !ok {
		t.Fatal("矩阵应可逆")
	}
	id := m.Mul(inv)
	for i := 0; i < 16; i++ {
		want := float32(0)
		if i%5 == 0 {
			want = 1
		}
		if !near(float64(id[i]), float64(want), 1e-4) {
			t.Fatalf("M*M⁻¹ 第 %d 项 = %v，期望 %v", i, id[i], want)
		}
	}
}

func TestPerspectiveAndLookAt(t *testing.T) {
	p := game.Perspective(game.Radians(90), 1, 0.1, 100)
	// 距离 1 的平面上，x = 1 应恰好落在 NDC 右边界
	v := p.TransformVec4(game.V4(1, 0, -1, 1))
	if !near(float64(v.X/v.W), 1, 1e-4) {
		t.Fatalf("NDC x = %v，期望 1", v.X/v.W)
	}
	view := game.LookAt(game.V3(0, 0, 5), game.V3(0, 0, 0), game.V3(0, 1, 0))
	got := view.TransformPoint(game.V3(0, 0, 0))
	if !near(float64(got.Z), -5, 1e-4) {
		t.Fatalf("视空间原点 z = %v，期望 -5", got.Z)
	}
}

func TestQuatRotateZ90(t *testing.T) {
	q := game.QuatAxisAngle(game.V3(0, 0, 1), float32(math.Pi)/2)
	v := q.Rotate(game.V3(1, 0, 0))
	if !near(float64(v.X), 0, 1e-5) || !near(float64(v.Y), 1, 1e-5) {
		t.Fatalf("绕 Z 转 90° 后 (1,0,0) → %v", v)
	}
}

// ---- B3 / B5 光栅化与背面剔除 ----

func TestRasterFrontTriangle(t *testing.T) {
	r := newTestRenderer(64, 64)
	mat := game.DefaultMaterial().WithColor(game.RGB(255, 0, 0)).WithShade(game.ShadeUnlit)
	img := r.Frame(simpleScene(mat, nil))

	st := r.Stats()
	if st.Triangles != 1 {
		t.Fatalf("应有 1 个三角形参与光栅化，实际 %d（Culled=%d）", st.Triangles, st.Culled)
	}
	if st.Pixels == 0 {
		t.Fatal("正面三角形未产生任何像素（绕序/背剔方向可能反了）")
	}
	c := px(img, 32, 40)
	if c.R != 255 || c.G != 0 || c.B != 0 {
		t.Fatalf("三角形内部像素 = %v，期望纯红", c)
	}
	if bg := px(img, 1, 1); bg.R != 0 || bg.G != 0 || bg.B != 0 {
		t.Fatalf("背景像素 = %v，期望黑色", bg)
	}
}

func TestRasterBackfaceCulled(t *testing.T) {
	m := &game.Mesh{
		Positions: []game.Vec3{game.V3(-1, -1, 0), game.V3(1, -1, 0), game.V3(0, 1, 0)},
		Indices:   []uint32{0, 2, 1}, // 反向绕序
	}
	m.RecomputeNormals()
	m.RecomputeBounds()
	r := newTestRenderer(32, 32)
	mat := game.DefaultMaterial().WithColor(game.RGB(0, 255, 0)).WithShade(game.ShadeUnlit)
	r.Frame(simpleScene(mat, m))
	if got := r.Stats().Triangles; got != 0 {
		t.Fatalf("背面三角形应被剔除，实际 Triangles=%d", got)
	}
}

// ---- B4 Z-Buffer ----

func TestZBufferOrderIndependent(t *testing.T) {
	mk := func(firstNear bool) game.RGBA {
		sc := game.NewScene()
		sc.SetAmbientIntensity(0)
		sc.Lights = nil
		nearMesh := game.NewBox(2, 2, 1)
		farMesh := game.NewBox(2, 2, 1)
		nearMat := game.DefaultMaterial().WithColor(game.RGB(255, 0, 0)).WithShade(game.ShadeUnlit)
		farMat := game.DefaultMaterial().WithColor(game.RGB(0, 0, 255)).WithShade(game.ShadeUnlit)
		nn := game.NewMeshNode("near", nearMesh, nearMat)
		nn.Pos = game.V3(0, 0, 1)
		fn := game.NewMeshNode("far", farMesh, farMat)
		fn.Pos = game.V3(0, 0, -1)
		if firstNear {
			sc.Add(nn, fn)
		} else {
			sc.Add(fn, nn)
		}
		sc.Camera.Up = game.V3(0, 1, 0)
		r := newTestRenderer(32, 32)
		return px(r.Frame(sc), 16, 16)
	}
	a := mk(true)
	b := mk(false)
	if a.R < 200 || a.B > 60 {
		t.Fatalf("近处红色平面应遮住远处蓝色，实际 = %v", a)
	}
	if a != b {
		t.Fatalf("绘制顺序影响了深度结果：%v vs %v", a, b)
	}
}

// ---- B6 近平面裁剪 ----

func TestNearPlaneClipNoPanic(t *testing.T) {
	sc := simpleScene(game.DefaultMaterial().WithColor(game.RGB(255, 255, 255)).WithShade(game.ShadeUnlit), nil)
	sc.Camera.Pos = game.V3(0, 0, 0) // 三角形位于 z=0，跨在近平面上
	r := newTestRenderer(32, 32)
	r.Frame(sc) // 不 panic 即通过；跨平面三角形应当不崩溃

	// 完全在相机背后：不产生像素
	sc2 := simpleScene(game.DefaultMaterial().WithColor(game.RGB(255, 255, 255)).WithShade(game.ShadeUnlit), nil)
	sc2.Root.Children[0].Pos = game.V3(0, 0, 8) // 相机在 +Z 看向 -Z，物体在相机后方
	r2 := newTestRenderer(32, 32)
	r2.Frame(sc2)
	if r2.Stats().Pixels != 0 {
		t.Fatalf("相机后方物体不应产生像素，实际 %d", r2.Stats().Pixels)
	}
}

// ---- B7 纹理 ----

func TestTextureSampling(t *testing.T) {
	tex := game.NewTexture(2, 2)
	tex.Pix = []game.RGBA{game.RGB(255, 0, 0), game.RGB(0, 255, 0), game.RGB(0, 0, 255), game.RGB(255, 255, 255)}
	if c := tex.SampleNearest(0.25, 0.25); c != game.RGB(255, 0, 0) {
		t.Fatalf("最近邻 (0.25,0.25) = %v", c)
	}
	if c := tex.SampleNearest(0.75, 0.25); c != game.RGB(0, 255, 0) {
		t.Fatalf("最近邻 (0.75,0.25) = %v", c)
	}
	mid := tex.SampleLinear(0.5, 0.25)
	if mid.R == 0 || mid.R == 255 {
		t.Fatalf("双线性插值应介于两色之间，实际 %v", mid)
	}
}

func TestTextureLoadPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checker.png")
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(1, 0, color.NRGBA{0, 255, 0, 255})
	img.Set(0, 1, color.NRGBA{0, 0, 255, 255})
	img.Set(1, 1, color.NRGBA{255, 255, 255, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	tex, err := game.LoadTexture(path)
	if err != nil {
		t.Fatal(err)
	}
	if tex.W != 2 || tex.H != 2 {
		t.Fatalf("尺寸 = %d×%d，期望 2×2", tex.W, tex.H)
	}
	if c := tex.At(0, 0); c.R != 255 || c.G != 0 {
		t.Fatalf("(0,0) = %v，期望红", c)
	}
}

// ---- B8 光照 ----

func avgBrightness(img *image.RGBA) float64 {
	sum, n := 0.0, 0
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			c := px(img, x, y)
			if c.R == 0 && c.G == 0 && c.B == 0 {
				continue // 背景
			}
			sum += float64(c.R) + float64(c.G) + float64(c.B)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n) / 3
}

func TestLightingBrightVsDark(t *testing.T) {
	render := func(lightDir game.Vec3) float64 {
		sc := simpleScene(game.DefaultMaterial().WithColor(game.RGB(200, 200, 200)).WithShade(game.ShadePhong), nil)
		sc.Ambient = game.RGB(0, 0, 0)
		sc.AddSun(lightDir, 1)
		r := newTestRenderer(32, 32)
		return avgBrightness(r.Frame(sc))
	}
	bright := render(game.V3(0, 0, -1)) // 从相机侧照向三角形
	dark := render(game.V3(0, 0, 1))    // 从背面照
	if bright <= 0 {
		t.Fatal("正对光源应有亮度")
	}
	if (bright-dark)/bright < 0.2 {
		t.Fatalf("光照差异不足：bright=%.1f dark=%.1f", bright, dark)
	}
}

// ---- B9 场景树 ----

func TestNodeHierarchyWorldMatrix(t *testing.T) {
	root := game.NewNode("root")
	child := game.NewNode("child")
	child.Pos = game.V3(2, 0, 0)
	grand := game.NewNode("grand")
	grand.Pos = game.V3(0, 3, 0)
	root.Add(child)
	child.Add(grand)
	root.BuildWorld(game.Identity())

	got := grand.WorldPos()
	if !near(float64(got.X), 2, 1e-5) || !near(float64(got.Y), 3, 1e-5) {
		t.Fatalf("孙节点世界坐标 = %v，期望 (2,3,0)", got)
	}
	if root.Find("grand") != grand {
		t.Fatal("Find 未找到孙节点")
	}
	if root.Find("missing") != nil {
		t.Fatal("Find 不应命中不存在的名字")
	}
}

// ---- B10 视锥剔除 / 相机 ----

func TestFrustumCullsBehindCamera(t *testing.T) {
	sc := game.NewScene()
	sc.SetAmbientIntensity(1)
	box := game.NewMeshNode("box", game.NewBox(1, 1, 1), game.DefaultMaterial())
	box.Pos = game.V3(0, 0, 20) // 相机在 z=5 看向 -Z，物体在身后
	sc.Add(box)
	r := newTestRenderer(32, 32)
	r.Frame(sc)
	if r.Stats().Culled == 0 {
		t.Fatalf("相机后的物体应被视锥剔除，Culled=%d", r.Stats().Culled)
	}
	if r.Stats().Triangles != 0 {
		t.Fatalf("相机后的物体不应参与光栅化，Triangles=%d", r.Stats().Triangles)
	}
}

func TestOrthoCamera(t *testing.T) {
	c := game.NewCamera()
	c.Ortho = true
	c.OrthoSize = 2
	c.Pos = game.V3(0, 0, 5)
	c.Target = game.V3(0, 0, 0)
	p := c.Proj(1)
	// 正交视野半高 = 2，所以 y=2 应落在 NDC 上边界
	v := p.TransformVec4(game.V4(0, 2, -5, 1))
	if !near(float64(v.Y), 1, 1e-4) {
		t.Fatalf("正交 NDC y = %v，期望 1", v.Y)
	}
}

// ---- B11 输入 ----

func TestInputSemantics(t *testing.T) {
	in := game.NewInput()
	in.HandleKey("W", true)
	if !in.KeyDown("w") || !in.KeyPressed("w") {
		t.Fatal("按下后 KeyDown/KeyPressed 应为真")
	}
	if !near(float64(in.Axis("s", "w")), 1, 1e-6) {
		t.Fatalf("Axis = %v，期望 1", in.Axis("s", "w"))
	}
	in.EndFrame()
	if in.KeyPressed("w") {
		t.Fatal("EndFrame 后 Pressed 应清空")
	}
	if !in.KeyDown("w") {
		t.Fatal("EndFrame 不应清除按住状态")
	}
	in.HandleKey("w", false)
	if !in.KeyReleased("w") || in.KeyDown("w") {
		t.Fatal("抬起后 KeyReleased 应为真、KeyDown 应为假")
	}
	in.HandleMouse(10, 10, -1, false)
	in.HandleMouse(13, 14, -1, false)
	if !near(float64(in.MouseDX), 3, 1e-6) || !near(float64(in.MouseDY), 4, 1e-6) {
		t.Fatalf("鼠标位移 = (%.1f,%.1f)", in.MouseDX, in.MouseDY)
	}
}

// ---- B12 固定步长 ----

func TestLoopFixedStep(t *testing.T) {
	c := &counter{}
	g := &loopProbe{c: c}
	l := game.NewLoop(g)
	l.Step(0.1, nil) // 100ms / (1/60) = 6 步
	if c.fixed != 6 {
		t.Fatalf("fixed update 次数 = %d，期望 6", c.fixed)
	}
	if c.update != 1 {
		t.Fatalf("update 次数 = %d，期望 1", c.update)
	}
}

type loopProbe struct{ c *counter }

type counter struct{ fixed, update int }

func (p *loopProbe) Init(*game.World)    {}
func (p *loopProbe) Update(float32)      { p.c.update++ }
func (p *loopProbe) FixedUpdate(float32) { p.c.fixed++ }
func (p *loopProbe) Draw(*game.Scene)    {}

// ---- B13 碰撞 ----

func TestRayIntersections(t *testing.T) {
	b := game.AABB{Min: game.V3(-1, -1, -1), Max: game.V3(1, 1, 1)}
	if tt, hit := game.RayAABB(game.V3(0, 0, 5), game.V3(0, 0, -1), b); !hit || !near(float64(tt), 4, 1e-4) {
		t.Fatalf("RayAABB t=%v hit=%v，期望 4", tt, hit)
	}
	if _, hit := game.RayAABB(game.V3(3, 0, 5), game.V3(0, 0, -1), b); hit {
		t.Fatal("射线未命中时不应返回 hit")
	}
	if tt, hit := game.RaySphere(game.V3(0, 0, 5), game.V3(0, 0, -1), game.Sphere{C: game.V3(0, 0, 0), R: 1}); !hit || !near(float64(tt), 4, 1e-4) {
		t.Fatalf("RaySphere t=%v hit=%v，期望 4", tt, hit)
	}
	if tt, hit := game.RayTriangle(game.V3(0, 0, 5), game.V3(0, 0, -1), game.V3(-1, -1, 0), game.V3(1, -1, 0), game.V3(0, 1, 0)); !hit || !near(float64(tt), 5, 1e-4) {
		t.Fatalf("RayTriangle t=%v hit=%v，期望 5", tt, hit)
	}
}

func TestRayMeshAndPick(t *testing.T) {
	e := game.NewEngine(64, 36)
	box := game.NewMeshNode("box", game.NewBox(1, 1, 1), game.DefaultMaterial())
	e.Scene.Add(box)
	e.Tick(1.0 / 60.0)
	hit, _, ok := e.Pick(32, 18, 64, 36)
	if !ok || hit != box {
		t.Fatalf("射线拾取应命中盒子，实际 hit=%v ok=%v", hit, ok)
	}
}

func TestSphereAABBPushOut(t *testing.T) {
	b := game.AABB{Min: game.V3(-1, -1, -1), Max: game.V3(1, 1, 1)}
	if _, hit := game.SphereAABB(game.Sphere{C: game.V3(5, 0, 0), R: 1}, b); hit {
		t.Fatal("分离时不应推出")
	}
	push, hit := game.SphereAABB(game.Sphere{C: game.V3(1.5, 0, 0), R: 1}, b)
	if !hit || push.X <= 0 {
		t.Fatalf("应沿 +X 推出，push=%v hit=%v", push, hit)
	}
}

// ---- B14 2D 叠加 ----

func TestSpriteBatch(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 8, 8))
	tex := game.NewTexture(2, 2)
	tex.Fill(game.RGB(0, 200, 0))
	b := game.NewSpriteBatch()
	b.Add(game.Sprite{Pos: game.V2(2, 2), Size: game.V2(4, 4), Texture: tex})
	b.DrawSprites(dst, 8, 8)
	c := px(dst, 4, 4)
	if c.G != 200 || c.A != 255 {
		t.Fatalf("精灵绘制像素 = %v，期望绿色不透明", c)
	}
	if b.SpriteCount() != 1 {
		t.Fatalf("SpriteCount = %d", b.SpriteCount())
	}
	b.Clear()
	if b.SpriteCount() != 0 {
		t.Fatal("Clear 后应为 0")
	}
}

func TestSpriteSrcRectAndTilemap(t *testing.T) {
	tex := game.NewTexture(4, 4)
	tex.Fill(game.RGB(10, 10, 10))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			tex.Set(x, y, game.RGB(250, 50, 50))
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 16, 16))
	game.DrawSprite(dst, game.Sprite{Pos: game.V2(0, 0), Size: game.V2(8, 8), Texture: tex, Src: game.Rect2{X: 0, Y: 0, W: 2, H: 2}})
	if c := px(dst, 4, 4); c.R != 250 {
		t.Fatalf("裁剪绘制像素 = %v", c)
	}

	atlas := game.NewAtlas(tex, 2, 2, 2)
	tm := game.NewTilemap(3, 3, 4, 4, atlas)
	tm.Set(1, 1, 0)
	if tm.SpriteCount() != 1 {
		t.Fatalf("瓦片数 = %d", tm.SpriteCount())
	}
	tm.DrawSprites(dst, 16, 16) // 越界/空瓦片不应 panic
}

// ---- B15 无每帧泄漏 ----

func TestNoFrameLeak(t *testing.T) {
	r := game.NewRenderer(160, 90)
	sc := game.NewScene()
	sc.SetAmbientIntensity(0.3)
	sc.AddSun(game.V3(-0.4, -1, -0.3), 0.9)
	sc.Add(game.NewMeshNode("box", game.NewBox(1, 1, 1), game.DefaultMaterial()))

	r.Frame(sc) // 预热
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 300; i++ {
		r.Frame(sc)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if growth := int64(after.HeapAlloc) - int64(before.HeapAlloc); growth > 5<<20 {
		t.Fatalf("300 帧后堆增长 %d 字节，超过 5MB（可能存在每帧泄漏）", growth)
	}
}

// ---- C1 / C2 性能基准 ----

func benchScene() *game.Scene {
	sc := game.NewScene()
	sc.SetAmbientIntensity(0.25)
	sc.AddSun(game.V3(-0.4, -1, -0.3), 1)
	sc.AddPointLight(game.V3(3, 3, 3), 10, 0.6)
	mesh := game.NewSphere(1, 24, 16) // ≈ 768 三角
	mat := game.DefaultMaterial().WithTexture(game.CheckerTexture(8, game.RGB(200, 200, 210), game.RGB(90, 100, 120)))
	for i := 0; i < 2; i++ {
		n := game.NewMeshNode("s", mesh, mat)
		n.Pos = game.V3(float32(i)*2.5-1.2, 0, 0)
		sc.Add(n)
	}
	sc.Camera.Pos = game.V3(0, 1.5, 6)
	sc.Camera.Target = game.V3(0, 0, 0)
	return sc
}

func BenchmarkFrame(b *testing.B) {
	r := game.NewRenderer(640, 360)
	sc := benchScene()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(sc)
	}
}

func BenchmarkFrameThreads4(b *testing.B) {
	r := game.NewRenderer(640, 360)
	r.Threads = 4
	sc := benchScene()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(sc)
	}
}

// ---- P2-3 超采样抗锯齿 ----

func TestSupersampleAntialias(t *testing.T) {
	stats := func(ss int) (partial, fg int) {
		r := newTestRenderer(48, 48)
		r.Supersample = ss
		mat := game.DefaultMaterial().WithColor(game.RGB(255, 0, 0)).WithShade(game.ShadeUnlit)
		img := r.Frame(simpleScene(mat, nil))
		if img.Rect.Dx() != 48 || img.Rect.Dy() != 48 {
			t.Fatalf("ss=%d 输出尺寸 = %v，应始终为 48×48", ss, img.Rect)
		}
		for y := 0; y < 48; y++ {
			for x := 0; x < 48; x++ {
				c := px(img, x, y)
				switch {
				case c.R > 10 && c.R < 245:
					partial++
				case c.R >= 245:
					fg++
				}
			}
		}
		return
	}

	p1, f1 := stats(1)
	p4, f4 := stats(4)
	if p1 != 0 {
		t.Fatalf("未开超采样时不应出现中间色，实际 %d 个", p1)
	}
	if p4 == 0 {
		t.Fatal("4× 超采样应产生边缘过渡像素")
	}
	if f1 == 0 || f4 == 0 {
		t.Fatalf("三角形覆盖丢失: ss1=%d ss4=%d", f1, f4)
	}
	if ratio := float64(f4) / float64(f1); ratio < 0.85 || ratio > 1.15 {
		t.Fatalf("超采样前后覆盖率差异过大: ss1=%d ss4=%d (%.2f)", f1, f4, ratio)
	}
}

func TestSupersampleResize(t *testing.T) {
	r := game.NewRenderer(32, 32)
	r.Supersample = 2
	r.Frame(simpleScene(game.DefaultMaterial().WithColor(game.RGB(0, 255, 0)).WithShade(game.ShadeUnlit), nil))
	if w, h := game.TestRendererBufferSize(r); w != 64 || h != 64 {
		t.Fatalf("内部缓冲尺寸 = %d×%d，期望 64×64", w, h)
	}
	r.Supersample = 1
	r.Resize(48, 24)
	r.Frame(simpleScene(game.DefaultMaterial(), nil))
	if w, h := game.TestRendererBufferSize(r); w != 48 || h != 24 {
		t.Fatalf("关闭超采样后尺寸 = %d×%d，期望 48×24", w, h)
	}
	if r.Image().Rect.Dx() != 48 {
		t.Fatalf("输出图宽度 = %d", r.Image().Rect.Dx())
	}
}

func BenchmarkFrameSS2(b *testing.B) {
	r := game.NewRenderer(640, 360)
	r.Supersample = 2
	sc := benchScene()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(sc)
	}
}

func BenchmarkFrameSingleThread(b *testing.B) {
	r := game.NewRenderer(640, 360)
	r.Threads = 1
	sc := benchScene()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Frame(sc)
	}
}
