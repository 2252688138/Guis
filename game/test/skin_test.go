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

func oneBoneSkeleton() *game.Skeleton {
	return &game.Skeleton{Bones: []game.Bone{{
		Name: "root", Parent: -1, Scl: game.Vec3{X: 1, Y: 1, Z: 1}, InverseBind: game.Identity(),
	}}}
}

func TestSkinnedVerticesOneBoneRotate(t *testing.T) {
	sk := oneBoneSkeleton()
	mesh := game.NewMesh([]game.Vec3{{X: 1}, {Y: 1}, {X: 1, Y: 1}}, []uint32{0, 1, 2})
	sm := game.NewSkinnedMesh(mesh, sk, nil)
	if len(sm.Skin) != 3 {
		t.Fatalf("Skin 长度 = %d，期望 3（自动绑 0 号骨）", len(sm.Skin))
	}
	clip := &game.SkinClip{
		Name: "turn", Duration: 1,
		Tracks: []game.BoneTrack{{
			Bone: 0,
			Keys: []game.KeyFrame{
				{Time: 0, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
				{Time: 1, Rot: game.QuatAxisAngle(game.Vec3{Y: 1}, math.Pi/2), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
			},
		}},
	}
	a := game.NewSkinAnimator(sm)
	if a == nil {
		t.Fatal("NewSkinAnimator 返回 nil")
	}
	a.Play(clip, false, 1)
	a.Update(1) // 非循环 → 停在末帧（90°）
	verts := sm.SkinnedVertices(a.Pose())
	if len(verts) != 3 {
		t.Fatalf("蒙皮顶点数 = %d", len(verts))
	}
	want := game.QuatAxisAngle(game.Vec3{Y: 1}, math.Pi/2).Rotate(game.Vec3{X: 1})
	if math.Abs(float64(verts[0].X-want.X)) > 1e-4 || math.Abs(float64(verts[0].Z-want.Z)) > 1e-4 {
		t.Fatalf("顶点0 = %+v，期望 %+v", verts[0], want)
	}
}

func TestSkinClipSampleMidpoint(t *testing.T) {
	clip := &game.SkinClip{
		Name: "walk", Duration: 1,
		Tracks: []game.BoneTrack{{
			Bone: 0,
			Keys: []game.KeyFrame{
				{Time: 0, Pos: game.Vec3{}, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
				{Time: 1, Pos: game.Vec3{X: 10}, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}},
			},
		}},
	}
	out := make([]game.BonePose, 1)
	clip.Sample(0.5, out)
	if math.Abs(float64(out[0].Pos.X-5)) > 1e-4 {
		t.Fatalf("t=0.5 的位移 = %v，期望 5", out[0].Pos.X)
	}
	clip.Sample(-1, out)
	if out[0].Pos.X != 0 {
		t.Fatalf("t<0 应夹持到首帧，得到 %v", out[0].Pos.X)
	}
	clip.Sample(99, out)
	if math.Abs(float64(out[0].Pos.X-10)) > 1e-4 {
		t.Fatalf("t>duration 应夹持到末帧，得到 %v", out[0].Pos.X)
	}
}

func TestSkinAnimatorCrossFade(t *testing.T) {
	sk := oneBoneSkeleton()
	mesh := game.NewMesh([]game.Vec3{{X: 1}, {Y: 1}, {X: 1, Y: 1}}, []uint32{0, 1, 2})
	sm := game.NewSkinnedMesh(mesh, sk, nil)
	a := game.NewSkinAnimator(sm)

	idle := &game.SkinClip{Name: "idle", Duration: 1, Tracks: []game.BoneTrack{{
		Bone: 0, Keys: []game.KeyFrame{{Time: 0, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}}},
	}}}
	turn := &game.SkinClip{Name: "turn", Duration: 1, Tracks: []game.BoneTrack{{
		Bone: 0, Keys: []game.KeyFrame{{Time: 0, Rot: game.QuatAxisAngle(game.Vec3{Y: 1}, math.Pi/2), Scl: game.Vec3{X: 1, Y: 1, Z: 1}}},
	}}}
	a.Play(idle, true, 1)
	a.Update(0.1)
	a.CrossFade(turn, 0.5)
	a.Update(0.25)
	if w := a.FadeWeight(); math.Abs(float64(w-0.5)) > 1e-3 {
		t.Fatalf("0.25s 后淡入权重 = %v，期望 0.5", w)
	}
	// 两段动画各半 → 关节旋转接近 45°
	verts := sm.SkinnedVertices(a.Pose())
	want := game.QuatAxisAngle(game.Vec3{Y: 1}, math.Pi/4).Rotate(game.Vec3{X: 1})
	if math.Abs(float64(verts[0].X-want.X)) > 0.05 || math.Abs(float64(verts[0].Z-want.Z)) > 0.05 {
		t.Fatalf("交叉淡入顶点 = %+v，期望 ≈ %+v", verts[0], want)
	}
	a.Update(0.5)
	if w := a.FadeWeight(); w != 1 {
		t.Fatalf("淡入结束后权重 = %v，期望 1", w)
	}
}

func TestSkinMarshalRoundTrip(t *testing.T) {
	sk := oneBoneSkeleton()
	mesh := game.NewMesh([]game.Vec3{{X: 1}, {Y: 1}, {X: 1, Y: 1}}, []uint32{0, 1, 2})
	sm := game.NewSkinnedMesh(mesh, sk, nil)
	clip := &game.SkinClip{Name: "idle", Duration: 2, Tracks: []game.BoneTrack{{
		Bone: 0, Keys: []game.KeyFrame{{Time: 0, Rot: game.QuatIdentity(), Scl: game.Vec3{X: 1, Y: 1, Z: 1}}},
	}}}
	b, err := game.MarshalSkin(sm, clip)
	if err != nil {
		t.Fatalf("MarshalSkin: %v", err)
	}
	got, gotClip, err := game.LoadSkinBytes(b)
	if err != nil {
		t.Fatalf("LoadSkinBytes: %v", err)
	}
	if got.Skeleton == nil || len(got.Skeleton.Bones) != 1 {
		t.Fatal("骨骼数不一致")
	}
	if len(got.Mesh.Positions) != 3 || len(got.Skin) != 3 {
		t.Fatalf("顶点/权重数不一致: %d/%d", len(got.Mesh.Positions), len(got.Skin))
	}
	if gotClip == nil || gotClip.Duration != 2 {
		t.Fatalf("动画不一致: %+v", gotClip)
	}
}

func TestSkinnedNodeRender(t *testing.T) {
	sk := oneBoneSkeleton()
	// 一个在相机前方的大三角形，绕 Y 旋转后应占据不同像素
	mesh := game.NewMesh([]game.Vec3{
		{X: -2, Y: -1, Z: -4}, {X: 2, Y: -1, Z: -4}, {X: 0, Y: 2, Z: -4},
	}, []uint32{0, 1, 2})
	sm := game.NewSkinnedMesh(mesh, sk, nil)
	n := game.NewNode("hero")
	n.Mesh = mesh
	n.Skinned = sm
	n.Material = game.DefaultMaterial()

	sc := game.NewScene()
	sc.Root.Children = []*game.Node{n}
	sc.Camera.Pos = game.Vec3{}
	sc.Camera.Target = game.Vec3{Z: -1}
	r := &game.Renderer{W: 64, H: 64, BgColor: game.Color(0, 0, 0, 255)}

	r.Frame(sc)
	before := append([]byte(nil), r.Image().Pix...)

	clip := &game.SkinClip{Name: "rot", Duration: 1, Tracks: []game.BoneTrack{{
		Bone: 0, Keys: []game.KeyFrame{{Time: 0, Rot: game.QuatAxisAngle(game.Vec3{Y: 1}, 0.9), Scl: game.Vec3{X: 1, Y: 1, Z: 1}}},
	}}}
	a := game.NewSkinAnimator(sm)
	a.Play(clip, true, 1)
	a.Update(0.0)
	n.SkinAnim = a

	r.Frame(sc)
	pix := r.Image().Pix
	diff := 0
	for i := range pix {
		if pix[i] != before[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("骨骼旋转后画面完全没变，蒙皮未接入渲染")
	}
}
