// guis — 字体图集预算回收回归
//
// 图集是「创建后按需扩容」的，旧实现只在新建字体时看一次 atlasTotal()，
// 字体建好后图集再涨没人管（32 个实例各涨到 1MB 就是 32MB）。
// 现在 Layout 每帧开头都会 enforceAtlasBudget，这里验证两条路径：
//  1) 超预算 → 回收最久未使用的；
//  2) 本帧 / 上一帧在用的字体不动，避免「淘汰→重栅格化→再淘汰」抖动。

package ui

import (
	"testing"

	"guis/internal/text"
)

// growAtlasFor 让字体光栅化足够多的汉字，把图集撑大。
func growAtlasFor(f *text.Font, n int) {
	for r := rune(0x4E00); r < rune(0x4E00+n); r++ {
		f.GlyphFor(r)
	}
}

// TestFontCacheAtlasBudgetEvicts 超预算时回收最久未使用的字体。
func TestFontCacheAtlasBudgetEvicts(t *testing.T) {
	old := fontAtlasBudget
	defer func() { fontAtlasBudget = old }()

	fc := newFontCache("Microsoft YaHei UI", 1)
	defer fc.close()

	a := fc.get(13, 400)
	growAtlasFor(a, 1200)
	b := fc.get(19, 400)
	growAtlasFor(b, 1200)

	if a.AtlasBytes() <= 0 || b.AtlasBytes() <= 0 {
		t.Fatalf("图集未分配：a=%d b=%d", a.AtlasBytes(), b.AtlasBytes())
	}

	// 推进两帧：两个字体都变成「上一帧之前用过的」，都可回收。
	fc.frame += 2
	// 预算只放得下其中较大的一张：必须恰好淘汰一个（较久未用的 13px）。
	fontAtlasBudget = a.AtlasBytes()
	if b.AtlasBytes() > fontAtlasBudget {
		fontAtlasBudget = b.AtlasBytes()
	}
	fc.enforceAtlasBudget()

	if len(fc.fonts) != 1 {
		t.Fatalf("超预算后应只保留 1 个字体，实际 %d", len(fc.fonts))
	}
	// 最近使用的 19px 应被保留
	if _, ok := fc.fonts[fontKey{family: "Microsoft YaHei UI", size: 19, weight: 400}]; !ok {
		t.Fatalf("应保留最近使用的 19px 字体，实际残留：%v", fc.fonts)
	}
	if got := fc.atlasBytes(); got > fontAtlasBudget {
		t.Fatalf("回收后图集 %d 字节仍超预算 %d", got, fontAtlasBudget)
	}
}

// TestFontCacheAtlasBudgetKeepsActive 本帧在用的字体不因超预算被淘汰。
func TestFontCacheAtlasBudgetKeepsActive(t *testing.T) {
	old := fontAtlasBudget
	defer func() { fontAtlasBudget = old }()

	fc := newFontCache("Microsoft YaHei UI", 1)
	defer fc.close()

	f := fc.get(14, 400)
	growAtlasFor(f, 1200)

	fontAtlasBudget = 1
	fc.frame++
	key := fontKey{family: "Microsoft YaHei UI", size: 14, weight: 400}
	fc.used[key] = fc.frame // 本帧在用

	fc.enforceAtlasBudget()
	if len(fc.fonts) != 1 {
		t.Fatalf("本帧在用的字体不应被淘汰，实际剩 %d", len(fc.fonts))
	}
}
