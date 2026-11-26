// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package netplay

import (
	"sort"
	"sync"
)

// Snapshot 一帧世界状态（Data 由上层编码，引擎不关心内部结构）。
type Snapshot struct {
	Tick uint32  `json:"tick"`
	Time float64 `json:"time"`
	Data []byte  `json:"data,omitempty"`
}

// Interpolator 快照缓冲与插值（客户端用）：
// 缓冲最近若干快照，按时间找到相邻两帧并给出插值系数，
// 上层用 alpha 混合两个世界状态即可获得平滑观感。
type Interpolator struct {
	// MaxBuffered 最多保留的快照数（默认 16）。
	MaxBuffered int

	mu  sync.Mutex
	buf []Snapshot
}

// NewInterpolator 创建插值缓冲。
func NewInterpolator() *Interpolator { return &Interpolator{MaxBuffered: 16} }

// Push 放入一条快照（按 Tick 去重与排序）。
func (ip *Interpolator) Push(s Snapshot) {
	if ip == nil {
		return
	}
	ip.mu.Lock()
	defer ip.mu.Unlock()
	for i := range ip.buf {
		if ip.buf[i].Tick == s.Tick {
			ip.buf[i] = s
			return
		}
	}
	ip.buf = append(ip.buf, s)
	sort.Slice(ip.buf, func(i, j int) bool { return ip.buf[i].Time < ip.buf[j].Time })
	max := ip.MaxBuffered
	if max <= 0 {
		max = 16
	}
	if len(ip.buf) > max {
		ip.buf = ip.buf[len(ip.buf)-max:]
	}
}

// Len 当前缓冲的快照数。
func (ip *Interpolator) Len() int {
	if ip == nil {
		return 0
	}
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return len(ip.buf)
}

// Latest 最新快照（无数据时返回零值与 false）。
func (ip *Interpolator) Latest() (Snapshot, bool) {
	if ip == nil {
		return Snapshot{}, false
	}
	ip.mu.Lock()
	defer ip.mu.Unlock()
	if len(ip.buf) == 0 {
		return Snapshot{}, false
	}
	return ip.buf[len(ip.buf)-1], true
}

// Sample 按时间取相邻两帧与插值系数 alpha（[0,1]）。
//
// now 早于最早快照时返回 (最早, 最早, 0)；now 晚于最新时返回 (最新, 最新, 0)；
// 缓冲为空时返回零值与 false。
func (ip *Interpolator) Sample(now float64) (prev, next Snapshot, alpha float32, ok bool) {
	if ip == nil {
		return Snapshot{}, Snapshot{}, 0, false
	}
	ip.mu.Lock()
	defer ip.mu.Unlock()
	if len(ip.buf) == 0 {
		return Snapshot{}, Snapshot{}, 0, false
	}
	if len(ip.buf) == 1 || now <= ip.buf[0].Time {
		return ip.buf[0], ip.buf[0], 0, true
	}
	last := ip.buf[len(ip.buf)-1]
	if now >= last.Time {
		return last, last, 0, true
	}
	i := 0
	for i+1 < len(ip.buf) && ip.buf[i+1].Time <= now {
		i++
	}
	prev, next = ip.buf[i], ip.buf[i+1]
	span := next.Time - prev.Time
	if span <= 0 {
		return prev, next, 0, true
	}
	return prev, next, float32((now - prev.Time) / span), true
}

// Lockstep 确定性帧同步：所有玩家提交本帧输入后统一放行。
type Lockstep struct {
	// Players 需要等待的玩家数。
	Players int
	// Tick 当前帧号（已放行的帧数）。
	Tick uint32

	mu     sync.Mutex
	inputs map[uint64][]byte
	order  []uint64
}

// NewLockstep 创建帧同步控制器。
func NewLockstep(players int) *Lockstep {
	if players < 1 {
		players = 1
	}
	return &Lockstep{Players: players, inputs: make(map[uint64][]byte)}
}

// Submit 提交某玩家本帧输入（同帧重复提交以最后一次为准）。
func (l *Lockstep) Submit(player uint64, input []byte) {
	if l == nil {
		return
	}
	l.mu.Lock()
	if _, ok := l.inputs[player]; !ok {
		l.order = append(l.order, player)
		sort.Slice(l.order, func(i, j int) bool { return l.order[i] < l.order[j] })
	}
	cp := make([]byte, len(input))
	copy(cp, input)
	l.inputs[player] = cp
	l.mu.Unlock()
}

// Pending 还差几人未提交。
func (l *Lockstep) Pending() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	n := l.Players - len(l.inputs)
	if n < 0 {
		n = 0
	}
	return n
}

// Advance 尝试放行一帧：输入齐全时返回该帧号与按玩家 ID 排序的输入，
// 并清空缓冲进入下一帧；不齐全时 ok = false。
func (l *Lockstep) Advance() (tick uint32, inputs map[uint64][]byte, ok bool) {
	if l == nil {
		return 0, nil, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.inputs) < l.Players {
		return 0, nil, false
	}
	l.Tick++
	tick = l.Tick
	inputs = make(map[uint64][]byte, len(l.inputs))
	for _, id := range l.order {
		inputs[id] = l.inputs[id]
	}
	l.inputs = make(map[uint64][]byte)
	l.order = l.order[:0]
	return tick, inputs, true
}
