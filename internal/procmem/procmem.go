// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package procmem 提供「进程内存读写」能力（Cheat Engine 类）：枚举/打开进程、
// 读写内存、枚举模块与内存区域、改保护属性、值扫描、指针链回溯、内存冻结。
//
// 平台支持：Windows 全实现（kernel32 延迟解析，缺符号只影响对应能力）；
// 其他平台为 stub，所有接口返回 ErrUnsupported，保证跨平台编译不破。
//
// 典型用法：
//
//	p, err := procmem.Open(pid)
//	defer p.Close()
//	base, _ := p.ModuleBase("game.exe")
//	hp, _ := p.ReadU32(base + 0x1A2B3C)
//	_ = p.WriteU32(base+0x1A2B3C, hp+100)
//
// 安全边界：只做「打开 → 读写 → 还原」的最小集合，不含 DLL 注入、远程线程、
// 驱动加载或任何反作弊绕过；OpenProcess 失败直接返回错误，不尝试提权。
package procmem

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

// 公共错误。
var (
	// ErrUnsupported 当前平台不支持该能力（非 Windows）。
	ErrUnsupported = errors.New("procmem: 当前平台不支持进程内存操作")
	// ErrClosed 进程句柄已关闭。
	ErrClosed = errors.New("procmem: 进程句柄已关闭")
	// ErrAccessDenied 目标进程拒绝访问（权限不足）。
	ErrAccessDenied = errors.New("procmem: 拒绝访问，目标进程权限不足（可尝试以管理员身份运行）")
	// ErrNotFound 未找到目标（模块 / 地址 / 指针链）。
	ErrNotFound = errors.New("procmem: 未找到目标")
	// ErrTruncated 扫描结果超过上限被截断（同时由 Scanner.Truncated 报告）。
	ErrTruncated = errors.New("procmem: 扫描结果超过上限，已截断")
)

// ProcInfo 进程基本信息。
type ProcInfo struct {
	// PID 进程 ID。
	PID int
	// Name 可执行文件名（不含路径）。
	Name string
	// Path 可执行文件完整路径，取不到时为空。
	Path string
}

// Module 已加载模块（DLL / EXE）信息。
type Module struct {
	// Name 模块名，如 kernel32.dll。
	Name string
	// Path 模块完整路径。
	Path string
	// Base 模块基址。
	Base uintptr
	// Size 模块映像大小（字节）。
	Size uintptr
}

// Region 内存区域（VirtualQueryEx 一条记录）。
type Region struct {
	// Base 区域起始地址。
	Base uintptr
	// Size 区域大小（字节）。
	Size uintptr
	// State 提交状态（MemCommit / MemReserve / MemFree）。
	State uint32
	// Protect 保护属性（PageReadWrite 等，可能含 PageGuard 等修饰位）。
	Protect uint32
	// Type 区域类型（MemImage / MemMapped / MemPrivate，0 表示未用）。
	Type uint32
}

// 内存区域状态（MEM_*）。
const (
	// MemCommit 已提交。
	MemCommit uint32 = 0x1000
	// MemReserve 已保留。
	MemReserve uint32 = 0x2000
	// MemFree 已释放。
	MemFree uint32 = 0x10000
)

// 内存页保护属性（PAGE_*）。
const (
	// PageNoAccess 不可访问。
	PageNoAccess uint32 = 0x01
	// PageReadOnly 只读。
	PageReadOnly uint32 = 0x02
	// PageReadWrite 可读写。
	PageReadWrite uint32 = 0x04
	// PageWriteCopy 写时复制。
	PageWriteCopy uint32 = 0x08
	// PageExecute 仅执行（不可读）。
	PageExecute uint32 = 0x10
	// PageExecuteRead 可执行可读。
	PageExecuteRead uint32 = 0x20
	// PageExecuteReadWrite 可执行可读写。
	PageExecuteReadWrite uint32 = 0x40
	// PageExecuteWriteCopy 可执行写时复制。
	PageExecuteWriteCopy uint32 = 0x80
	// PageGuard 保护页（访问触发异常，读会失败）。
	PageGuard uint32 = 0x100
	// PageNoCache 不缓存。
	PageNoCache uint32 = 0x200
	// PageWriteCombine 合并写。
	PageWriteCombine uint32 = 0x400
)

// 内存区域类型（MEM_*）。
const (
	// MemImage 映像（模块）。
	MemImage uint32 = 0x1000000
	// MemMapped 内存映射文件。
	MemMapped uint32 = 0x40000
	// MemPrivate 私有内存（堆 / 栈）。
	MemPrivate uint32 = 0x20000
)

// Readable 该区域当前是否可读（已提交、非保护页、非仅执行）。
func (r Region) Readable() bool {
	if r.State != MemCommit {
		return false
	}
	if r.Protect&PageGuard != 0 {
		return false
	}
	switch r.Protect & 0xff {
	case PageReadOnly, PageReadWrite, PageWriteCopy,
		PageExecuteRead, PageExecuteReadWrite, PageExecuteWriteCopy:
		return true
	}
	return false
}

// Writable 该区域当前是否可写。
func (r Region) Writable() bool {
	if !r.Readable() {
		return false
	}
	switch r.Protect & 0xff {
	case PageReadWrite, PageWriteCopy, PageExecuteReadWrite, PageExecuteWriteCopy:
		return true
	}
	return false
}

// ValueKind 扫描值类型。
type ValueKind int

// 支持的值类型。
const (
	// KindInt32 32 位有符号整数。
	KindInt32 ValueKind = iota
	// KindUint32 32 位无符号整数。
	KindUint32
	// KindInt64 64 位有符号整数。
	KindInt64
	// KindFloat32 单精度浮点。
	KindFloat32
	// KindFloat64 双精度浮点。
	KindFloat64
	// KindBytes 定长字节串。
	KindBytes
	// KindString UTF-8 字符串。
	KindString
	// KindUTF16String UTF-16LE 字符串。
	KindUTF16String
)

// String 值类型名称。
func (k ValueKind) String() string {
	switch k {
	case KindInt32:
		return "int32"
	case KindUint32:
		return "uint32"
	case KindInt64:
		return "int64"
	case KindFloat32:
		return "float32"
	case KindFloat64:
		return "float64"
	case KindBytes:
		return "bytes"
	case KindString:
		return "string"
	case KindUTF16String:
		return "utf16"
	}
	return fmt.Sprintf("ValueKind(%d)", int(k))
}

// Value 扫描用的值：类型 + 原始字节（小端）。
type Value struct {
	// Kind 值类型。
	Kind ValueKind
	// Raw 原始字节，长度需与 Kind 匹配。
	Raw []byte
}

// Int32 构造 int32 值。
func Int32(v int32) Value {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return Value{Kind: KindInt32, Raw: b}
}

// Uint32 构造 uint32 值。
func Uint32(v uint32) Value {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return Value{Kind: KindUint32, Raw: b}
}

// Int64 构造 int64 值。
func Int64(v int64) Value {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return Value{Kind: KindInt64, Raw: b}
}

// Float32 构造 float32 值。
func Float32(v float32) Value {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
	return Value{Kind: KindFloat32, Raw: b}
}

// Float64 构造 float64 值。
func Float64(v float64) Value {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return Value{Kind: KindFloat64, Raw: b}
}

// Bytes 构造定长字节值（会复制一份）。
func Bytes(b []byte) Value {
	return Value{Kind: KindBytes, Raw: append([]byte(nil), b...)}
}

// String 构造 UTF-8 字符串值。
func String(s string) Value {
	return Value{Kind: KindString, Raw: []byte(s)}
}

// UTF16String 构造 UTF-16LE 字符串值（不含结尾 0）。
func UTF16String(s string) Value {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return Value{Kind: KindUTF16String, Raw: b}
}

// Size 值的字节长度。
func (v Value) Size() int { return len(v.Raw) }

// validate 检查类型与字节长度是否自洽。
func (v Value) validate() error {
	switch v.Kind {
	case KindInt32, KindUint32, KindFloat32:
		if len(v.Raw) != 4 {
			return fmt.Errorf("procmem: %s 值长度应为 4，实际 %d", v.Kind, len(v.Raw))
		}
	case KindInt64, KindFloat64:
		if len(v.Raw) != 8 {
			return fmt.Errorf("procmem: %s 值长度应为 8，实际 %d", v.Kind, len(v.Raw))
		}
	case KindBytes, KindString, KindUTF16String:
		if len(v.Raw) == 0 {
			return fmt.Errorf("procmem: %s 值不能为空", v.Kind)
		}
	default:
		return fmt.Errorf("procmem: 未知值类型 %d", int(v.Kind))
	}
	return nil
}

// Filter 扫描过滤条件。
type Filter int

// 支持的过滤条件。
const (
	// FilterEqual 等于给定值。
	FilterEqual Filter = iota
	// FilterNotEqual 不等于给定值。
	FilterNotEqual
	// FilterChanged 与上次扫描相比发生变化。
	FilterChanged
	// FilterUnchanged 与上次扫描相比未变化。
	FilterUnchanged
	// FilterIncreased 与上次扫描相比变大（仅数值类型）。
	FilterIncreased
	// FilterDecreased 与上次扫描相比变小（仅数值类型）。
	FilterDecreased
	// FilterGreaterThan 大于给定值（仅数值类型）。
	FilterGreaterThan
	// FilterLessThan 小于给定值（仅数值类型）。
	FilterLessThan
)

// String 过滤条件名称。
func (f Filter) String() string {
	switch f {
	case FilterEqual:
		return "equal"
	case FilterNotEqual:
		return "not_equal"
	case FilterChanged:
		return "changed"
	case FilterUnchanged:
		return "unchanged"
	case FilterIncreased:
		return "increased"
	case FilterDecreased:
		return "decreased"
	case FilterGreaterThan:
		return "greater_than"
	case FilterLessThan:
		return "less_than"
	}
	return fmt.Sprintf("Filter(%d)", int(f))
}

// NeedsValue 该过滤条件是否需要给定值参与比较。
func (f Filter) NeedsValue() bool {
	switch f {
	case FilterEqual, FilterNotEqual, FilterGreaterThan, FilterLessThan:
		return true
	}
	return false
}

// ScanOptions 扫描参数。
type ScanOptions struct {
	// MaxResults 结果上限，<=0 用默认 1<<20（100 万条）。
	MaxResults int
	// ChunkSize 每次读取的块大小，<4096 用默认 4MB。
	ChunkSize int
	// Align 地址对齐（1/2/4/8），<=0 用 1。
	Align int
	// WritableOnly 只扫描可写区域（排除只读代码段，通常更快）。
	WritableOnly bool
}

// 扫描默认参数。
const (
	defaultMaxResults = 1 << 20
	defaultChunkSize  = 4 << 20
)

// withDefaults 补默认值并夹紧非法参数。
func (o ScanOptions) withDefaults() ScanOptions {
	if o.MaxResults <= 0 {
		o.MaxResults = defaultMaxResults
	}
	if o.ChunkSize < 4096 {
		o.ChunkSize = defaultChunkSize
	}
	if o.Align <= 0 {
		o.Align = 1
	}
	if o.Align > 8 {
		o.Align = 8
	}
	return o
}

// acceptRegion 该区域是否参与扫描。
func (o ScanOptions) acceptRegion(r Region) bool {
	if r.Size == 0 || !r.Readable() {
		return false
	}
	if o.WritableOnly && !r.Writable() {
		return false
	}
	return true
}

// ChainOptions 指针链搜索参数。
type ChainOptions struct {
	// MaxLevel 最大层级（默认 3）。
	MaxLevel int
	// MaxOffset 单级最大偏移（默认 0x1000）。
	MaxOffset uintptr
	// MaxResults 最多返回多少条链（默认 64）。
	MaxResults int
}

// withDefaults 补默认值。
func (o ChainOptions) withDefaults() ChainOptions {
	if o.MaxLevel <= 0 {
		o.MaxLevel = 3
	}
	if o.MaxOffset == 0 {
		o.MaxOffset = 0x1000
	}
	if o.MaxResults <= 0 {
		o.MaxResults = 64
	}
	return o
}

// FreezeEntry 一条冻结项。
type FreezeEntry struct {
	// Addr 目标地址。
	Addr uintptr
	// Data 需要保持的字节内容。
	Data []byte
	// Err 最近一次回写的错误（成功后清空）。
	Err error
}

// compareNumeric 按值类型比较两段原始字节，返回 -1 / 0 / 1；非数值类型返回 false。
func compareNumeric(kind ValueKind, a, b []byte) (int, bool) {
	switch kind {
	case KindInt32:
		if len(a) < 4 || len(b) < 4 {
			return 0, false
		}
		return cmpOrdered(int64(int32(binary.LittleEndian.Uint32(a))), int64(int32(binary.LittleEndian.Uint32(b)))), true
	case KindUint32:
		if len(a) < 4 || len(b) < 4 {
			return 0, false
		}
		return cmpOrdered(int64(binary.LittleEndian.Uint32(a)), int64(binary.LittleEndian.Uint32(b))), true
	case KindInt64:
		if len(a) < 8 || len(b) < 8 {
			return 0, false
		}
		x := int64(binary.LittleEndian.Uint64(a))
		y := int64(binary.LittleEndian.Uint64(b))
		return cmpOrdered(x, y), true
	case KindFloat32:
		if len(a) < 4 || len(b) < 4 {
			return 0, false
		}
		x := float64(math.Float32frombits(binary.LittleEndian.Uint32(a)))
		y := float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
		return cmpOrdered(x, y), true
	case KindFloat64:
		if len(a) < 8 || len(b) < 8 {
			return 0, false
		}
		x := math.Float64frombits(binary.LittleEndian.Uint64(a))
		y := math.Float64frombits(binary.LittleEndian.Uint64(b))
		return cmpOrdered(x, y), true
	}
	return 0, false
}

// cmpOrdered 泛型有序比较（NaN 视为相等，避免比较结果抖动）。
func cmpOrdered[T int64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// matchFilter 判断候选地址的当前值是否通过过滤。
//
// prev 是上次扫描时该地址的值，cur 是当前值，want 是调用方给定的目标值。
func matchFilter(kind ValueKind, f Filter, prev, cur, want []byte) (bool, error) {
	switch f {
	case FilterEqual:
		return bytes.Equal(cur, want), nil
	case FilterNotEqual:
		return !bytes.Equal(cur, want), nil
	case FilterChanged:
		return !bytes.Equal(cur, prev), nil
	case FilterUnchanged:
		return bytes.Equal(cur, prev), nil
	}

	prevCmp, ok := compareNumeric(kind, cur, prev)
	if !ok {
		return false, fmt.Errorf("procmem: 值类型 %s 不支持 %s 过滤，请改用 equal / changed 等", kind, f)
	}
	switch f {
	case FilterIncreased:
		return prevCmp > 0, nil
	case FilterDecreased:
		return prevCmp < 0, nil
	}

	wantCmp, ok := compareNumeric(kind, cur, want)
	if !ok {
		return false, fmt.Errorf("procmem: 值类型 %s 不支持 %s 过滤", kind, f)
	}
	switch f {
	case FilterGreaterThan:
		return wantCmp > 0, nil
	case FilterLessThan:
		return wantCmp < 0, nil
	}
	return false, fmt.Errorf("procmem: 未知过滤条件 %d", int(f))
}
