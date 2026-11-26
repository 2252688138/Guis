package game_test

import (
	"image"
	"testing"

	"guis/game"
)

// absf 浮点绝对值（float32），测试内自用。
func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestAABB2Basics(t *testing.T) {
	a := game.Box2(0, 0, 10, 10)
	if a.W() != 10 || a.H() != 10 {
		t.Fatalf("尺寸 = %v x %v", a.W(), a.H())
	}
	if !a.Contains(5, 5) || !a.Contains(10, 5) || a.Contains(11, 5) {
		t.Fatal("Contains 边界错")
	}
	if !a.Overlaps(game.Box2(9, 9, 4, 4)) {
		t.Fatal("应重叠")
	}
	if a.Overlaps(game.Box2(10, 0, 4, 4)) {
		t.Fatal("相切不应算重叠")
	}
	c := a.Center()
	if c.X != 5 || c.Y != 5 {
		t.Fatalf("中心 = %v", c)
	}
	if m := a.Move(3, 4); m.Min.X != 3 || m.Min.Y != 4 {
		t.Fatalf("平移错误 = %v", m.Min)
	}
}

func TestCamera2DRoundTrip(t *testing.T) {
	c := game.NewCamera2D(320, 180)
	c.X, c.Y = 100, 50
	// 中心映射到屏幕中心（且 Y 翻转）
	sx, sy := c.WorldToScreen(100, 50)
	if sx != 160 || sy != 90 {
		t.Fatalf("中心映射 = %v,%v", sx, sy)
	}
	// 世界 Y 向上：更高的点屏幕 y 更小
	_, top := c.WorldToScreen(100, 60)
	if top >= sy {
		t.Fatalf("Y 轴未翻转: %v", top)
	}
	wx, wy := c.ScreenToWorld(sx, sy)
	if absf(wx-100) > 0.001 || absf(wy-50) > 0.001 {
		t.Fatalf("往返偏差 = %v,%v", wx, wy)
	}
	// 缩放
	c.Zoom = 2
	sx2, _ := c.WorldToScreen(110, 50)
	if absf(sx2-180) > 0.001 {
		t.Fatalf("2 倍缩放映射 = %v，期望 180", sx2)
	}
}

func TestCamera2DClamp(t *testing.T) {
	c := game.NewCamera2D(320, 180)
	c.MinX, c.MinY, c.MaxX, c.MaxY = 0, 0, 1000, 400
	c.Follow(0, 0, 1) // 立即到位 + 夹紧
	if c.X != 160 || c.Y != 90 {
		t.Fatalf("左下角夹紧 = %.1f,%.1f", c.X, c.Y)
	}
	c.Follow(1000, 400, 1)
	if c.X != 840 || c.Y != 310 {
		t.Fatalf("右上角夹紧 = %.1f,%.1f", c.X, c.Y)
	}
	// 平滑跟随
	c.X, c.Y = 500, 200
	c.Follow(600, 200, 0.5)
	if c.X != 550 {
		t.Fatalf("lerp 跟随 = %.1f，期望 550", c.X)
	}
}

func TestPlatformerGroundAndJump(t *testing.T) {
	ground := game.Box2(-100, -20, 400, 20) // 顶面 y = 0
	b := game.NewPlatformerBody(0, 100, 20, 20)
	for i := 0; i < 240; i++ {
		b.Step(1.0/60, []game.AABB2{ground}, nil)
	}
	if !b.Grounded {
		t.Fatal("应落到地面")
	}
	if absf(b.Bottom()) > 0.5 {
		t.Fatalf("底边未贴合地面: %.2f", b.Bottom())
	}
	// 跳起后回落
	b.Vel.Y = 600
	b.Step(1.0/60, []game.AABB2{ground}, nil)
	if b.Grounded || b.Vel.Y <= 0 {
		t.Fatal("应处于上升状态")
	}
	for i := 0; i < 240; i++ {
		b.Step(1.0/60, []game.AABB2{ground}, nil)
	}
	if !b.Grounded {
		t.Fatal("应落回地面")
	}
}

func TestPlatformerWallBlock(t *testing.T) {
	wall := game.Box2(50, 0, 20, 200)
	b := game.NewPlatformerBody(0, 10, 20, 20)
	b.Gravity = -1 // 忽略重力影响
	b.Vel.X = 300
	for i := 0; i < 60; i++ {
		b.Vel.X = 300
		b.Step(1.0/60, []game.AABB2{wall}, nil)
	}
	if b.Right() > 50.01 {
		t.Fatalf("穿墙: right=%.2f", b.Right())
	}
	if !b.HitWall || b.Vel.X != 0 {
		t.Fatalf("撞墙未清零: hit=%v vx=%.1f", b.HitWall, b.Vel.X)
	}
}

func TestPlatformerOneWay(t *testing.T) {
	plat := game.Box2(-50, 0, 100, 10) // 单向平台，顶面 y = 10
	b := game.NewPlatformerBody(0, 120, 20, 20)
	for i := 0; i < 300; i++ {
		b.Step(1.0/60, nil, []game.AABB2{plat})
	}
	if !b.Grounded || absf(b.Bottom()-10) > 0.5 {
		t.Fatalf("应停在平台顶: grounded=%v bottom=%.2f", b.Grounded, b.Bottom())
	}

	// 从下往上穿过平台（顶头不阻挡）
	b2 := game.NewPlatformerBody(0, -100, 20, 20)
	b2.Gravity = 0
	b2.Vel.Y = 400
	for i := 0; i < 60; i++ {
		b2.Step(1.0/60, nil, []game.AABB2{plat})
	}
	if b2.HitCeiling {
		t.Fatal("单向平台不应阻挡上升")
	}
	if b2.Pos.Y <= 20 {
		t.Fatalf("应能穿过单向平台: y=%.2f", b2.Pos.Y)
	}
}

func TestPlatformerCeilingBump(t *testing.T) {
	ceil := game.Box2(-50, 80, 200, 20)
	b := game.NewPlatformerBody(0, 20, 20, 20)
	b.Gravity = 0
	b.Vel.Y = 500
	for i := 0; i < 30 && !b.HitCeiling; i++ {
		b.Step(1.0/60, []game.AABB2{ceil}, nil)
	}
	if !b.HitCeiling || b.Vel.Y != 0 {
		t.Fatalf("应顶到天花板: hit=%v vy=%.1f", b.HitCeiling, b.Vel.Y)
	}
	if b.Top() > 80.01 {
		t.Fatalf("穿透天花板: top=%.2f", b.Top())
	}
}

func TestSpriteFlipSamplesMirrored(t *testing.T) {
	tx := game.NewTexture(2, 1)
	tx.Pix[0] = game.RGB(255, 0, 0)
	tx.Pix[1] = game.RGB(0, 0, 255)
	dst := image.NewRGBA(image.Rect(0, 0, 4, 2))
	game.DrawSprite(dst, game.Sprite{Pos: game.V2(0, 0), Size: game.V2(2, 2), Texture: tx})
	if c := dst.RGBAAt(0, 0); c.R < 200 {
		t.Fatalf("未翻转时左侧应是红色: %v", c)
	}
	dst2 := image.NewRGBA(image.Rect(0, 0, 4, 2))
	game.DrawSprite(dst2, game.Sprite{Pos: game.V2(0, 0), Size: game.V2(2, 2), Texture: tx, FlipX: true})
	if c := dst2.RGBAAt(0, 0); c.B < 200 {
		t.Fatalf("水平翻转后左侧应是蓝色: %v", c)
	}
}
