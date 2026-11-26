// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"guis/game"
)

// solidTex 生成 w×h 的不透明纯色纹理。
func solidTex(w, h int, c color.RGBA) *game.Texture {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return game.TextureFromImage(img)
}

// drawnBox 返回非透明像素的包围盒（无像素时 ok=false）。
func drawnBox(dst *image.RGBA) (minX, minY, maxX, maxY int, ok bool) {
	minX, minY = 1<<30, 1<<30
	maxX, maxY = -1, -1
	for y := 0; y < dst.Rect.Dy(); y++ {
		for x := 0; x < dst.Rect.Dx(); x++ {
			if _, _, _, a := dst.At(x, y).RGBA(); a != 0 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
				ok = true
			}
		}
	}
	return
}

func newCanvas(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }

func TestSpriteOriginRotatesAroundAnchor(t *testing.T) {
	tex := solidTex(4, 4, color.RGBA{R: 255, A: 255})

	// 默认锚点 = 中心：绕中心旋转 90° 后仍以 Pos+Size/2 为中心
	dst := newCanvas(40, 40)
	game.DrawSprite(dst, game.Sprite{
		Pos: game.V2(10, 10), Size: game.V2(4, 4), Texture: tex, Rot: math.Pi / 2,
	})
	minX, minY, maxX, maxY, ok := drawnBox(dst)
	if !ok {
		t.Fatal("默认锚点旋转后没有绘制任何像素")
	}
	if float32(minX+maxX)/2 < 11 || float32(minX+maxX)/2 > 13 || float32(minY+maxY)/2 < 11 || float32(minY+maxY)/2 > 13 {
		t.Fatalf("默认锚点旋转中心偏离中心：box=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}

	// 锚点 = 左上角：绕 (10,10) 旋转 90° 后矩形落在左侧下方
	dst = newCanvas(40, 40)
	game.DrawSprite(dst, game.Sprite{
		Pos: game.V2(10, 10), Size: game.V2(4, 4), Texture: tex, Rot: math.Pi / 2,
		UseOrigin: true, Origin: game.V2(0, 0),
	})
	minX, minY, maxX, maxY, ok = drawnBox(dst)
	if !ok {
		t.Fatal("左上锚点旋转后没有绘制任何像素")
	}
	if maxX > 10 || minY < 9 {
		t.Fatalf("左上锚点包围盒不符合预期：box=(%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}

	// 无旋转时锚点不影响绘制矩形
	dst = newCanvas(40, 40)
	game.DrawSprite(dst, game.Sprite{
		Pos: game.V2(10, 10), Size: game.V2(4, 4), Texture: tex,
		UseOrigin: true, Origin: game.V2(1, 1),
	})
	minX, minY, maxX, maxY, ok = drawnBox(dst)
	if !ok || minX != 10 || minY != 10 || maxX != 13 || maxY != 13 {
		t.Fatalf("无旋转时锚点不应改变矩形：box=(%d,%d)-(%d,%d) ok=%v", minX, minY, maxX, maxY, ok)
	}
}

func TestSpriteSortY(t *testing.T) {
	tex := solidTex(2, 2, color.RGBA{G: 255, A: 255})
	b := game.NewSpriteBatch()
	// 同一层同一 Z：脚底 Y 大的应后画
	b.Add(game.Sprite{Pos: game.V2(0, 30), Size: game.V2(8, 8), Texture: tex})
	b.Add(game.Sprite{Pos: game.V2(0, 0), Size: game.V2(8, 8), Texture: tex})
	b.Add(game.Sprite{Pos: game.V2(0, 15), Size: game.V2(8, 8), Texture: tex})
	b.SortY()
	sp := b.Sprites()
	if !(sp[0].Pos.Y < sp[1].Pos.Y && sp[1].Pos.Y < sp[2].Pos.Y) {
		t.Fatalf("SortY 未按脚底 Y 排序: %v %v %v", sp[0].Pos.Y, sp[1].Pos.Y, sp[2].Pos.Y)
	}

	// 显式 Z 优先于脚底 Y
	b.Clear()
	b.Add(game.Sprite{Pos: game.V2(0, 0), Size: game.V2(8, 8), Texture: tex, Z: 5})
	b.Add(game.Sprite{Pos: game.V2(0, 100), Size: game.V2(8, 8), Texture: tex, Z: -1})
	b.SortY()
	if b.Sprites()[0].Z != -1 {
		t.Fatalf("Z 小的应先画，实际首项 Z=%v", b.Sprites()[0].Z)
	}

	// Layer 优先于 Z
	b.Clear()
	b.Add(game.Sprite{Pos: game.V2(0, 0), Size: game.V2(8, 8), Texture: tex, Layer: 2, Z: -10})
	b.Add(game.Sprite{Pos: game.V2(0, 0), Size: game.V2(8, 8), Texture: tex, Layer: 1, Z: 10})
	b.SortY()
	if b.Sprites()[0].Layer != 1 {
		t.Fatalf("Layer 小的应先画，实际首项 Layer=%d", b.Sprites()[0].Layer)
	}
}

func TestNode2DWorldTransform(t *testing.T) {
	parent := game.NewNode2D("body")
	parent.Rot = math.Pi / 2
	child := game.NewNode2D("gun")
	child.Pos = game.V2(10, 0)
	parent.Add(child)

	wp := child.WorldPos()
	if math.Abs(float64(wp.X)) > 1e-3 || math.Abs(float64(wp.Y-10)) > 1e-3 {
		t.Fatalf("子节点世界坐标 = (%v,%v)，期望 (0,10)", wp.X, wp.Y)
	}
	if math.Abs(float64(child.WorldRot()-math.Pi/2)) > 1e-3 {
		t.Fatalf("子节点世界旋转 = %v", child.WorldRot())
	}
	parent.Pos = game.V2(100, 50)
	wp = child.WorldPos()
	if math.Abs(float64(wp.X-100)) > 1e-3 || math.Abs(float64(wp.Y-60)) > 1e-3 {
		t.Fatalf("父位移后子节点世界坐标 = (%v,%v)，期望 (100,60)", wp.X, wp.Y)
	}
}

func TestNode2DAddToBatch(t *testing.T) {
	tex := solidTex(4, 4, color.RGBA{B: 255, A: 255})
	root := game.NewNode2D("root")
	root.Alpha = 0.5
	root.Sprite = &game.Sprite{Size: game.V2(8, 8), Texture: tex, Color: game.RGB(255, 255, 255)}
	kid := game.NewNode2D("kid")
	kid.Pos = game.V2(20, 0)
	kid.Alpha = 0.5
	kid.Sprite = &game.Sprite{Size: game.V2(8, 8), Texture: tex, Color: game.RGB(255, 255, 255)}
	root.Add(kid)

	b := game.NewSpriteBatch()
	b.AddNode2D(root)
	sp := b.Sprites()
	if len(sp) != 2 {
		t.Fatalf("展开后的精灵数 = %d，期望 2", len(sp))
	}
	// 根节点居中在 (0,0) → 左上角 (-4,-4)；子节点在 (20,0) → 左上角 (16,-4)
	if math.Abs(float64(sp[0].Pos.X+4)) > 1e-3 || math.Abs(float64(sp[0].Pos.Y+4)) > 1e-3 {
		t.Fatalf("根精灵位置 = (%v,%v)，期望 (-4,-4)", sp[0].Pos.X, sp[0].Pos.Y)
	}
	if math.Abs(float64(sp[1].Pos.X-16)) > 1e-3 || math.Abs(float64(sp[1].Pos.Y+4)) > 1e-3 {
		t.Fatalf("子精灵位置 = (%v,%v)，期望 (16,-4)", sp[1].Pos.X, sp[1].Pos.Y)
	}
	// 透明度沿子树累乘：0.5 * 0.5 = 0.25 -> A ≈ 64
	if sp[1].Color.A < 55 || sp[1].Color.A > 70 {
		t.Fatalf("子精灵继承透明度 A = %d，期望 ≈64", sp[1].Color.A)
	}

	// 隐藏父节点时子树不展开
	root.Hidden = true
	b.Clear()
	b.AddNode2D(root)
	if b.SpriteCount() != 0 {
		t.Fatalf("Hidden 节点不应展开，实际 %d 个精灵", b.SpriteCount())
	}
}
