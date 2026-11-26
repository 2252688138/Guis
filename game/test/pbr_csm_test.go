// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

// quadMesh 返回一张朝向 +Z、位于 z 处的 1×1 面片（带法线与 UV）。
func quadMesh(z float32) *game.Mesh {
	m := game.NewMesh([]game.Vec3{
		{X: -1, Y: -1, Z: z},
		{X: 1, Y: -1, Z: z},
		{X: 1, Y: 1, Z: z},
		{X: -1, Y: 1, Z: z},
	}, []uint32{0, 1, 2, 0, 2, 3})
	m.Normals = []game.Vec3{{Z: 1}, {Z: 1}, {Z: 1}, {Z: 1}}
	m.UVs = []game.Vec2{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	m.RecomputeBounds()
	return m
}

// probeScene 构造一个相机朝 -Z、带方向光与环境光的场景。
func probeScene() *game.Scene {
	sc := game.NewScene()
	sc.Root.Children = nil
	sc.Camera.Pos = game.V3(0, 0, 0)
	sc.Camera.Target = game.V3(0, 0, -1)
	sc.Ambient = game.RGB(45, 45, 50)
	sc.AddSun(game.V3(0.3, -0.6, -0.7), 1)
	return sc
}

// nodeWith 把网格与材质挂成一个节点（不加入场景）。
func nodeWith(name string, m *game.Mesh, mat *game.Material) *game.Node {
	n := game.NewNode(name)
	n.Mesh = m
	n.Material = mat
	return n
}

// renderCenter 渲染场景并返回中心像素与帧统计。
func renderCenter(sc *game.Scene) (game.RGBA, game.FrameStats) {
	r := &game.Renderer{W: 48, H: 36, BgColor: game.Color(0, 0, 0, 255)}
	img := r.Frame(sc)
	c := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	return game.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}, r.Stats()
}

// lum 颜色亮度（0~765）。
func lum(c game.RGBA) int {
	return int(c.R) + int(c.G) + int(c.B)
}

// pbrMaterial 构造一个基础 PBR 材质。
func pbrMaterial() *game.Material {
	m := game.DefaultMaterial()
	m.Shade = game.ShadePBR
	m.BaseColor = game.RGB(180, 120, 60)
	m.Metallic = 0
	m.Roughness = 0.4
	return m
}

func TestPBRIsLit(t *testing.T) {
	sc := probeScene()
	sc.Root.Children = append(sc.Root.Children, nodeWith("q", quadMesh(-3), pbrMaterial()))
	c, _ := renderCenter(sc)
	if lum(c) < 60 {
		t.Fatalf("PBR 材质在方向光下应该被照亮，中心像素 = %v", c)
	}
}

func TestPBRMetallicDiffersFromDielectric(t *testing.T) {
	dielectric := pbrMaterial()
	metal := dielectric.Clone()
	metal.Metallic = 1
	metal.Roughness = 0.15

	sc1 := probeScene()
	sc1.Root.Children = append(sc1.Root.Children, nodeWith("d", quadMesh(-3), dielectric))
	c1, _ := renderCenter(sc1)

	sc2 := probeScene()
	sc2.Root.Children = append(sc2.Root.Children, nodeWith("m", quadMesh(-3), metal))
	c2, _ := renderCenter(sc2)

	if diff := lum(c1) - lum(c2); diff < 0 {
		diff = -diff
		if diff < 6 {
			t.Fatalf("金属与电介质渲染应不同：%v vs %v", c1, c2)
		}
	} else if diff < 6 {
		t.Fatalf("金属与电介质渲染应不同：%v vs %v", c1, c2)
	}
}

func TestPBRRoughnessChangesHighlight(t *testing.T) {
	// 金属的高光强弱完全由粗糙度决定，比电介质更容易区分
	smooth := pbrMaterial()
	smooth.Metallic = 1
	smooth.Roughness = 0.05
	rough := smooth.Clone()
	rough.Roughness = 0.95

	sc1 := probeScene()
	sc1.Root.Children = append(sc1.Root.Children, nodeWith("s", quadMesh(-3), smooth))
	c1, _ := renderCenter(sc1)

	sc2 := probeScene()
	sc2.Root.Children = append(sc2.Root.Children, nodeWith("r", quadMesh(-3), rough))
	c2, _ := renderCenter(sc2)

	diff := lum(c1) - lum(c2)
	if diff < 0 {
		diff = -diff
	}
	if diff < 4 {
		t.Fatalf("粗糙度应改变高光：%v vs %v", c1, c2)
	}
}

func TestPBRNormalMapApplied(t *testing.T) {
	flat := game.NewTexture(2, 2)
	spiky := game.NewTexture(2, 2)
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			flat.Set(x, y, game.RGB(128, 128, 255))
			spiky.Set(x, y, game.RGB(255, 8, 128))
		}
	}

	m1 := pbrMaterial()
	m1.NormalMap = flat
	sc1 := probeScene()
	sc1.Root.Children = append(sc1.Root.Children, nodeWith("f", quadMesh(-3), m1))
	c1, _ := renderCenter(sc1)

	m2 := pbrMaterial()
	m2.NormalMap = spiky
	sc2 := probeScene()
	sc2.Root.Children = append(sc2.Root.Children, nodeWith("s", quadMesh(-3), m2))
	c2, _ := renderCenter(sc2)

	if lum(c1) < 40 {
		t.Fatalf("平坦法线贴图不应把表面压黑：%v", c1)
	}
	if diff := lum(c1) - lum(c2); diff > -6 && diff < 6 {
		t.Fatalf("法线贴图应改变光照结果：%v vs %v", c1, c2)
	}
}

func TestPBRWithoutUVOrNormal(t *testing.T) {
	m := game.NewMesh([]game.Vec3{
		{X: -1, Y: -1, Z: -3},
		{X: 1, Y: -1, Z: -3},
		{X: 1, Y: 1, Z: -3},
	}, []uint32{0, 1, 2}) // 无 UV、无法线：切线退化，应走正交基兜底
	m.RecomputeBounds()

	nrm := game.NewTexture(2, 2)
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			nrm.Set(x, y, game.RGB(120, 140, 250))
		}
	}
	mat := pbrMaterial()
	mat.NormalMap = nrm

	sc := probeScene()
	sc.Root.Children = append(sc.Root.Children, nodeWith("t", m, mat))
	if _, st := renderCenter(sc); st.Triangles == 0 {
		t.Fatal("退化切线的 PBR 网格应该仍能画出来")
	}
}

func TestCSMSplitsAndFactor(t *testing.T) {
	sc := probeScene()
	sc.Root.Children = append(sc.Root.Children, nodeWith("q", quadMesh(-3), game.DefaultMaterial()))

	csm := game.NewCSMShadow()
	csm.Levels = 3
	csm.Size = 64
	csm.Aspect = 1.5
	csm.MaxDistance = 120
	csm.Build(sc)
	if !csm.Valid() {
		t.Fatal("有方向光时 CSM 应该构建成功")
	}
	sp := csm.Splits()
	if len(sp) != 3 {
		t.Fatalf("应有 3 级分割，实际 %d", len(sp))
	}
	for i := 1; i < len(sp); i++ {
		if sp[i] <= sp[i-1] {
			t.Fatalf("分割距离必须严格递增：%v", sp)
		}
	}
	total, per := csm.Stats()
	if len(per) != 3 {
		t.Fatalf("每级统计应有 3 项，实际 %d", len(per))
	}
	if total < 0 {
		t.Fatalf("阴影三角形数异常：%d", total)
	}
	for _, d := range []float32{0.5, 5, 50, 200} {
		f := csm.FactorCSM(game.V3(0, 0, -d), game.V3(0, 0, 0))
		if f < 0 || f > 1 {
			t.Fatalf("FactorCSM 应在 0~1，实际 %v（距离 %v）", f, d)
		}
	}
}

func TestCSMCastsShadow(t *testing.T) {
	sc := probeScene()
	sc.Lights = nil
	sc.AddSun(game.V3(0, -1, 0), 1) // 垂直向下照，影子正好在盒子下面
	sc.Ambient = game.RGB(18, 18, 20)
	sc.Camera.Pos = game.V3(0, 6, 18)
	sc.Camera.Target = game.V3(0, 0, 0)

	ground := game.NewMeshNode("ground", game.NewPlane(80, 80, 1), game.DefaultMaterial())
	sc.Add(ground)
	box := game.NewMeshNode("box", game.NewBox(5, 2, 5), game.DefaultMaterial())
	box.Pos = game.V3(0, 3, 0)
	sc.Add(box)

	csm := game.NewCSMShadow()
	csm.Levels = 2
	csm.Size = 128
	csm.Aspect = 1.6
	csm.Bias = 0.004
	csm.Build(sc)
	if !csm.Valid() {
		t.Fatal("CSM 应构建成功")
	}
	cam := sc.Camera.Pos
	inShadow := csm.FactorCSM(game.V3(0, 0.02, 0), cam) // 盒子正下方
	lit := csm.FactorCSM(game.V3(28, 0.02, 28), cam)    // 空旷地面
	if inShadow >= lit {
		t.Fatalf("盒子下方应产生阴影：阴影处 %v，空旷处 %v", inShadow, lit)
	}
}

func TestCSMNoDirectionalLight(t *testing.T) {
	sc := probeScene()
	sc.Lights = nil
	csm := game.NewCSMShadow()
	csm.Build(sc)
	if csm.Valid() {
		t.Fatal("没有方向光时 CSM 不应该构建")
	}
	if f := csm.FactorCSM(game.V3(0, 0, -3), game.V3(0, 0, 0)); f != 1 {
		t.Fatalf("无效 CSM 应返回 1，实际 %v", f)
	}
}

func TestRendererCascadesAndShadowStats(t *testing.T) {
	sc := probeScene()
	sc.Root.Children = append(sc.Root.Children, nodeWith("q", quadMesh(-3), game.DefaultMaterial()))

	r := &game.Renderer{W: 48, H: 36, BgColor: game.Color(0, 0, 0, 255)}
	csm := game.NewCSMShadow()
	csm.Size = 64
	r.Cascades = csm
	r.Frame(sc)
	if !csm.Valid() {
		t.Fatal("Renderer 应每帧构建 CSM")
	}
	if st := r.Stats(); st.ShadowTris < 0 {
		t.Fatalf("ShadowTris 统计异常：%d", st.ShadowTris)
	}
}
