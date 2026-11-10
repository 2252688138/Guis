//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

// 把文件拖出到其它应用（OLE 拖放 / DoDragDrop）。
//
// 窗口收到的 WM_DROPFILES 是「别人拖进来」，这里做的是反向：从本窗口拖出去。
// 需要实现两个 COM 接口：
//
//	IDataObject  —— 提供 CF_HDROP（文件列表）
//	IDropSource  —— 决定何时开始/取消/完成拖放
//
// 都是最小实现（只支持 CF_HDROP），够覆盖「从文件列表拖到资源管理器/微信」。
// vtbl 与回调只在进程内创建一次；每次拖放新建一组对象（vtable 相同、状态不同）。

var (
	procDoDragDrop    = ole32L.NewProc("DoDragDrop")
	procOleInitialize = ole32L.NewProc("OleInitialize")
)

// IDataObject vtable 槽位。
const (
	idoQueryInterface = 0
	idoGetData        = 3
)

// IDropSource vtable 槽位。
const (
	idsQueryContinueDrag = 3
	idsGiveFeedback      = 4
)

const (
	sOK                  = 0
	eNotImpl             = 0x80004001
	eNoInterface         = 0x80004002
	dragdropSDrop        = 0x00040100
	dragdropSCancel      = 0x00040101
	dragdropSUseDefault  = 0x00040102
	tymedHGlobal         = 1
	cfHDROPFormat        = 15
	dropeffectCopy       = 0x0001
	dropeffectMove       = 0x0002
	dropeffectLink       = 0x0004
	gmemFixed            = 0x0000
)

// formatEtc FORMATETC 结构（amd64 下：cfFormat@0, dwAspect@4, lindex@8, tymed@12, ptd@16，共 24 字节）。
type formatEtc struct {
	CfFormat uint16
	_        uint16
	DwAspect uint32
	Lindex   int32
	Tymed    uint32
	Ptd      uintptr
}

// stgMedium STGMEDIUM 结构。
type stgMedium struct {
	Tymed          uint32
	_              uint32
	HGlobal        uintptr
	PUnkForRelease uintptr
}

var (
	dragOnce     sync.Once
	vtableData   uintptr
	vtableSource uintptr
	// dragPayload 当前拖放携带的 CF_HDROP 内存块（DoDragDrop 阻塞期间有效）。
	dragPayloadMu sync.Mutex
	dragPayload   []byte
)

// buildHDrop 构造 DROPFILES + 路径列表（宽字符）。
func buildHDrop(paths []string) []byte {
	var body []uint16
	for _, p := range paths {
		if p == "" {
			continue
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		body = append(body, syscall.StringToUTF16(p)...)
	}
	if len(body) == 0 {
		return nil
	}
	body = append(body, 0) // 列表结束

	const hdr = 20
	buf := make([]byte, hdr+len(body)*2)
	// DROPFILES{ pFiles=20, pt={0,0}, fNC=0, fWide=1 }
	*(*uint32)(unsafe.Pointer(&buf[0])) = hdr
	*(*int32)(unsafe.Pointer(&buf[16])) = 1
	for i, c := range body {
		*(*uint16)(unsafe.Pointer(&buf[hdr+i*2])) = c
	}
	return buf
}

// initDragCallbacks 创建 COM 回调与两张虚表（进程内只做一次）。
func initDragCallbacks() {
	dragOnce.Do(func() {
		cbQI := purego.NewCallback(dragQueryInterface)
		cbAddRef := purego.NewCallback(comStubAddRef)
		cbRelease := purego.NewCallback(comStubRelease)
		cbStub := purego.NewCallback(comStubNotImpl)
		cbGetData := purego.NewCallback(dragGetData)
		cbQueryContinue := purego.NewCallback(dropQueryContinue)
		cbGiveFeedback := purego.NewCallback(dropGiveFeedback)

		vtableData = allocUintptrs([]uintptr{
			cbQI, cbAddRef, cbRelease, cbGetData, cbStub, cbStub, cbStub,
			cbStub, cbStub, cbStub, cbStub, cbStub,
		})
		vtableSource = allocUintptrs([]uintptr{
			cbQI, cbAddRef, cbRelease, cbQueryContinue, cbGiveFeedback,
		})
	})
}

// allocUintptrs 把一组函数指针写进一块固定（非 Go 堆）内存，作为 COM 虚表。
func allocUintptrs(fns []uintptr) uintptr {
	h := GlobalAlloc(gmemFixed, uintptr(len(fns))*unsafe.Sizeof(uintptr(0)))
	if h == 0 {
		return 0
	}
	p := GlobalLock(h)
	if p == 0 {
		return 0
	}
	for i, fn := range fns {
		*(*uintptr)(unsafe.Pointer(p + uintptr(i)*unsafe.Sizeof(uintptr(0)))) = fn
	}
	GlobalUnlock(h)
	return p
}

// comStubNotImpl 返回 E_NOTIMPL 的占位方法（多传的参数会被直接忽略）。
func comStubNotImpl(this, a, b uintptr) uintptr { return eNotImpl }

// comStubAddRef 恒返回 2（引用计数对单次拖放没有实际意义）。
func comStubAddRef(this uintptr) uintptr { return 2 }

// comStubRelease 引用计数 1 → 不释放（对象由 DoDragDrop 结束后的 Go 侧回收）。
func comStubRelease(this uintptr) uintptr { return 1 }

// dragQueryInterface IDataObject / IDropSource 的 QueryInterface。
//
// 对象布局（各 32 字节，GlobalAlloc 分配）：
//
//	0  vtable
//	8  refcount
//	16 peer（IDataObject ↔ IDropSource 互相指向）
//	24 reserved
func dragQueryInterface(this, riid, ppv uintptr) uintptr {
	if ppv == 0 {
		return eNoInterface
	}
	// IUnknown / IDataObject 的 IID 都以 00000000-0000-0000-C000-000000000046 结尾，
	// 这里只比较 Data1 区分（DataObject: 0000010E，DropSource: 0000011A，Unknown: 0）。
	type g32 struct {
		Data1 uint32
		Data2 uint16
		Data3 uint16
		Rest  [8]byte
	}
	g := (*g32)(unsafe.Pointer(riid))
	switch g.Data1 {
	case 0, 0x0000010E: // IUnknown / IDataObject
		*(*uintptr)(unsafe.Pointer(ppv)) = this
		return sOK
	case 0x0000011A: // IDropSource
		peer := *(*uintptr)(unsafe.Pointer(this + 16))
		*(*uintptr)(unsafe.Pointer(ppv)) = peer
		return sOK
	}
	*(*uintptr)(unsafe.Pointer(ppv)) = 0
	return eNoInterface
}

// dragGetData 把 CF_HDROP 交给目标应用。
func dragGetData(this, pformatetc, pmedium uintptr) uintptr {
	if pformatetc == 0 || pmedium == 0 {
		return 0x80004003 // E_POINTER
	}
	fe := (*formatEtc)(unsafe.Pointer(pformatetc))
	if fe.CfFormat != cfHDROPFormat {
		return 0x80040064 // DV_E_FORMATETC
	}
	dragPayloadMu.Lock()
	payload := dragPayload
	dragPayloadMu.Unlock()
	if len(payload) == 0 {
		return 0x80040069 // DV_E_TYMED
	}
	h := GlobalAlloc(gmemFixed, uintptr(len(payload)))
	if h == 0 {
		return 0x8007000E // E_OUTOFMEMORY
	}
	dst := GlobalLock(h)
	if dst == 0 {
		GlobalFree(h)
		return 0x8007000E
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(dst)), len(payload)), payload)
	GlobalUnlock(h)

	sm := (*stgMedium)(unsafe.Pointer(pmedium))
	sm.Tymed = tymedHGlobal
	sm.HGlobal = h
	sm.PUnkForRelease = 0
	return sOK
}

// dropQueryContinue 根据鼠标左键是否松开决定继续 / 放下 / 取消。
func dropQueryContinue(this, escPressed, keyState uintptr) uintptr {
	if escPressed != 0 {
		return dragdropSCancel
	}
	const mkLButton = 0x0001
	if keyState&mkLButton == 0 {
		return dragdropSDrop
	}
	return sOK
}

// dropGiveFeedback 使用系统默认光标。
func dropGiveFeedback(this, effect uintptr) uintptr { return dragdropSUseDefault }

// DragFiles 开始一次 OLE 拖放。x, y 为屏幕坐标（-1 表示用当前鼠标位置）。
//
// 必须在处理鼠标按下的回调里调用：DoDragDrop 会进入自己的消息循环，直到用户
// 放下文件或按 ESC 才返回。
func DragFiles(hwnd uintptr, paths []string, x, y int32) error {
	if len(paths) == 0 {
		return syscall.EINVAL
	}
	payload := buildHDrop(paths)
	if len(payload) == 0 {
		return syscall.EINVAL
	}
	initDragCallbacks()
	if vtableData == 0 || vtableSource == 0 {
		return syscall.EWINDOWS
	}
	if procOleInitialize.Find() == nil {
		procOleInitialize.Call(0) // 已初始化时返回 S_FALSE，可安全重复调用
	}

	// 两块对象：IDataObject 与 IDropSource 互相持有对端指针
	objA := GlobalAlloc(gmemFixed, 32)
	objB := GlobalAlloc(gmemFixed, 32)
	if objA == 0 || objB == 0 {
		if objA != 0 {
			GlobalFree(objA)
		}
		if objB != 0 {
			GlobalFree(objB)
		}
		return syscall.EWINDOWS
	}
	defer GlobalFree(objA)
	defer GlobalFree(objB)
	pa, pb := GlobalLock(objA), GlobalLock(objB)
	if pa == 0 || pb == 0 {
		return syscall.EWINDOWS
	}
	*(*uintptr)(unsafe.Pointer(pa + 0)) = vtableData
	*(*uintptr)(unsafe.Pointer(pa + 8)) = 1
	*(*uintptr)(unsafe.Pointer(pa + 16)) = pb
	*(*uintptr)(unsafe.Pointer(pb + 0)) = vtableSource
	*(*uintptr)(unsafe.Pointer(pb + 8)) = 1
	*(*uintptr)(unsafe.Pointer(pb + 16)) = pa
	GlobalUnlock(objB)

	dragPayloadMu.Lock()
	dragPayload = payload
	dragPayloadMu.Unlock()
	defer func() {
		dragPayloadMu.Lock()
		dragPayload = nil
		dragPayloadMu.Unlock()
	}()

	// 有指针坐标时先把鼠标移过去（从别处触发的拖放需要对齐到按下点）
	if x >= 0 && y >= 0 {
		procSetCursorPos.Call(uintptr(x), uintptr(y))
	}
	var effect uint32 = dropeffectCopy | dropeffectMove
	hr, _, _ := procDoDragDrop.Call(pa, pb, dropeffectCopy|dropeffectMove|dropeffectLink, uintptr(unsafe.Pointer(&effect)))
	GlobalUnlock(objA)
	if int32(hr) < 0 {
		return syscall.EWINDOWS
	}
	return nil
}

var procSetCursorPos = user32L.NewProc("SetCursorPos")
