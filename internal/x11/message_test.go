// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package x11

import "testing"

// TestBuildClientMessage XClientMessageEvent 的字段偏移（XEmbed / XDND 都依赖它）。
func TestBuildClientMessage(t *testing.T) {
	ev := BuildClientMessage(0x1234, 0x5678, [5]int64{1, 2, 3, 4, 5})

	if got := ev[0]; got != ClientMessageEventType {
		t.Fatalf("type = %d, want %d", got, ClientMessageEventType)
	}
	// send_event 必须在偏移 16 处为 1（对端据此判定是人工发送）
	if got := at(ev[:], 16); got != 1 {
		t.Fatalf("send_event = %d, want 1", got)
	}
	if got := at(ev[:], 32); got != 0x1234 {
		t.Fatalf("window = %#x, want 0x1234", got)
	}
	if got := at(ev[:], 40); got != 0x5678 {
		t.Fatalf("message_type = %#x, want 0x5678", got)
	}
	if got := at(ev[:], 48); got != 32 {
		t.Fatalf("format = %d, want 32", got)
	}
	for i := 0; i < 5; i++ {
		if got := ReadClientData(ev[:], i); got != int64(i+1) {
			t.Fatalf("data[%d] = %d, want %d", i, got, i+1)
		}
	}
}

// TestReadClientDataBounds 越界读不能崩（事件类型未知时字段没有意义）。
func TestReadClientDataBounds(t *testing.T) {
	buf := make([]byte, 8)
	for _, i := range []int{-1, 0, 1, 100} {
		_ = ReadClientData(buf, i) // 只要不 panic 即通过
	}
}
