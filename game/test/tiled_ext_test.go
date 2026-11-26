// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"guis/game"
)

// writePNG 把纯色小图写到 dir/name，返回文件名。
func writePNG(t *testing.T, dir, name string, w, h int, cells []color.RGBA, cellW int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, c := range cells {
		x0 := i * cellW
		for y := 0; y < h; y++ {
			for x := x0; x < x0+cellW && x < w; x++ {
				img.Set(x, y, c)
			}
		}
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("创建图块集图失败: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码 PNG 失败: %v", err)
	}
}

const tiledFullJSON = `{
 "width": 4, "height": 2, "tilewidth": 8, "tileheight": 8,
 "properties": [{"name": "level", "type": "int", "value": 3}],
 "tilesets": [
  {"firstgid": 1, "name": "setA", "image": "a.png", "imagewidth": 16, "imageheight": 8,
   "tilewidth": 8, "tileheight": 8, "columns": 2, "tilecount": 2,
   "tiles": [{"id": 0, "animation": [{"tileid": 0, "duration": 100}, {"tileid": 1, "duration": 100}]}]},
  {"firstgid": 11, "name": "setB", "image": "b.png", "imagewidth": 8, "imageheight": 8,
   "tilewidth": 8, "tileheight": 8, "columns": 1, "tilecount": 1}
 ],
 "layers": [
  {"type": "group", "name": "world", "opacity": 1, "layers": [
    {"type": "tilelayer", "name": "ground", "width": 4, "height": 2, "visible": true,
     "data": [1, 1, 1, 1, 0, 0, 0, 0]},
    {"type": "tilelayer", "name": "deco", "width": 4, "height": 2, "opacity": 0.5,
     "data": [11, 0, 0, 0, 0, 0, 0, 0]}
  ]},
  {"type": "objectgroup", "name": "ents", "objects": [
    {"id": 1, "name": "spawn", "type": "player", "x": 16, "y": 24, "width": 8, "height": 8,
     "properties": [{"name": "hp", "type": "int", "value": 100}]},
    {"id": 2, "name": "path", "type": "route", "x": 0, "y": 0,
     "polyline": [{"x": 0, "y": 0}, {"x": 16, "y": 0}]}
  ]}
 ]
}`

func newFullMap(t *testing.T) *game.TiledMapFull {
	t.Helper()
	dir := t.TempDir()
	writePNG(t, dir, "a.png", 16, 8, []color.RGBA{{R: 255, A: 255}, {B: 255, A: 255}}, 8)
	writePNG(t, dir, "b.png", 8, 8, []color.RGBA{{G: 255, A: 255}}, 8)
	m, err := game.LoadTiledMapBytes([]byte(tiledFullJSON), dir)
	if err != nil {
		t.Fatalf("解析完整 Tiled 地图失败: %v", err)
	}
	return m
}

func TestTiledMapFullParse(t *testing.T) {
	m := newFullMap(t)
	if m.W != 4 || m.H != 2 || m.TileW != 8 {
		t.Fatalf("地图尺寸 = %d×%d tile=%d", m.W, m.H, m.TileW)
	}
	if len(m.Tilesets) != 2 {
		t.Fatalf("图块集数 = %d，期望 2", len(m.Tilesets))
	}
	if m.Props["level"] != "3" {
		t.Fatalf("地图属性 level = %q，期望 3", m.Props["level"])
	}
	if g := m.Layer("ground"); g == nil || !g.Visible || g.Opacity != 1 {
		t.Fatalf("ground 图层解析不正确: %+v", g)
	}
	// group 层内的子层应可递归查到
	if l := m.Layer("deco"); l == nil || l.Opacity != 0.5 {
		t.Fatalf("子层 deco 解析不正确: %+v", l)
	}
	if len(m.LayersOfType("object")) != 1 {
		t.Fatalf("对象层数 = %d", len(m.LayersOfType("object")))
	}
	objs := m.Objects("ents")
	if len(objs) != 2 {
		t.Fatalf("对象数 = %d，期望 2", len(objs))
	}
	if objs[0].Type != "player" || objs[0].X != 16 || objs[0].Y != 24 {
		t.Fatalf("对象坐标/类型不正确: %+v", objs[0])
	}
	if objs[0].Prop("hp") != "100" {
		t.Fatalf("对象属性 hp = %q，期望 100", objs[0].Prop("hp"))
	}
	if len(objs[1].Polyline) != 2 || objs[1].Polyline[1].X != 16 {
		t.Fatalf("折线点解析不正确: %+v", objs[1].Polyline)
	}
	if byType := m.ObjectsByType("player"); len(byType) != 1 || byType[0].Name != "spawn" {
		t.Fatalf("按类型查对象失败: %+v", byType)
	}
}

func TestTiledTilesetLookup(t *testing.T) {
	m := newFullMap(t)
	ts, id := m.TilesetFor(1)
	if ts == nil || ts.Name != "setA" || id != 0 {
		t.Fatalf("gid=1 -> %v/%d", ts, id)
	}
	ts, id = m.TilesetFor(12)
	if ts == nil || ts.Name != "setB" || id != 1 {
		t.Fatalf("gid=12 -> %v/%d", ts, id)
	}
	// 翻转位应被忽略
	ts, id = m.TilesetFor(int(uint32(11) | game.TiledFlipX | game.TiledFlipY))
	if ts == nil || ts.Name != "setB" || id != 0 {
		t.Fatalf("翻转 gid -> %v/%d", ts, id)
	}
	src, ok := ts.SrcFor(0)
	if !ok || src.W != 8 || src.H != 8 {
		t.Fatalf("源矩形 = %+v ok=%v", src, ok)
	}
	if _, ok := ts.SrcFor(5); ok {
		t.Fatal("越界图块 id 不应返回矩形")
	}
}

func TestTiledAnimatedTile(t *testing.T) {
	m := newFullMap(t)
	if g := m.CurrentGID(1); g != 1 {
		t.Fatalf("t=0 时动画 GID = %d，期望 1", g)
	}
	m.Update(0.15) // 150ms，落在第二帧（100~200ms）
	if g := m.CurrentGID(1); g != 2 {
		t.Fatalf("t=150ms 时动画 GID = %d，期望 2", g)
	}
	m.Update(0.1) // 250ms，回绕到第一帧
	if g := m.CurrentGID(1); g != 1 {
		t.Fatalf("t=250ms 时动画 GID = %d，期望回绕到 1", g)
	}
	// 无动画的图块保持不变
	if g := m.CurrentGID(11); g != 11 {
		t.Fatalf("无动画图块 GID 被改变: %d", g)
	}
}

func TestTiledMapFullDraw(t *testing.T) {
	m := newFullMap(t)
	dst := newCanvas(32, 16)
	m.DrawSprites(dst, 32, 16)
	// ground 层 4 个 gid=1 瓦片（setA 的 tile0，红色）；第二格无叠加
	if c := dst.RGBAAt(12, 4); c.R < 200 || c.G != 0 {
		t.Fatalf("瓦片 (1,0) 应为纯红，实际 %+v", c)
	}
	// deco 层 gid=11（setB 绿色）在 (0,0)，透明度 0.5 叠在红色上
	c := dst.RGBAAt(4, 4)
	if c.R == 0 || c.G == 0 {
		t.Fatalf("多图块集叠加后应为红绿混合，实际 %+v", c)
	}
	// 第二行无瓦片
	if c := dst.RGBAAt(4, 12); c.A != 0 {
		t.Fatalf("空格子不应绘制，实际 %+v", c)
	}
	// 图层隐藏后不再绘制
	m.Layer("ground").Visible = false
	m.Layer("deco").Visible = false
	dst2 := newCanvas(32, 16)
	m.DrawSprites(dst2, 32, 16)
	if c := dst2.RGBAAt(4, 4); c.A != 0 {
		t.Fatalf("隐藏图层不应绘制，实际 %+v", c)
	}
}
