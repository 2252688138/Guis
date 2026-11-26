// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"image"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// ASCIIChars 可打印 ASCII 字符集（TTF 烘焙的默认字集）。
const ASCIIChars = " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"

// TTFOptions TTF 烘焙参数。
type TTFOptions struct {
	// Size 字号（像素，默认 16）。烘焙后按该尺寸等比绘制，改尺寸请重新烘焙。
	Size float32
	// Chars 要烘焙的字符集（默认 ASCIIChars）；中文按需给常用字，全集烘焙很耗时。
	Chars string
	// Cols 图集列数（默认 16）。
	Cols int
	// Padding 每格内边距（像素，默认 1），防止描边/斜体被裁切。
	Padding int
	// Hinting 字形微调（默认 font.HintingFull；设为 font.HintingNone 可得到更平滑的圆润字形）。
	Hinting font.Hinting
}

func (o TTFOptions) withDefaults() TTFOptions {
	if o.Size <= 0 {
		o.Size = 16
	}
	if o.Chars == "" {
		o.Chars = ASCIIChars
	}
	if o.Cols <= 0 {
		o.Cols = 16
	}
	if o.Padding < 0 {
		o.Padding = 0
	}
	return o
}

// LoadTTFFile 从文件读取 TTF/OTF 并烘焙成位图字体（纯 Go，无 cgo）。
//
//	f, err := game.LoadTTFFile("assets/font.ttf", game.TTFOptions{Size: 20, Chars: "分数生命0123456789"})
//	f.DrawText(img, "分数 120", 8, 8, 1, game.RGB(255, 255, 255))
func LoadTTFFile(path string, opts TTFOptions) (*BitmapFont, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("game: 读取字体失败: %w", err)
	}
	return LoadTTF(data, opts)
}

// LoadTTF 把 TTF/OTF 字体按字符集烘焙成位图字体（字形写进一张白字透明底图集）。
//
// 烘焙结果按等宽格推进，中文（全角）与 ASCII 共用同一格宽；字符集越小越快。
// 解析失败（非 sfnt、字形缺失）返回错误，不会 panic。
func LoadTTF(data []byte, opts TTFOptions) (*BitmapFont, error) {
	opts = opts.withDefaults()
	if len(data) >= 4 && string(data[:4]) == "ttcf" {
		return nil, fmt.Errorf("game: 不支持字体集合（.ttc），请改用 .ttf/.otf 单字体文件")
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("game: 解析字体失败: %w", err)
	}
	hinting := opts.Hinting
	if hinting == 0 {
		hinting = font.HintingFull
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    float64(opts.Size),
		DPI:     72,
		Hinting: hinting,
	})
	if err != nil {
		return nil, fmt.Errorf("game: 创建字体面失败: %w", err)
	}
	defer face.Close()

	// 去重保序，避免同一字符浪费格子。
	seen := make(map[rune]bool, len(opts.Chars))
	chars := make([]rune, 0, len(opts.Chars))
	for _, ch := range opts.Chars {
		if ch == '\r' || ch == '\n' || seen[ch] {
			continue
		}
		seen[ch] = true
		chars = append(chars, ch)
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("game: 字体字符集为空")
	}

	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	cellH := ascent + descent + opts.Padding*2
	if cellH <= 0 {
		cellH = int(opts.Size) + opts.Padding*2
	}
	cellW := cellH
	cols := opts.Cols
	rows := (len(chars) + cols - 1) / cols

	atlas := NewTexture(cols*cellW, rows*cellH)
	index := make(map[rune]int, len(chars))
	cell := image.NewRGBA(image.Rect(0, 0, cellW, cellH))
	for i, ch := range chars {
		index[ch] = i
		// 每格重画一次：先把字形画进临时图，再按透明底拷贝到图集。
		for k := range cell.Pix {
			cell.Pix[k] = 0
		}
		d := font.Drawer{
			Dst:  cell,
			Src:  image.White,
			Face: face,
			Dot:  fixed.P(opts.Padding, ascent+opts.Padding),
		}
		d.DrawString(string(ch))

		cx, cy := (i%cols)*cellW, (i/cols)*cellH
		for y := 0; y < cellH; y++ {
			for x := 0; x < cellW; x++ {
				si := (y*cell.Stride + x*4)
				di := (cy+y)*atlas.W + (cx + x)
				if di < 0 || di >= len(atlas.Pix) {
					continue
				}
				atlas.Pix[di] = RGBA{
					R: cell.Pix[si], G: cell.Pix[si+1],
					B: cell.Pix[si+2], A: cell.Pix[si+3],
				}
			}
		}
	}

	return &BitmapFont{
		Tex:    atlas,
		CellW:  cellW,
		CellH:  cellH,
		GlyphW: cellW,
		GlyphH: cellH,
		Cols:   cols,
		First:  chars[0],
		Last:   chars[len(chars)-1] + 1,
		Index:  index,
	}, nil
}
