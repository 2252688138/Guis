// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func testClip() *game.AnimClip {
	c := &game.AnimClip{FPS: 10, Loop: true}
	for i := 0; i < 4; i++ {
		c.Frames = append(c.Frames, game.Rect2{X: float32(i * 8), Y: 0, W: 8, H: 8})
	}
	c.Events = []game.AnimEvent{{Frame: 2, Name: "hit"}}
	return c
}

func TestSpriteAnimAdvance(t *testing.T) {
	a := &game.SpriteAnim{}
	a.Play(testClip())
	if a.Index() != 0 {
		t.Fatalf("起始帧 = %d", a.Index())
	}
	a.Update(0.1) // 10fps，0.1s → 第 1 帧
	if a.Index() != 1 {
		t.Fatalf("0.1s 后帧 = %d，期望 1", a.Index())
	}
	if f := a.Frame(); f.X != 8 {
		t.Fatalf("帧矩形 X = %v，期望 8", f.X)
	}
	if ev := a.Update(0.1); len(ev) != 1 || ev[0] != 2 {
		t.Fatalf("应在第 2 帧触发事件，实际 %v", ev)
	}
	a.Update(0.2) // 跨到第 4 帧（回绕）
	if a.Index() != 0 && a.Index() != 3 {
		t.Fatalf("回绕后帧 = %d", a.Index())
	}
}

func TestSpriteAnimNonLoopStop(t *testing.T) {
	c := testClip()
	c.Loop = false
	a := &game.SpriteAnim{}
	a.Play(c)
	a.Update(10)
	if !a.Done {
		t.Fatal("非循环片段播完应置 Done")
	}
	if a.Index() != 3 {
		t.Fatalf("播完应停在末帧，实际 %d", a.Index())
	}
	if ev := a.Update(1); ev != nil {
		t.Fatalf("Done 后不应再触发事件: %v", ev)
	}
}

func TestSpriteAnimPause(t *testing.T) {
	a := &game.SpriteAnim{}
	a.Play(testClip())
	a.Update(0.1)
	a.Stop()
	before := a.Index()
	a.Update(1)
	if a.Index() != before {
		t.Fatalf("暂停后帧应不变: %d → %d", before, a.Index())
	}
}

func TestAnimClipDuration(t *testing.T) {
	c := testClip()
	if d := c.Duration(); d < 0.39 || d > 0.41 {
		t.Fatalf("4 帧 10fps 时长 = %v，期望 0.4", d)
	}
	var nilClip *game.AnimClip
	if nilClip.Count() != 0 || nilClip.Duration() != 0 {
		t.Fatal("nil 片段应安全返回 0")
	}
}
