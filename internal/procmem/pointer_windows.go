//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package procmem

import (
	"encoding/binary"
	"fmt"
	"sort"
	"unsafe"
)

// 指针链回溯：给一个「数据地址」，在模块的可读区域里逐级找「谁存了指向它的指针」，
// 直到模块静态地址，得到形如 {slot, off1, off2, ...} 的链：
//
//	读 slot 处的指针 → +off1 → 读指针 → +off2 → … → 目标地址
//
// 这是 Cheat Engine「找出基址 + 偏移」的做法：动态数据地址每次重启都变，
// 但「模块基址 + 固定偏移」的链是稳定的。

// 指针链搜索的资源上限。
const (
	// pointerChunkSize 每次读取的块大小。
	pointerChunkSize = 8 << 20
	// pointerRegionCap 单区域上限，超过则不参与（通常是映射文件 / 大堆，不是静态数据）。
	pointerRegionCap = 128 << 20
)

// ptrSize 目标平台指针宽度（与被调试进程一致，不支持跨位宽调试）。
const ptrSize = int(unsafe.Sizeof(uintptr(0)))

// FindPointerChains 搜索能解析到 target 的指针链，结果按首地址升序。
func FindPointerChains(p *Process, target uintptr, opt ChainOptions) ([][]uintptr, error) {
	if p == nil {
		return nil, ErrClosed
	}
	if target == 0 {
		return nil, fmt.Errorf("procmem: 目标地址为 0")
	}
	opt = opt.withDefaults()

	mods, err := p.Modules()
	if err != nil {
		return nil, err
	}
	regions, err := p.Regions()
	if err != nil {
		return nil, err
	}

	// 只在「模块内 + 可读 + 大小合理」的区域里找指针槽。
	var scan []Region
	for _, r := range regions {
		if !r.Readable() || r.Size < uintptr(ptrSize) || r.Size > pointerRegionCap {
			continue
		}
		if !inAnyModule(mods, r) {
			continue
		}
		scan = append(scan, r)
	}
	if len(scan) == 0 {
		return nil, fmt.Errorf("%w: 模块内没有可读区域", ErrNotFound)
	}

	// chains[a] = 从 a 读出指针后还要依次加的偏移（不含 a 自身）。
	chains := map[uintptr][]uintptr{target: nil}
	cands := []uintptr{target}
	var results [][]uintptr
	full := false

	for level := 1; level <= opt.MaxLevel && len(cands) > 0 && !full; level++ {
		sort.Slice(cands, func(i, j int) bool { return cands[i] < cands[j] })
		next := make([]uintptr, 0, 32)
		seen := make(map[uintptr]struct{}, 32)
		for _, reg := range scan {
			scanPointers(p, reg, func(slot, val uintptr) bool {
				if val == 0 {
					return true
				}
				// 在 cands 中找 c ∈ [val, val+MaxOffset] 的最近一项。
				idx := sort.Search(len(cands), func(k int) bool { return cands[k] >= val })
				for ; idx < len(cands); idx++ {
					c := cands[idx]
					d := c - val
					if d > opt.MaxOffset {
						break
					}
					if _, dup := seen[slot]; dup {
						return true
					}
					seen[slot] = struct{}{}
					ch := append([]uintptr{d}, chains[c]...)
					chains[slot] = ch
					results = append(results, append([]uintptr{slot}, ch...))
					next = append(next, slot)
					if len(results) >= opt.MaxResults {
						full = true
						return false
					}
					return true
				}
				return true
			})
			if full {
				break
			}
		}
		cands = next
	}

	sort.Slice(results, func(i, j int) bool { return results[i][0] < results[j][0] })
	return results, nil
}

// scanPointers 按块遍历区域内的指针槽，fn 返回 false 表示停止。
func scanPointers(p *Process, reg Region, fn func(slot, val uintptr) bool) {
	off := uintptr(0)
	for off < reg.Size {
		n := uintptr(pointerChunkSize)
		if reg.Size-off < n {
			n = reg.Size - off
		}
		buf, err := p.Read(reg.Base+off, int(n))
		for i := 0; i+ptrSize <= len(buf); i += ptrSize {
			if !fn(reg.Base+off+uintptr(i), readPtr(buf[i:])) {
				return
			}
		}
		if err != nil || off+n >= reg.Size {
			return
		}
		// 块间留 ptrSize-1 字节重叠，避免指针跨块被漏掉；回退后重新对齐。
		off += n - uintptr(ptrSize-1)
		off -= off % uintptr(ptrSize)
	}
}

// readPtr 从字节切片按本机字节序读一个指针。
func readPtr(b []byte) uintptr {
	if ptrSize == 8 {
		if len(b) < 8 {
			return 0
		}
		return uintptr(binary.LittleEndian.Uint64(b))
	}
	if len(b) < 4 {
		return 0
	}
	return uintptr(binary.LittleEndian.Uint32(b))
}

// inAnyModule 区域是否落在某个模块映像范围内。
func inAnyModule(mods []Module, r Region) bool {
	end := r.Base + r.Size
	for _, m := range mods {
		if m.Size == 0 {
			continue
		}
		mEnd := m.Base + m.Size
		if r.Base >= m.Base && r.Base < mEnd {
			return true
		}
		if end > m.Base && end <= mEnd {
			return true
		}
	}
	return false
}
