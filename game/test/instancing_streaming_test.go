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

// batchScene 构造一个 100 个四边形的实例批次场景。
func batchScene(count int, useBatch bool) *game.Scene {
	mesh := quadMesh(0)
	mat := game.DefaultMaterial()
	mat.Shade = game.ShadeUnlit
	mat.BaseColor = game.RGB(200, 190, 160)

	sc := probeScene()
	sc.Root.Children = nil
	sc.Camera.Pos = game.V3(0, 0, 12)
	sc.Camera.Target = game.V3(0, 0, 0)

	if useBatch {
		b := game.NewInstanceBatch(mesh, mat)
		for i := 0; i < count; i++ {
			x := float32(i%10)*0.6 - 3
			y := float32(i/10)*0.6 - 3
			b.Add(game.V3(x, y, 0), 0.25, game.RGB(255, 255, 255))
		}
		n := game.NewNode("batch")
		n.Instances = b
		sc.Root.Children = append(sc.Root.Children, n)
		return sc
	}
	for i := 0; i < count; i++ {
		x := float32(i%10)*0.6 - 3
		y := float32(i/10)*0.6 - 3
		n := nodeWith("one", mesh, mat)
		n.Pos = game.V3(x, y, 0)
		n.Scl = game.V3(0.25, 0.25, 0.25)
		sc.Root.Children = append(sc.Root.Children, n)
	}
	return sc
}

func TestInstanceBatchStats(t *testing.T) {
	sc := batchScene(100, true)
	_, st := renderCenter(sc)
	if st.Instances != 100 {
		t.Fatalf("Instances 应为 100，实际 %d", st.Instances)
	}
	if st.Batches != 1 {
		t.Fatalf("Batches 应为 1，实际 %d", st.Batches)
	}
	if st.DrawCalls != 1 {
		t.Fatalf("合批后 DrawCalls 应为 1，实际 %d", st.DrawCalls)
	}
	if want := 99 * 2; st.TrianglesSaved != want {
		t.Fatalf("TrianglesSaved 应为 %d，实际 %d", want, st.TrianglesSaved)
	}
	if st.Triangles != 200 {
		t.Fatalf("三角形数应为 200，实际 %d", st.Triangles)
	}
}

func TestDrawCallsWithoutBatch(t *testing.T) {
	sc := batchScene(100, false)
	_, st := renderCenter(sc)
	if st.DrawCalls != 100 {
		t.Fatalf("100 个独立节点应有 100 次提交，实际 %d", st.DrawCalls)
	}
	if st.Batches != 0 || st.Instances != 0 {
		t.Fatalf("非实例路径不应计批次：%+v", st)
	}
}

func TestInstanceTint(t *testing.T) {
	mesh := quadMesh(0)
	mat := game.DefaultMaterial()
	mat.Shade = game.ShadeUnlit
	mat.BaseColor = game.RGB(255, 255, 255)

	b := game.NewInstanceBatch(mesh, mat)
	b.Add(game.V3(-2, 0, 0), 0.5, game.RGB(255, 0, 0))
	b.Add(game.V3(2, 0, 0), 0.5, game.RGB(0, 0, 255))

	n := game.NewNode("two")
	n.Instances = b
	sc := probeScene()
	sc.Root.Children = nil
	sc.Camera.Pos = game.V3(0, 0, 12)
	sc.Camera.Target = game.V3(0, 0, 0)
	sc.Root.Children = append(sc.Root.Children, n)

	r := &game.Renderer{W: 96, H: 72, BgColor: game.Color(0, 0, 0, 255)}
	img := r.Frame(sc)
	// 找出纯红与纯蓝像素，并检查它们分别落在左右两侧（说明 tint 生效且逐实例独立）
	redX, blueX := -1, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := img.RGBAAt(x, y)
			if redX < 0 && c.R > 200 && c.G < 60 && c.B < 60 {
				redX = x
			}
			if blueX < 0 && c.B > 200 && c.R < 60 && c.G < 60 {
				blueX = x
			}
		}
	}
	if redX < 0 {
		t.Fatal("没有找到红色实例像素")
	}
	if blueX < 0 {
		t.Fatal("没有找到蓝色实例像素")
	}
	if redX >= img.Bounds().Dx()/2 {
		t.Fatalf("红色实例应在左侧，实际 x=%d", redX)
	}
	if blueX < img.Bounds().Dx()/2 {
		t.Fatalf("蓝色实例应在右侧，实际 x=%d", blueX)
	}
}

func TestInstanceMatrixOf(t *testing.T) {
	b := game.NewInstanceBatch(quadMesh(0), game.DefaultMaterial())
	b.Add(game.V3(3, 4, 5), 2, game.RGB(255, 255, 255))
	b.Data[0].RotY = math.Pi / 2 // 绕 Y 轴 90°：+X → -Z
	b.Data[0].ScaleY = 3

	w := b.MatrixOf(0)
	// 偏移生效
	if p := w.TransformPoint(game.V3(0, 0, 0)); p.X != 3 || p.Y != 4 || p.Z != 5 {
		t.Fatalf("实例偏移不对：%v", p)
	}
	// 旋转生效：局部 +X 转成世界 -Z（缩放 2 → z = 5-2 = 3）
	if p := w.TransformPoint(game.V3(1, 0, 0)); math.Abs(float64(p.Z-3)) > 1e-3 {
		t.Fatalf("实例绕 Y 旋转未生效：%v", p)
	}
	// 垂直缩放独立于水平缩放
	if p := w.TransformPoint(game.V3(0, 1, 0)); math.Abs(float64(p.Y-7)) > 1e-3 {
		t.Fatalf("ScaleY 未生效：%v", p)
	}
	// 越界返回单位矩阵
	if got := b.MatrixOf(99); got != game.Identity() {
		t.Fatalf("越界应返回单位矩阵")
	}
}

func TestStreamingMountBudget(t *testing.T) {
	mesh := quadMesh(0)
	s := game.NewStreaming(10, 1, 3)
	s.MaxMountPerFrame = 2
	s.Loader = func(cx, cz int) *game.Node {
		n := game.NewNode("chunk")
		n.Mesh = mesh
		return n
	}
	sc := game.NewScene()
	sc.Root.Children = nil

	loaded, _ := s.Update(game.V3(5, 0, 5), sc)
	if loaded != 2 {
		t.Fatalf("预算 2 时首帧应只挂载 2 块，实际 %d", loaded)
	}
	if got := s.Stats().Loaded; got != 2 {
		t.Fatalf("Stats().Loaded 应为 2，实际 %d", got)
	}

	total := loaded
	for i := 0; i < 10 && total < 9; i++ {
		l, _ := s.Update(game.V3(5, 0, 5), sc)
		total += l
	}
	if total != 9 {
		t.Fatalf("半径 1 应最终加载 9 块，实际 %d", total)
	}
}

func TestStreamingUnload(t *testing.T) {
	mesh := quadMesh(0)
	s := game.NewStreaming(10, 1, 3)
	s.Loader = func(cx, cz int) *game.Node {
		n := game.NewNode("chunk")
		n.Mesh = mesh
		return n
	}
	sc := game.NewScene()
	sc.Root.Children = nil

	if _, _ = s.Update(game.V3(5, 0, 5), sc); s.Stats().Loaded != 9 {
		t.Fatalf("应加载 9 块，实际 %d", s.Stats().Loaded)
	}
	_, unloaded := s.Update(game.V3(500, 0, 500), sc)
	if unloaded < 9 {
		t.Fatalf("离开卸载半径后应卸载 9 块，实际 %d", unloaded)
	}
	st := s.Stats()
	if st.TotalUnloaded < 9 {
		t.Fatalf("累计卸载数异常：%+v", st)
	}
	if st.Loaded != 9 {
		t.Fatalf("新位置应挂载 9 块，实际 %d", st.Loaded)
	}
}
