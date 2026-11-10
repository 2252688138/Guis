//go:build windows

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

// 任务栏集成（ITaskbarList3）：进度条、未读角标。
//
// 下载 / 导出这类长任务把进度画到任务栏图标上，用户切走窗口也能看到进展；
// 聊天 / 邮件类应用用角标显示未读数。
//
// COM 约束：ITaskbarList3 必须在创建它的 STA 线程上调用，因此这里用
// 「首个调用线程初始化一次」的策略 —— 框架的所有窗口操作都在 UI 线程，
// 天然满足。

var (
	ole32L = syscall.NewLazyDLL("ole32.dll")

	procCoCreateInstance = ole32L.NewProc("CoCreateInstance")
	procCreateIconIndirect = user32L.NewProc("CreateIconIndirect")
	procDestroyIcon        = user32L.NewProc("DestroyIcon")
)

// clsidTaskbarList / iidITaskbarList3。
var (
	clsidTaskbarList = guid{0x56FDF344, 0xFD6D, 0x11D0, [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3  = guid{0xEA1AFB91, 0x9E28, 0x4B86, [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF}}
)

// ITaskbarList3 的虚表槽位（IUnknown 占 0~2）。
const (
	tblHrInit               = 3
	tblSetProgressValue     = 9
	tblSetProgressState     = 10
	tblSetOverlayIcon       = 18
)

// 任务栏进度状态（TBPF_*）。
const (
	// TBPFNoProgress 不显示进度。
	TBPFNoProgress = 0
	// TBPFIndeterminate 不确定进度（跑马灯）。
	TBPFIndeterminate = 1
	// TBPFNormal 正常进度。
	TBPFNormal = 2
	// TBPFError 红色错误进度。
	TBPFError = 4
	// TBPFPaused 黄色暂停进度。
	TBPFPaused = 8
)

var (
	taskbarMu   sync.Mutex
	taskbarObj  uintptr
	taskbarTried bool
)

// taskbar 懒创建 ITaskbarList3（失败后不再重试，避免每帧 CoCreateInstance）。
func taskbar() uintptr {
	taskbarMu.Lock()
	defer taskbarMu.Unlock()
	if taskbarObj != 0 || taskbarTried {
		return taskbarObj
	}
	taskbarTried = true
	if procCoCreateInstance.Find() != nil {
		return 0
	}
	// 保证当前线程有 COM 单元（STA）。已初始化时返回 S_FALSE，可安全重复调用。
	if coInitializeExFn != nil {
		coInitializeExFn(0, 2) // COINIT_APARTMENTTHREADED
	}
	var obj uintptr
	const clsctxInprocServer = 0x1
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidTaskbarList)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidTaskbarList3)),
		uintptr(unsafe.Pointer(&obj)),
	)
	if int32(hr) < 0 || obj == 0 {
		return 0
	}
	if hr := comCall(obj, tblHrInit); comFail(hr) {
		comRelease(obj)
		return 0
	}
	taskbarObj = obj
	return taskbarObj
}

// SetTaskbarProgress 设置任务栏进度：total<=0 表示不确定进度，done>=total 表示完成。
func SetTaskbarProgress(hwnd uintptr, done, total int64) {
	obj := taskbar()
	if obj == 0 || hwnd == 0 {
		return
	}
	if total <= 0 {
		comCall(obj, tblSetProgressState, hwnd, TBPFIndeterminate)
		return
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	comCall(obj, tblSetProgressState, hwnd, TBPFNormal)
	comCall(obj, tblSetProgressValue, hwnd, uintptr(done), uintptr(total))
}

// ClearTaskbarProgress 清除任务栏进度。
func ClearTaskbarProgress(hwnd uintptr) {
	obj := taskbar()
	if obj == 0 || hwnd == 0 {
		return
	}
	comCall(obj, tblSetProgressState, hwnd, TBPFNoProgress)
}

// SetTaskbarBadge 在任务栏图标右下角显示未读数角标（n<=0 清除）。
//
// 角标是 16×16 的 HICON：红底白字，超过 99 显示 "99+"。图标句柄由系统在
// SetOverlayIcon 后复制，因此这里创建完即可销毁。
func SetTaskbarBadge(hwnd uintptr, n int, desc string) {
	obj := taskbar()
	if obj == 0 || hwnd == 0 {
		return
	}
	if n <= 0 {
		comCall(obj, tblSetOverlayIcon, hwnd, 0, 0)
		return
	}
	icon := makeBadgeIcon(n)
	if icon == 0 {
		return
	}
	defer procDestroyIcon.Call(icon)
	d, _ := syscall.UTF16PtrFromString(desc)
	comCall(obj, tblSetOverlayIcon, hwnd, icon, uintptr(unsafe.Pointer(d)))
}

// ---- 角标位图 ----

// 3×5 点阵数字（每行 3 位）。
var digitGlyphs = [10][5]uint8{
	{0b111, 0b101, 0b101, 0b101, 0b111}, // 0
	{0b010, 0b110, 0b010, 0b010, 0b111}, // 1
	{0b111, 0b001, 0b111, 0b100, 0b111}, // 2
	{0b111, 0b001, 0b111, 0b001, 0b111}, // 3
	{0b101, 0b101, 0b111, 0b001, 0b001}, // 4
	{0b111, 0b100, 0b111, 0b001, 0b111}, // 5
	{0b111, 0b100, 0b111, 0b101, 0b111}, // 6
	{0b111, 0b001, 0b001, 0b001, 0b001}, // 7
	{0b111, 0b101, 0b111, 0b101, 0b111}, // 8
	{0b111, 0b101, 0b111, 0b001, 0b111}, // 9
}

// makeBadgeIcon 生成 16×16 的红底白字角标图标（失败返回 0）。
func makeBadgeIcon(n int) uintptr {
	if procCreateIconIndirect.Find() != nil {
		return 0
	}
	const size = 16
	// 画到 32bpp 的 DIB 上（BGRA）
	bi := BITMAPINFO{}
	bi.Header.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.Header.BiWidth = size
	bi.Header.BiHeight = -size // 自顶向下
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = BI_RGB
	var bits unsafe.Pointer
	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return 0
	}
	defer procDeleteDC.Call(hdc)
	hbm, _, _ := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return 0
	}
	defer procDeleteObject.Call(hbm)

	px := unsafe.Slice((*uint32)(bits), size*size)
	// 圆 + 白字
	cx, cy, r := float32(size-1)/2, float32(size-1)/2, float32(size)/2-0.5
	put := func(x, y int, v uint32) {
		if x < 0 || y < 0 || x >= size || y >= size {
			return
		}
		px[y*size+x] = v
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float32(x)-cx, float32(y)-cy
			if dx*dx+dy*dy <= r*r {
				put(x, y, 0xFF3B30E5) // BGRA: 红 0xE5 0x30 0x3B, alpha 0xFF
			}
		}
	}
	// 数字（1~2 位）
	digits := []int{}
	if n > 99 {
		digits = []int{9, 9}
	} else if n >= 10 {
		digits = []int{n / 10, n % 10}
	} else {
		digits = []int{n}
	}
	scale := 1
	if len(digits) == 1 {
		scale = 2
	}
	gw, gh := 3*scale, 5*scale
	totalW := len(digits)*gw + (len(digits)-1)*scale
	ox := (size - totalW) / 2
	oy := (size - gh) / 2
	for i, d := range digits {
		g := digitGlyphs[d]
		for row := 0; row < 5; row++ {
			for col := 0; col < 3; col++ {
				if g[row]&(1<<uint(2-col)) == 0 {
					continue
				}
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						put(ox+i*(gw+scale)+col*scale+sx, oy+row*scale+sy, 0xFFFFFFFF)
					}
				}
			}
		}
	}

	// 掩码：全 0 表示「按 color 位图的 alpha 合成」
	mask, _, _ := procCreateBitmap.Call(size, size, 1, 1, 0)
	if mask == 0 {
		return 0
	}
	defer procDeleteObject.Call(mask)

	ii := iconInfo{FIcon: 1, HbmMask: mask, HbmColor: hbm}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return icon
}

// iconInfo ICONINFO 结构。
type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// procCreateBitmap 单色掩码位图（其它 GDI 过程在 desktop_extra.go 里已声明）。
var procCreateBitmap = gdi32.NewProc("CreateBitmap")
