//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// 门面冒烟用的全局变量（模块数据段，地址稳定）。
var memProbe int32 = 0x1234ABCD

func TestFindProcessesContainsSelf(t *testing.T) {
	list, err := FindProcesses("")
	if err != nil {
		t.Fatalf("FindProcesses: %v", err)
	}
	self := os.Getpid()
	found := false
	for _, p := range list {
		if p.PID == self {
			found = true
			if p.Name == "" {
				t.Fatalf("自身进程名为空: %+v", p)
			}
			break
		}
	}
	if !found {
		t.Fatalf("进程列表里没有自己（pid=%d，共 %d 条）", self, len(list))
	}

	// 按名字过滤（自身可执行文件名，去掉扩展名更稳）
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
	if name != "" {
		list2, err := FindProcesses(name)
		if err != nil {
			t.Fatalf("FindProcesses(%q): %v", name, err)
		}
		if len(list2) == 0 {
			t.Logf("提示：按 %q 过滤没有结果（测试进程名可能不同）", name)
		}
	}
}

func TestProcessMemRoundTrip(t *testing.T) {
	m, err := OpenProcessMem(os.Getpid())
	if err != nil {
		t.Fatalf("OpenProcessMem: %v", err)
	}
	defer func() { _ = m.Close() }()

	if m.PID() != os.Getpid() {
		t.Fatalf("PID = %d，期望 %d", m.PID(), os.Getpid())
	}
	if !m.Alive() {
		t.Fatal("自身进程应处于运行状态")
	}

	addr := uintptr(unsafe.Pointer(&memProbe))
	if err := m.WriteI32(addr, 0x0A0B0C0D); err != nil {
		t.Fatalf("WriteI32: %v", err)
	}
	v, err := m.ReadI32(addr)
	if err != nil {
		t.Fatalf("ReadI32: %v", err)
	}
	if v != 0x0A0B0C0D {
		t.Fatalf("读回值 = 0x%X", v)
	}

	if _, err := m.ModuleBase(""); err != nil {
		t.Fatalf("ModuleBase(空) 应返回主模块基址: %v", err)
	}
	if regs, err := m.Regions(); err != nil || len(regs) == 0 {
		t.Fatalf("Regions: len=%d err=%v", len(regs), err)
	}
	if mods, err := m.Modules(); err != nil || len(mods) == 0 {
		t.Fatalf("Modules: len=%d err=%v", len(mods), err)
	}

	// 关闭后统一返回 ErrMemClosed
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := m.Read(addr, 4); !errors.Is(err, ErrMemClosed) {
		t.Fatalf("关闭后读取应返回 ErrMemClosed，实际 %v", err)
	}
	if err := m.Write(addr, []byte{1, 2, 3, 4}); !errors.Is(err, ErrMemClosed) {
		t.Fatalf("关闭后写入应返回 ErrMemClosed，实际 %v", err)
	}
}

func TestMemScannerAndFreezerSmoke(t *testing.T) {
	m, err := OpenProcessMem(os.Getpid())
	if err != nil {
		t.Fatalf("OpenProcessMem: %v", err)
	}
	defer func() { _ = m.Close() }()

	const marker int32 = 0x0BADF00D
	memProbe = marker
	addr := uintptr(unsafe.Pointer(&memProbe))

	s := NewMemScanner(m, MemScanOptions{WritableOnly: true, Align: 4})
	if err := s.First(MemInt32(marker)); err != nil {
		t.Fatalf("First: %v", err)
	}
	hit := false
	for _, a := range s.Results() {
		if a == addr {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatalf("扫描未命中 0x%X（共 %d 条）", addr, s.Len())
	}

	// 冻结：改值后应被回写
	fz := NewMemFreezer(m, 0)
	defer func() { _ = fz.Close() }()
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, uint32(marker))
	if err := fz.Add(addr, raw); err != nil {
		t.Fatalf("Freezer.Add: %v", err)
	}
	if err := m.WriteI32(addr, 1); err != nil {
		t.Fatalf("WriteI32: %v", err)
	}
	// 给后台回写一点时间（interval=0 走默认 50ms）
	for i := 0; i < 100; i++ {
		if v, err := m.ReadI32(addr); err == nil && v == marker {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if v, err := m.ReadI32(addr); err != nil || v != marker {
		t.Fatalf("冻结未回写：v=%d err=%v", v, err)
	}
}

func TestOpenProcessMemInvalidPID(t *testing.T) {
	if _, err := OpenProcessMem(-1); err == nil {
		t.Fatal("非法 pid 应返回错误")
	}
}
