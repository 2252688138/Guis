// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// 并行作业系统：大型游戏里最耗时的是「每个实体的独立计算」
// （AI 决策 / 粒子更新 / 碰撞粗筛 / 网格生成 / 资源解码）。
//
//   - Jobs.Do：把 n 个独立任务铺到工作线程上（自适应窃取，主线程也参与）
//   - Jobs.Run：并行执行若干函数
//   - ParallelReduce：并行归约（求和 / 求最小 / 合并列表）
//   - Jobs.Submit + Wait：异步任务队列（资源加载 / 后台烘焙）
//
// 注意：Do 会阻塞直到全部完成；不要在作业内部再调用 Do（避免线程饥饿，
// 需要嵌套时用 Submit）。

// Jobs 并行作业池。
type Jobs struct {
	workers int
	queue   chan func()
	wg      sync.WaitGroup
	closed  atomic.Bool
	// Stats 统计（原子累加）。
	ran atomic.Int64
}

// DefaultWorkers 默认工作线程数（逻辑 CPU 数，上限 32）。
func DefaultWorkers() int {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > 32 {
		n = 32
	}
	return n
}

// NewJobs 创建工作池（workers <= 0 用 DefaultWorkers）。
func NewJobs(workers int) *Jobs {
	if workers <= 0 {
		workers = DefaultWorkers()
	}
	j := &Jobs{workers: workers, queue: make(chan func(), workers*8)}
	for i := 0; i < workers; i++ {
		go j.worker()
	}
	return j
}

// worker 消费异步任务。
func (j *Jobs) worker() {
	for fn := range j.queue {
		if fn != nil {
			fn()
			j.ran.Add(1)
		}
		j.wg.Done()
	}
}

// Workers 工作线程数。
func (j *Jobs) Workers() int {
	if j == nil {
		return 1
	}
	return j.workers
}

// Ran 已完成任务数。
func (j *Jobs) Ran() int64 {
	if j == nil {
		return 0
	}
	return j.ran.Load()
}

// Do 并行执行 n 个独立任务（fn 的 i 取值范围 0..n-1）。
//
// n 小于 2 或只有 1 个 worker 时退化为串行，调用开销接近直接循环。
func (j *Jobs) Do(n int, fn func(i int)) {
	if n <= 0 || fn == nil {
		return
	}
	workers := j.Workers()
	if workers > n {
		workers = n
	}
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&next, 1)) - 1
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}

// Run 并行执行若干函数。
func (j *Jobs) Run(tasks ...func()) {
	if len(tasks) == 0 {
		return
	}
	j.Do(len(tasks), func(i int) {
		if tasks[i] != nil {
			tasks[i]()
		}
	})
}

// Submit 提交异步任务（不等待，配合 Wait 使用）。
func (j *Jobs) Submit(fn func()) {
	if j == nil || fn == nil || j.closed.Load() {
		return
	}
	j.wg.Add(1)
	j.queue <- fn
}

// Wait 等待所有已提交的异步任务完成。
func (j *Jobs) Wait() {
	if j == nil {
		return
	}
	j.wg.Wait()
}

// Close 关闭工作池（等待在途任务后停止）。
func (j *Jobs) Close() {
	if j == nil || j.closed.Swap(true) {
		return
	}
	j.Wait()
	close(j.queue)
}

// ParallelReduce 并行归约：把 n 份结果按 merge 合并。
//
// fn 返回第 i 份结果，merge 需满足结合律（求和 / 取最小 / 拼接）。
func ParallelReduce[T any](j *Jobs, n int, fn func(i int) T, merge func(a, b T) T) T {
	var zero T
	if n <= 0 || fn == nil {
		return zero
	}
	workers := j.Workers()
	if workers > n {
		workers = n
	}
	if workers <= 1 || merge == nil {
		acc := fn(0)
		for i := 1; i < n; i++ {
			if merge != nil {
				acc = merge(acc, fn(i))
			} else {
				acc = fn(i)
			}
		}
		return acc
	}
	partial := make([]T, workers)
	var next int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			have := false
			var acc T
			for {
				i := int(atomic.AddInt64(&next, 1)) - 1
				if i >= n {
					break
				}
				v := fn(i)
				if !have {
					acc, have = v, true
					continue
				}
				acc = merge(acc, v)
			}
			partial[w] = acc
		}(w)
	}
	wg.Wait()
	acc := partial[0]
	for i := 1; i < workers; i++ {
		acc = merge(acc, partial[i])
	}
	return acc
}

// ForEachChunk 把 [0,n) 切成块并行处理（适合有局部性的循环）。
func (j *Jobs) ForEachChunk(n, chunk int, fn func(lo, hi int)) {
	if n <= 0 || fn == nil {
		return
	}
	if chunk <= 0 {
		chunk = 64
	}
	blocks := (n + chunk - 1) / chunk
	j.Do(blocks, func(b int) {
		lo := b * chunk
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		if lo < hi {
			fn(lo, hi)
		}
	})
}

// ---- 分帧任务调度：把一次性的大任务摊到多帧，避免卡帧（加载 / 烘焙） ----

// Budget 每帧时间预算（毫秒），超预算就留到下一帧。
type Budget struct {
	// MsPerFrame 每帧预算（默认 4ms）。
	MsPerFrame float64
	// MaxItemsPerFrame 每帧最多处理条目数（0 = 不限）。
	MaxItemsPerFrame int
}

type budgetTask struct {
	items []func()
	pos   int
	done  func()
}

// Scheduler 分帧调度器（主线程驱动，适合 IO 后的解码与对象构建）。
type Scheduler struct {
	budget Budget
	tasks  []*budgetTask
}

// NewScheduler 创建调度器。
func NewScheduler(b Budget) *Scheduler {
	if b.MsPerFrame <= 0 {
		b.MsPerFrame = 4
	}
	return &Scheduler{budget: b}
}

// Add 添加一个可分帧任务：items 逐个执行，全部完成后调用 done（可为 nil）。
func (s *Scheduler) Add(items []func(), done func()) {
	if s == nil || len(items) == 0 {
		if done != nil {
			done()
		}
		return
	}
	s.tasks = append(s.tasks, &budgetTask{items: items, done: done})
}

// Pending 未完成任务数。
func (s *Scheduler) Pending() int {
	if s == nil {
		return 0
	}
	return len(s.tasks)
}

// Tick 每帧调用一次：在预算内尽量推进，返回本帧执行的条目数。
func (s *Scheduler) Tick() int {
	if s == nil || len(s.tasks) == 0 {
		return 0
	}
	start := time.Now()
	ran := 0
	for i := 0; i < len(s.tasks); i++ {
		t := s.tasks[i]
		for t.pos < len(t.items) {
			if s.budget.MaxItemsPerFrame > 0 && ran >= s.budget.MaxItemsPerFrame {
				return ran
			}
			t.items[t.pos]()
			t.pos++
			ran++
			if time.Since(start).Seconds()*1000 >= s.budget.MsPerFrame {
				return ran
			}
		}
		if t.done != nil {
			t.done()
		}
		s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
		i--
	}
	return ran
}

// Flush 强制跑完全部任务（加载过场时用）。
func (s *Scheduler) Flush() int {
	if s == nil {
		return 0
	}
	ran := 0
	for len(s.tasks) > 0 {
		t := s.tasks[0]
		for t.pos < len(t.items) {
			t.items[t.pos]()
			t.pos++
			ran++
		}
		if t.done != nil {
			t.done()
		}
		s.tasks = s.tasks[1:]
	}
	return ran
}
