// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package gpu

import (
	"testing"

	"guis/internal/geom"
)

// TestReleaseVertsShrinks 空闲回收必须把顶点暂存区收回到初始容量。
// 一屏顶点上限为 6 万个（形状 22 float/顶点 ≈ 5MB、文字 9 float/顶点 ≈ 2MB），
// 画过十万行表格后这些内存会一直常驻，窗口最小化时应当归还。
func TestReleaseVertsShrinks(t *testing.T) {
	r := NewRenderer()
	r.shapeData = make([]float32, 0, maxVerts*floatsShapeVert)
	r.textData = make([]float32, 0, maxVerts*floatsTextVert)
	r.imgData = make([]float32, 0, 4096*floatsImgVert*4)
	r.clipSaved = make([]geom.Rect, 0, 64)
	r.shapeN, r.textN, r.imgN = 600, 600, 6

	r.ReleaseVerts()

	if cap(r.shapeData) != maxVertsShapeInitial*floatsShapeVert {
		t.Fatalf("shapeData 容量 = %d，期望收缩到 %d", cap(r.shapeData),
			maxVertsShapeInitial*floatsShapeVert)
	}
	if cap(r.textData) != maxVertsTextInitial*floatsTextVert {
		t.Fatalf("textData 容量 = %d，期望收缩到 %d", cap(r.textData),
			maxVertsTextInitial*floatsTextVert)
	}
	if cap(r.imgData) > 4096*floatsImgVert {
		t.Fatalf("imgData 容量 = %d，未收缩", cap(r.imgData))
	}
	if len(r.shapeData) != 0 || len(r.textData) != 0 || len(r.imgData) != 0 {
		t.Fatal("收缩后长度应为 0")
	}
	if r.shapeN != 0 || r.textN != 0 || r.imgN != 0 {
		t.Fatal("收缩后顶点计数应清零")
	}
}

// TestGrowFloatsBeyondCap 顶点切片扩展到超出容量时必须扩容而不是 panic。
//
// 回归用例：长文本页单个文字批次超过初始容量（6144 顶点 × 9 float = 55296）时，
// 直接写 buf[:n+54] 会 panic：slice bounds out of range [:55350] with capacity 55296。
func TestGrowFloatsBeyondCap(t *testing.T) {
	buf := make([]float32, 0, maxVertsTextInitial*floatsTextVert)
	full := cap(buf)
	buf = buf[:full]
	for i := range buf {
		buf[i] = float32(i)
	}

	got := growFloats(buf, full+6*floatsTextVert)
	if len(got) != full+6*floatsTextVert {
		t.Fatalf("长度 = %d，期望 %d", len(got), full+6*floatsTextVert)
	}
	if cap(got) < len(got) {
		t.Fatalf("容量 = %d，小于长度 %d", cap(got), len(got))
	}
	for i := 0; i < full; i++ {
		if got[i] != float32(i) {
			t.Fatalf("扩容后旧数据丢失：index %d = %v", i, got[i])
		}
	}

	// 容量够时只改长度，不重新分配
	small := make([]float32, 0, 64)
	small = growFloats(small, 6)
	if cap(small) != 64 || len(small) != 6 {
		t.Fatalf("容量内扩展不应重新分配：len=%d cap=%d", len(small), cap(small))
	}
}

// TestReleaseVertsKeepsSmallBuffers 已在小容量时只清长度，不重新分配（避免空闲期反复 GC）。
func TestReleaseVertsKeepsSmallBuffers(t *testing.T) {
	r := NewRenderer()
	r.shapeData = make([]float32, 0, 128)
	before := &r.shapeData[:1][0]
	r.shapeData = r.shapeData[:1]
	r.ReleaseVerts()
	if len(r.shapeData) != 0 {
		t.Fatal("应清空长度")
	}
	if &r.shapeData[:1][0] != before {
		t.Fatal("小容量缓冲不应重新分配")
	}
}
