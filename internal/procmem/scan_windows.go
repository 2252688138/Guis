//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import (
	"bytes"
	"fmt"
	"sync"
)

// 值扫描器：首次扫描在全部可读区域里找「等于给定值」的地址，
// 之后每次 Next 用「变化 / 未变 / 变大 / 变小 / 等于给定值」等条件把候选集收敛，
// 这是 Cheat Engine 找动态地址的核心玩法。
//
// 实现要点：
//   - 分块读取（默认 4MB/块），块与块之间留 size-1 的重叠，避免值被块边界切断；
//   - 结果上限（默认 100 万）防止把整片堆都收进来，超限置 Truncated 标记；
//   - 只保留「上一轮的值」用于变化判断，不保存整块内存，内存占用与结果数成正比。

// Scanner 值扫描器（同一实例只能扫一种值类型）。
type Scanner struct {
	mu    sync.Mutex
	p     *Process
	kind  ValueKind
	opt   ScanOptions
	hits  []scanHit
	trunc bool
}

// scanHit 一条命中记录：地址 + 上一轮的值。
type scanHit struct {
	addr uintptr
	val  []byte
}

// NewScanner 创建扫描器（opt 零值用默认参数）。
func NewScanner(p *Process, opt ScanOptions) *Scanner {
	return &Scanner{p: p, opt: opt.withDefaults()}
}

// First 首次扫描：找出全部等于 v 的地址。
func (s *Scanner) First(v Value) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return ErrClosed
	}
	if err := v.validate(); err != nil {
		return err
	}
	size := len(v.Raw)
	align := uintptr(s.opt.Align)

	s.kind = v.Kind
	s.hits = s.hits[:0]
	s.trunc = false

	regions, err := s.p.Regions()
	if err != nil {
		return err
	}

	// 块间重叠量（按对齐向上取整），保证跨界值不被漏掉。
	overlap := uintptr(size - 1)
	if r := overlap % align; r != 0 {
		overlap += align - r
	}

	for _, reg := range regions {
		if !s.opt.acceptRegion(reg) {
			continue
		}
		var off uintptr
		for off < reg.Size {
			n := uintptr(s.opt.ChunkSize)
			if reg.Size-off < n {
				n = reg.Size - off
			}
			if n < uintptr(size) {
				break
			}
			buf, rerr := s.p.Read(reg.Base+off, int(n))
			for i := 0; i+size <= len(buf); i += int(align) {
				if !bytes.Equal(buf[i:i+size], v.Raw) {
					continue
				}
				s.hits = append(s.hits, scanHit{
					addr: reg.Base + off + uintptr(i),
					val:  append([]byte(nil), buf[i:i+size]...),
				})
				if len(s.hits) >= s.opt.MaxResults {
					s.trunc = true
					return nil
				}
			}
			if rerr != nil {
				break // 该区域读不全（保护页 / 已释放），跳过剩余部分
			}
			step := n - overlap
			if step <= 0 {
				break
			}
			off += step
		}
	}
	return nil
}

// Next 用过滤条件收敛上一次的扫描结果。
func (s *Scanner) Next(v Value, f Filter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return ErrClosed
	}
	if v.Kind != s.kind {
		return fmt.Errorf("procmem: 过滤值类型 %s 与首扫类型 %s 不一致", v.Kind, s.kind)
	}
	if err := v.validate(); err != nil {
		return err
	}
	if len(s.hits) == 0 {
		return nil
	}
	out := make([]scanHit, 0, len(s.hits))
	for _, h := range s.hits {
		cur := make([]byte, len(h.val))
		if _, err := s.p.ReadInto(h.addr, cur); err != nil {
			continue // 地址已失效（对象被释放 / 页被回收），直接丢弃
		}
		ok, err := matchFilter(s.kind, f, h.val, cur, v.Raw)
		if err != nil {
			return err
		}
		if ok {
			out = append(out, scanHit{addr: h.addr, val: cur})
		}
	}
	s.hits = out
	return nil
}

// Results 当前结果地址列表（副本）。
func (s *Scanner) Results() []uintptr {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]uintptr, len(s.hits))
	for i, h := range s.hits {
		out[i] = h.addr
	}
	return out
}

// Len 当前结果数量。
func (s *Scanner) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.hits)
}

// Truncated 结果是否因超过上限被截断。
func (s *Scanner) Truncated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trunc
}

// Reset 清空结果（保留参数，可重新 First）。
func (s *Scanner) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits = nil
	s.trunc = false
}
