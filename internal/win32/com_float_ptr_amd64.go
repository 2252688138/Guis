//go:build windows && amd64 && !(cgo && guis_cgo)

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"sync"
	"syscall"
	"unsafe"
)

// fn(self, float, void*) 形式的 COM 调用（例如 IAudioEndpointVolume::SetMasterVolumeLevelScalar）。
//
// Win64 约定下 float 走 XMM0，指针走 R8：SyscallN 只能传通用寄存器，因此这里
// 运行时生成一段 trampoline：
//
//	mov r11, rcx        ; fn
//	mov rcx, rdx        ; self
//	movd xmm0, r8d      ; float → XMM0（float 不占整型寄存器）
//	mov rdx, r9         ; 第 3 个参数（指针）→ RDX，而不是 R8
//	sub rsp, 0x28       ; 影子空间 + 16 字节对齐
//	call r11
//	add rsp, 0x28
//	ret
//
// 关键点：Win64 的浮点参数不进通用寄存器，因此 self(RCX) 之后的第一个指针参数
// 落在 RDX 上 —— 写进 R8 会让被调方把 self 当成指针（实测返回 E_INVALIDARG 或
// 直接访问违例）。

var (
	fpTrampOnce sync.Once
	fpTramp     uintptr
	fpTrampErr  error
)

func ensureFPTramp() error {
	fpTrampOnce.Do(func() {
		k32 := syscall.NewLazyDLL("kernel32.dll")
		alloc := k32.NewProc("VirtualAlloc")
		const memCommitReserve = 0x3000
		const pageExecuteReadWrite = 0x40

		code := []byte{
			0x49, 0x89, 0xCB, // mov r11, rcx
			0x48, 0x89, 0xD1, // mov rcx, rdx
			0x66, 0x41, 0x0F, 0x6E, 0xC0, // movd xmm0, r8d
			0x4C, 0x89, 0xCA, // mov rdx, r9
			0x48, 0x83, 0xEC, 0x28, // sub rsp, 0x28
			0x41, 0xFF, 0xD3, // call r11
			0x48, 0x83, 0xC4, 0x28, // add rsp, 0x28
			0xC3, // ret
		}
		p, _, _ := alloc.Call(0, 64, memCommitReserve, pageExecuteReadWrite)
		if p == 0 {
			fpTrampErr = syscall.EINVAL
			return
		}
		copy(unsafe.Slice((*byte)(unsafe.Pointer(p)), len(code)), code)
		fpTramp = p
	})
	return fpTrampErr
}

// ComCallFloatPtr 调用 fn(self, float, void*)，返回 HRESULT。
func ComCallFloatPtr(obj uintptr, slot int, f float32, p uintptr) uint32 {
	if ensureFPTramp() != nil {
		return 0x80004005
	}
	fn := ComSlot(obj, slot)
	if fn == 0 {
		return 0x80004003
	}
	bits := *(*uint32)(unsafe.Pointer(&f))
	r, _, _ := syscall.SyscallN(fpTramp, fn, obj, uintptr(bits), p)
	return uint32(r)
}
