// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"path/filepath"
	"testing"

	"guis/game"
)

func TestAssetsCacheAndAsync(t *testing.T) {
	loads := 0
	png := mustPNG(t)
	a := game.NewAssets(func(name string) ([]byte, error) {
		loads++
		if name == "bad.png" {
			return []byte("nope"), nil
		}
		return png, nil
	})
	if _, err := a.Texture("a.png"); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if _, err := a.Texture("a.png"); err != nil {
		t.Fatalf("二次加载失败: %v", err)
	}
	if loads != 1 {
		t.Fatalf("缓存未命中，实际加载 %d 次", loads)
	}
	if tex, mesh, snd := a.Cached(); tex != 1 || mesh != 0 || snd != 0 {
		t.Fatalf("缓存统计 = %d/%d/%d", tex, mesh, snd)
	}
	if _, err := a.Texture("bad.png"); err == nil {
		t.Fatal("损坏资源应报错")
	}

	done := 0
	ch := a.LoadAsync([]string{"b.png", "c.png"}, func(d, total int) { done = d })
	if err := <-ch; err != nil {
		t.Fatalf("异步加载失败: %v", err)
	}
	if done != 2 {
		t.Fatalf("进度回调 = %d，期望 2", done)
	}
}

func TestSceneJSONRoundTrip(t *testing.T) {
	sc := game.NewScene()
	sc.SetAmbientIntensity(0.4)
	sc.AddSun(game.V3(-0.5, -1, -0.3), 1)
	box := game.NewBox(1, 2, 3)
	n := game.NewMeshNode("箱子", box, game.DefaultMaterial().WithColor(game.RGB(10, 20, 30)).WithShade(game.ShadeToon))
	n.Pos = game.V3(1, 2, 3)
	n.Rot = game.V3(0.1, 0.2, 0.3)
	child := game.NewMeshNode("子节点", box, game.DefaultMaterial())
	child.Pos = game.V3(0, 1, 0)
	n.Children = append(n.Children, child)
	sc.Add(n)

	b, err := game.SceneJSON(sc, func(m *game.Mesh) string { return "box" })
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	sc2, err := game.LoadSceneJSON(b, func(name string) *game.Mesh {
		if name == "box" {
			return game.NewBox(1, 2, 3)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if sc2.Camera == nil || len(sc2.Lights) != 1 || len(sc2.Root.Children) != 1 {
		t.Fatalf("场景结构不对: cam=%v lights=%d children=%d", sc2.Camera != nil, len(sc2.Lights), len(sc2.Root.Children))
	}
	n2 := sc2.Root.Children[0]
	if n2.Name != "箱子" || n2.Pos != game.V3(1, 2, 3) {
		t.Fatalf("节点还原错误: %q %v", n2.Name, n2.Pos)
	}
	if n2.Material == nil || n2.Material.BaseColor.G != 20 || n2.Material.Shade != game.ShadeToon {
		t.Fatalf("材质还原错误: %+v", n2.Material)
	}
	if len(n2.Children) != 1 {
		t.Fatalf("子节点丢失: %d", len(n2.Children))
	}
	if sc2.Ambient.A == 0 {
		t.Fatal("环境光未还原")
	}
}

func TestSaveStore(t *testing.T) {
	dir := t.TempDir()
	s := game.SaveStore{Dir: dir}
	type save struct {
		Score int
		Name  string
	}
	if err := s.Put("slot1", save{Score: 42, Name: "玩家"}); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if !s.Exists("slot1") {
		t.Fatal("槽位应存在")
	}
	var got save
	if err := s.Get("slot1", &got); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.Score != 42 || got.Name != "玩家" {
		t.Fatalf("读取内容 = %+v", got)
	}
	// 路径穿越过滤：同名字能读回，且没有写到目录外
	if err := s.Put("../evil", save{Score: 7}); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	var back save
	if err := s.Get("../evil", &back); err != nil || back.Score != 7 {
		t.Fatalf("读取带穿越手法槽位失败: %v %+v", err, back)
	}
	if _, err := osStat(filepath.Join(dir, "..", "evil.json")); err == nil {
		t.Fatal("不应写到存档目录之外")
	}
	if err := s.Delete("slot1"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if s.Exists("slot1") {
		t.Fatal("删除后不应存在")
	}
	if err := s.Delete("slot1"); err != nil {
		t.Fatalf("重复删除不应报错: %v", err)
	}
}

func TestRecorderReplay(t *testing.T) {
	r := game.NewRecorder()
	in := game.NewInput()
	r.StartRecord()
	in.HandleKey("w", true)
	r.Record(in)
	in.HandleKey("d", true)
	r.Record(in)
	in.HandleKey("w", false)
	r.Record(in)
	if n := r.StopRecord(); n != 3 {
		t.Fatalf("录制帧数 = %d，期望 3", n)
	}

	path := filepath.Join(t.TempDir(), "replay.json")
	if err := r.Save(path); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	r2 := game.NewRecorder()
	if err := r2.Load(path); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if r2.FrameCount() != 3 {
		t.Fatalf("读取帧数 = %d", r2.FrameCount())
	}

	r2.StartPlayback()
	played := game.NewInput()
	frames := 0
	for r2.Playback(played) {
		frames++
		if frames == 1 && (!played.KeyDown("w") || played.KeyDown("d")) {
			t.Fatalf("第 1 帧回放错误: %v", played.Keys)
		}
		if frames == 2 && !played.KeyDown("d") {
			t.Fatalf("第 2 帧回放错误: %v", played.Keys)
		}
	}
	if frames != 3 {
		t.Fatalf("回放帧数 = %d，期望 3", frames)
	}
}
