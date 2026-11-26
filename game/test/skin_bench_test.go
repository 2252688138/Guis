// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"math/rand"
	"testing"

	"guis/game"
)

// benchSkinnedMesh 生成 1000 顶点 / 4 骨 / 每顶点 4 权重的网格。
func benchSkinnedMesh() *game.SkinnedMesh {
	const n = 1000
	rnd := rand.New(rand.NewSource(3))
	pos := make([]game.Vec3, n)
	idx := make([]uint32, 0, n*3)
	for i := range pos {
		pos[i] = game.Vec3{X: rnd.Float32()*2 - 1, Y: rnd.Float32()*2 - 1, Z: rnd.Float32()*2 - 1}
	}
	for i := 0; i+2 < n; i += 3 {
		idx = append(idx, uint32(i), uint32(i+1), uint32(i+2))
	}
	sk := &game.Skeleton{Bones: []game.Bone{
		{Name: "b0", Parent: -1, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
		{Name: "b1", Parent: 0, Pos: game.Vec3{Y: 1}, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
		{Name: "b2", Parent: 0, Pos: game.Vec3{X: 1}, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
		{Name: "b3", Parent: 1, Pos: game.Vec3{Z: 1}, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity()},
	}}
	skin := make([]game.SkinVertex, n)
	for i := range skin {
		skin[i] = game.SkinVertex{
			Joints:  [4]uint8{0, 1, 2, 3},
			Weights: [4]float32{0.4, 0.3, 0.2, 0.1},
		}
	}
	return game.NewSkinnedMesh(game.NewMesh(pos, idx), sk, skin)
}

func BenchmarkSkin(b *testing.B) {
	sm := benchSkinnedMesh()
	a := game.NewSkinAnimator(sm)
	clip := &game.SkinClip{Name: "bench", Duration: 1, Tracks: []game.BoneTrack{{
		Bone: 1,
		Keys: []game.KeyFrame{
			{Time: 0, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
			{Time: 1, Rot: game.QuatAxisAngle(game.Vec3{Y: 1}, 1.5), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
		},
	}}}
	a.Play(clip, true, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Update(0.016)
		sm.SkinnedVertices(a.Pose())
	}
}

// planeMesh 生成 n×n 的规则平面网格（2n² 三角），用于测 BVH 剪枝效果：
// 随机散点网格的 AABB 互相重叠严重，剪枝率不具代表性。
func planeMesh(n int) *game.Mesh {
	pos := make([]game.Vec3, 0, (n+1)*(n+1))
	for y := 0; y <= n; y++ {
		for x := 0; x <= n; x++ {
			pos = append(pos, game.Vec3{X: float32(x)/float32(n)*10 - 5, Y: float32(y)/float32(n)*10 - 5, Z: -5})
		}
	}
	idx := make([]uint32, 0, n*n*6)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			a := uint32(y*(n+1) + x)
			b := a + 1
			c := a + uint32(n+1)
			d := c + 1
			idx = append(idx, a, b, d, a, d, c)
		}
	}
	return game.NewMesh(pos, idx)
}

func BenchmarkBVHRay(b *testing.B) {
	mesh := planeMesh(50)
	bvh := game.NewBVH(mesh)
	if bvh == nil {
		b.Fatal("BVH 构建失败")
	}
	rnd := rand.New(rand.NewSource(5))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o := game.Vec3{X: rnd.Float32()*8 - 4, Y: rnd.Float32()*8 - 4, Z: 4}
		d := game.Vec3{X: rnd.Float32()*0.2 - 0.1, Y: rnd.Float32()*0.2 - 0.1, Z: -1}
		bvh.Ray(o, d, game.Identity())
	}
}

func BenchmarkRayMeshLinear(b *testing.B) {
	mesh := planeMesh(50)
	rnd := rand.New(rand.NewSource(5))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o := game.Vec3{X: rnd.Float32()*8 - 4, Y: rnd.Float32()*8 - 4, Z: 4}
		d := game.Vec3{X: rnd.Float32()*0.2 - 0.1, Y: rnd.Float32()*0.2 - 0.1, Z: -1}
		game.RayMesh(o, d, mesh, game.Identity())
	}
}
