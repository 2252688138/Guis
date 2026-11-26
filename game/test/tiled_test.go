// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"os"
	"testing"

	"guis/game"
)

func loadTestMap(t *testing.T) *game.TiledMap {
	t.Helper()
	b, err := os.ReadFile("testdata/tilemap.json")
	if err != nil {
		t.Fatalf("读取测试地图失败: %v", err)
	}
	m, err := game.LoadTiledJSONBytes(b, "testdata")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return m
}

func TestTiledParse(t *testing.T) {
	m := loadTestMap(t)
	if m.W != 4 || m.H != 3 || m.TileW != 16 {
		t.Fatalf("地图尺寸 = %d×%d tile=%d", m.W, m.H, m.TileW)
	}
	if len(m.Layers) != 2 {
		t.Fatalf("只应保留 tilelayer，实际 %d 层", len(m.Layers))
	}
	if g := m.Tile("ground", 0, 0); g != 1 {
		t.Fatalf("ground(0,0) = %d，期望 1", g)
	}
	if g := m.Tile("ground", 1, 1); g != 0 {
		t.Fatalf("ground(1,1) = %d，期望 0", g)
	}
	if g := m.Tile("walls", 1, 1); g != 2 {
		t.Fatalf("walls(1,1) = %d，期望 2", g)
	}
	if g := m.Tile("ground", 99, 99); g != 0 {
		t.Fatalf("越界应返回 0，实际 %d", g)
	}
	if m.Atlas != nil {
		t.Fatal("图块集图片缺失时应 Atlas = nil")
	}
}

func TestTiledCollisionGrid(t *testing.T) {
	m := loadTestMap(t)
	g := m.CollisionGrid("walls")
	if g.W != 4 || g.H != 3 || g.Cell != 16 {
		t.Fatalf("网格 = %d×%d cell=%v", g.W, g.H, g.Cell)
	}
	if !g.Solid(1, 1) || !g.Solid(2, 1) {
		t.Fatal("(1,1)/(2,1) 应为实体")
	}
	if g.Solid(0, 0) {
		t.Fatal("(0,0) 不应为实体")
	}
	if !g.Solid(-1, 0) || !g.Solid(0, 99) {
		t.Fatal("越界应视为实体")
	}
	if !g.SolidAt(20, 20) {
		t.Fatal("世界坐标 (20,20) 应命中实体格")
	}
	if g.SolidAt(4, 4) {
		t.Fatal("世界坐标 (4,4) 应为空")
	}
	b := g.Bounds()
	if b.Max.X != 64 || b.Max.Z != 48 {
		t.Fatalf("网格包围盒 = %v", b)
	}
	if !g.OverlapAABB(game.AABB{Min: game.Vec3{X: 18, Y: 0, Z: 18}, Max: game.Vec3{X: 22, Y: 1, Z: 22}}) {
		t.Fatal("盒体应命中实体格")
	}
}

func TestGridFindPath(t *testing.T) {
	g := game.NewCollisionGrid(5, 5, 1)
	g.SetSolid(2, 0, true)
	g.SetSolid(2, 1, true)
	g.SetSolid(2, 2, true)
	g.SetSolid(2, 3, true)
	// 只有 (2,4) 可绕
	path := g.FindPath(0, 0, 4, 0)
	if len(path) == 0 {
		t.Fatal("应能找到绕行路径")
	}
	if path[0] != [2]int{0, 0} || path[len(path)-1] != [2]int{4, 0} {
		t.Fatalf("路径端点不对: %v ... %v", path[0], path[len(path)-1])
	}
	for _, p := range path {
		if g.Solid(p[0], p[1]) {
			t.Fatalf("路径穿过实体格: %v", p)
		}
	}
	// 完全封死
	g2 := game.NewCollisionGrid(3, 3, 1)
	for y := 0; y < 3; y++ {
		g2.SetSolid(1, y, true)
	}
	if p := g2.FindPath(0, 0, 2, 0); p != nil {
		t.Fatalf("封死时不应有路径，实际 %v", p)
	}
}

func TestTiledDraw(t *testing.T) {
	m := loadTestMap(t)
	tex := game.NewTexture(64, 16)
	tex.Fill(game.RGB(200, 120, 60))
	m.Atlas = game.NewAtlas(tex, 16, 16, 4)
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	m.DrawSprites(img, 64, 48)
	c := img.RGBAAt(4, 4)
	if c.R < 100 {
		t.Fatalf("图块应被绘制，实际像素 %v", c)
	}
	// 隐藏 walls 层后，(1,1) 应为空（ground 该格 GID=0）
	if w := m.Layer("walls"); w != nil {
		w.Visible = false
	}
	img2 := image.NewRGBA(image.Rect(0, 0, 64, 48))
	m.DrawSprites(img2, 64, 48)
	if c := img2.RGBAAt(20, 20); c.A != 0 {
		t.Fatalf("隐藏图层后空格不应绘制，实际 %v", c)
	}
}
