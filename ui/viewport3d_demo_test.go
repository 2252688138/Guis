// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 程序化 3D 视口源：验证「ViewportSource → 像素 → 纹理」链路真的能出画面。

import "testing"

func optsCount(pix []byte) int {
	n := 0
	for i := 3; i < len(pix); i += 4 {
		if pix[i] != 0 {
			n++
		}
	}
	return n
}

func TestDemoViewportSourceRender(t *testing.T) {
	src := NewDemoViewportSource()
	pix, tw, th := src.RenderViewport(64, 64, 0.016)
	if tw != 64 || th != 64 {
		t.Fatalf("尺寸 %dx%d", tw, th)
	}
	if len(pix) != 64*64*4 {
		t.Fatalf("像素长度 %d", len(pix))
	}
	if n := optsCount(pix); n < 64*64 {
		t.Fatalf("有效像素太少: %d", n)
	}
	// 方块区域应当接近白色
	i := (32*64 + 32) * 4
	if pix[i] < 200 || pix[i+1] < 200 {
		t.Fatalf("中心不像方块: %d,%d,%d,%d", pix[i], pix[i+1], pix[i+2], pix[i+3])
	}
}

func TestDemoViewportSourceAnimates(t *testing.T) {
	src := NewDemoViewportSource()
	a, _, _ := src.RenderViewport(48, 48, 0.0)
	b, _, _ := src.RenderViewport(48, 48, 0.25)
	if len(a) != len(b) {
		t.Fatal("两帧长度不一致")
	}
	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("两帧完全相同，动画没生效")
	}
	if src.Time() <= 0 {
		t.Fatalf("累计时间未推进: %v", src.Time())
	}
}

// 通过 ui.Viewport3D 组节点：Kind / Viewport 正确。
func TestDemoViewportNode(t *testing.T) {
	src := NewDemoViewportSource()
	n := Viewport3D(&Viewport3DOpts{Src: src, Key: "demo", Resolution: [2]int{64, 64}})
	if n == nil || n.Kind != KViewport3D || n.Viewport == nil {
		t.Fatalf("视口节点不对: %+v", n)
	}
}
