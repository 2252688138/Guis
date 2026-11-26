//go:build !windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import "time"

// 非 Windows 平台占位实现：签名与 Windows 版完全一致，全部返回 ErrUnsupported，
// 保证 ui 等上层包跨平台编译不破，也让调用方能用 errors.Is 统一判断。

// Process 已打开的进程句柄（占位）。
type Process struct {
	pid int
}

// Open 打开进程（当前平台不支持）。
func Open(pid int) (*Process, error) { return nil, ErrUnsupported }

// OpenReadOnly 只读打开进程（当前平台不支持）。
func OpenReadOnly(pid int) (*Process, error) { return nil, ErrUnsupported }

// PID 进程 ID。
func (p *Process) PID() int { return p.pid }

// ReadOnly 是否为只读句柄。
func (p *Process) ReadOnly() bool { return false }

// Close 关闭句柄。
func (p *Process) Close() error { return nil }

// Alive 目标进程是否仍在运行。
func (p *Process) Alive() bool { return false }

// ReadInto 读取内存到 buf。
func (p *Process) ReadInto(addr uintptr, buf []byte) (int, error) { return 0, ErrUnsupported }

// Read 读取 n 字节。
func (p *Process) Read(addr uintptr, n int) ([]byte, error) { return nil, ErrUnsupported }

// Write 写入内存。
func (p *Process) Write(addr uintptr, b []byte) error { return ErrUnsupported }

// ReadU32 读 32 位无符号整数。
func (p *Process) ReadU32(addr uintptr) (uint32, error) { return 0, ErrUnsupported }

// WriteU32 写 32 位无符号整数。
func (p *Process) WriteU32(addr uintptr, v uint32) error { return ErrUnsupported }

// ReadI32 读 32 位有符号整数。
func (p *Process) ReadI32(addr uintptr) (int32, error) { return 0, ErrUnsupported }

// WriteI32 写 32 位有符号整数。
func (p *Process) WriteI32(addr uintptr, v int32) error { return ErrUnsupported }

// ReadU64 读 64 位无符号整数。
func (p *Process) ReadU64(addr uintptr) (uint64, error) { return 0, ErrUnsupported }

// WriteU64 写 64 位无符号整数。
func (p *Process) WriteU64(addr uintptr, v uint64) error { return ErrUnsupported }

// ReadF32 读单精度浮点。
func (p *Process) ReadF32(addr uintptr) (float32, error) { return 0, ErrUnsupported }

// WriteF32 写单精度浮点。
func (p *Process) WriteF32(addr uintptr, v float32) error { return ErrUnsupported }

// ReadF64 读双精度浮点。
func (p *Process) ReadF64(addr uintptr) (float64, error) { return 0, ErrUnsupported }

// WriteF64 写双精度浮点。
func (p *Process) WriteF64(addr uintptr, v float64) error { return ErrUnsupported }

// ReadCString 读 0 结尾字符串。
func (p *Process) ReadCString(addr uintptr, max int) (string, error) { return "", ErrUnsupported }

// WriteCString 写 0 结尾字符串。
func (p *Process) WriteCString(addr uintptr, s string) error { return ErrUnsupported }

// ReadUTF16 读 0 结尾 UTF-16 字符串。
func (p *Process) ReadUTF16(addr uintptr, maxChars int) (string, error) { return "", ErrUnsupported }

// WriteUTF16 写 0 结尾 UTF-16 字符串。
func (p *Process) WriteUTF16(addr uintptr, s string) error { return ErrUnsupported }

// Modules 枚举模块。
func (p *Process) Modules() ([]Module, error) { return nil, ErrUnsupported }

// ModuleBase 查模块基址。
func (p *Process) ModuleBase(name string) (uintptr, error) { return 0, ErrUnsupported }

// Regions 枚举内存区域。
func (p *Process) Regions() ([]Region, error) { return nil, ErrUnsupported }

// SetProtect 修改保护属性。
func (p *Process) SetProtect(addr, size uintptr, protect uint32) (uint32, error) {
	return 0, ErrUnsupported
}

// FindProcesses 查找进程（当前平台不支持）。
func FindProcesses(nameLike string) ([]ProcInfo, error) { return nil, ErrUnsupported }

// Scanner 值扫描器（占位）。
type Scanner struct {
	opt ScanOptions
}

// NewScanner 创建扫描器。
func NewScanner(p *Process, opt ScanOptions) *Scanner { return &Scanner{opt: opt.withDefaults()} }

// First 首次扫描。
func (s *Scanner) First(v Value) error { return ErrUnsupported }

// Next 过滤上一次扫描结果。
func (s *Scanner) Next(v Value, f Filter) error { return ErrUnsupported }

// Results 当前结果地址列表。
func (s *Scanner) Results() []uintptr { return nil }

// Len 当前结果数量。
func (s *Scanner) Len() int { return 0 }

// Truncated 结果是否因超上限被截断。
func (s *Scanner) Truncated() bool { return false }

// Reset 清空结果。
func (s *Scanner) Reset() {}

// FindPointerChains 搜索指针链（当前平台不支持）。
func FindPointerChains(p *Process, target uintptr, opt ChainOptions) ([][]uintptr, error) {
	return nil, ErrUnsupported
}

// Freezer 内存冻结器（占位）。
type Freezer struct{}

// NewFreezer 创建冻结器。
func NewFreezer(p *Process, interval time.Duration) *Freezer { return &Freezer{} }

// Add 添加冻结项。
func (f *Freezer) Add(addr uintptr, data []byte) error { return ErrUnsupported }

// Remove 移除冻结项。
func (f *Freezer) Remove(addr uintptr) {}

// List 列出冻结项。
func (f *Freezer) List() []FreezeEntry { return nil }

// Close 停止冻结并释放资源。
func (f *Freezer) Close() error { return nil }
