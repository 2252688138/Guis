// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"errors"
	"fmt"
	"time"

	"guis/internal/procmem"
)

// 进程内存读写（Cheat Engine 类能力）的框架门面。
//
// 底层实现在 internal/procmem（Windows 原生，其他平台返回 ErrUnsupported），
// 这一层只做「类型映射 + 错误翻译」，不含平台分支，保证 ui 包跨平台编译一致。
//
// 最小示例：
//
//	p, err := ui.OpenProcessMem(pid)
//	if err != nil { return err }
//	defer p.Close()
//	base, _ := p.ModuleBase("game.exe")
//	hp, _ := p.ReadU32(base + 0x1A2B3C)
//	_ = p.WriteU32(base+0x1A2B3C, hp+100)
//
// 安全边界：只做打开 / 读写 / 还原，不含 DLL 注入、远程线程与任何反作弊绕过；
// OpenProcess 失败直接返回错误，不尝试提权。

// ProcInfo 进程信息。
type ProcInfo struct {
	// PID 进程 ID。
	PID int
	// Name 可执行文件名（不含路径）。
	Name string
	// Path 可执行文件完整路径，取不到时为空。
	Path string
}

// FindProcesses 按名称子串查找进程（nameLike 为空表示全部），按 PID 升序。
func FindProcesses(nameLike string) ([]ProcInfo, error) {
	list, err := procmem.FindProcesses(nameLike)
	if err != nil {
		return nil, memErr("查找进程", err)
	}
	out := make([]ProcInfo, 0, len(list))
	for _, p := range list {
		out = append(out, ProcInfo{PID: p.PID, Name: p.Name, Path: p.Path})
	}
	return out, nil
}

// ProcessMem 已打开的进程内存句柄，用完必须 Close。
type ProcessMem struct {
	p *procmem.Process
}

// OpenProcessMem 以读写权限打开进程内存。
func OpenProcessMem(pid int) (*ProcessMem, error) {
	p, err := procmem.Open(pid)
	if err != nil {
		return nil, memErr("打开进程", err)
	}
	return &ProcessMem{p: p}, nil
}

// OpenProcessMemReadOnly 以只读权限打开进程内存（不能写，但可读受保护进程）。
func OpenProcessMemReadOnly(pid int) (*ProcessMem, error) {
	p, err := procmem.OpenReadOnly(pid)
	if err != nil {
		return nil, memErr("打开进程", err)
	}
	return &ProcessMem{p: p}, nil
}

// PID 进程 ID。
func (m *ProcessMem) PID() int {
	if m == nil || m.p == nil {
		return 0
	}
	return m.p.PID()
}

// Alive 目标进程是否仍在运行。
func (m *ProcessMem) Alive() bool {
	if m == nil || m.p == nil {
		return false
	}
	return m.p.Alive()
}

// Close 关闭句柄（幂等）。
func (m *ProcessMem) Close() error {
	if m == nil || m.p == nil {
		return nil
	}
	return m.p.Close()
}

// Read 读取 n 字节（出错时返回已读到的部分）。
func (m *ProcessMem) Read(addr uintptr, n int) ([]byte, error) {
	if m == nil || m.p == nil {
		return nil, ErrMemClosed
	}
	b, err := m.p.Read(addr, n)
	return b, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// Write 写入字节（只读页需先 SetProtect）。
func (m *ProcessMem) Write(addr uintptr, b []byte) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.Write(addr, b))
}

// ReadU32 读 32 位无符号整数。
func (m *ProcessMem) ReadU32(addr uintptr) (uint32, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	v, err := m.p.ReadU32(addr)
	return v, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// WriteU32 写 32 位无符号整数。
func (m *ProcessMem) WriteU32(addr uintptr, v uint32) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.WriteU32(addr, v))
}

// ReadI32 读 32 位有符号整数。
func (m *ProcessMem) ReadI32(addr uintptr) (int32, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	v, err := m.p.ReadI32(addr)
	return v, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// WriteI32 写 32 位有符号整数。
func (m *ProcessMem) WriteI32(addr uintptr, v int32) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.WriteI32(addr, v))
}

// ReadU64 读 64 位无符号整数。
func (m *ProcessMem) ReadU64(addr uintptr) (uint64, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	v, err := m.p.ReadU64(addr)
	return v, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// WriteU64 写 64 位无符号整数。
func (m *ProcessMem) WriteU64(addr uintptr, v uint64) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.WriteU64(addr, v))
}

// ReadF32 读单精度浮点。
func (m *ProcessMem) ReadF32(addr uintptr) (float32, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	v, err := m.p.ReadF32(addr)
	return v, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// WriteF32 写单精度浮点。
func (m *ProcessMem) WriteF32(addr uintptr, v float32) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.WriteF32(addr, v))
}

// ReadF64 读双精度浮点。
func (m *ProcessMem) ReadF64(addr uintptr) (float64, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	v, err := m.p.ReadF64(addr)
	return v, memErr(fmt.Sprintf("读取 0x%X", addr), err)
}

// WriteF64 写双精度浮点。
func (m *ProcessMem) WriteF64(addr uintptr, v float64) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入 0x%X", addr), m.p.WriteF64(addr, v))
}

// ReadCString 读 0 结尾的 UTF-8 字符串，最多读 max 字节。
func (m *ProcessMem) ReadCString(addr uintptr, max int) (string, error) {
	if m == nil || m.p == nil {
		return "", ErrMemClosed
	}
	s, err := m.p.ReadCString(addr, max)
	return s, memErr(fmt.Sprintf("读取字符串 0x%X", addr), err)
}

// WriteCString 写 0 结尾的 UTF-8 字符串。
func (m *ProcessMem) WriteCString(addr uintptr, s string) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入字符串 0x%X", addr), m.p.WriteCString(addr, s))
}

// ReadUTF16 读 0 结尾的 UTF-16LE 字符串，最多 maxChars 个字符。
func (m *ProcessMem) ReadUTF16(addr uintptr, maxChars int) (string, error) {
	if m == nil || m.p == nil {
		return "", ErrMemClosed
	}
	s, err := m.p.ReadUTF16(addr, maxChars)
	return s, memErr(fmt.Sprintf("读取字符串 0x%X", addr), err)
}

// WriteUTF16 写 0 结尾的 UTF-16LE 字符串。
func (m *ProcessMem) WriteUTF16(addr uintptr, s string) error {
	if m == nil || m.p == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("写入字符串 0x%X", addr), m.p.WriteUTF16(addr, s))
}

// MemModule 已加载模块信息。
type MemModule struct {
	// Name 模块名，如 kernel32.dll。
	Name string
	// Path 模块完整路径。
	Path string
	// Base 模块基址。
	Base uintptr
	// Size 模块映像大小（字节）。
	Size uintptr
}

// Modules 枚举目标进程已加载模块。
func (m *ProcessMem) Modules() ([]MemModule, error) {
	if m == nil || m.p == nil {
		return nil, ErrMemClosed
	}
	mods, err := m.p.Modules()
	if err != nil {
		return nil, memErr("枚举模块", err)
	}
	out := make([]MemModule, 0, len(mods))
	for _, mod := range mods {
		out = append(out, MemModule{Name: mod.Name, Path: mod.Path, Base: mod.Base, Size: mod.Size})
	}
	return out, nil
}

// ModuleBase 按名称查模块基址（不区分大小写，可省略扩展名）。
func (m *ProcessMem) ModuleBase(name string) (uintptr, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	base, err := m.p.ModuleBase(name)
	return base, memErr("查模块基址", err)
}

// MemRegion 内存区域信息。
type MemRegion struct {
	// Base 区域起始地址。
	Base uintptr
	// Size 区域大小（字节）。
	Size uintptr
	// State 提交状态（MemCommit / MemReserve / MemFree）。
	State uint32
	// Protect 保护属性（含 PageGuard 等修饰位）。
	Protect uint32
	// Type 区域类型（MemImage / MemMapped / MemPrivate）。
	Type uint32
}

// Readable 该区域是否可读。
func (r MemRegion) Readable() bool {
	return procmem.Region{Base: r.Base, Size: r.Size, State: r.State, Protect: r.Protect, Type: r.Type}.Readable()
}

// Writable 该区域是否可写。
func (r MemRegion) Writable() bool {
	return procmem.Region{Base: r.Base, Size: r.Size, State: r.State, Protect: r.Protect, Type: r.Type}.Writable()
}

// Regions 枚举目标进程全部内存区域。
func (m *ProcessMem) Regions() ([]MemRegion, error) {
	if m == nil || m.p == nil {
		return nil, ErrMemClosed
	}
	regs, err := m.p.Regions()
	if err != nil {
		return nil, memErr("枚举内存区域", err)
	}
	out := make([]MemRegion, 0, len(regs))
	for _, r := range regs {
		out = append(out, MemRegion{Base: r.Base, Size: r.Size, State: r.State, Protect: r.Protect, Type: r.Type})
	}
	return out, nil
}

// SetProtect 修改区域保护属性，返回旧属性（配合 Write 改只读页）。
func (m *ProcessMem) SetProtect(addr, size uintptr, protect uint32) (uint32, error) {
	if m == nil || m.p == nil {
		return 0, ErrMemClosed
	}
	old, err := m.p.SetProtect(addr, size, protect)
	return old, memErr("修改内存保护", err)
}

// ---- 值扫描 ----

// MemValueKind 扫描值类型。
type MemValueKind = procmem.ValueKind

// 支持的值类型。
const (
	// MemKindInt32 32 位有符号整数。
	MemKindInt32 = procmem.KindInt32
	// MemKindUint32 32 位无符号整数。
	MemKindUint32 = procmem.KindUint32
	// MemKindInt64 64 位有符号整数。
	MemKindInt64 = procmem.KindInt64
	// MemKindFloat32 单精度浮点。
	MemKindFloat32 = procmem.KindFloat32
	// MemKindFloat64 双精度浮点。
	MemKindFloat64 = procmem.KindFloat64
	// MemKindBytes 定长字节串。
	MemKindBytes = procmem.KindBytes
	// MemKindString UTF-8 字符串。
	MemKindString = procmem.KindString
	// MemKindUTF16String UTF-16LE 字符串。
	MemKindUTF16String = procmem.KindUTF16String
)

// MemFilter 扫描过滤条件。
type MemFilter = procmem.Filter

// 支持的过滤条件。
const (
	// MemFilterEqual 等于给定值。
	MemFilterEqual = procmem.FilterEqual
	// MemFilterNotEqual 不等于给定值。
	MemFilterNotEqual = procmem.FilterNotEqual
	// MemFilterChanged 与上次扫描相比发生变化。
	MemFilterChanged = procmem.FilterChanged
	// MemFilterUnchanged 与上次扫描相比未变化。
	MemFilterUnchanged = procmem.FilterUnchanged
	// MemFilterIncreased 与上次扫描相比变大（仅数值类型）。
	MemFilterIncreased = procmem.FilterIncreased
	// MemFilterDecreased 与上次扫描相比变小（仅数值类型）。
	MemFilterDecreased = procmem.FilterDecreased
	// MemFilterGreaterThan 大于给定值（仅数值类型）。
	MemFilterGreaterThan = procmem.FilterGreaterThan
	// MemFilterLessThan 小于给定值（仅数值类型）。
	MemFilterLessThan = procmem.FilterLessThan
)

// MemValue 扫描值（类型 + 原始字节）。
type MemValue = procmem.Value

// MemInt32 构造 int32 扫描值。
func MemInt32(v int32) MemValue { return procmem.Int32(v) }

// MemUint32 构造 uint32 扫描值。
func MemUint32(v uint32) MemValue { return procmem.Uint32(v) }

// MemInt64 构造 int64 扫描值。
func MemInt64(v int64) MemValue { return procmem.Int64(v) }

// MemFloat32 构造 float32 扫描值。
func MemFloat32(v float32) MemValue { return procmem.Float32(v) }

// MemFloat64 构造 float64 扫描值。
func MemFloat64(v float64) MemValue { return procmem.Float64(v) }

// MemBytes 构造定长字节扫描值。
func MemBytes(b []byte) MemValue { return procmem.Bytes(b) }

// MemString 构造 UTF-8 字符串扫描值。
func MemString(s string) MemValue { return procmem.String(s) }

// MemUTF16String 构造 UTF-16LE 字符串扫描值。
func MemUTF16String(s string) MemValue { return procmem.UTF16String(s) }

// MemScanOptions 扫描参数（零值即默认：上限 100 万、块 4MB、对齐 1）。
type MemScanOptions struct {
	// MaxResults 结果上限。
	MaxResults int
	// ChunkSize 每次读取块大小。
	ChunkSize int
	// Align 地址对齐（1/2/4/8）。
	Align int
	// WritableOnly 只扫可写区域。
	WritableOnly bool
}

// MemScanner 值扫描器。
type MemScanner struct {
	s *procmem.Scanner
}

// NewMemScanner 创建扫描器。
func NewMemScanner(m *ProcessMem, opt MemScanOptions) *MemScanner {
	var p *procmem.Process
	if m != nil {
		p = m.p
	}
	return &MemScanner{s: procmem.NewScanner(p, procmem.ScanOptions{
		MaxResults:   opt.MaxResults,
		ChunkSize:    opt.ChunkSize,
		Align:        opt.Align,
		WritableOnly: opt.WritableOnly,
	})}
}

// First 首次扫描。
func (s *MemScanner) First(v MemValue) error {
	if s == nil || s.s == nil {
		return ErrMemClosed
	}
	return memErr("首次扫描", s.s.First(v))
}

// Next 用过滤条件收敛上一次结果。
func (s *MemScanner) Next(v MemValue, f MemFilter) error {
	if s == nil || s.s == nil {
		return ErrMemClosed
	}
	return memErr("过滤扫描结果", s.s.Next(v, f))
}

// Results 当前结果地址列表。
func (s *MemScanner) Results() []uintptr {
	if s == nil || s.s == nil {
		return nil
	}
	return s.s.Results()
}

// Len 当前结果数量。
func (s *MemScanner) Len() int {
	if s == nil || s.s == nil {
		return 0
	}
	return s.s.Len()
}

// Truncated 结果是否因超上限被截断。
func (s *MemScanner) Truncated() bool {
	if s == nil || s.s == nil {
		return false
	}
	return s.s.Truncated()
}

// Reset 清空结果。
func (s *MemScanner) Reset() {
	if s == nil || s.s == nil {
		return
	}
	s.s.Reset()
}

// MemChainOptions 指针链搜索参数（零值即默认：3 级、偏移 0x1000、最多 64 条）。
type MemChainOptions struct {
	// MaxLevel 最大层级。
	MaxLevel int
	// MaxOffset 单级最大偏移。
	MaxOffset uintptr
	// MaxResults 最多返回多少条链。
	MaxResults int
}

// FindMemPointerChains 搜索能解析到 target 的指针链。
//
// 每条链形如 {slot, off1, off2, ...}：读 slot 处的指针 → +off1 → 读指针 → +off2 → … → target。
func FindMemPointerChains(m *ProcessMem, target uintptr, opt MemChainOptions) ([][]uintptr, error) {
	if m == nil || m.p == nil {
		return nil, ErrMemClosed
	}
	chains, err := procmem.FindPointerChains(m.p, target, procmem.ChainOptions{
		MaxLevel:   opt.MaxLevel,
		MaxOffset:  opt.MaxOffset,
		MaxResults: opt.MaxResults,
	})
	if err != nil {
		return nil, memErr("搜索指针链", err)
	}
	return chains, nil
}

// MemFreezeEntry 一条冻结项。
type MemFreezeEntry struct {
	// Addr 目标地址。
	Addr uintptr
	// Data 保持的字节内容。
	Data []byte
	// Err 最近一次回写的错误。
	Err error
}

// MemFreezer 内存冻结器（定时回写）。
type MemFreezer struct {
	f *procmem.Freezer
}

// NewMemFreezer 创建冻结器，interval <= 0 时用 50ms。
func NewMemFreezer(m *ProcessMem, interval time.Duration) *MemFreezer {
	var p *procmem.Process
	if m != nil {
		p = m.p
	}
	return &MemFreezer{f: procmem.NewFreezer(p, interval)}
}

// Add 添加（或覆盖）冻结项并立即回写一次。
func (f *MemFreezer) Add(addr uintptr, data []byte) error {
	if f == nil || f.f == nil {
		return ErrMemClosed
	}
	return memErr(fmt.Sprintf("冻结 0x%X", addr), f.f.Add(addr, data))
}

// Remove 移除冻结项。
func (f *MemFreezer) Remove(addr uintptr) {
	if f == nil || f.f == nil {
		return
	}
	f.f.Remove(addr)
}

// List 当前冻结项。
func (f *MemFreezer) List() []MemFreezeEntry {
	if f == nil || f.f == nil {
		return nil
	}
	list := f.f.List()
	out := make([]MemFreezeEntry, 0, len(list))
	for _, e := range list {
		out = append(out, MemFreezeEntry{Addr: e.Addr, Data: e.Data, Err: e.Err})
	}
	return out
}

// Close 停止回写并等待后台协程退出（幂等）。
func (f *MemFreezer) Close() error {
	if f == nil || f.f == nil {
		return nil
	}
	return f.f.Close()
}

// ErrMemClosed 进程内存句柄已关闭或未打开。
var ErrMemClosed = errors.New("ui: 进程内存句柄未打开或已关闭")

// memErr 把底层错误翻译成带中文提示的错误，保持 errors.Is 语义。
func memErr(op string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, procmem.ErrUnsupported):
		return fmt.Errorf("%s: 当前平台不支持进程内存操作（仅 Windows 可用）", op)
	case errors.Is(err, procmem.ErrAccessDenied):
		return fmt.Errorf("%s: 拒绝访问，目标进程权限不足（可尝试以管理员身份运行）", op)
	case errors.Is(err, procmem.ErrClosed):
		return fmt.Errorf("%s: %w", op, ErrMemClosed)
	case errors.Is(err, procmem.ErrNotFound):
		return fmt.Errorf("%s: 未找到目标", op)
	}
	return fmt.Errorf("%s: %w", op, err)
}
