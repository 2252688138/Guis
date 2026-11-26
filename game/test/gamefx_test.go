// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"math"
	"testing"

	"guis/game"
)

// 建立一个小场景：地面 + 悬浮立方体 + 顶光。
func fxScene() *game.Scene {
	sc := game.NewScene()
	sc.Camera.Pos = game.V3(0, 3, 6)
	sc.Camera.Target = game.V3(0, 0.5, 0)
	sc.AddSun(game.V3(0, -1, 0), 1)
	sc.SetAmbientIntensity(0.3)
	ground := game.NewMeshNode("ground", game.NewPlane(20, 20, 1), game.DefaultMaterial())
	box := game.NewMeshNode("box", game.NewBox(1, 1, 1), game.DefaultMaterial())
	box.Pos = game.V3(0, 1.5, 0)
	sc.Add(ground, box)
	return sc
}

func TestTextureMipmap(t *testing.T) {
	tx := game.CheckerTexture(4, game.RGB(255, 255, 255), game.RGB(0, 0, 0))
	if tx.MipCount() != 0 {
		t.Fatalf("未构建时 MipCount 应为 0，得到 %d", tx.MipCount())
	}
	tx.BuildMips()
	if got := tx.MipCount(); got != 4 { // 8x8 -> 4,2,1
		t.Fatalf("8x8 纹理应有 4 层 mip，得到 %d", got)
	}
	if m := tx.Mip(1); m == nil || m.W != 4 || m.H != 4 {
		t.Fatalf("第 1 级应为 4x4，得到 %+v", m)
	}
	if m := tx.Mip(99); m != nil {
		t.Fatalf("越界层级应返回 nil")
	}
	// lod = 0 应与双线性一致
	a := tx.SampleLinear(0.3, 0.7)
	b := tx.SampleTrilinear(0.3, 0.7, 0)
	if a != b {
		t.Fatalf("lod=0 应退化为双线性：%v vs %v", a, b)
	}
	// 高 lod 应收敛到最模糊一级（不 panic 且不透明）
	c := tx.SampleTrilinear(0.3, 0.7, 9)
	if c.A == 0 {
		t.Fatalf("高 lod 采样不应为全透明")
	}
	if lod := tx.TexelLod(0, 0); lod != 0 {
		t.Fatalf("零变化率的 lod 应为 0，得到 %f", lod)
	}
	if lod := tx.TexelLod(0.25, 0); lod <= 0 {
		t.Fatalf("大变化率的 lod 应 > 0，得到 %f", lod)
	}
	tx.FreeMips()
	if tx.MipCount() != 0 {
		t.Fatalf("FreeMips 后应为 0")
	}
}

func TestPostFXApply(t *testing.T) {
	r := game.NewRenderer(64, 36)
	sc := fxScene()
	r.Frame(sc)

	// 亮度 + 暗角 + 泛光：不应panic，且画面发生变化
	img := r.Frame(sc)
	before := append([]byte(nil), img.Pix...)
	p := &game.PostFX{On: true, Brightness: 0.2, Vignette: 0.6, Bloom: true, BloomThreshold: 0.2, BloomIntensity: 1, Scanlines: 0.3, Noise: 0.1}
	p.Apply(img)
	diff := 0
	for i := 0; i < len(img.Pix) && i < len(before); i++ {
		if img.Pix[i] != before[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatalf("后处理未改变任何像素")
	}

	// 未开启时不应改动
	img2 := r.Frame(sc)
	snap := append([]byte(nil), img2.Pix...)
	(&game.PostFX{On: false, Vignette: 1}).Apply(img2)
	for i := range snap {
		if snap[i] != img2.Pix[i] {
			t.Fatalf("On=false 时不应改动像素")
		}
	}

	// 灰度
	img3 := r.Frame(sc)
	(&game.PostFX{On: true, Saturation: 0}).Apply(img3)
	for i := 0; i+3 < len(img3.Pix); i += 4 {
		if img3.Pix[i] != img3.Pix[i+1] || img3.Pix[i+1] != img3.Pix[i+2] {
			t.Fatalf("饱和度为 0 时应为灰度像素")
		}
	}
}

func TestSkyGradientAndCube(t *testing.T) {
	sc := fxScene()
	sc.Sky = game.GradientSky(game.RGB(40, 80, 200), game.RGB(180, 210, 240), game.RGB(20, 20, 30))
	r := game.NewRenderer(64, 36)
	img := r.Frame(sc)
	// 画面顶部应是天空（蓝），底部是地面（非天空）
	top := game.Color(img.Pix[0], img.Pix[1], img.Pix[2], img.Pix[3])
	if top.B < top.R {
		t.Fatalf("渐变天空顶部应偏蓝，得到 %v", top)
	}

	// 立方体天空盒：构造纯红色天空，背景像素应偏红
	cube := game.NewTexture(4*4, 3*4)
	cube.Fill(game.Color(200, 30, 30, 255))
	sc.Sky = game.CubeSky(cube, 4)
	img2 := r.Frame(sc)
	c := game.Color(img2.Pix[0], img2.Pix[1], img2.Pix[2], img2.Pix[3])
	if c.R <= c.G || c.R <= c.B {
		t.Fatalf("立方体天空盒背景应偏红，得到 %v", c)
	}

	// 关闭天空：背景用 Background 纯色
	sc.Sky = game.Sky{}
	sc.Background = game.RGB(10, 60, 20)
	img3 := r.Frame(sc)
	bg := game.Color(img3.Pix[0], img3.Pix[1], img3.Pix[2], img3.Pix[3])
	if bg.G <= bg.R {
		t.Fatalf("关闭天空时应用 Background，得到 %v", bg)
	}
}

func TestShadowMap(t *testing.T) {
	sc := fxScene()
	sh := game.NewShadowMap()
	sh.On = true
	sh.Size = 128
	sh.Strength = 0.8
	sc.BuildWorld()
	sh.Build(sc)
	if !sh.Valid() {
		t.Fatalf("阴影图应构建成功")
	}
	if sh.Triangles() == 0 {
		t.Fatalf("阴影图应写入三角形")
	}
	// 立方体正下方（被遮挡）应比远处更暗
	under := sh.Factor(game.V3(0, 0.02, 0))
	far := sh.Factor(game.V3(6, 0.02, 0))
	if under >= far {
		t.Fatalf("遮挡处系数应小于开阔处：under=%f far=%f", under, far)
	}
	if far < 0.99 {
		t.Fatalf("开阔处应完全受光，得到 %f", far)
	}

	// Renderer 集成：启用后画面应出现明暗差
	r := game.NewRenderer(96, 54)
	r.Shadow = sh
	img := r.Frame(sc)
	if len(img.Pix) == 0 {
		t.Fatalf("渲染结果为空")
	}
	if sh.Once {
		t.Fatalf("默认不应启用 Once")
	}

	// Once 模式：第二次 Build 不再重建
	sh.Once = true
	sh.Reset()
	sh.Build(sc)
	tris := sh.Triangles()
	sh.Build(sc)
	if sh.Triangles() != tris {
		t.Fatalf("Once 模式不应重复构建")
	}

	// 无方向光时不构建
	sc2 := game.NewScene()
	sc2.Add(game.NewMeshNode("b", game.NewBox(1, 1, 1), game.DefaultMaterial()))
	sh2 := game.NewShadowMap()
	sh2.On = true
	sh2.Build(sc2)
	if sh2.Valid() {
		t.Fatalf("无方向光时不应产生阴影图")
	}
}

func TestCamera2DEnhancements(t *testing.T) {
	cam := game.NewCamera2D(320, 180)
	cam.X, cam.Y = 100, 50
	// 世界↔屏幕互逆
	for _, p := range []game.Vec2{game.V2(100, 50), game.V2(0, 0), game.V2(250, 130)} {
		sx, sy := cam.Screen(p.X, p.Y)
		wx, wy := cam.World(sx, sy)
		if math.Abs(float64(wx-p.X)) > 1e-3 || math.Abs(float64(wy-p.Y)) > 1e-3 {
			t.Fatalf("Screen/World 不互逆：%v -> (%f,%f) -> (%f,%f)", p, sx, sy, wx, wy)
		}
	}
	// 中心对齐视口中心
	if sx, sy := cam.Screen(cam.X, cam.Y); math.Abs(float64(sx-160)) > 1e-3 || math.Abs(float64(sy-90)) > 1e-3 {
		t.Fatalf("相机中心应映射到视口中心，得到 (%f,%f)", sx, sy)
	}
	// 缩放
	cam.Zoom = 2
	if sx, _ := cam.Screen(cam.X+10, cam.Y); math.Abs(float64(sx-180)) > 1e-3 {
		t.Fatalf("缩放 2 时 10 世界单位应为 20 像素，得到 %f", sx)
	}
	cam.Zoom = 0
	if sx, _ := cam.Screen(cam.X+10, cam.Y); math.Abs(float64(sx-170)) > 1e-3 {
		t.Fatalf("Zoom=0 应按 1 处理，得到 %f", sx)
	}
	cam.Zoom = 1

	// 抖屏：Shake 后 Update 会衰减
	cam.Shake(8, 0.2)
	if !cam.Shaking() {
		t.Fatalf("Shake 后应处于抖屏状态")
	}
	cam.Update(0.1)
	cam.Update(0.2)
	if cam.Shaking() {
		t.Fatalf("抖屏应已结束")
	}

	// 平滑缩放
	cam.ZoomTo(3, 0.2)
	cam.Update(0.2)
	if math.Abs(float64(cam.Zoom-3)) > 1e-3 {
		t.Fatalf("ZoomTo 后缩放应为 3，得到 %f", cam.Zoom)
	}

	// 精灵坐标转换（以相机中心为参照，中心偏移 × 缩放 映射到屏幕）
	sp := game.Sprite{Pos: game.V2(100, 50), Size: game.V2(16, 16)}
	out := cam.Apply(sp)
	wantSX, wantSY := cam.Screen(108, 58)
	if math.Abs(float64(out.Pos.X+out.Size.X/2-wantSX)) > 1e-2 || math.Abs(float64(out.Size.X-48)) > 1e-2 {
		t.Fatalf("Apply 后精灵位置/尺寸不对：中心(%f,%f) 期望(%f,%f) 尺寸 %f", out.Pos.X+out.Size.X/2, out.Pos.Y+out.Size.Y/2, wantSX, wantSY, out.Size.X)
	}

	// 可见性
	if !cam.VisibleRect(90, 40, 120, 70) {
		t.Fatalf("视口内矩形应可见")
	}
	if cam.VisibleRect(10000, 10000, 10100, 10100) {
		t.Fatalf("远处矩形不应可见")
	}
	b := cam.VisibleBounds()
	if b.Min.X >= b.Max.X || b.Min.Y >= b.Max.Y {
		t.Fatalf("可见包围盒无效：%+v", b)
	}

	// 批量
	batch := game.NewSpriteBatch()
	cam.ApplyBatch(batch, []game.Sprite{sp, {Pos: game.V2(1e6, 1e6), Size: game.V2(8, 8)}})
	if batch.SpriteCount() != 1 {
		t.Fatalf("视口剔除后应剩 1 个精灵，得到 %d", batch.SpriteCount())
	}
}

func TestNineSliceDraw(t *testing.T) {
	tex := game.NineSliceTexture(16, game.Color(60, 90, 160, 255), game.Color(240, 240, 240, 255), 4)
	dst := image.NewRGBA(image.Rect(0, 0, 64, 48))
	n := game.NineSlice{Texture: tex, Dst: game.Rect2{X: 4, Y: 4, W: 40, H: 30}, Border: game.Rect2{X: 6, Y: 6, W: 6, H: 6}}
	n.Draw(dst)
	nonEmpty := 0
	for i := 3; i < len(dst.Pix); i += 4 {
		if dst.Pix[i] != 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Fatalf("九宫格未绘制任何像素")
	}
	// 目标小于边框时应收缩而不越界
	small := image.NewRGBA(image.Rect(0, 0, 8, 8))
	game.NineSlice{Texture: tex, Dst: game.Rect2{X: 0, Y: 0, W: 4, H: 4}, Border: game.Rect2{X: 6, Y: 6, W: 6, H: 6}}.Draw(small)
	game.NineSlice{Texture: tex, Dst: game.Rect2{X: 0, Y: 0, W: 4, H: 4}}.Draw(nil)
}

func TestBitmapFont(t *testing.T) {
	f := game.BuiltinFont()
	if f == nil || f.Tex == nil {
		t.Fatalf("内置字体未初始化")
	}
	w, h := f.Measure("SCORE 120", 1)
	if w <= 0 || h <= 0 {
		t.Fatalf("Measure 结果无效：%f x %f", w, h)
	}
	w2, h2 := f.Measure("AB\nCD", 2)
	if math.Abs(float64(w2-20)) > 1e-3 || math.Abs(float64(h2-28)) > 1e-3 {
		t.Fatalf("多行文本尺寸异常：%f x %f（期望 20 x 28）", w2, h2)
	}
	dst := image.NewRGBA(image.Rect(0, 0, 120, 40))
	got := f.DrawText(dst, "HI 42", 2, 2, 2, game.RGB(255, 240, 60))
	if got <= 0 {
		t.Fatalf("DrawText 应返回宽度")
	}
	nonEmpty := 0
	for i := 3; i < len(dst.Pix); i += 4 {
		if dst.Pix[i] != 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Fatalf("文本未绘制任何像素")
	}
	// 小写映射到大写
	if _, ok := f.Glyph('a'); !ok {
		t.Fatalf("小写字母应映射到大写字形")
	}
	if _, ok := f.Glyph('\u4e2d'); ok {
		t.Fatalf("内置字体不应声称支持中文")
	}
	f.DrawTextCentered(dst, "X", 60, 20, 1, game.RGB(255, 255, 255))
	f.DrawTextShadow(dst, "X", 0, 0, 1, game.RGB(255, 255, 255), game.RGB(0, 0, 0))
}

func TestParallaxDraw(t *testing.T) {
	sky := game.StarfieldTexture(32, 40, game.RGB(5, 8, 20))
	p := &game.Parallax{}
	p.Add(game.ParallaxLayer{Texture: sky, Factor: game.V2(0.1, 0)})
	p.Add(game.ParallaxLayer{Texture: sky, Factor: game.V2(0.5, 0.2), Scale: 2})
	if len(p.Layers) != 2 {
		t.Fatalf("层数错误：%d", len(p.Layers))
	}
	dst := image.NewRGBA(image.Rect(0, 0, 80, 40))
	cam := game.NewCamera2D(80, 40)
	cam.X, cam.Y = 123, 45
	p.Draw(dst, cam)
	nonEmpty := 0
	for i := 3; i < len(dst.Pix); i += 4 {
		if dst.Pix[i] != 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Fatalf("视差背景未绘制")
	}
	p.Draw(dst, nil)
	p.Clear()
	if len(p.Layers) != 0 {
		t.Fatalf("Clear 未清空")
	}
}

func TestTileCollision(t *testing.T) {
	col := game.NewTileCollision(16, 16)
	col.MarkSolid(1).MarkOneWay(9)
	if !col.IsSolid(1) || col.IsSolid(2) || !col.IsOneWay(9) {
		t.Fatalf("GID 标记错误")
	}
	// 3×2 网格：第一行 1,1,9；第二行 0,1,0
	data := []int{1, 1, 9, 0, 1, 0}
	solid, oneWay := col.Boxes(data, 3, 2, game.V2(0, 0))
	if len(solid) != 2 {
		t.Fatalf("水平合并后应有 2 个实心盒，得到 %d", len(solid))
	}
	if len(oneWay) != 1 {
		t.Fatalf("应有 1 个单向平台盒，得到 %d", len(oneWay))
	}
	if solid[0].W() != 32 {
		t.Fatalf("合并宽度应为 32，得到 %f", solid[0].W())
	}

	// Tiled 图层集成
	m := &game.TiledMap{W: 2, H: 1, TileW: 8, TileH: 8, Atlas: game.NewAtlas(game.NewTexture(16, 16), 8, 8, 2), FirstGID: 1}
	m.Layers = []game.TileLayer{{Name: "col", W: 2, H: 1, Data: []int{1, 9}, Visible: true}}
	s2, o2 := col.Collect(m, "col")
	if len(s2) != 1 || len(o2) != 1 {
		t.Fatalf("Collect 结果异常：solid=%d oneWay=%d", len(s2), len(o2))
	}

	// 推出解算
	box := game.Box2(0, 0, 10, 10)
	wall := game.Box2(8, 0, 10, 10)
	out, hit := game.ResolveAABB(box, []game.AABB2{wall})
	if !hit {
		t.Fatalf("应检测到碰撞")
	}
	if out.Max.X > wall.Min.X+1e-3 {
		t.Fatalf("应被推到墙左侧，得到 %+v", out)
	}
}

func TestSceneManagerAndPool(t *testing.T) {
	eng := game.NewEngine(64, 36)
	mgr := game.NewSceneManager()
	mgr.Register("menu", func(w *game.World) game.Game { return nil }, nil, nil)
	mgr.Register("level1", func(w *game.World) game.Game { return nil }, nil, nil)
	if len(mgr.Levels()) != 2 {
		t.Fatalf("关卡注册失败")
	}
	if err := mgr.Switch("nope", eng); err == nil {
		t.Fatalf("未注册关卡应报错")
	}
	if err := mgr.Switch("menu", eng); err != nil {
		t.Fatalf("切换到 menu 失败：%v", err)
	}
	if mgr.Current() != "menu" {
		t.Fatalf("当前关卡应为 menu，得到 %q", mgr.Current())
	}
	// 带过渡切换
	mgr.FadeTime = 0.1
	if err := mgr.FadeTo("level1", eng); err != nil {
		t.Fatalf("FadeTo 失败：%v", err)
	}
	if !mgr.Busy() {
		t.Fatalf("FadeTo 后应处于过渡中")
	}
	for i := 0; i < 20; i++ {
		mgr.Update(0.02, eng)
	}
	if mgr.Current() != "level1" || mgr.Busy() {
		t.Fatalf("过渡后应切到 level1 且结束：cur=%q busy=%v", mgr.Current(), mgr.Busy())
	}
	// 过渡遮罩绘制
	game.TestSceneManagerSetFade(mgr, 1)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	mgr.Draw(img)
	if img.Pix[3] == 0 {
		t.Fatalf("遮罩应不透明")
	}
	// FadeTime = 0 时立即切换
	mgr.FadeTime = 0
	if err := mgr.FadeTo("menu", eng); err != nil || mgr.Current() != "menu" {
		t.Fatalf("FadeTime=0 应立即切换：%v %q", err, mgr.Current())
	}

	// 对象池
	pool := game.NewPool(func() *game.Sprite { return &game.Sprite{} }, func(s *game.Sprite) { *s = game.Sprite{} })
	pool.Prewarm(2)
	if pool.Len() != 2 {
		t.Fatalf("预热后应有 2 个对象，得到 %d", pool.Len())
	}
	a := pool.Get()
	a.Size = game.V2(3, 3)
	pool.Put(a)
	b := pool.Get()
	if b.Size.X != 0 {
		t.Fatalf("Put 应重置对象")
	}
	pool.Clear()
	if pool.Len() != 0 {
		t.Fatalf("Clear 后应为空")
	}
}
