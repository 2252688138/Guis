//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Windows 原生实现：kernel32 延迟解析（syscall.NewLazyDLL），缺符号只影响对应能力，
// 不在包初始化阶段 panic；句柄访问权限取最小集，OpenProcess 失败不重试、不提权。

var (
	kernel32DLL = syscall.NewLazyDLL("kernel32.dll")

	procOpenProcess                = kernel32DLL.NewProc("OpenProcess")
	procCloseHandle                = kernel32DLL.NewProc("CloseHandle")
	procReadProcessMemory          = kernel32DLL.NewProc("ReadProcessMemory")
	procWriteProcessMemory         = kernel32DLL.NewProc("WriteProcessMemory")
	procVirtualQueryEx             = kernel32DLL.NewProc("VirtualQueryEx")
	procVirtualProtectEx           = kernel32DLL.NewProc("VirtualProtectEx")
	procCreateToolhelp32Snapshot   = kernel32DLL.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = kernel32DLL.NewProc("Process32FirstW")
	procProcess32NextW             = kernel32DLL.NewProc("Process32NextW")
	procModule32FirstW             = kernel32DLL.NewProc("Module32FirstW")
	procModule32NextW              = kernel32DLL.NewProc("Module32NextW")
	procGetExitCodeProcess         = kernel32DLL.NewProc("GetExitCodeProcess")
	procQueryFullProcessImageNameW = kernel32DLL.NewProc("QueryFullProcessImageNameW")
)

// 访问权限与快照常量。
const (
	processVMOperation             = 0x0008
	processVMRead                  = 0x0010
	processVMWrite                 = 0x0020
	processQueryInformation        = 0x0400
	processQueryLimitedInformation = 0x1000

	th32csSnapProcess  = 0x00000002
	th32csSnapModule   = 0x00000008
	th32csSnapModule32 = 0x00000010

	stillActive = 259
	maxPathLen  = 260
)

// Win32 错误码（syscall 包未导出这两个常量）。
const (
	errnoInvalidParameter = syscall.Errno(87)
	errnoPartialCopy      = syscall.Errno(299)
)

// Process 已打开的进程句柄。
type Process struct {
	mu       sync.Mutex
	pid      int
	handle   uintptr
	closed   bool
	readOnly bool
}

// Open 以读写权限打开进程。
func Open(pid int) (*Process, error) { return openProcess(pid, false) }

// OpenReadOnly 以只读权限打开进程（写操作会失败，但可读受保护进程的内存）。
func OpenReadOnly(pid int) (*Process, error) { return openProcess(pid, true) }

func openProcess(pid int, readOnly bool) (*Process, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("procmem: 非法进程 ID %d", pid)
	}
	if err := procOpenProcess.Find(); err != nil {
		return nil, ErrUnsupported
	}
	access := uintptr(processVMRead | processQueryInformation)
	if !readOnly {
		access |= processVMWrite | processVMOperation
	}
	h, _, e := procOpenProcess.Call(access, 0, uintptr(pid))
	if h == 0 {
		return nil, wrapWinErr("OpenProcess", e)
	}
	return &Process{pid: pid, handle: h, readOnly: readOnly}, nil
}

// PID 进程 ID。
func (p *Process) PID() int {
	if p == nil {
		return 0
	}
	return p.pid
}

// ReadOnly 是否为只读句柄。
func (p *Process) ReadOnly() bool { return p != nil && p.readOnly }

// Close 关闭句柄（幂等）。
func (p *Process) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.handle == 0 {
		p.closed = true
		return nil
	}
	h := p.handle
	p.handle = 0
	p.closed = true
	r, _, e := procCloseHandle.Call(h)
	if r == 0 {
		return wrapWinErr("CloseHandle", e)
	}
	return nil
}

// Alive 目标进程是否仍在运行。
func (p *Process) Alive() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h, err := p.handleLocked()
	if err != nil {
		return false
	}
	if err := procGetExitCodeProcess.Find(); err != nil {
		return false
	}
	var code uint32
	r, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code)))
	return r != 0 && code == stillActive
}

// handleLocked 取可用句柄，调用方需持有 p.mu。
func (p *Process) handleLocked() (uintptr, error) {
	if p.closed || p.handle == 0 {
		return 0, ErrClosed
	}
	return p.handle, nil
}

// ReadInto 把内存读进 buf，返回实际读取字节数。
func (p *Process) ReadInto(addr uintptr, buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	if p == nil {
		return 0, ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h, err := p.handleLocked()
	if err != nil {
		return 0, err
	}
	var n uintptr
	r, _, e := procReadProcessMemory.Call(h, addr,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&n)))
	runtime.KeepAlive(buf)
	if r == 0 {
		return int(n), wrapWinErr("ReadProcessMemory", e)
	}
	return int(n), nil
}

// Read 读取 n 字节；出错时返回已读到的部分（可能短于 n）。
func (p *Process) Read(addr uintptr, n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("procmem: 非法读取长度 %d", n)
	}
	buf := make([]byte, n)
	if n == 0 {
		return buf, nil
	}
	if _, err := p.ReadInto(addr, buf); err != nil {
		return buf, err
	}
	return buf, nil
}

// Write 向目标内存写入 b（只读页需先 SetProtect）。
func (p *Process) Write(addr uintptr, b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if p == nil {
		return ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h, err := p.handleLocked()
	if err != nil {
		return err
	}
	var n uintptr
	r, _, e := procWriteProcessMemory.Call(h, addr,
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&n)))
	runtime.KeepAlive(b)
	if r == 0 {
		return wrapWinErr("WriteProcessMemory", e)
	}
	return nil
}

// ---- 定长数值读写 ----

// ReadU32 读 32 位无符号整数。
func (p *Process) ReadU32(addr uintptr) (uint32, error) {
	b, err := p.Read(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// WriteU32 写 32 位无符号整数。
func (p *Process) WriteU32(addr uintptr, v uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return p.Write(addr, b)
}

// ReadI32 读 32 位有符号整数。
func (p *Process) ReadI32(addr uintptr) (int32, error) {
	v, err := p.ReadU32(addr)
	return int32(v), err
}

// WriteI32 写 32 位有符号整数。
func (p *Process) WriteI32(addr uintptr, v int32) error { return p.WriteU32(addr, uint32(v)) }

// ReadU64 读 64 位无符号整数。
func (p *Process) ReadU64(addr uintptr) (uint64, error) {
	b, err := p.Read(addr, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

// WriteU64 写 64 位无符号整数。
func (p *Process) WriteU64(addr uintptr, v uint64) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return p.Write(addr, b)
}

// ReadF32 读单精度浮点。
func (p *Process) ReadF32(addr uintptr) (float32, error) {
	v, err := p.ReadU32(addr)
	return math.Float32frombits(v), err
}

// WriteF32 写单精度浮点。
func (p *Process) WriteF32(addr uintptr, v float32) error {
	return p.WriteU32(addr, math.Float32bits(v))
}

// ReadF64 读双精度浮点。
func (p *Process) ReadF64(addr uintptr) (float64, error) {
	v, err := p.ReadU64(addr)
	return math.Float64frombits(v), err
}

// WriteF64 写双精度浮点。
func (p *Process) WriteF64(addr uintptr, v float64) error {
	return p.WriteU64(addr, math.Float64bits(v))
}

// ---- 字符串读写 ----

// ReadCString 读以 0 结尾的 ANSI/UTF-8 字符串，最多读 max 字节。
func (p *Process) ReadCString(addr uintptr, max int) (string, error) {
	if max <= 0 {
		max = 256
	}
	const chunk = 64
	out := make([]byte, 0, max)
	for len(out) < max {
		n := chunk
		if rem := max - len(out); rem < n {
			n = rem
		}
		b, err := p.Read(addr+uintptr(len(out)), n)
		if i := bytes.IndexByte(b, 0); i >= 0 {
			return string(append(out, b[:i]...)), nil
		}
		if err != nil {
			return string(out), err
		}
		out = append(out, b...)
	}
	return string(out), nil
}

// WriteCString 写入以 0 结尾的 UTF-8 字符串（含结尾 0）。
func (p *Process) WriteCString(addr uintptr, s string) error {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return p.Write(addr, b)
}

// ReadUTF16 读以 0 结尾的 UTF-16LE 字符串，最多读 maxChars 个字符。
func (p *Process) ReadUTF16(addr uintptr, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 128
	}
	b, err := p.Read(addr, maxChars*2)
	units := make([]uint16, 0, maxChars)
	for i := 0; i+1 < len(b); i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			return string(utf16.Decode(units)), nil
		}
		units = append(units, u)
	}
	if err != nil {
		return string(utf16.Decode(units)), err
	}
	return string(utf16.Decode(units)), nil
}

// WriteUTF16 写入以 0 结尾的 UTF-16LE 字符串。
func (p *Process) WriteUTF16(addr uintptr, s string) error {
	u := append(utf16.Encode([]rune(s)), 0)
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return p.Write(addr, b)
}

// ---- 模块 ----

// processEntry32W PROCESSENTRY32W。
type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [maxPathLen]uint16
}

// moduleEntry32W MODULEENTRY32W。
type moduleEntry32W struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	ModuleHandle uintptr
	Module       [256]uint16
	ExePath      [maxPathLen]uint16
}

// memoryBasicInformation MEMORY_BASIC_INFORMATION（Go 自动按指针宽度对齐）。
type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

// Modules 枚举目标进程已加载模块。
func (p *Process) Modules() ([]Module, error) {
	if p == nil {
		return nil, ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.handleLocked(); err != nil {
		return nil, err
	}
	if err := procModule32FirstW.Find(); err != nil {
		return nil, ErrUnsupported
	}
	snap, _, e := procCreateToolhelp32Snapshot.Call(th32csSnapModule|th32csSnapModule32, uintptr(p.pid))
	if snap == 0 || snap == ^uintptr(0) {
		return nil, wrapWinErr("CreateToolhelp32Snapshot", e)
	}
	defer func() { procCloseHandle.Call(snap) }()

	var me moduleEntry32W
	me.Size = uint32(unsafe.Sizeof(me))
	r, _, e := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	var out []Module
	for r != 0 {
		out = append(out, Module{
			Name: syscall.UTF16ToString(me.Module[:]),
			Path: syscall.UTF16ToString(me.ExePath[:]),
			Base: me.ModBaseAddr,
			Size: uintptr(me.ModBaseSize),
		})
		r, _, e = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
	}
	if len(out) == 0 {
		return nil, wrapWinErr("Module32First", e)
	}
	return out, nil
}

// ModuleBase 按名称查模块基址（不区分大小写，可省略扩展名）。
func (p *Process) ModuleBase(name string) (uintptr, error) {
	mods, err := p.Modules()
	if err != nil {
		return 0, err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return mods[0].Base, nil
	}
	for _, m := range mods {
		if strings.ToLower(m.Name) == want {
			return m.Base, nil
		}
	}
	trim := func(s string) string {
		if i := strings.LastIndexByte(s, '.'); i > 0 {
			return s[:i]
		}
		return s
	}
	for _, m := range mods {
		if trim(strings.ToLower(m.Name)) == trim(want) {
			return m.Base, nil
		}
	}
	return 0, fmt.Errorf("%w: 模块 %q", ErrNotFound, name)
}

// ---- 内存区域 ----

// Regions 枚举目标进程全部内存区域（VirtualQueryEx 逐段）。
func (p *Process) Regions() ([]Region, error) {
	if p == nil {
		return nil, ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h, err := p.handleLocked()
	if err != nil {
		return nil, err
	}
	if err := procVirtualQueryEx.Find(); err != nil {
		return nil, ErrUnsupported
	}
	var out []Region
	var addr uintptr
	for {
		var mbi memoryBasicInformation
		r, _, e := procVirtualQueryEx.Call(h, addr,
			uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
		if r == 0 {
			if len(out) == 0 {
				return nil, wrapWinErr("VirtualQueryEx", e)
			}
			break
		}
		out = append(out, Region{
			Base:    mbi.BaseAddress,
			Size:    mbi.RegionSize,
			State:   mbi.State,
			Protect: mbi.Protect,
			Type:    mbi.Type,
		})
		next := mbi.BaseAddress + mbi.RegionSize
		if next <= addr {
			break
		}
		addr = next
	}
	return out, nil
}

// SetProtect 修改区域保护属性，返回旧属性。
func (p *Process) SetProtect(addr, size uintptr, protect uint32) (uint32, error) {
	if p == nil {
		return 0, ErrClosed
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	h, err := p.handleLocked()
	if err != nil {
		return 0, err
	}
	if err := procVirtualProtectEx.Find(); err != nil {
		return 0, ErrUnsupported
	}
	var old uint32
	r, _, e := procVirtualProtectEx.Call(h, addr, size, uintptr(protect), uintptr(unsafe.Pointer(&old)))
	if r == 0 {
		return 0, wrapWinErr("VirtualProtectEx", e)
	}
	return old, nil
}

// ---- 进程枚举 ----

// FindProcesses 按名称子串查找进程（nameLike 为空表示全部），结果按 PID 升序。
func FindProcesses(nameLike string) ([]ProcInfo, error) {
	if err := procProcess32FirstW.Find(); err != nil {
		return nil, ErrUnsupported
	}
	snap, _, e := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil, wrapWinErr("CreateToolhelp32Snapshot", e)
	}
	defer func() { procCloseHandle.Call(snap) }()

	needle := strings.ToLower(strings.TrimSpace(nameLike))
	var out []ProcInfo
	var pe processEntry32W
	pe.Size = uint32(unsafe.Sizeof(pe))
	r, _, e := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for r != 0 {
		name := syscall.UTF16ToString(pe.ExeFile[:])
		if needle == "" || strings.Contains(strings.ToLower(name), needle) {
			pid := pe.ProcessID
			out = append(out, ProcInfo{PID: int(pid), Name: name, Path: processPath(pid)})
		}
		r, _, e = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	if len(out) == 0 && needle != "" {
		return nil, nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

// processPath 取进程可执行文件路径（取不到返回空串，不报错）。
func processPath(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer func() { procCloseHandle.Call(h) }()
	if err := procQueryFullProcessImageNameW.Find(); err != nil {
		return ""
	}
	buf := make([]uint16, maxPathLen)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 || int(size) > len(buf) {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}

// wrapWinErr 把 Win32 错误翻译成带中文提示的错误（保留 errors.Is 语义）。
func wrapWinErr(op string, e error) error {
	if errno, ok := e.(syscall.Errno); ok && errno != 0 {
		switch errno {
		case syscall.ERROR_ACCESS_DENIED:
			return fmt.Errorf("%w（%s）", ErrAccessDenied, op)
		case errnoInvalidParameter:
			return fmt.Errorf("procmem: %s 参数无效（地址或长度不合法）", op)
		case errnoPartialCopy:
			return fmt.Errorf("procmem: %s 只能部分读取（地址不可读或跨越区域边界）", op)
		}
		return fmt.Errorf("procmem: %s 失败: %v", op, errno)
	}
	return fmt.Errorf("procmem: %s 失败", op)
}
