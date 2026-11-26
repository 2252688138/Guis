// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"image"
	"os"
	"runtime"
	"testing"

	"guis/game"
)

// fontCandidates 各平台常见字体文件（依次尝试）。
var fontCandidates = map[string][]string{
	"windows": {
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\Deng.ttf`,
		`C:\Windows\Fonts\arial.ttf`,
	},
	"linux": {
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	},
	"darwin": {
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/PingFang.ttc",
	},
}

// readFont 读取一个可用字体，找不到则跳过测试。
func readFont(t *testing.T) []byte {
	t.Helper()
	for _, p := range fontCandidates[runtime.GOOS] {
		data, err := os.ReadFile(p)
		if err == nil && len(data) > 0 {
			return data
		}
	}
	t.Skip("本机没有可用的系统字体，跳过 TTF 烘焙测试")
	return nil
}

func TestLoadTTFASCII(t *testing.T) {
	data := readFont(t)
	f, err := game.LoadTTF(data, game.TTFOptions{Size: 18, Chars: "ABCabc0123"})
	if err != nil {
		t.Skipf("该字体无法用 opentype 解析（部分 .ttc 集合不支持）: %v", err)
	}
	if f == nil || f.Tex == nil {
		t.Fatal("烘焙结果为空")
	}
	if f.CellW <= 0 || f.CellH <= 18 {
		t.Fatalf("字形格尺寸异常: %dx%d", f.CellW, f.CellH)
	}
	if _, ok := f.Glyph('A'); !ok {
		t.Fatal("字符 A 应能取到字形")
	}
	if _, ok := f.Glyph('0'); !ok {
		t.Fatal("字符 0 应能取到字形")
	}
	if _, ok := f.Glyph('中'); ok {
		t.Fatal("未烘焙的中文不应命中字形")
	}

	// 实际绘制：应有点亮的像素。
	img := image.NewRGBA(image.Rect(0, 0, 160, 40))
	f.DrawText(img, "ABC 012", 2, 2, 1, game.RGB(255, 255, 255))
	lit := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("DrawText 后没有任何像素被点亮")
	}
	if w, h := f.Measure("ABC", 1); w <= 0 || h <= 0 {
		t.Fatalf("Measure 结果异常: %v %v", w, h)
	}
}

func TestLoadTTFCJK(t *testing.T) {
	data := readFont(t)
	chars := "分数生命值暂停开始"
	f, err := game.LoadTTF(data, game.TTFOptions{Size: 16, Chars: chars})
	if err != nil {
		t.Skipf("该字体不支持烘焙: %v", err)
	}
	if _, ok := f.Glyph('分'); !ok {
		t.Skip("该字体不含中文字形")
	}
	if f.Cols <= 0 || f.Tex.W <= 0 {
		t.Fatalf("图集异常: cols=%d w=%d", f.Cols, f.Tex.W)
	}
	if len(f.Index) != len([]rune(chars)) {
		t.Fatalf("字符映射数量不符: %d", len(f.Index))
	}
}

func TestLoadTTFBadData(t *testing.T) {
	if _, err := game.LoadTTF([]byte("not a font"), game.TTFOptions{Size: 12}); err == nil {
		t.Fatal("非字体数据应返回错误")
	}
	if _, err := game.LoadTTFFile("不存在的字体文件.ttf", game.TTFOptions{Size: 12}); err == nil {
		t.Fatal("读取不存在的字体应返回错误")
	}
}
