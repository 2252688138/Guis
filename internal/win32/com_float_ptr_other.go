//go:build windows && (!amd64 || (cgo && guis_cgo))

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

// ComCallFloatPtr 非 amd64（或 cgo 变体）下不实现：这些平台上的音量设置会
// 返回 E_NOTIMPL，业务侧按「不支持」降级。
func ComCallFloatPtr(obj uintptr, slot int, f float32, p uintptr) uint32 {
	return 0x80004005
}
