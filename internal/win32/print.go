//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// GDI 打印：PrintDlgW 选打印机 → StartDocW/StartPage → 回调里用 GDI 绘制 → EndDoc。
// 回调拿到的 dc 是打印机设备上下文，dpiX/dpiY 为打印机分辨率，
// 页面尺寸 pageW/pageH 单位为 0.01 毫米（与 DEVMODE 一致）。

var (
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	winspool = syscall.NewLazyDLL("winspool.drv")

	procPrintDlgW        = comdlg32.NewProc("PrintDlgW")
	procComDlgExtErr     = comdlg32.NewProc("CommDlgExtendedError")
	procGetDefaultPrinterW = winspool.NewProc("GetDefaultPrinterW")
	procEnumPrintersW      = winspool.NewProc("EnumPrintersW")
	procCreateDCW          = gdi32.NewProc("CreateDCW")
	procStartDocW        = gdi32.NewProc("StartDocW")
	procStartPage        = gdi32.NewProc("StartPage")
	procEndPage          = gdi32.NewProc("EndPage")
	procEndDoc           = gdi32.NewProc("EndDoc")
	procAbortDoc         = gdi32.NewProc("AbortDoc")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procTextOutW         = gdi32.NewProc("TextOutW")
	procMoveToEx         = gdi32.NewProc("MoveToEx")
	procLineTo           = gdi32.NewProc("LineTo")
	procRectangle        = gdi32.NewProc("Rectangle")
	procEllipse          = gdi32.NewProc("Ellipse")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procSetTextAlign     = gdi32.NewProc("SetTextAlign")
	procGetDeviceCaps    = gdi32.NewProc("GetDeviceCaps")
)

const (
	pdReturnDC         = 0x00000100
	pdNoWarning        = 0x00000080
	pdReturnDefault    = 0x00000400
	pdUseDevModeCopies = 0x00040000
	psSolid            = 0
	transparentBk      = 1
	logPixelsX         = 88
	logPixelsY         = 90
	horzRes            = 8
	vertRes            = 10
	physWidth          = 110
	physHeight         = 111
)

type docInfoW struct {
	CbSize       int32
	LpszDocName  *uint16
	LpszOutput   *uint16
	LpszDatatype *uint16
	FwType       uint32
}

type printDlgW struct {
	LStructSize    uint32
	HwndOwner      uintptr
	HDevMode       uintptr
	HDevNames      uintptr
	HDC            uintptr
	Flags          uint32
	NFromPage      uint16
	NToPage        uint16
	NMinPage       uint16
	NMaxPage       uint16
	NCopies        uint16
	HInstance      uintptr
	LCustData      uintptr
	LpfnHook       uintptr
	LpTemplateName *uint16
}

var nullBrush = ^uintptr(0)

// PrintInfo 一次打印任务的输出信息。
type PrintInfo struct {
	DPIx, DPIy   int
	PageW, PageH int // 0.01mm
}

// PrintDialog 弹出打印对话框并打印 pages 页。
// draw 返回 false 可中止后续页。用户取消返回 (false, nil)。
func PrintDialog(owner uintptr, title string, pages int, draw func(dc uintptr, page int, info PrintInfo) bool) (bool, error) {
	pd := printDlgW{
		LStructSize: uint32(unsafe.Sizeof(printDlgW{})),
		HwndOwner:   owner,
		Flags:       pdReturnDC | pdNoWarning | pdUseDevModeCopies,
		NMinPage:    1,
		NMaxPage:    uint16(maxInt(pages, 1)),
		NFromPage:   1,
		NToPage:     uint16(maxInt(pages, 1)),
	}
	r, _, _ := procPrintDlgW.Call(uintptr(unsafe.Pointer(&pd)))
	if r == 0 {
		// 0 可能是用户取消（CommDlgExtendedError 也为 0），也可能是对话框没弹出就直接失败：
		// 后者必须报错，否则调用方会表现得「点了没反应」。
		if e, _, _ := procComDlgExtErr.Call(); e != 0 {
			return false, fmt.Errorf("win32: 打印对话框无法打开（%s）", comDlgErrText(uint32(e)))
		}
		return false, nil
	}
	if pd.HDC == 0 {
		return false, errors.New("win32: 打印对话框未返回 DC")
	}
	defer procDeleteDC.Call(pd.HDC)

	return runPrintJob(pd.HDC, title, pages, draw)
}

// runPrintJob 在已拿到的打印机 DC 上把 pages 页画出去（StartDoc → StartPage … → EndDoc）。
func runPrintJob(hdc uintptr, title string, pages int, draw func(dc uintptr, page int, info PrintInfo) bool) (bool, error) {
	info := PrintInfo{}
	if v, _, _ := procGetDeviceCaps.Call(hdc, logPixelsX); v != 0 {
		info.DPIx = int(v)
	}
	if v, _, _ := procGetDeviceCaps.Call(hdc, logPixelsY); v != 0 {
		info.DPIy = int(v)
	}
	if v, _, _ := procGetDeviceCaps.Call(hdc, physWidth); v != 0 {
		info.PageW = int(v)
	}
	if v, _, _ := procGetDeviceCaps.Call(hdc, physHeight); v != 0 {
		info.PageH = int(v)
	}
	if info.DPIx <= 0 {
		info.DPIx = 300
	}
	if info.DPIy <= 0 {
		info.DPIy = 300
	}

	t, _ := syscall.UTF16PtrFromString(title)
	di := docInfoW{CbSize: int32(unsafe.Sizeof(docInfoW{})), LpszDocName: t}
	if r, _, err := procStartDocW.Call(hdc, uintptr(unsafe.Pointer(&di))); int32(r) <= 0 {
		return false, errors.New("win32: StartDoc 失败: " + errText(err))
	}
	defer procEndDoc.Call(hdc)
	for p := 0; p < pages; p++ {
		if r, _, _ := procStartPage.Call(hdc); int32(r) <= 0 {
			procAbortDoc.Call(hdc)
			return false, errors.New("win32: StartPage 失败")
		}
		ok := true
		if draw != nil {
			ok = draw(hdc, p, info)
		}
		procEndPage.Call(hdc)
		if !ok {
			procAbortDoc.Call(hdc)
			return false, nil
		}
	}
	return true, nil
}

// PrintTo 不弹对话框，直接打印到指定打印机。
// 用于系统打印对话框不可用（PrintDlgW 返回 CDERR_DIALOGFAILURE）的环境。
func PrintTo(printer, title string, pages int, draw func(dc uintptr, page int, info PrintInfo) bool) (bool, error) {
	name := strings.TrimSpace(printer)
	if name == "" {
		return false, errors.New("win32: 未指定打印机")
	}
	if pages <= 0 {
		pages = 1
	}
	driver := mustUTF16("WINSPOOL")
	device, err := syscall.UTF16FromString(name)
	if err != nil {
		return false, fmt.Errorf("win32: 打印机名称非法: %v", err)
	}
	dc, _, callErr := procCreateDCW.Call(
		uintptr(unsafe.Pointer(&driver[0])),
		uintptr(unsafe.Pointer(&device[0])), 0, 0)
	runtime.KeepAlive(driver)
	runtime.KeepAlive(device)
	if dc == 0 {
		return false, fmt.Errorf("win32: 打开打印机 %q 失败: %s", name, errText(callErr))
	}
	defer procDeleteDC.Call(dc)
	return runPrintJob(dc, title, pages, draw)
}

// DefaultPrinter 默认打印机名（没有则空串）。
func DefaultPrinter() string {
	n := uint32(0)
	procGetDefaultPrinterW.Call(0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	if r, _, _ := procGetDefaultPrinterW.Call(uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// printerInfo4 EnumPrinters 的 level 4 条目（名称 / 服务器 / 属性）。
type printerInfo4 struct {
	PrinterName *uint16
	ServerName  *uint16
	Attributes  uint32
}

// Printers 枚举本机与已连接的打印机名（默认打印机排在第一位）。
func Printers() []string {
	const (
		printerEnumLocal       = 0x00000002
		printerEnumConnections = 0x00000004
	)
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	needed, returned := uint32(0), uint32(0)
	procEnumPrintersW.Call(flags, 0, 4, 0, 0,
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return nil
	}
	buf := make([]byte, needed)
	if r, _, _ := procEnumEnumPrintersCall(flags, buf, &needed, &returned); r == 0 || returned == 0 {
		return nil
	}
	items := unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buf[0])), int(returned))
	names := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		name := utf16PtrString(it.PrinterName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	runtime.KeepAlive(buf)
	// 默认打印机前置，界面上第一项就是「直接回车打印」的那台。
	if def := DefaultPrinter(); def != "" {
		for i, n := range names {
			if n == def && i > 0 {
				names[0], names[i] = names[i], names[0]
				break
			}
		}
	}
	return names
}

// utf16PtrString 读 NUL 结尾的 UTF-16 字符串（最多 1024 字符，遇到 NUL 即停）。
func utf16PtrString(p *uint16) string {
	if p == nil {
		return ""
	}
	n := 0
	for n < 1024 {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(n)*2)) == 0 {
			break
		}
		n++
	}
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// procEnumEnumPrintersCall 第二趟 EnumPrintersW（带缓冲区）。
func procEnumEnumPrintersCall(flags uintptr, buf []byte, needed, returned *uint32) (uintptr, uintptr, error) {
	return procEnumPrintersW.Call(flags, 0, 4,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(needed)), uintptr(unsafe.Pointer(returned)))
}

// PagePixels 按打印机 DPI 换算页面像素尺寸（GDI 坐标单位）。
func PagePixels(info PrintInfo) (w, h int) {
	w = int(float32(info.PageW) / 2540 * float32(info.DPIx))
	h = int(float32(info.PageH) / 2540 * float32(info.DPIy))
	if w <= 0 {
		w = 2480
	}
	if h <= 0 {
		h = 3508
	}
	return w, h
}

// ---- GDI 绘制原语（打印回调内使用，坐标单位 = 设备像素）----

// PrintText 在 DC 上绘制文本（坐标像素，fontPx 为字体像素高）。
func PrintText(dc uintptr, x, y, fontPx float32, text, rgb string) {
	r, g, b := hexRGB(rgb)
	face := mustUTF16("Microsoft YaHei")
	font, _, _ := procCreateFontW.Call(
		uintptr(int32(-fontPx)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&face[0])))
	old, _, _ := procSelectObject.Call(dc, font)
	defer func() {
		procSelectObject.Call(dc, old)
		procDeleteObject.Call(font)
	}()
	procSetBkMode.Call(dc, transparentBk)
	procSetTextColor.Call(dc, uintptr(rgbRef(r, g, b)))
	p, _ := syscall.UTF16FromString(text)
	procTextOutW.Call(dc, uintptr(int32(x)), uintptr(int32(y)), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1))
}

// PrintRect 画矩形（fill 为空表示不填充，strokeW 为 0 表示不描边；单位像素）。
func PrintRect(dc uintptr, x, y, w, h, strokeW float32, strokeRGB, fillRGB string) {
	var brush, pen uintptr
	if fillRGB != "" {
		r, g, b := hexRGB(fillRGB)
		brush, _, _ = procCreateSolidBrush.Call(uintptr(rgbRef(r, g, b)))
	} else {
		brush = nullBrush
	}
	if strokeW > 0 {
		r, g, b := hexRGB(strokeRGB)
		pen, _, _ = procCreatePen.Call(psSolid, uintptr(int32(strokeW)), uintptr(rgbRef(r, g, b)))
	}
	oldB, _, _ := procSelectObject.Call(dc, brush)
	oldP, _, _ := procSelectObject.Call(dc, pen)
	procRectangle.Call(dc, uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(x+w)), uintptr(int32(y+h)))
	procSelectObject.Call(dc, oldB)
	procSelectObject.Call(dc, oldP)
	if brush != nullBrush {
		procDeleteObject.Call(brush)
	}
	if pen != 0 {
		procDeleteObject.Call(pen)
	}
}

// PrintLine 画直线（单位像素）。
func PrintLine(dc uintptr, x1, y1, x2, y2, width float32, rgb string) {
	r, g, b := hexRGB(rgb)
	pen, _, _ := procCreatePen.Call(psSolid, uintptr(int32(width)), uintptr(rgbRef(r, g, b)))
	old, _, _ := procSelectObject.Call(dc, pen)
	procMoveToEx.Call(dc, uintptr(int32(x1)), uintptr(int32(y1)), 0)
	procLineTo.Call(dc, uintptr(int32(x2)), uintptr(int32(y2)))
	procSelectObject.Call(dc, old)
	procDeleteObject.Call(pen)
}

func hexRGB(s string) (int, int, int) {
	if len(s) == 7 && s[0] == '#' {
		v := 0
		for i := 1; i < 7; i++ {
			c := s[i]
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= int(c - '0')
			case c >= 'a' && c <= 'f':
				v |= int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				v |= int(c-'A') + 10
			}
		}
		return (v >> 16) & 0xff, (v >> 8) & 0xff, v & 0xff
	}
	return 0, 0, 0
}

// rgbRef 转为 Win32 COLORREF（0x00BBGGRR）。
func rgbRef(r, g, b int) uint32 {
	return uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

func mustUTF16(s string) []uint16 {
	p, _ := syscall.UTF16FromString(s)
	return p
}

// comDlgErrText 把 CommDlgExtendedError 的值翻成可读文案（只列常见项）。
func comDlgErrText(code uint32) string {
	switch code {
	case 1:
		return "CDERR_DIALOGFAILURE: 对话框创建失败，系统打印对话框不可用"
	case 2:
		return "CDERR_GENERALCODES"
	case 4:
		return "CDERR_STRUCTSIZE: 结构体大小不合法"
	case 5:
		return "CDERR_INITIALIZATION: 初始化失败"
	case 6:
		return "CDERR_NOHINSTANCE: 缺少 hInstance"
	case 7:
		return "CDERR_NOHOOK"
	case 0x1002:
		return "PDERR_SETUPFAILURE: 打印机未正确安装"
	case 0x1005:
		return "PDERR_NODEFAULTPRN: 没有默认打印机"
	case 0x1008:
		return "PDERR_DNDMMISMATCH: DEVMODE 与 DEVNAMES 不匹配"
	case 0x1009:
		return "PDERR_CREATEICFAILURE: 创建打印机 DC 失败"
	case 0x100A:
		return "PDERR_PRINTERNOTFOUND: 找不到打印机"
	case 0x100C:
		return "PDERR_DEFAULTDIFFERENT: 默认打印机不一致"
	}
	return fmt.Sprintf("CommDlgExtendedError=0x%x", code)
}

func errText(err error) string {
	if err == nil {
		return "unknown"
	}
	return err.Error()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
