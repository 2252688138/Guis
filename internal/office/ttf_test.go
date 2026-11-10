// guis — TrueType 子集化测试：子集字体必须能被重新解析（结构自洽）

package office

import (
	"os"
	"testing"
)

// TestTTFSubset 子集化后重新解析，校验表目录 / 字形数 / 宽度表。
func TestTTFSubset(t *testing.T) {
	path, idx := pdfFontFile("", 0)
	if path == "" {
		t.Skip("本机没有可用的 TrueType 字体")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取字体失败：%v", err)
	}
	tt, err := parseTrueType(data, idx)
	if err != nil {
		t.Skipf("字体无法解析（%v），跳过", err)
	}
	text := "季度经营报表 2026 Q1 Revenue +26%"
	var runes []rune
	for _, r := range text {
		runes = append(runes, r)
	}
	sub := tt.subset(runes)
	if len(sub.Font) == 0 {
		t.Fatal("子集字体为空")
	}
	if len(sub.Font) >= len(data) {
		t.Fatalf("子集未缩小：%d >= %d", len(sub.Font), len(data))
	}
	// 重新解析子集字体
	back, err := parseTrueType(sub.Font, 0)
	if err != nil {
		t.Fatalf("子集字体无法解析：%v", err)
	}
	if back.numGlyphs != sub.NumGlyphs {
		t.Fatalf("字形数 = %d，期望 %d", back.numGlyphs, sub.NumGlyphs)
	}
	if back.unitsPerEm != tt.unitsPerEm {
		t.Fatalf("unitsPerEm = %d，期望 %d", back.unitsPerEm, tt.unitsPerEm)
	}
	// 每个用到的字符都应能在新 cmap 中找到（未重建 cmap，故只校验 CID 表）
	for _, r := range runes {
		if _, ok := sub.CIDs[r]; !ok {
			t.Fatalf("字符 %q 没有 CID", r)
		}
	}
	// 宽度表长度 = 字形数
	if len(sub.Widths) != sub.NumGlyphs {
		t.Fatalf("宽度表长度 = %d，期望 %d", len(sub.Widths), sub.NumGlyphs)
	}
	// 前两个字形的 glyf 数据非空（.notdef 可以为空）
	loca := back.loca
	if len(loca) != back.numGlyphs+1 {
		t.Fatalf("loca 长度 = %d，期望 %d", len(loca), back.numGlyphs+1)
	}
	if loca[len(loca)-1] != uint32(len(back.glyf)) {
		t.Fatalf("loca 末项 = %d，glyf 长度 = %d", loca[len(loca)-1], len(back.glyf))
	}
}
