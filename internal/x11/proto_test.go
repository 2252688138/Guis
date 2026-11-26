//go:build linux || freebsd

package x11

import "testing"

// TestAllocColor 真彩色下像素值就是 0xRRGGBB。
func TestAllocColor(t *testing.T) {
	cases := []struct {
		r, g, b uint8
		want    uintptr
	}{
		{0, 0, 0, 0x000000},
		{255, 255, 255, 0xFFFFFF},
		{255, 0, 0, 0xFF0000},
		{0, 128, 255, 0x0080FF},
		{18, 52, 86, 0x123456},
	}
	for _, c := range cases {
		if got := AllocColor(c.r, c.g, c.b); got != c.want {
			t.Fatalf("AllocColor(%d,%d,%d) = %#x，期望 %#x", c.r, c.g, c.b, got, c.want)
		}
	}
}

// TestAtSetLongRoundTrip 机器字读写往返一致，越界与负偏移安全返回。
func TestAtSetLongRoundTrip(t *testing.T) {
	b := make([]byte, 16)
	setLong(b, 4, 0x1122334455667788)
	if got := at(b, 4); got != 0x1122334455667788 {
		t.Fatalf("读写不一致：%#x", got)
	}

	// 越界：off+8 超出长度
	if got := at(b, 12); got != 0 {
		t.Fatalf("越界读应返回 0，实际 %#x", got)
	}
	// 负偏移
	if got := at(b, -1); got != 0 {
		t.Fatalf("负偏移读应返回 0，实际 %#x", got)
	}
	// 越界写不能 panic，也不能改动缓冲区
	setLong(b, 100, 0xFF)
	if got := at(b, 4); got != 0x1122334455667788 {
		t.Fatalf("越界写不应改动缓冲，实际 %#x", got)
	}
}

// TestDisplayNilSafe 库没加载 / 未连接时，Display 方法必须安静返回，
// 否则 Linux 上没有 X 服务器的场景（CI / 服务端渲染）会直接 panic。
func TestDisplayNilSafe(t *testing.T) {
	var d *Display
	d.RaiseWindow(1)
	d.SetSelectionOwner(0, 0)
	d.SelectInput(1, 0)
}
