package game_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"guis/game"
)

// TestEditorUndoRedo 编辑器变换/增删的撤销重做。
func TestEditorUndoRedo(t *testing.T) {
	sc := game.NewScene()
	cube := game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial())
	sc.Root.Add(cube)
	ed := game.NewEditor(sc)
	if ed.Camera() == nil {
		t.Fatalf("编辑器应有相机")
	}

	ed.Select(cube)
	if ed.Primary() != cube || !ed.IsSelected(cube) {
		t.Fatalf("选择失败")
	}

	ed.BeginTransform()
	ed.TranslateSelected(game.V3(1, 2, 3), true)
	ed.CommitTransform("移动")
	if cube.Pos != game.V3(1, 2, 3) {
		t.Fatalf("平移结果 %v", cube.Pos)
	}
	if _, ok := ed.Undo(); !ok {
		t.Fatalf("应有可撤销步骤")
	}
	if cube.Pos != game.V3(0, 0, 0) {
		t.Fatalf("撤销后应回到原点，实为 %v", cube.Pos)
	}
	if _, ok := ed.Redo(); !ok {
		t.Fatalf("应有可重做步骤")
	}
	if cube.Pos != game.V3(1, 2, 3) {
		t.Fatalf("重做后应回到 (1,2,3)，实为 %v", cube.Pos)
	}

	// 旋转与缩放
	ed.BeginTransform()
	ed.RotateSelected(game.QuatAxisAngle(game.V3(0, 1, 0), 1.5707964))
	ed.ScaleSelected(2)
	ed.CommitTransform("旋转缩放")
	if cube.Quat.W == 0 && cube.Quat.Y == 0 {
		t.Fatalf("旋转未写入四元数: %+v", cube.Quat)
	}
	if cube.Scl.X != 2 {
		t.Fatalf("缩放未生效: %v", cube.Scl)
	}
	ed.Undo()

	// 新增 / 删除
	box2 := game.NewMeshNode("box2", game.NewBox(1, 1, 1), nil)
	ed.AddNode(box2)
	if len(sc.Root.Children) != 2 {
		t.Fatalf("新增后子节点数 %d", len(sc.Root.Children))
	}
	ed.Select(box2)
	if n := ed.DeleteSelected(); n != 1 {
		t.Fatalf("应删除 1 个节点，实际 %d", n)
	}
	if len(sc.Root.Children) != 1 {
		t.Fatalf("删除后子节点数 %d", len(sc.Root.Children))
	}
	ed.Undo()
	if len(sc.Root.Children) != 2 || sc.Root.Children[1] != box2 {
		t.Fatalf("撤销删除后应恢复原位置")
	}
	ed.Redo()
	if len(sc.Root.Children) != 1 {
		t.Fatalf("重做删除失败")
	}
}

// TestEditorPickRay 射线拾取命中盒体并返回命中点。
func TestEditorPickRay(t *testing.T) {
	sc := game.NewScene()
	cube := game.NewMeshNode("cube", game.NewBox(2, 2, 2), game.DefaultMaterial())
	sc.Root.Add(cube)
	ed := game.NewEditor(sc)

	n, hit, d, ok := ed.PickRay(game.V3(0, 0, 5), game.V3(0, 0, -1))
	if !ok || n != cube {
		t.Fatalf("应命中立方体: ok=%v n=%v", ok, n)
	}
	if d < 3.9 || d > 4.1 {
		t.Fatalf("命中距离应约 4，实为 %.3f", d)
	}
	if hit.Z < 0.9 || hit.Z > 1.1 {
		t.Fatalf("命中点 z 应约为 1，实为 %.3f", hit.Z)
	}
	// 偏离方向不应命中
	if _, _, _, ok := ed.PickRay(game.V3(0, 0, 5), game.V3(1, 0, 0)); ok {
		t.Fatalf("无关方向不应命中")
	}
}

// TestEditorPickScreen 屏幕坐标 → 射线 → 拾取，并校验聚焦。
func TestEditorPickScreen(t *testing.T) {
	sc := game.NewScene()
	cube := game.NewMeshNode("cube", game.NewBox(2, 2, 2), game.DefaultMaterial())
	sc.Root.Add(cube)
	sc.Camera.Pos = game.V3(0, 0, 6)
	sc.Camera.Target = game.V3(0, 0, 0)
	ed := game.NewEditor(sc)

	n, _, _, ok := ed.PickScreen(sc.Camera, 1.0, 320, 180, 640, 360)
	if !ok || n != cube {
		t.Fatalf("画面中心应命中立方体: ok=%v", ok)
	}
	if _, _, _, ok := ed.PickScreen(sc.Camera, 1.0, 4, 4, 640, 360); ok {
		t.Fatalf("画面角落不应命中")
	}
	ed.Select(cube)
	ed.FrameNode(cube)
	if sc.Camera.Target.Sub(game.V3(0, 0, 0)).Len() > 0.01 {
		t.Fatalf("聚焦后注视点应在节点中心")
	}
}

// TestHotReloader 文件变化后触发重载回调。
func TestHotReloader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "level.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := 0
	h := game.NewHotReloader()
	h.Watch(path, func(p string) error {
		hits++
		return nil
	})
	if len(h.Paths()) != 1 {
		t.Fatalf("应监视 1 个文件")
	}
	if n := h.Poll(time.Now()); n != 0 {
		t.Fatalf("未变化不应重载，实际 %d", n)
	}
	// 改内容并推进 mtime
	if err := os.WriteFile(path, []byte("{\"a\":1}"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if n := h.Poll(time.Now()); n != 1 || hits != 1 {
		t.Fatalf("应重载 1 次: n=%d hits=%d", n, hits)
	}
	if n := h.Poll(time.Now()); n != 0 {
		t.Fatalf("重复轮询不应再次重载")
	}
	if err := h.Reload(path); err != nil {
		t.Fatalf("强制重载失败: %v", err)
	}
	if hits != 2 || h.Reloads != 2 {
		t.Fatalf("强制重载计数错误 hits=%d reloads=%d", hits, h.Reloads)
	}
	h.Unwatch(path)
	if len(h.Paths()) != 0 {
		t.Fatalf("取消监视后列表应为空")
	}
}
