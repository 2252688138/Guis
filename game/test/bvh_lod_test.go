// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"math"
	"math/rand"
	"testing"

	"guis/game"
)

func randomMesh(rnd *rand.Rand, tris int) *game.Mesh {
	pos := make([]game.Vec3, 0, tris*3)
	idx := make([]uint32, 0, tris*3)
	for i := 0; i < tris; i++ {
		base := uint32(len(pos))
		for k := 0; k < 3; k++ {
			pos = append(pos, game.Vec3{
				X: rnd.Float32()*10 - 5,
				Y: rnd.Float32()*10 - 5,
				Z: rnd.Float32()*10 - 5,
			})
		}
		idx = append(idx, base, base+1, base+2)
	}
	return game.NewMesh(pos, idx)
}

func TestBVHMatchesRayMesh(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	mesh := randomMesh(rnd, 300)
	bvh := game.NewBVH(mesh)
	if bvh == nil {
		t.Fatal("BVH 构建失败")
	}
	if bvh.TriCount() != 300 {
		t.Fatalf("TriCount = %d，期望 300", bvh.TriCount())
	}

	hits := 0
	for i := 0; i < 100; i++ {
		o := game.Vec3{X: rnd.Float32()*16 - 8, Y: rnd.Float32()*16 - 8, Z: rnd.Float32()*16 - 8}
		d := game.Vec3{X: rnd.Float32()*2 - 1, Y: rnd.Float32()*2 - 1, Z: rnd.Float32()*2 - 1}
		if d.Len() < 0.2 {
			continue
		}
		t1, tri1, ok1 := game.RayMesh(o, d, mesh, game.Identity())
		t2, tri2, ok2 := bvh.Ray(o, d, game.Identity())
		if ok1 != ok2 {
			t.Fatalf("命中不一致 ray#%d: RayMesh=%v BVH=%v", i, ok1, ok2)
		}
		if !ok1 {
			continue
		}
		hits++
		if math.Abs(float64(t1-t2)) > 1e-3 {
			t.Fatalf("距离不一致 ray#%d: RayMesh=%v BVH=%v", i, t1, t2)
		}
		if tri1 != tri2 && math.Abs(float64(t1-t2)) > 1e-5 {
			t.Fatalf("三角形不一致 ray#%d: RayMesh=%d BVH=%d", i, tri1, tri2)
		}
	}
	if hits == 0 {
		t.Fatal("100 组随机射线全部未命中，测试无效")
	}
}

func TestMeshBVHCache(t *testing.T) {
	rnd := rand.New(rand.NewSource(11))
	mesh := randomMesh(rnd, 20)
	b1 := mesh.BVH()
	b2 := mesh.BVH()
	if b1 == nil || b1 != b2 {
		t.Fatal("BVH 应缓存复用")
	}
	mesh.InvalidateBVH()
	b3 := mesh.BVH()
	if b3 == nil || b3 == b1 {
		t.Fatal("InvalidateBVH 后应重建")
	}
}

func TestNodePickLOD(t *testing.T) {
	hi := game.NewMesh([]game.Vec3{{X: -1, Z: -3}, {X: 1, Z: -3}, {X: 0, Y: 1, Z: -3}}, []uint32{0, 1, 2})
	lo := game.NewMesh([]game.Vec3{{X: -2, Z: -3}, {X: 2, Z: -3}, {X: 0, Y: 2, Z: -3}}, []uint32{0, 1, 2})
	n := game.NewNode("lod")
	n.Mesh = hi
	n.LODs = []game.LODLevel{{Distance: 2, Mesh: hi}, {Distance: 100, Mesh: lo}}
	if got := n.PickLOD(1); got != hi {
		t.Fatal("近距离应选高模")
	}
	if got := n.PickLOD(50); got != lo {
		t.Fatal("远距离应选低模")
	}
	if got := n.PickLOD(1e6); got != lo {
		t.Fatal("超远距离应回退最后一级")
	}
	// 无 LOD 时始终用 Mesh
	plain := game.NewNode("plain")
	plain.Mesh = hi
	if got := plain.PickLOD(1e6); got != hi {
		t.Fatal("无 LOD 应返回 Mesh")
	}
}

func TestRendererLODSwitch(t *testing.T) {
	mk := func(x float32) *game.Node {
		hi := game.NewMesh([]game.Vec3{
			{X: x - 1, Y: -1, Z: -3}, {X: x + 1, Y: -1, Z: -3}, {X: x + 1, Y: 1, Z: -3}, {X: x - 1, Y: 1, Z: -3},
		}, []uint32{0, 1, 2, 0, 2, 3})
		lo := game.NewMesh([]game.Vec3{
			{X: x - 1, Y: -1, Z: -3}, {X: x + 1, Y: -1, Z: -3}, {X: x, Y: 1, Z: -3},
		}, []uint32{0, 1, 2})
		n := game.NewNode("quad")
		n.Mesh = hi
		n.LODs = []game.LODLevel{{Distance: 1, Mesh: hi}, {Distance: 1000, Mesh: lo}}
		n.Material = game.DefaultMaterial()
		return n
	}
	sc := game.NewScene()
	sc.Root.Children = nil
	sc.Root.Children = append(sc.Root.Children, mk(0))
	sc.Camera.Pos = game.Vec3{}
	sc.Camera.Target = game.Vec3{Z: -1}
	r := &game.Renderer{W: 64, H: 48, BgColor: game.Color(0, 0, 0, 255)}
	r.Frame(sc)
	if st := r.Stats(); st.LODSwitches != 1 {
		t.Fatalf("LODSwitches = %d，期望 1（相机距离远超第一层阈值）", st.LODSwitches)
	}
}
