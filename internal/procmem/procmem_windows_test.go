//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"
	"unsafe"
)

// 自测用的全局变量：位于测试进程模块的数据段，地址稳定，
// 便于「读自己 / 写自己 / 扫描 / 指针链 / 冻结」用同一批目标。
var (
	testI32   int32 = 0x11223344
	testBytes       = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	scanSink        = make([]int32, 4096)
	freezeVar int32 = 1
	ptrVal    int32 = 424242
	ptrSlot   *int32
)

// openSelf 打开当前测试进程。
func openSelf(t *testing.T) *Process {
	t.Helper()
	p, err := Open(os.Getpid())
	if err != nil {
		t.Fatalf("Open(self) 失败: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestReadWriteRoundTrip(t *testing.T) {
	p := openSelf(t)
	addr := uintptr(unsafe.Pointer(&testI32))

	if err := p.WriteI32(addr, 0x55667788); err != nil {
		t.Fatalf("WriteI32: %v", err)
	}
	got, err := p.ReadI32(addr)
	if err != nil {
		t.Fatalf("ReadI32: %v", err)
	}
	if got != 0x55667788 {
		t.Fatalf("读回值 = 0x%X，期望 0x55667788", got)
	}

	b, err := p.Read(uintptr(unsafe.Pointer(&testBytes[0])), len(testBytes))
	if err != nil {
		t.Fatalf("Read bytes: %v", err)
	}
	if !bytes.Equal(b, testBytes[:]) {
		t.Fatalf("字节读回不一致: %v", b)
	}

	if _, err := p.Read(0x10, 16); err == nil {
		t.Fatal("读非法地址应返回错误")
	}
}

func TestNumericAndStringAccessors(t *testing.T) {
	p := openSelf(t)
	addr := uintptr(unsafe.Pointer(&testI32))

	if err := p.WriteF32(addr, 3.5); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if v, err := p.ReadF32(addr); err != nil || v != 3.5 {
		t.Fatalf("ReadF32 = %v, err=%v", v, err)
	}
	if err := p.WriteU32(addr, 7); err != nil {
		t.Fatalf("WriteU32: %v", err)
	}
	if v, err := p.ReadU32(addr); err != nil || v != 7 {
		t.Fatalf("ReadU32 = %v, err=%v", v, err)
	}

	// 字符串：写进 testBytes 后读回（4 个字节够 "ab" + 0）。
	saddr := uintptr(unsafe.Pointer(&testBytes[0]))
	if err := p.WriteCString(saddr, "ab"); err != nil {
		t.Fatalf("WriteCString: %v", err)
	}
	s, err := p.ReadCString(saddr, 16)
	if err != nil {
		t.Fatalf("ReadCString: %v", err)
	}
	if s != "ab" {
		t.Fatalf("ReadCString = %q，期望 \"ab\"", s)
	}
}

func TestClosedHandle(t *testing.T) {
	p, err := Open(os.Getpid())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := p.Read(0x1000, 4); !errors.Is(err, ErrClosed) {
		t.Fatalf("关闭后读取应返回 ErrClosed，实际 %v", err)
	}
	if err := p.Write(0x1000, []byte{1}); !errors.Is(err, ErrClosed) {
		t.Fatalf("关闭后写入应返回 ErrClosed，实际 %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close 应幂等，实际 %v", err)
	}
}

func TestModulesAndRegions(t *testing.T) {
	p := openSelf(t)
	mods, err := p.Modules()
	if err != nil {
		t.Fatalf("Modules: %v", err)
	}
	if len(mods) == 0 {
		t.Fatal("Modules 不应为空")
	}
	if mods[0].Base == 0 {
		t.Fatalf("模块基址不应为 0: %+v", mods[0])
	}
	base, err := p.ModuleBase(mods[0].Name)
	if err != nil {
		t.Fatalf("ModuleBase(%q): %v", mods[0].Name, err)
	}
	if base != mods[0].Base {
		t.Fatalf("ModuleBase = 0x%X，期望 0x%X", base, mods[0].Base)
	}
	if _, err := p.ModuleBase("不存在的模块xyz.dll"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("查不存在的模块应返回 ErrNotFound，实际 %v", err)
	}

	regs, err := p.Regions()
	if err != nil {
		t.Fatalf("Regions: %v", err)
	}
	if len(regs) == 0 {
		t.Fatal("Regions 不应为空")
	}
	var readable, committed int
	for _, r := range regs {
		if r.State == MemCommit {
			committed++
		}
		if r.Readable() {
			readable++
		}
	}
	if committed == 0 || readable == 0 {
		t.Fatalf("应有已提交且可读的区域：commit=%d readable=%d", committed, readable)
	}
}

func TestSetProtect(t *testing.T) {
	p := openSelf(t)
	addr := uintptr(unsafe.Pointer(&testI32)) &^ 0xfff

	// 用与现状一致的保护属性，避免真的把数据段改成只读（同页可能有运行时数据）。
	old, err := p.SetProtect(addr, 0x1000, PageReadWrite)
	if err != nil {
		t.Fatalf("SetProtect: %v", err)
	}
	if old == 0 {
		t.Fatal("旧保护属性不应为 0")
	}
	if _, err := p.SetProtect(addr, 0x1000, old); err != nil {
		t.Fatalf("还原保护属性: %v", err)
	}
}

func TestScannerFirstAndFilters(t *testing.T) {
	const marker int32 = 0x0BADF00D
	const idx = 1234
	scanSink[idx] = marker
	addr := uintptr(unsafe.Pointer(&scanSink[idx]))

	p := openSelf(t)
	s := NewScanner(p, ScanOptions{WritableOnly: true, Align: 4})
	if err := s.First(Int32(marker)); err != nil {
		t.Fatalf("First: %v", err)
	}
	if s.Len() == 0 {
		t.Fatal("首扫应至少命中自己构造的值")
	}
	if !containsAddr(s.Results(), addr) {
		t.Fatalf("首扫未命中目标地址 0x%X（共 %d 条）", addr, s.Len())
	}

	// 值变化 → Changed 应保留
	scanSink[idx] = marker + 1
	if err := s.Next(Int32(marker+1), FilterChanged); err != nil {
		t.Fatalf("Next(Changed): %v", err)
	}
	if !containsAddr(s.Results(), addr) {
		t.Fatal("Changed 过滤后目标地址应仍存在")
	}

	// 等于新值
	if err := s.Next(Int32(marker+1), FilterEqual); err != nil {
		t.Fatalf("Next(Equal): %v", err)
	}
	if !containsAddr(s.Results(), addr) {
		t.Fatal("Equal 过滤后目标地址应仍存在")
	}

	// 值变小 → Increased 应剔除
	scanSink[idx] = marker
	if err := s.Next(Int32(marker), FilterIncreased); err != nil {
		t.Fatalf("Next(Increased): %v", err)
	}
	if containsAddr(s.Results(), addr) {
		t.Fatal("值变小后 Increased 过滤应剔除目标地址")
	}

	// 类型不一致要报错
	if err := s.Next(Float32(1), FilterChanged); err == nil {
		t.Fatal("过滤值类型与首扫不一致时应报错")
	}
}

func TestScannerTruncated(t *testing.T) {
	scanSink[7] = 0x0BADF00D
	p := openSelf(t)
	s := NewScanner(p, ScanOptions{WritableOnly: true, Align: 4, MaxResults: 1})
	if err := s.First(Int32(0x0BADF00D)); err != nil {
		t.Fatalf("First: %v", err)
	}
	if s.Len() != 1 || !s.Truncated() {
		t.Fatalf("上限 1 时应截断：len=%d truncated=%v", s.Len(), s.Truncated())
	}
	s.Reset()
	if s.Len() != 0 || s.Truncated() {
		t.Fatal("Reset 应清空结果与截断标记")
	}
}

func TestFindPointerChains(t *testing.T) {
	ptrSlot = &ptrVal
	target := uintptr(unsafe.Pointer(&ptrVal))
	slot := uintptr(unsafe.Pointer(&ptrSlot))

	p := openSelf(t)
	chains, err := FindPointerChains(p, target, ChainOptions{MaxLevel: 2, MaxOffset: 0x1000, MaxResults: 64})
	if err != nil {
		t.Fatalf("FindPointerChains: %v", err)
	}
	if len(chains) == 0 {
		t.Fatal("未找到任何指针链")
	}
	ok := false
	for _, ch := range chains {
		if resolveChain(p, ch) == target {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("%d 条链都解析不到目标 0x%X", len(chains), target)
	}
	hitSlot := false
	for _, ch := range chains {
		if len(ch) > 0 && ch[0] == slot {
			hitSlot = true
			break
		}
	}
	if !hitSlot {
		t.Logf("提示：未直接命中测试槽位 0x%X（共 %d 条链），但存在可解析链", slot, len(chains))
	}
}

func TestFreezer(t *testing.T) {
	p := openSelf(t)
	addr := uintptr(unsafe.Pointer(&freezeVar))
	const want int32 = 0x00C0FFEE

	f := NewFreezer(p, 10*time.Millisecond)
	defer func() { _ = f.Close() }()

	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(want))
	if err := f.Add(addr, buf); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// 手工改值，等一个 tick 后被回写。
	if err := p.WriteI32(addr, 999); err != nil {
		t.Fatalf("WriteI32: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		v, err := p.ReadI32(addr)
		if err != nil {
			t.Fatalf("ReadI32: %v", err)
		}
		if v == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("冻结未回写，当前值 %d", v)
		}
		time.Sleep(5 * time.Millisecond)
	}

	if list := f.List(); len(list) != 1 || list[0].Addr != addr {
		t.Fatalf("List 异常: %+v", list)
	}

	// Close 之后不再回写。
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.WriteI32(addr, 555); err != nil {
		t.Fatalf("WriteI32: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if v, err := p.ReadI32(addr); err != nil || v != 555 {
		t.Fatalf("Close 后不应再回写：v=%d err=%v", v, err)
	}
}

// containsAddr 结果列表里是否有指定地址。
func containsAddr(list []uintptr, addr uintptr) bool {
	for _, a := range list {
		if a == addr {
			return true
		}
	}
	return false
}

// resolveChain 按 {slot, off1, off2, ...} 解析出最终地址，失败返回 0。
func resolveChain(p *Process, ch []uintptr) uintptr {
	if len(ch) == 0 {
		return 0
	}
	addr := readPtrAt(p, ch[0])
	if addr == 0 {
		return 0
	}
	for i := 1; i < len(ch); i++ {
		addr += ch[i]
		if i+1 < len(ch) {
			addr = readPtrAt(p, addr)
			if addr == 0 {
				return 0
			}
		}
	}
	return addr
}

// readPtrAt 读一个指针宽度的值。
func readPtrAt(p *Process, addr uintptr) uintptr {
	if ptrSize == 8 {
		v, err := p.ReadU64(addr)
		if err != nil {
			return 0
		}
		return uintptr(v)
	}
	v, err := p.ReadU32(addr)
	if err != nil {
		return 0
	}
	return uintptr(v)
}
