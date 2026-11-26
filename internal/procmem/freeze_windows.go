//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import (
	"fmt"
	"sync"
	"time"
)

// 内存冻结：后台按固定间隔把指定地址写回固定内容（锁定血量 / 冷却 / 坐标等）。
//
// 只做「定时回写」，不改代码段、不挂钩子；Close 会等后台协程退出，不泄漏 goroutine。

// Freezer 内存冻结器。
type Freezer struct {
	mu      sync.Mutex
	p       *Process
	tick    time.Duration
	entries []*FreezeEntry
	stop    chan struct{}
	done    chan struct{}
	started bool
	closed  bool
}

// NewFreezer 创建冻结器，interval <= 0 时用 50ms。
func NewFreezer(p *Process, interval time.Duration) *Freezer {
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}
	return &Freezer{p: p, tick: interval}
}

// Add 添加（或覆盖）一个冻结项，并立即回写一次。
func (f *Freezer) Add(addr uintptr, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("procmem: 冻结数据不能为空")
	}
	if f == nil || f.p == nil {
		return ErrClosed
	}
	cp := append([]byte(nil), data...)

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return ErrClosed
	}
	replaced := false
	for _, e := range f.entries {
		if e.Addr == addr {
			e.Data = cp
			e.Err = nil
			replaced = true
			break
		}
	}
	if !replaced {
		f.entries = append(f.entries, &FreezeEntry{Addr: addr, Data: cp})
	}
	if !f.started {
		f.started = true
		f.stop = make(chan struct{})
		f.done = make(chan struct{})
		go f.loop(f.stop, f.done)
	}
	f.mu.Unlock()

	return f.p.Write(addr, cp)
}

// Remove 移除一个冻结项。
func (f *Freezer) Remove(addr uintptr) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.entries {
		if e.Addr == addr {
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			return
		}
	}
}

// List 当前冻结项（副本，含最近一次回写错误）。
func (f *Freezer) List() []FreezeEntry {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FreezeEntry, 0, len(f.entries))
	for _, e := range f.entries {
		out = append(out, FreezeEntry{
			Addr: e.Addr,
			Data: append([]byte(nil), e.Data...),
			Err:  e.Err,
		})
	}
	return out
}

// Close 停止后台回写并等待协程退出（幂等）。
func (f *Freezer) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	stop, done, started := f.stop, f.done, f.started
	f.started = false
	f.mu.Unlock()

	if started && stop != nil {
		close(stop)
		<-done
	}
	return nil
}

// loop 后台回写循环。
func (f *Freezer) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	t := time.NewTicker(f.tick)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			f.apply()
		}
	}
}

// apply 把全部冻结项回写一次，记录每项的错误。
func (f *Freezer) apply() {
	f.mu.Lock()
	list := make([]FreezeEntry, 0, len(f.entries))
	for _, e := range f.entries {
		list = append(list, FreezeEntry{Addr: e.Addr, Data: e.Data})
	}
	f.mu.Unlock()

	for _, e := range list {
		err := f.p.Write(e.Addr, e.Data)
		f.mu.Lock()
		for _, en := range f.entries {
			if en.Addr == e.Addr {
				en.Err = err
				break
			}
		}
		f.mu.Unlock()
	}
}
