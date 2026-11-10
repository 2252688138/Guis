package text

import "testing"

// glyphCoverage 统计字形位图里实心像素（alpha>=200）与半透明过渡像素的数量。
func glyphCoverage(f *Font, g *Glyph) (solid, soft int) {
	if f == nil || g == nil || !g.Valid {
		return 0, 0
	}
	dim := f.AtlasSize
	if dim <= 0 {
		return 0, 0
	}
	u0 := int(g.U0*float32(dim) + 0.5)
	u1 := int(g.U1*float32(dim) + 0.5)
	v0 := int(g.V0*float32(dim) + 0.5)
	v1 := int(g.V1*float32(dim) + 0.5)
	for v := v0; v < v1; v++ {
		// 图集数组第 0 行对应纹理 v=0：字形写入时已把 y 翻转过，这里直接用 v 作行号
		row := v
		if row < 0 || row >= dim {
			continue
		}
		for u := u0; u < u1; u++ {
			if u < 0 || u >= dim {
				continue
			}
			a := f.Atlas[row*dim+u]
			switch {
			case a >= 200:
				solid++
			case a > 0:
				soft++
			}
		}
	}
	return solid, soft
}

// TestSharpHintingRaisesSolidCoverage 清晰档必须让笔画更「实」：
// 实心像素增加、半透明过渡像素减少，且水平 advance 对齐到整数像素。
func TestSharpHintingRaisesSolidCoverage(t *testing.T) {
	defer SetSharp(SharpStandard)

	f := NewFont("Microsoft YaHei", 14, 400, 1)
	defer f.Close()

	SetSharp(SharpStandard)
	// 优先用汉字（小字号中文最吃 hinting），环境缺字形时退到拉丁字母
	probe := '永'
	if g := f.GlyphFor(probe); g == nil || !g.Valid {
		probe = 'H'
	}
	base := f.GlyphFor(probe)
	solidBase, softBase := glyphCoverage(f, base)
	if solidBase == 0 {
		t.Logf("dim=%d atlas=%d glyph=%+v", f.AtlasSize, len(f.Atlas), base)
		t.Skipf("当前环境无法光栅化 %q，跳过字形覆盖率对比", probe)
	}

	SetSharp(SharpClear)
	f.RebuildSharp()
	if !f.TakeReset() {
		t.Fatal("RebuildSharp 后应标记图集需重建")
	}

	sharp := f.GlyphFor(probe)
	solidSharp, softSharp := glyphCoverage(f, sharp)
	t.Logf("标准 solid=%d soft=%d；清晰 solid=%d soft=%d", solidBase, softBase, solidSharp, softSharp)

	if solidSharp <= solidBase {
		t.Fatalf("清晰档实心像素应增加：标准 %d → 清晰 %d", solidBase, solidSharp)
	}
	if softSharp >= softBase {
		t.Fatalf("清晰档半透明过渡像素应减少：标准 %d → 清晰 %d", softBase, softSharp)
	}
}

// TestSharpAdvanceAligned 清晰档的拉丁字形 advance 应落在整像素上（hinting 生效）。
func TestSharpAdvanceAligned(t *testing.T) {
	defer SetSharp(SharpStandard)

	f := NewFont("Microsoft YaHei", 14, 400, 1)
	defer f.Close()

	SetSharp(SharpClear)
	f.RebuildSharp()
	g := f.GlyphFor('a')
	if g == nil || !g.Valid {
		t.Skip("当前环境缺字体，跳过 advance 校验")
	}
	if adv := g.Advance; adv != float32(int(adv)) {
		t.Fatalf("清晰档 advance 应对齐整数像素，得到 %v", adv)
	}
}

// TestSharpClamp 档位夹取。
func TestSharpClamp(t *testing.T) {
	defer SetSharp(SharpStandard)
	SetSharp(-5)
	if Sharp() != SharpStandard {
		t.Fatalf("负档位应夹到 SharpStandard，得到 %d", Sharp())
	}
	SetSharp(99)
	if Sharp() != SharpMax {
		t.Fatalf("超上限应夹到 SharpMax，得到 %d", Sharp())
	}
	SetSharp(SharpClear)
	if Sharp() != SharpClear {
		t.Fatalf("应保持 SharpClear，得到 %d", Sharp())
	}
}
