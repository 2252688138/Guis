// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"testing"

	"guis/game"
)

func TestParticlesEmitAndExpire(t *testing.T) {
	p := game.NewParticleSystem(game.Emitter{Life: [2]float32{0.2, 0.2}, Max: 16})
	p.Emit(5)
	if p.Count() != 5 {
		t.Fatalf("发射后粒子数 = %d，期望 5", p.Count())
	}
	p.Update(0.1)
	if p.Count() != 5 {
		t.Fatalf("未到寿命不应减少，实际 %d", p.Count())
	}
	p.Update(0.2)
	if p.Count() != 0 {
		t.Fatalf("寿命结束后应清空，实际 %d", p.Count())
	}
}

func TestParticlesPoolCap(t *testing.T) {
	p := game.NewParticleSystem(game.Emitter{Life: [2]float32{9, 9}, Max: 4})
	p.Emit(100)
	if p.Count() != 4 {
		t.Fatalf("不应超过池上限，实际 %d", p.Count())
	}
}

func TestParticlesRate(t *testing.T) {
	p := game.NewParticleSystem(game.Emitter{Rate: 100, Life: [2]float32{5, 5}, Max: 200})
	p.Update(0.1) // 100/s * 0.1s = 10
	if p.Count() != 10 {
		t.Fatalf("按速率发射 = %d，期望 10", p.Count())
	}
}

func TestParticlesDraw(t *testing.T) {
	p := game.NewParticleSystem(game.Emitter{
		Pos:   game.Vec2{X: 16, Y: 16},
		Life:  [2]float32{9, 9},
		Speed: [2]float32{0, 0},
		Size:  [2]float32{8, 8},
		Color: [2]game.RGBA{{R: 255, A: 255}, {R: 255, A: 0}},
		Max:   4,
	})
	p.Emit(1)
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	p.DrawSprites(img, 32, 32)
	c := img.RGBAAt(16, 16)
	if c.R < 100 {
		t.Fatalf("中心像素应被粒子染色，实际 %v", c)
	}
}

func TestParticlesNoAlloc(t *testing.T) {
	p := game.NewParticleSystem(game.Emitter{Rate: 60, Life: [2]float32{0.5, 1}, Max: 256})
	p.Update(1.0 / 60)
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	n := testing.AllocsPerRun(30, func() {
		p.Update(1.0 / 60)
		p.DrawSprites(img, 64, 64)
	})
	if n > 4 {
		t.Fatalf("每帧分配次数过多: %.1f", n)
	}
}
