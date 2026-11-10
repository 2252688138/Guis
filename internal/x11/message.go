// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// 纯 Go 的 X11 消息构造（无 build tag）：不依赖 Xlib，因此可以在任何系统上单测。
package x11

// ClientMessageEventType XClientMessageEvent 的事件类型号。
const ClientMessageEventType = 33

// BuildClientMessage 构造一条 XClientMessageEvent（196 字节缓冲，按 Xlib 64 位偏移）。
//
// 布局：type(0)=33 / serial(8) / send_event(16)=1 / display(24)=0 /
// window(32) / message_type(40) / format(48)=32 / data.l[5](56..)。
//
// XEmbed 托盘 docking 与 XDND 拖入回执都靠它，偏移写错会表现为
// 「消息发出去了但对端没反应」，所以单独抽出来做单测。
func BuildClientMessage(window, messageType uintptr, data [5]int64) [196]byte {
	var ev [196]byte
	setLong(ev[:], 0, ClientMessageEventType)
	setLong(ev[:], 16, 1)
	setLong(ev[:], 32, uint64(window))
	setLong(ev[:], 40, uint64(messageType))
	setLong(ev[:], 48, 32)
	for i := 0; i < 5; i++ {
		setLong(ev[:], 56+8*i, uint64(data[i]))
	}
	return ev
}

// ReadClientData 从一条已构造的 ClientMessage 里读第 i 个数据字（单测 / 调试用）。
func ReadClientData(ev []byte, i int) int64 {
	return int64(at(ev, 56+8*i))
}
