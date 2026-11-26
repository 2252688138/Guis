// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
)

// Tiled GID 高位标志位（Tiled 1.x）。
const (
	TiledFlipX   uint32 = 0x80000000
	TiledFlipY   uint32 = 0x40000000
	TiledFlipD   uint32 = 0x20000000
	tiledGIDMask        = 0x1FFFFFFF
)

// TiledAnimFrame 动画瓦片的一帧（Duration 为毫秒）。
type TiledAnimFrame struct {
	TileID   int
	Duration int
}

// TiledTile 单个图块的扩展信息（动画 / 图像 / 属性）。
type TiledTile struct {
	ID        int
	Image     string
	Animation []TiledAnimFrame
	Props     map[string]string
}

// Tileset 图块集（内嵌或外部 .tsx）。
type Tileset struct {
	Name      string
	FirstGID  int
	TileW     int
	TileH     int
	Columns   int
	TileCount int
	Margin    int
	Spacing   int
	Image     string
	// Tex 图块集纹理（图片缺失时为 nil，此时不绘制该图块集的图块）。
	Tex   *Texture
	Tiles []TiledTile
	Props map[string]string
}

// SrcFor 返回图块集内 id 的源矩形（考虑 margin / spacing）。
func (ts *Tileset) SrcFor(id int) (Rect2, bool) {
	if ts == nil || id < 0 || ts.TileW <= 0 || ts.TileH <= 0 {
		return Rect2{}, false
	}
	cols := ts.Columns
	if cols <= 0 {
		if ts.Tex == nil {
			return Rect2{}, false
		}
		span := ts.TileW + ts.Spacing
		if span <= 0 {
			return Rect2{}, false
		}
		cols = (ts.Tex.W - 2*ts.Margin + ts.Spacing) / span
	}
	if cols <= 0 {
		return Rect2{}, false
	}
	if ts.TileCount > 0 && id >= ts.TileCount {
		return Rect2{}, false
	}
	col, row := id%cols, id/cols
	x := float32(ts.Margin + col*(ts.TileW+ts.Spacing))
	y := float32(ts.Margin + row*(ts.TileH+ts.Spacing))
	return Rect2{X: x, Y: y, W: float32(ts.TileW), H: float32(ts.TileH)}, true
}

// Tile 取图块集内的图块定义（无扩展信息时返回零值）。
func (ts *Tileset) Tile(id int) TiledTile {
	if ts == nil {
		return TiledTile{}
	}
	for _, t := range ts.Tiles {
		if t.ID == id {
			return t
		}
	}
	return TiledTile{ID: id}
}

// TiledObject Tiled 对象（矩形 / 点 / 折线 / 多边形 / 瓦片对象）。
//
// 坐标与 Tiled 一致：像素、相对图层原点、Y 轴向下。多边形/折线顶点相对 (X, Y)。
type TiledObject struct {
	ID       int
	Name     string
	Type     string
	Class    string
	X, Y     float32
	W, H     float32
	Rotation float32
	GID      int
	Point    bool
	Visible  bool
	Polyline []Vec2
	Polygon  []Vec2
	Props    map[string]string
}

// Prop 取属性（整数/浮点/布尔/字符串均以字符串返回）。
func (o TiledObject) Prop(name string) string {
	if o.Props == nil {
		return ""
	}
	return o.Props[name]
}

// TiledLayer 通用图层（tile / object / image / group）。
type TiledLayer struct {
	Name     string
	Type     string
	W, H     int
	Data     []int
	Visible  bool
	Opacity  float32
	Offset   Vec2
	Parallax Vec2
	Image    string
	// ImageTex 图像层纹理（仅 Type == "image"）。
	ImageTex *Texture
	Objects  []TiledObject
	Layers   []*TiledLayer
	Props    map[string]string
}

// TiledMapFull 完整的 Tiled 正交地图（多图块集 / 对象层 / 图像层 / 组层 / 动画瓦片）。
type TiledMapFull struct {
	W, H         int
	TileW, TileH int
	Infinite     bool
	Props        map[string]string
	Layers       []*TiledLayer
	Tilesets     []*Tileset
	// Camera 视差相机位置（世界像素，外部每帧写入；0 表示无视差偏移）。
	Camera Vec2

	baseDir string
	time    float32
}

// tiledJSON / tiledLayerJSON ... 为 Tiled JSON 的原始结构。
type tiledJSON struct {
	Width      int              `json:"width"`
	Height     int              `json:"height"`
	TileWidth  int              `json:"tilewidth"`
	TileHeight int              `json:"tileheight"`
	Infinite   bool             `json:"infinite"`
	Properties []tiledPropJSON  `json:"properties"`
	Layers     []tiledLayerJSON `json:"layers"`
	Tilesets   []tiledSetJSON   `json:"tilesets"`
}

type tiledPropJSON struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type tiledLayerJSON struct {
	Type       string           `json:"type"`
	Name       string           `json:"name"`
	Width      int              `json:"width"`
	Height     int              `json:"height"`
	Data       []int            `json:"data"`
	Visible    *bool            `json:"visible"`
	Opacity    float64          `json:"opacity"`
	OffsetX    float64          `json:"offsetx"`
	OffsetY    float64          `json:"offsety"`
	ParallaxX  *float64         `json:"parallaxx"`
	ParallaxY  *float64         `json:"parallaxy"`
	Image      string           `json:"image"`
	Objects    []tiledObjJSON   `json:"objects"`
	Layers     []tiledLayerJSON `json:"layers"`
	Properties []tiledPropJSON  `json:"properties"`
}

type tiledObjJSON struct {
	ID         int             `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Class      string          `json:"class"`
	X          float64         `json:"x"`
	Y          float64         `json:"y"`
	Width      float64         `json:"width"`
	Height     float64         `json:"height"`
	Rotation   float64         `json:"rotation"`
	GID        int             `json:"gid"`
	Point      bool            `json:"point"`
	Visible    *bool           `json:"visible"`
	Polyline   []tiledPtJSON   `json:"polyline"`
	Polygon    []tiledPtJSON   `json:"polygon"`
	Properties []tiledPropJSON `json:"properties"`
}

type tiledPtJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type tiledSetJSON struct {
	FirstGID    int             `json:"firstgid"`
	Source      string          `json:"source"`
	Name        string          `json:"name"`
	Image       string          `json:"image"`
	ImageWidth  int             `json:"imagewidth"`
	ImageHeight int             `json:"imageheight"`
	TileWidth   int             `json:"tilewidth"`
	TileHeight  int             `json:"tileheight"`
	Columns     int             `json:"columns"`
	Margin      int             `json:"margin"`
	Spacing     int             `json:"spacing"`
	TileCount   int             `json:"tilecount"`
	Tiles       []tiledTileJSON `json:"tiles"`
	Properties  []tiledPropJSON `json:"properties"`
}

type tiledTileJSON struct {
	ID        int    `json:"id"`
	Image     string `json:"image"`
	Animation []struct {
		TileID   int `json:"tileid"`
		Duration int `json:"duration"`
	} `json:"animation"`
	Properties []tiledPropJSON `json:"properties"`
}

// LoadTiledMap 读取 Tiled 导出的 .tmj/.json（支持多图块集、对象层、图像层、组层、动画瓦片）。
func LoadTiledMap(path string) (*TiledMapFull, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadTiledMapBytes(b, filepath.Dir(path))
}

// LoadTiledMapBytes 从内存解析 Tiled JSON（baseDir 用于解析相对资源路径）。
func LoadTiledMapBytes(b []byte, baseDir string) (*TiledMapFull, error) {
	var f tiledJSON
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("game: 解析 Tiled 地图失败: %w", err)
	}
	m := &TiledMapFull{
		W: f.Width, H: f.Height,
		TileW: f.TileWidth, TileH: f.TileHeight,
		Infinite: f.Infinite,
		Props:    propsFromJSON(f.Properties),
		baseDir:  baseDir,
	}
	if m.TileW <= 0 {
		m.TileW = 16
	}
	if m.TileH <= 0 {
		m.TileH = 16
	}
	for _, ts := range f.Tilesets {
		m.Tilesets = append(m.Tilesets, loadTileset(ts, baseDir))
	}
	for _, l := range f.Layers {
		m.Layers = append(m.Layers, m.buildLayer(l, 0))
	}
	return m, nil
}

func (m *TiledMapFull) buildLayer(l tiledLayerJSON, depth int) *TiledLayer {
	lt := &TiledLayer{
		Name: l.Name, Type: l.Type,
		W: l.Width, H: l.Height,
		Data: l.Data, Visible: true, Opacity: 1,
		Offset:   Vec2{X: float32(l.OffsetX), Y: float32(l.OffsetY)},
		Parallax: Vec2{X: 1, Y: 1},
		Image:    l.Image,
		Props:    propsFromJSON(l.Properties),
	}
	if lt.Type == "" {
		lt.Type = "tile"
	}
	if lt.Type == "tile" || lt.Type == "tilelayer" {
		lt.Type = "tile"
		if lt.W <= 0 {
			lt.W = m.W
		}
		if lt.H <= 0 {
			lt.H = m.H
		}
	} else if lt.Type == "objectgroup" {
		lt.Type = "object"
	} else if lt.Type == "imagelayer" {
		lt.Type = "image"
	} else if lt.Type == "group" {
		lt.Type = "group"
	}
	if l.Visible != nil {
		lt.Visible = *l.Visible
	}
	if l.Opacity > 0 {
		lt.Opacity = float32(l.Opacity)
	}
	if l.ParallaxX != nil {
		lt.Parallax.X = float32(*l.ParallaxX)
	}
	if l.ParallaxY != nil {
		lt.Parallax.Y = float32(*l.ParallaxY)
	}
	if lt.Type == "image" && lt.Image != "" {
		if tex, err := LoadTexture(m.resolve(lt.Image)); err == nil {
			lt.ImageTex = tex
		}
	}
	for _, o := range l.Objects {
		obj := TiledObject{
			ID: o.ID, Name: o.Name, Type: o.Type, Class: o.Class,
			X: float32(o.X), Y: float32(o.Y), W: float32(o.Width), H: float32(o.Height),
			Rotation: float32(o.Rotation), GID: o.GID & tiledGIDMask,
			Point: o.Point, Visible: true,
			Props: propsFromJSON(o.Properties),
		}
		if o.Visible != nil {
			obj.Visible = *o.Visible
		}
		for _, p := range o.Polyline {
			obj.Polyline = append(obj.Polyline, Vec2{X: float32(p.X), Y: float32(p.Y)})
		}
		for _, p := range o.Polygon {
			obj.Polygon = append(obj.Polygon, Vec2{X: float32(p.X), Y: float32(p.Y)})
		}
		lt.Objects = append(lt.Objects, obj)
	}
	if depth < 8 {
		for _, c := range l.Layers {
			lt.Layers = append(lt.Layers, m.buildLayer(c, depth+1))
		}
	}
	return lt
}

func (m *TiledMapFull) resolve(p string) string {
	if p == "" || filepath.IsAbs(p) || m.baseDir == "" {
		return p
	}
	return filepath.Join(m.baseDir, p)
}

func loadTileset(ts tiledSetJSON, baseDir string) *Tileset {
	out := &Tileset{
		FirstGID: ts.FirstGID, Name: ts.Name,
		TileW: ts.TileWidth, TileH: ts.TileHeight,
		Columns: ts.Columns, TileCount: ts.TileCount,
		Margin: ts.Margin, Spacing: ts.Spacing,
		Image: ts.Image,
		Props: propsFromJSON(ts.Properties),
	}
	imgPath := ts.Image
	if out.FirstGID <= 0 {
		out.FirstGID = 1
	}
	if ts.Source != "" {
		if ext, err := loadTSX(resolvePath(baseDir, ts.Source)); err == nil {
			if out.Name == "" {
				out.Name = ext.Name
			}
			if out.TileW <= 0 {
				out.TileW = ext.TileW
			}
			if out.TileH <= 0 {
				out.TileH = ext.TileH
			}
			if out.Columns <= 0 {
				out.Columns = ext.Columns
			}
			if out.TileCount <= 0 {
				out.TileCount = ext.TileCount
			}
			if out.Margin == 0 {
				out.Margin = ext.Margin
			}
			if out.Spacing == 0 {
				out.Spacing = ext.Spacing
			}
			imgPath = ext.Image
			out.Tiles = ext.Tiles
			if len(ext.Props) > 0 && out.Props == nil {
				out.Props = ext.Props
			}
		}
	} else {
		for _, t := range ts.Tiles {
			tile := TiledTile{ID: t.ID, Image: t.Image, Props: propsFromJSON(t.Properties)}
			for _, a := range t.Animation {
				tile.Animation = append(tile.Animation, TiledAnimFrame{TileID: a.TileID, Duration: a.Duration})
			}
			out.Tiles = append(out.Tiles, tile)
		}
	}
	if imgPath != "" {
		out.Image = imgPath
		if tex, err := LoadTexture(resolvePath(baseDir, imgPath)); err == nil {
			out.Tex = tex
		}
	}
	return out
}

func resolvePath(baseDir, p string) string {
	if p == "" || filepath.IsAbs(p) || baseDir == "" {
		return p
	}
	return filepath.Join(baseDir, p)
}

// tsxFile / tsxTile ... 为外部图块集 .tsx（XML）结构。
type tsxFile struct {
	XMLName    xml.Name      `xml:"tileset"`
	Name       string        `xml:"name,attr"`
	TileWidth  int           `xml:"tilewidth,attr"`
	TileHeight int           `xml:"tileheight,attr"`
	TileCount  int           `xml:"tilecount,attr"`
	Columns    int           `xml:"columns,attr"`
	Margin     int           `xml:"margin,attr"`
	Spacing    int           `xml:"spacing,attr"`
	Image      tsxImage      `xml:"image"`
	Tiles      []tsxTile     `xml:"tile"`
	Properties tsxProperties `xml:"properties"`
}

type tsxImage struct {
	Source string `xml:"source,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type tsxTile struct {
	ID         int           `xml:"id,attr"`
	Image      tsxImage      `xml:"image"`
	Animation  tsxAnimation  `xml:"animation"`
	Properties tsxProperties `xml:"properties"`
}

type tsxAnimation struct {
	Frames []tsxFrame `xml:"frame"`
}

type tsxFrame struct {
	TileID   int `xml:"tileid,attr"`
	Duration int `xml:"duration,attr"`
}

type tsxProperties struct {
	Props []tsxProp `xml:"property"`
}

type tsxProp struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

func loadTSX(path string) (*Tileset, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f tsxFile
	if err := xml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	ts := &Tileset{
		Name: f.Name, TileW: f.TileWidth, TileH: f.TileHeight,
		Columns: f.Columns, TileCount: f.TileCount,
		Margin: f.Margin, Spacing: f.Spacing,
		Image: f.Image.Source,
	}
	for _, p := range f.Properties.Props {
		if ts.Props == nil {
			ts.Props = map[string]string{}
		}
		ts.Props[p.Name] = p.Value
	}
	for _, t := range f.Tiles {
		tile := TiledTile{ID: t.ID, Image: t.Image.Source}
		for _, fr := range t.Animation.Frames {
			tile.Animation = append(tile.Animation, TiledAnimFrame{TileID: fr.TileID, Duration: fr.Duration})
		}
		for _, p := range t.Properties.Props {
			if tile.Props == nil {
				tile.Props = map[string]string{}
			}
			tile.Props[p.Name] = p.Value
		}
		ts.Tiles = append(ts.Tiles, tile)
	}
	return ts, nil
}

func propsFromJSON(props []tiledPropJSON) map[string]string {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]string, len(props))
	for _, p := range props {
		out[p.Name] = rawToString(p.Value)
	}
	return out
}

func rawToString(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(v, &f); err == nil {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return string(v)
}

// Update 推进动画瓦片时间（dt 秒）。
func (m *TiledMapFull) Update(dt float32) {
	if m == nil || dt <= 0 {
		return
	}
	m.time += dt
}

// Time 当前动画时间（秒）。
func (m *TiledMapFull) Time() float32 {
	if m == nil {
		return 0
	}
	return m.time
}

// TilesetFor 按 GID 找到图块集与集内 id（自动去除翻转位）。
func (m *TiledMapFull) TilesetFor(gid int) (*Tileset, int) {
	if m == nil || gid <= 0 {
		return nil, 0
	}
	g := int(uint32(gid) & tiledGIDMask)
	var best *Tileset
	for _, ts := range m.Tilesets {
		if g >= ts.FirstGID && (best == nil || ts.FirstGID > best.FirstGID) {
			best = ts
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, g - best.FirstGID
}

// CurrentGID 返回考虑动画帧后的当前 GID（保留翻转位）。
func (m *TiledMapFull) CurrentGID(gid int) int {
	ts, id := m.TilesetFor(gid)
	if ts == nil {
		return gid
	}
	anim := ts.Tile(id).Animation
	if len(anim) == 0 {
		return gid
	}
	total := 0
	for _, f := range anim {
		total += f.Duration
	}
	if total <= 0 {
		return gid
	}
	t := int(m.time*1000) % total
	sel := anim[0].TileID
	for _, f := range anim {
		if t < f.Duration {
			sel = f.TileID
			break
		}
		t -= f.Duration
	}
	flip := uint32(gid) &^ tiledGIDMask
	return int(flip) | (ts.FirstGID + sel)
}

// Layer 按名字递归查找图层。
func (m *TiledMapFull) Layer(name string) *TiledLayer {
	if m == nil {
		return nil
	}
	var found *TiledLayer
	var walk func(ls []*TiledLayer)
	walk = func(ls []*TiledLayer) {
		for _, l := range ls {
			if found != nil {
				return
			}
			if l.Name == name {
				found = l
				return
			}
			walk(l.Layers)
		}
	}
	walk(m.Layers)
	return found
}

// LayersOfType 收集指定类型图层（tile / object / image / group）。
func (m *TiledMapFull) LayersOfType(typ string) []*TiledLayer {
	if m == nil {
		return nil
	}
	var out []*TiledLayer
	var walk func(ls []*TiledLayer)
	walk = func(ls []*TiledLayer) {
		for _, l := range ls {
			if l.Type == typ {
				out = append(out, l)
			}
			walk(l.Layers)
		}
	}
	walk(m.Layers)
	return out
}

// Objects 返回对象层内全部对象（图层不存在时返回 nil）。
func (m *TiledMapFull) Objects(layer string) []TiledObject {
	if l := m.Layer(layer); l != nil {
		return l.Objects
	}
	return nil
}

// ObjectsByType 返回全部对象层中 type 匹配的对象（type 为空时匹配所有）。
func (m *TiledMapFull) ObjectsByType(typ string) []TiledObject {
	var out []TiledObject
	for _, l := range m.LayersOfType("object") {
		for _, o := range l.Objects {
			if typ == "" || o.Type == typ || o.Class == typ {
				out = append(out, o)
			}
		}
	}
	return out
}

// Tile 取图层某格 GID（越界返回 0）。
func (m *TiledMapFull) Tile(layer string, tx, ty int) int {
	l := m.Layer(layer)
	if l == nil || tx < 0 || ty < 0 || tx >= l.W || ty >= l.H {
		return 0
	}
	i := ty*l.W + tx
	if i < 0 || i >= len(l.Data) {
		return 0
	}
	return l.Data[i]
}

// DrawSprites 实现 Sprite2D：按图层树顺序绘制图像层与图块层（自动剔除屏外）。
func (m *TiledMapFull) DrawSprites(dst *image.RGBA, w, h int) {
	if m == nil || dst == nil {
		return
	}
	for _, l := range m.Layers {
		m.drawLayer(l, dst, w, h, 1, Vec2{})
	}
}

func (m *TiledMapFull) drawLayer(l *TiledLayer, dst *image.RGBA, w, h int, alpha float32, off Vec2) {
	if l == nil || !l.Visible {
		return
	}
	a := alpha * l.Opacity
	if a <= 0 {
		return
	}
	// 视差：parallax=1 跟随相机（无偏移），<1 时随相机少动
	ox := off.X + l.Offset.X + (l.Parallax.X-1)*m.Camera.X
	oy := off.Y + l.Offset.Y + (l.Parallax.Y-1)*m.Camera.Y

	switch l.Type {
	case "group":
		for _, c := range l.Layers {
			m.drawLayer(c, dst, w, h, a, Vec2{X: ox, Y: oy})
		}
	case "image":
		if l.ImageTex == nil {
			return
		}
		tw, th := float32(l.ImageTex.W), float32(l.ImageTex.H)
		if ox+tw < 0 || oy+th < 0 || ox > float32(w) || oy > float32(h) {
			return
		}
		DrawSprite(dst, Sprite{
			Pos: V2(ox, oy), Size: V2(tw, th), Texture: l.ImageTex,
			Color: mulSpriteAlpha(RGB(255, 255, 255), a),
		})
	case "tile":
		m.drawTileLayer(l, dst, w, h, a, ox, oy)
	}
}

func (m *TiledMapFull) drawTileLayer(l *TiledLayer, dst *image.RGBA, w, h int, alpha, ox, oy float32) {
	tw, th := float32(m.TileW), float32(m.TileH)
	for ty := 0; ty < l.H; ty++ {
		y := oy + float32(ty*m.TileH)
		if y > float32(h) || y+th < 0 {
			continue
		}
		for tx := 0; tx < l.W; tx++ {
			i := ty*l.W + tx
			if i >= len(l.Data) {
				break
			}
			gid := l.Data[i]
			if gid <= 0 {
				continue
			}
			x := ox + float32(tx*m.TileW)
			if x > float32(w) || x+tw < 0 {
				continue
			}
			m.drawTile(dst, gid, x, y, tw, th, alpha)
		}
	}
}

func (m *TiledMapFull) drawTile(dst *image.RGBA, gid int, x, y, tw, th, alpha float32) {
	cur := m.CurrentGID(gid)
	ts, id := m.TilesetFor(cur)
	if ts == nil || ts.Tex == nil {
		return
	}
	src, ok := ts.SrcFor(id)
	if !ok {
		return
	}
	u := uint32(gid)
	DrawSprite(dst, Sprite{
		Pos: V2(x, y), Size: V2(tw, th),
		Texture: ts.Tex, Src: src,
		Color: mulSpriteAlpha(RGB(255, 255, 255), alpha),
		FlipX: u&TiledFlipX != 0,
		FlipY: u&TiledFlipY != 0,
	})
}
