// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Pool 可复用对象池（子弹 / 粒子 / 敌人 / 伤害数字），避免每帧 new 造成的 GC 抖动。
//
//	pool := game.NewPool(func() *Bullet { return &Bullet{} }, func(b *Bullet) { *b = Bullet{} })
//	b := pool.Get()
//	...
//	pool.Put(b)
//
// 池非并发安全（游戏主循环单 goroutine 使用）。
type Pool[T any] struct {
	// New 创建新对象（为空时返回零值）。
	New func() T
	// Reset 回收时重置（可为空）。
	Reset func(T)

	items []T
}

// NewPool 创建对象池。
func NewPool[T any](newFn func() T, resetFn func(T)) *Pool[T] {
	return &Pool[T]{New: newFn, Reset: resetFn}
}

// Get 取出一个对象（池空时调用 New）。
func (p *Pool[T]) Get() T {
	if p == nil {
		var zero T
		return zero
	}
	n := len(p.items)
	if n == 0 {
		if p.New != nil {
			return p.New()
		}
		var zero T
		return zero
	}
	v := p.items[n-1]
	p.items = p.items[:n-1]
	return v
}

// Put 归还对象（可先 Reset 再入池；池容量上限 4096，超出的直接丢弃）。
func (p *Pool[T]) Put(v T) {
	if p == nil || len(p.items) >= 4096 {
		return
	}
	if p.Reset != nil {
		p.Reset(v)
	}
	p.items = append(p.items, v)
}

// Len 池中空闲对象数量。
func (p *Pool[T]) Len() int {
	if p == nil {
		return 0
	}
	return len(p.items)
}

// Clear 清空池。
func (p *Pool[T]) Clear() {
	if p == nil {
		return
	}
	p.items = p.items[:0]
}

// Prewarm 预热：预先创建 n 个对象入池（开场加载时避免运行时分配）。
func (p *Pool[T]) Prewarm(n int) {
	if p == nil || p.New == nil || n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		p.items = append(p.items, p.New())
	}
}
