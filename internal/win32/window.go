//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package win32

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// EventType 平台无关事件类型。
type EventType int

const (
	EvNone EventType = iota
	EvResize
	EvClose
	EvFocusGained
	EvFocusLost
	EvMouseMove
	EvMouseDown
	EvMouseUp
	EvMouseWheel
	EvMouseLeave
	EvKeyDown
	EvKeyUp
	EvChar
	// IME 组合输入（中文输入法）
	EvCompositionStart
	EvComposition
	EvCompositionEnd
	// 系统拖动/缩放窗口开始与结束（WM_ENTERSIZEMOVE / WM_EXITSIZEMOVE）
	EvSizeMoveStart
	EvSizeMoveEnd
	// 文件拖入窗口（WM_DROPFILES），Event.Paths 为路径列表，X/Y 为松手落点
	EvDropFiles
	// 全局热键命中（WM_HOTKEY），Event.HotkeyID 为注册时给的 id
	EvHotkey
)

// Event 传给上层的事件。
type Event struct {
	Type   EventType
	X, Y   float32 // 逻辑像素（已除 DPI）
	DX, DY float32
	Button int
	Key    uint32
	Char   rune
	Runes  []rune
	Width  float32
	Height float32
	Scale  float32

	// Text / Caret 用于 IME 组合串（EvComposition*）与多字符上屏。
	Text  string
	Caret int

	// Paths 为拖入的文件/文件夹路径（EvDropFiles），HotkeyID 为全局热键 id（EvHotkey）。
	Paths    []string
	HotkeyID int
}

// Config 窗口配置。
type Config struct {
	Title       string
	Width       int
	Height      int
	MinWidth    int
	MinHeight   int
	Resizable   bool
	Decorated   bool
	Transparent bool
	// CornerRadius 窗口圆角半径（逻辑像素；0 = 不设置，>0 = 圆角）。
	CornerRadius float32
	// NoDrops 不接收文件拖入（默认接收，见 Window.SetAcceptDrops）。
	NoDrops bool
}

// Window 一个原生窗口 + GL 上下文。
type Window struct {
	Hwnd  uintptr
	HDC   uintptr
	HGLRC uintptr

	cfg   Config
	w     int32
	h     int32
	scale float32
	dpi   uint32
	// borderQuiet 边框策略首次应用后置位（窗口激活时会反复应用，只首次打日志）
	borderQuiet bool
	onEvent     func(Event)
	running     bool
	id          int

	// IME 组合状态
	compText  string
	compCaret int
	imeIME    bool

	// 全屏状态（进入前保存样式与窗口矩形）
	fullscreen bool
	savedStyle uintptr
	savedRect  RECT

	// 窗口圆角（逻辑像素；0 = 未设置）
	cornerRadius float32
	// acceptDrops 当前是否接收文件拖入
	acceptDrops bool
	// topmost 当前是否置顶（SetAlwaysOnTop）
	topmost bool

	// 最小 / 最大尺寸（逻辑像素，0 = 不限制；由 WM_GETMINMAXINFO 应用）
	minW, minH int32
	maxW, maxH int32

	// 窗口特效状态（wineffects.go）；变更后需要查询时直接用缓存值
	opacity         float32
	clickThrough    bool
	bottom          bool
	skipTaskbar     bool
	captureExcluded bool
	darkTitle       bool
	backdrop        int
	// vetoClose 本次 WM_CLOSE 是否被上层放弃（收进托盘时不销毁窗口）
	vetoClose atomic.Bool
	// roundedDWM / roundedRgn 记录当前生效的圆角实现，便于切换与取消
	roundedDWM bool
	roundedRgn bool
}

var (
	mu              sync.Mutex
	wndByHwnd       = map[uintptr]*Window{}
	traysByHwnd     = map[uintptr]*Tray{}
	wndNextID       int
	wndProcCallback uintptr
	classAtom       uint16
	classOnce       sync.Once
)

const (
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // -4
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE    = ^uintptr(1) // -2
)

// NewWindow 创建窗口并初始化 OpenGL 上下文（需在 UI 线程调用）。
func NewWindow(cfg Config) (*Window, error) {
	if cfg.Width <= 0 {
		cfg.Width = 1024
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}

	w := &Window{cfg: cfg, scale: 1}

	hInst := GetModuleHandleW(nil)

	classOnce.Do(func() {
		if SetProcessDpiAwarenessContext != nil {
			if SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2) == 0 {
				SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE)
			}
		} else if SetProcessDPIAware != nil {
			SetProcessDPIAware()
		}

		wndProcCallback = purego.NewCallback(wndProc)
		className, _ := syscall.UTF16PtrFromString("GoGuiWindowClass")
		wc := WNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
			Style:         CS_HREDRAW | CS_VREDRAW | CS_DBLCLKS | CS_OWNDC,
			LpfnWndProc:   wndProcCallback,
			HInstance:     hInst,
			HCursor:       LoadCursorW(0, IDC_ARROW),
			LpszClassName: className,
			// 背景刷：非客户区（无标题栏时的边框带）用深色，
			// 否则系统会按默认白色绘制，表现为“隐藏标题栏后顶部一条白边”。
			HbrBackground: CreateSolidBrush(0x000A0605), // #05060a（与启动动画底色一致，杜绝白闪）
		}
		classAtom = RegisterClassExW(&wc)
		if classAtom == 0 {
			return
		}
	})

	if classAtom == 0 {
		return nil, fmt.Errorf("win32: RegisterClassExW failed (err=%d)", GetLastError())
	}
	dbg("class registered atom=%d hinst=%#x", classAtom, hInst)

	style := uint32(WS_OVERLAPPEDWINDOW | WS_CLIPCHILDREN)
	if !cfg.Resizable {
		style &^= 0x00040000 // WS_THICKFRAME
	}
	if !cfg.Decorated {
		// 无系统标题栏（自绘标题栏场景），保留可调尺寸的边框
		style = WS_POPUP | WS_CLIPCHILDREN
		if cfg.Resizable {
			style |= 0x00040000
		}
	}

	mu.Lock()
	wndNextID++
	w.id = wndNextID
	mu.Unlock()

	className, _ := syscall.UTF16PtrFromString("GoGuiWindowClass")
	title, _ := syscall.UTF16PtrFromString(cfg.Title)

	hwnd := CreateWindowExW(0,
		className,
		title,
		style,
		CW_USEDEFAULT, CW_USEDEFAULT,
		int32(cfg.Width), int32(cfg.Height),
		0, 0, hInst, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("win32: CreateWindowExW failed (err=%d)", GetLastError())
	}

	w.Hwnd = hwnd
	dbg("window created hwnd=%#x", hwnd)

	rect := RECT{0, 0, int32(cfg.Width), int32(cfg.Height)}
	AdjustWindowRectEx(&rect, style, 0, 0)
	runtime.KeepAlive(title)
	runtime.KeepAlive(className)

	mu.Lock()
	wndByHwnd[hwnd] = w
	mu.Unlock()

	// 启动即居中：按窗口所在显示器的工作区（排除任务栏）水平 + 垂直居中。
	// 必须在显示之前，避免窗口先在默认位置闪一下再瞬移。
	w.Center()

	// 注意：这里**不**立刻 ShowWindow。窗口先保持隐藏，等 ui 层画完第一帧再显示
	// （App.present 里首次 Swap 后调用 SetVisible(true)），否则会先看到窗口类背景色
	// 或白色客户区，也就是启动时的「白屏/闪一下」。
	UpdateWindow(hwnd)

	hdc := GetDC(hwnd)
	if hdc == 0 {
		return nil, fmt.Errorf("win32: GetDC failed")
	}
	w.HDC = hdc

	pfd := PIXELFORMATDESCRIPTOR{
		NSize:        uint16(unsafe.Sizeof(PIXELFORMATDESCRIPTOR{})),
		NVersion:     1,
		DwFlags:      PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER,
		IPixelType:   PFD_TYPE_RGBA,
		CColorBits:   32,
		CDepthBits:   0,
		CStencilBits: 8,
		ILayerType:   PFD_MAIN_PLANE,
	}
	pf := ChoosePixelFormat(hdc, &pfd)
	if pf == 0 {
		return nil, fmt.Errorf("win32: ChoosePixelFormat failed")
	}
	if SetPixelFormat(hdc, pf, &pfd) == 0 {
		return nil, fmt.Errorf("win32: SetPixelFormat failed")
	}

	rc := wglCreateContext(hdc)
	if rc == 0 {
		return nil, fmt.Errorf("win32: wglCreateContext failed")
	}
	if wglMakeCurrent(hdc, rc) == 0 {
		return nil, fmt.Errorf("win32: wglMakeCurrent failed")
	}
	w.HGLRC = rc
	dbg("gl context ready rc=%#x", rc)

	InitGL()
	// 开启垂直同步：SwapBuffers 与显示器刷新对齐，避免撕裂/帧率抖动
	if wglSwapIntervalEXT != nil {
		wglSwapIntervalEXT(1)
	}
	w.refreshMetrics()
	// 无标题栏窗口：把 DWM 边框向外扩 1px，压掉 DWM 在四周画的那条 1px 边框线
	// （这是“窗口周围一圈浅色细线”的根源；仅靠 DWMWA_BORDER_COLOR 在当前 Win11
	//  版本上仍有残留，两条一起用才彻底）。
	if !cfg.Decorated && DwmExtendFrameIntoClientArea != nil {
		m := MARGINS{Left: 1, Right: 1, Top: 1, Bottom: 1}
		if DwmExtendFrameIntoClientArea(w.Hwnd, &m) == 0 {
			dbg("border: DWM 边框已向外扩展 1px")
		}
	}
	// 去掉 DWM 绘制的 1px 系统边框（无标题栏与带标题栏窗口都处理，见 hideSystemBorder）
	w.hideSystemBorder()
	// 文件拖入（WM_DROPFILES）：默认接收，可用 SetAcceptDrops(false) 关闭
	w.SetAcceptDrops(!cfg.NoDrops)
	// 尺寸约束：Options / display.Config 里的 MinWidth/MinHeight 直接生效，
	// 业务后续可用 App.SetMinSize / SetMaxSize 调整。
	if cfg.MinWidth > 0 || cfg.MinHeight > 0 {
		s := w.scale
		if s <= 0 {
			s = 1
		}
		w.minW = int32(float32(cfg.MinWidth) * s)
		w.minH = int32(float32(cfg.MinHeight) * s)
	}
	// 剪贴板变化通知：注册后系统通过 WM_CLIPBOARDUPDATE 主动推送，
	// WatchClipboard 因此不再需要轮询序号。
	if procAddClipboardFormatListener.Find() == nil {
		procAddClipboardFormatListener.Call(w.Hwnd)
	}
	w.running = true
	// 初始圆角（需在 scale 就绪后应用，SetWindowRgn 按物理像素计算）
	if cfg.CornerRadius > 0 {
		w.SetCornerRadius(cfg.CornerRadius)
	}
	dbg("window ready %dx%d scale=%.2f", w.w, w.h, w.scale)
	return w, nil
}

func (w *Window) refreshMetrics() {
	hdc := w.HDC
	if hdc == 0 {
		hdc = GetDC(w.Hwnd)
	}
	var r RECT
	GetClientRect(w.Hwnd, &r)
	w.w = r.Right - r.Left
	w.h = r.Bottom - r.Top

	dpi := uint32(96)
	if GetDpiForWindow != nil {
		if d := GetDpiForWindow(w.Hwnd); d > 0 {
			dpi = d
		}
	}
	w.dpi = dpi
	w.scale = float32(dpi) / 96.0
}

// Size 返回逻辑尺寸。
func (w *Window) Size() (float32, float32) {
	return float32(w.w) / w.scale, float32(w.h) / w.scale
}

// PixelSize 返回物理像素尺寸。
func (w *Window) PixelSize() (int32, int32) { return w.w, w.h }

// Scale 返回 DPI 缩放。
func (w *Window) Scale() float32 { return w.scale }

// IsMinimized 窗口是否最小化（最小化时跳过渲染，避免空耗 GPU/CPU）。
func (w *Window) IsMinimized() bool {
	if IsIconic == nil || w.Hwnd == 0 {
		return false
	}
	return IsIconic(w.Hwnd) != 0
}

// SetTitle 设置窗口标题。
func (w *Window) SetTitle(s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	SetWindowTextW(w.Hwnd, p)
}

// SetEventHandler 注册事件回调（UI 线程）。
func (w *Window) SetEventHandler(fn func(Event)) { w.onEvent = fn }

// RequestRedraw 触发重绘。
func (w *Window) RequestRedraw() {
	PostMessageW(w.Hwnd, WM_PAINT, 0, 0)
}

// Swap 交换缓冲。
func (w *Window) Swap() {
	if w.HDC != 0 {
		SwapBuffers(w.HDC)
	}
}

// MakeCurrent 确保当前 GL 上下文。
func (w *Window) MakeCurrent() {
	if wglGetCurrentContext() != w.HGLRC {
		wglMakeCurrent(w.HDC, w.HGLRC)
	}
}

// Destroy 销毁窗口。
func (w *Window) Destroy() {
	w.running = false
	mu.Lock()
	if t := traysByHwnd[w.Hwnd]; t != nil {
		// 必须用 removeLocked：这里已持有 mu，再走 Remove() 会重复加锁自锁死
		t.removeLocked()
	}
	delete(wndByHwnd, w.Hwnd)
	mu.Unlock()
	if w.HGLRC != 0 {
		wglMakeCurrent(0, 0)
		wglDeleteContext(w.HGLRC)
	}
	if w.HDC != 0 {
		ReleaseDC(w.Hwnd, w.HDC)
	}
	DestroyWindow(w.Hwnd)
}

// PollMessages 泵一次消息队列，返回是否仍在运行。
func PollMessages() bool {
	var msg MSG
	for PeekMessageW(&msg, 0, 0, 0, PM_REMOVE) != 0 {
		if msg.Message == WM_QUIT {
			return false
		}
		TranslateMessage(&msg)
		DispatchMessageW(&msg)
	}
	return true
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	mu.Lock()
	w := wndByHwnd[hwnd]
	mu.Unlock()

	if w == nil {
		return DefWindowProcW(hwnd, msg, wp, lp)
	}

	// 托盘消息（鼠标点击/右键菜单）
	if msg == WM_TRAYICON {
		handleTray(hwnd, lp)
		return 0
	}

	switch msg {
	case WM_CLIPBOARDUPDATE:
		// 剪贴板变化（AddClipboardFormatListener 注册后由系统投递）：
		// 直接转发给注册方，避免 500ms 轮询序号带来的延迟与常驻开销。
		dispatchClipboardChange()
		return 0
	case WM_GETMINMAXINFO:
		// 窗口尺寸约束：业务用 SetMinSize / SetMaxSize 设置后在这里生效。
		// 不填 PiMinTrackSize 时系统允许把窗口拖成 0×0，自绘布局会瞬间塌掉。
		if lp != 0 && (w.minW > 0 || w.minH > 0 || w.maxW > 0 || w.maxH > 0) {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lp))
			if w.minW > 0 {
				mmi.PtMinTrackSize.X = w.minW
			}
			if w.minH > 0 {
				mmi.PtMinTrackSize.Y = w.minH
			}
			if w.maxW > 0 {
				mmi.PtMaxTrackSize.X = w.maxW
			}
			if w.maxH > 0 {
				mmi.PtMaxTrackSize.Y = w.maxH
			}
			return 0
		}
	case WM_CLOSE:
		// 先派发关闭事件：上层可在回调里调用 BlockClose 放弃这次关闭
		//（「关闭时最小化到托盘」只隐藏窗口，不销毁）。标志每次重置，
		// 避免上一次拦截影响下一次真正的退出。
		w.vetoClose.Store(false)
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvClose})
		}
		if w.vetoClose.Load() {
			w.running = true
			ShowWindow(hwnd, SW_HIDE)
			return 0
		}
		w.running = false
		DestroyWindow(hwnd)
		return 0

	case WM_DESTROY:
		// 注销剪贴板监听，避免窗口销毁后仍收到 WM_CLIPBOARDUPDATE
		if procRemoveClipboardFormatListener.Find() == nil {
			procRemoveClipboardFormatListener.Call(hwnd)
		}
		mu.Lock()
		// 直接关窗口（WM_CLOSE 未被拦截）不会走 Window.Destroy，托盘图标会残留成幽灵图标
		if t := traysByHwnd[hwnd]; t != nil {
			t.removeLocked()
		}
		delete(wndByHwnd, hwnd)
		empty := len(wndByHwnd) == 0
		mu.Unlock()
		releaseHotkeys(hwnd)
		if empty {
			PostQuitMessage(0)
		}
		return 0

	case WM_SIZE:
		w.w = int32(lp & 0xFFFF)
		w.h = int32((lp >> 16) & 0xFFFF)
		// 尺寸变化后 SetWindowRgn 的区域需要按新尺寸重建
		w.reapplyCorner()
		if w.onEvent != nil && w.w > 0 && w.h > 0 {
			lw, lh := w.Size()
			w.onEvent(Event{Type: EvResize, Width: lw, Height: lh, Scale: w.scale})
		}
		return 0

	case WM_DROPFILES:
		// 文件拖入：**HDROP 在 wParam 里**（lParam 恒为 0），里而是路径列表；
		// 同时取出松手落点（客户区物理像素）。
		ev := w.dropEvent(wp)
		dbg("drop: hdrop=%#x paths=%d at %.0f,%.0f", wp, len(ev.Paths), ev.X, ev.Y)
		if w.onEvent != nil {
			w.onEvent(ev)
		}
		SetForegroundWindow(hwnd)
		return 0

	case WM_HOTKEY:
		// 全局热键（RegisterHotKey 注册，见 system.go）
		dispatchHotkey(hwnd, uint32(wp))
		return 0

	case WM_ENTERSIZEMOVE:
		// 系统拖动/缩放窗口：进入模态循环后鼠标消息不再送到窗口，
		// 上层据此隐藏滚动条与悬停高亮（否则窗边会闪出一条“假滚动条”）。
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvSizeMoveStart})
		}
		return 0

	case WM_EXITSIZEMOVE:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvSizeMoveEnd})
		}
		return 0

	case WM_DPICHANGED:
		w.refreshMetrics()
		w.reapplyCorner()
		lw, lh := w.Size()
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvResize, Width: lw, Height: lh, Scale: w.scale})
		}
		return 0

	case WM_PAINT:
		ValidateRect(hwnd, nil)
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvNone})
		}
		return 0

	case WM_ERASEBKGND:
		return 1

	case WM_NCCALCSIZE:
		// 无系统标题栏：客户区直接铺满整个窗口，不留系统边框。
		// 否则 WS_POPUP|WS_THICKFRAME 会留约 9px 非客户区，
		// 那一条被系统画成白色（顶部白边的真正来源）。
		if !w.cfg.Decorated && wp != 0 {
			return 0
		}
		return DefWindowProcW(hwnd, msg, wp, lp)

	case WM_NCHITTEST:
		// 客户区铺满整窗后，系统边框的“拉边缩放”命中失效，
		// 这里按 8px 边缘手动给命中值，保留隐藏标题栏时的缩放能力。
		if !w.cfg.Decorated {
			x := int32(int16(uint16(lp & 0xFFFF)))
			y := int32(int16(uint16((lp >> 16) & 0xFFFF)))
			var r RECT
			GetWindowRect(hwnd, &r)
			b := int32(8)
			if w.scale > 1 {
				b = int32(8 * w.scale)
			}
			left, right := x < r.Left+b, x >= r.Right-b
			top, bottom := y < r.Top+b, y >= r.Bottom-b
			switch {
			case top && left:
				return HTTOPLEFT
			case top && right:
				return HTTOPRIGHT
			case bottom && left:
				return HTBOTTOMLEFT
			case bottom && right:
				return HTBOTTOMRIGHT
			case left:
				return HTLEFT
			case right:
				return HTRIGHT
			case top:
				return HTTOP
			case bottom:
				return HTBOTTOM
			}
		}
		return DefWindowProcW(hwnd, msg, wp, lp)

	case WM_NCPAINT:
		// 无标题栏：客户区已铺满整窗，非客户区没有任何东西可画。
		// 交给系统绘制就会在窗口四周留下 1px 边框线（首次激活时尤其明显）。
		if !w.cfg.Decorated {
			w.hideSystemBorder()
			return 0
		}
		return DefWindowProcW(hwnd, msg, wp, lp)

	case WM_NCUAHDRAWCAPTION, WM_NCUAHDRAWFRAME:
		// 主题引擎通过这两条未公开消息绘制标题栏/框架（即使窗口没有标题栏）。
		// 无标题栏窗口直接吞掉，避免激活瞬间在四周闪出一圈系统边框。
		if !w.cfg.Decorated {
			return 0
		}
		return DefWindowProcW(hwnd, msg, wp, lp)

	case WM_SETFOCUS:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvFocusGained})
		}
		return 0

	case WM_NCACTIVATE:
		// 自绘边框窗口：绝不把 WM_NCACTIVATE 交给 DefWindowProc，
		// 否则系统会按“活动窗口”重画整个非客户区框架：从别的窗口点回来
		// （首次获取焦点）时，窗口四周就冒出一圈系统边框，直到下一次重绘才消失。
		// 客户区已铺满整窗，返回 TRUE 即可，无需系统绘制框架。
		if !w.cfg.Decorated {
			w.hideSystemBorder()
			if w.onEvent != nil {
				w.onEvent(Event{Type: EvNone})
			}
			return 1
		}
		r := DefWindowProcW(hwnd, msg, wp, lp)
		w.hideSystemBorder()
		// 同时重绘一帧：激活瞬间客户区可能残留 1px 旧像素/错位，整帧 glClear 后彻底干净
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvNone})
		}
		return r

	case WM_ACTIVATE:
		r := DefWindowProcW(hwnd, msg, wp, lp)
		w.hideSystemBorder()
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvNone})
		}
		return r

	case WM_KILLFOCUS:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvFocusLost})
		}
		return 0

	case WM_MOUSEMOVE:
		tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: 0x00000002, HwndTrack: hwnd}
		TrackMouseEvent(&tme)
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseMove, X: float32(loword(lp)) / w.scale, Y: float32(hiword(lp)) / w.scale})
		}
		return 0

	case WM_MOUSELEAVE:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseLeave})
		}
		return 0

	case WM_LBUTTONDOWN, WM_LBUTTONDBLCLK:
		SetCapture(hwnd)
		SetFocus(hwnd)
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseDown, Button: 0, X: float32(loword(lp)) / w.scale, Y: float32(hiword(lp)) / w.scale})
		}
		return 0

	case WM_LBUTTONUP:
		ReleaseCapture()
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseUp, Button: 0, X: float32(loword(lp)) / w.scale, Y: float32(hiword(lp)) / w.scale})
		}
		return 0

	case WM_RBUTTONDOWN:
		SetCapture(hwnd)
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseDown, Button: 1, X: float32(loword(lp)) / w.scale, Y: float32(hiword(lp)) / w.scale})
		}
		return 0

	case WM_RBUTTONUP:
		ReleaseCapture()
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvMouseUp, Button: 1, X: float32(loword(lp)) / w.scale, Y: float32(hiword(lp)) / w.scale})
		}
		return 0

	case WM_MOUSEWHEEL:
		delta := int16((wp >> 16) & 0xFFFF)
		if w.onEvent != nil {
			// 注意：WM_MOUSEWHEEL 的 lParam 是「屏幕坐标」（物理像素），不是客户区坐标。
			// 旧实现直接把它当客户区坐标用，窗口一偏离屏幕原点命中测试就完全跑偏，
			// 滚轮永远打不到滚动容器（表现为「有滚动条但滚轮没反应」）。
			pt := POINT{X: int32(int16(loword(lp))), Y: int32(int16(hiword(lp)))}
			if ScreenToClient != nil {
				ScreenToClient(w.Hwnd, &pt)
			}
			w.onEvent(Event{Type: EvMouseWheel, DY: float32(delta) / 120.0, X: float32(pt.X) / w.scale, Y: float32(pt.Y) / w.scale})
		}
		return 0

	case WM_KEYDOWN, WM_SYSKEYDOWN:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvKeyDown, Key: uint32(wp)})
		}
		if msg == WM_SYSKEYDOWN {
			return DefWindowProcW(hwnd, msg, wp, lp)
		}
		return 0

	case WM_KEYUP, WM_SYSKEYUP:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvKeyUp, Key: uint32(wp)})
		}
		if msg == WM_SYSKEYUP {
			return DefWindowProcW(hwnd, msg, wp, lp)
		}
		return 0

	case WM_CHAR:
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvChar, Char: rune(wp)})
		}
		return 0

	case WM_IME_STARTCOMPOSITION:
		w.compText, w.compCaret = "", 0
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvCompositionStart})
		}
		return 0

	case WM_IME_COMPOSITION:
		w.handleIME(uintptr(hwnd), lp)
		return 0

	case WM_IME_ENDCOMPOSITION:
		w.compText, w.compCaret = "", 0
		if w.onEvent != nil {
			w.onEvent(Event{Type: EvCompositionEnd})
		}
		return 0
	}

	return DefWindowProcW(hwnd, msg, wp, lp)
}

// SetCornerRadius 设置窗口圆角半径（逻辑像素；<=0 取消圆角恢复直角）。
// Win11 走 DWM 原生圆角（边缘平滑抗锯齿），DWM 不可用时回退 SetWindowRgn 圆角区域。
// 全屏时自动取消，退出全屏后恢复。
func (w *Window) SetCornerRadius(radius float32) {
	if w == nil {
		return
	}
	w.cornerRadius = radius
	w.applyCorner()
}

// CornerRadius 返回当前圆角半径（逻辑像素）。
func (w *Window) CornerRadius() float32 {
	if w == nil {
		return 0
	}
	return w.cornerRadius
}

// applyCorner 按当前半径与状态应用/取消圆角。
func (w *Window) applyCorner() {
	if w == nil || w.Hwnd == 0 {
		return
	}
	clearRgn := func() {
		if w.roundedRgn {
			SetWindowRgn(w.Hwnd, 0, 1)
			w.roundedRgn = false
		}
	}
	clearDWM := func() {
		if w.roundedDWM {
			w.setDwmCorner(DWMWCP_DONOTROUND)
			w.roundedDWM = false
		}
	}
	if w.cornerRadius <= 0 || w.fullscreen {
		clearRgn()
		clearDWM()
		return
	}
	if w.setDwmCorner(DWMWCP_ROUND) {
		w.roundedDWM = true
		clearRgn()
		dbg("corner: DWM 圆角 radius=%.1f", w.cornerRadius)
		return
	}
	w.setRegionCorner()
	dbg("corner: 区域圆角 radius=%.1f rgn=%v", w.cornerRadius, w.roundedRgn)
}

// setDwmCorner 调用 DwmSetWindowAttribute 设置圆角偏好，成功返回 true（Windows 11 起支持）。
func (w *Window) setDwmCorner(pref int32) bool {
	if DwmSetWindowAttribute == nil || w.Hwnd == 0 {
		return false
	}
	v := pref
	return DwmSetWindowAttribute(w.Hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, unsafe.Pointer(&v), 4) == 0
}

// setRegionCorner 用 CreateRoundRectRgn + SetWindowRgn 做圆角（Win10 及 DWM 失效时的兜底）。
func (w *Window) setRegionCorner() {
	if SetWindowRgn == nil || CreateRoundRectRgn == nil {
		return
	}
	var r RECT
	GetWindowRect(w.Hwnd, &r)
	cw, ch := r.Right-r.Left, r.Bottom-r.Top
	if cw <= 0 || ch <= 0 {
		return
	}
	rad := int32(w.cornerRadius*w.scale + 0.5)
	if rad < 1 {
		rad = 1
	}
	if rad*2 > cw {
		rad = cw / 2
	}
	if rad*2 > ch {
		rad = ch / 2
	}
	hrgn := CreateRoundRectRgn(0, 0, cw+1, ch+1, rad*2, rad*2)
	if hrgn == 0 {
		return
	}
	// SetWindowRgn 成功后区域归系统所有，不能再 DeleteObject；失败则释放
	if SetWindowRgn(w.Hwnd, hrgn, 1) == 0 {
		DeleteObject(hrgn)
		return
	}
	w.roundedRgn = true
}

// reapplyCorner 尺寸/DPI 变化后重建圆角区域。
func (w *Window) reapplyCorner() {
	if w != nil && w.cornerRadius > 0 {
		w.applyCorner()
	}
}

// hideSystemBorder 去掉 DWM 绘制的 1px 系统边框（Win11 上是四周与强调色/系统主题相关的细线）。
// Win11 用 DWMWA_BORDER_COLOR = DWMWA_COLOR_NONE（带标题栏的窗口同样有效）；
// 旧版本回退关闭非客户区渲染——该回退会把标题栏一起去掉，所以只在无标题栏窗口上做。
func (w *Window) hideSystemBorder() {
	if w == nil || w.Hwnd == 0 || DwmSetWindowAttribute == nil {
		return
	}
	none := uint32(DWMWA_COLOR_NONE)
	if DwmSetWindowAttribute(w.Hwnd, DWMWA_BORDER_COLOR, unsafe.Pointer(&none), 4) == 0 {
		// 激活时会反复调用，只在首次打日志，避免刷屏
		if !w.borderQuiet {
			dbg("border: DWM 边框颜色已置为透明")
			w.borderQuiet = true
		}
		return
	}
	if w.cfg.Decorated {
		return
	}
	// Win10 及更早：关闭 DWM 非客户区渲染，避免残留白边
	off := int32(DWMNCRP_DISABLED)
	DwmSetWindowAttribute(w.Hwnd, DWMWA_NCRENDERING_POLICY, unsafe.Pointer(&off), 4)
	dbg("border: DWM 非客户区渲染已关闭")
}

// SleepFrame 让出 CPU（vsync 已由 SwapBuffers 控制时调用）。
func SleepFrame(d time.Duration) {
	if d > 0 {
		time.Sleep(d)
	}
}
