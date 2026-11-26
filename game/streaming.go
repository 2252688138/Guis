// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// 流式加载与大世界：
//   - Future / LoadAsync：后台加载（解码模型 / 生成网格），主线程随时取结果
//   - Streaming：按相机位置动态加载/卸载世界分块（无限世界）
//   - 与 Scheduler 配合可把构建摊到多帧，避免加载卡顿

// LoadContext 加载上下文（支持取消与进度上报）。
type LoadContext struct {
	canceled atomic.Bool
	progress atomic.Int32
}

// Cancel 请求取消（加载函数需自行检查）。
func (c *LoadContext) Cancel() {
	if c != nil {
		c.canceled.Store(true)
	}
}

// Canceled 是否已取消。
func (c *LoadContext) Canceled() bool {
	return c != nil && c.canceled.Load()
}

// SetProgress 上报进度（0..100）。
func (c *LoadContext) SetProgress(p int) {
	if c == nil {
		return
	}
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	c.progress.Store(int32(p))
}

// Progress 当前进度（0..100）。
func (c *LoadContext) Progress() int {
	if c == nil {
		return 0
	}
	return int(c.progress.Load())
}

// Future 异步结果句柄。
type Future[T any] struct {
	done chan struct{}
	val  T
	err  error
}

func newFuture[T any]() *Future[T] {
	return &Future[T]{done: make(chan struct{})}
}

// Ready 结果是否已就绪（不阻塞）。
func (f *Future[T]) Ready() bool {
	if f == nil {
		return true
	}
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// Wait 阻塞等待并返回结果。
func (f *Future[T]) Wait() (T, error) {
	var zero T
	if f == nil {
		return zero, nil
	}
	<-f.done
	return f.val, f.err
}

// TryGet 非阻塞取结果。
func (f *Future[T]) TryGet() (T, bool, error) {
	var zero T
	if f == nil {
		return zero, true, nil
	}
	select {
	case <-f.done:
		return f.val, true, f.err
	default:
		return zero, false, nil
	}
}

type loadEntry struct {
	f    any
	refs int
}

// Loader 异步加载器（按 key 去重 + 引用计数）。
type Loader struct {
	jobs    *Jobs
	mu      sync.Mutex
	entries map[string]*loadEntry
	loaded  int
}

// NewLoader 创建加载器（jobs 为 nil 时退化为同步加载）。
func NewLoader(jobs *Jobs) *Loader {
	return &Loader{jobs: jobs, entries: map[string]*loadEntry{}}
}

// LoadAsync 异步加载一个资源。
//
// 相同 key 多次调用会复用同一个 Future（引用计数递增，用 Release 释放）。
// 返回的 Future 可立即交给渲染循环，Ready() 为 true 时再取用。
func LoadAsync[T any](l *Loader, key string, load func(ctx *LoadContext) (T, error)) *Future[T] {
	if load == nil {
		f := newFuture[T]()
		close(f.done)
		return f
	}
	if l == nil {
		f := newFuture[T]()
		f.val, f.err = load(&LoadContext{})
		close(f.done)
		return f
	}
	l.mu.Lock()
	if e, ok := l.entries[key]; ok {
		if prev, ok2 := e.f.(*Future[T]); ok2 {
			e.refs++
			l.mu.Unlock()
			return prev
		}
	}
	f := newFuture[T]()
	l.entries[key] = &loadEntry{f: f, refs: 1}
	l.mu.Unlock()

	ctx := &LoadContext{}
	run := func() {
		v, err := load(ctx)
		f.val, f.err = v, err
		close(f.done)
		l.mu.Lock()
		l.loaded++
		l.mu.Unlock()
	}
	if l.jobs == nil {
		run()
		return f
	}
	l.jobs.Submit(run)
	return f
}

// Release 释放一次引用（引用归零时移出缓存）。
func (l *Loader) Release(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return
	}
	e.refs--
	if e.refs <= 0 {
		delete(l.entries, key)
	}
}

// Stats 已加载数量与在途数量。
func (l *Loader) Stats() (loaded, pending int) {
	if l == nil {
		return 0, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if f, ok := e.f.(interface{ Ready() bool }); ok && !f.Ready() {
			pending++
		}
	}
	return l.loaded, pending
}

// Clear 清空缓存（不取消已在途任务）。
func (l *Loader) Clear() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.entries = map[string]*loadEntry{}
	l.mu.Unlock()
}

// StreamChunk 世界分块状态。
type StreamChunk struct {
	// CX, CZ 分块坐标。
	CX, CZ int
	// Node 已加载的节点（nil = 未加载）。
	Node *Node
	// Pending 正在加载。
	Pending bool
	// Future 加载任务。
	Future *Future[*Node]
}

// Streaming 无限世界流式管理器。
//
//	sm := game.NewStreaming(64, 2, 3)
//	sm.Loader = func(cx, cz int) *Node { return buildChunk(cx, cz) }
//	每帧: sm.Update(cam.Pos, scene)
type Streaming struct {
	// CellSize 分块世界尺寸。
	CellSize float32
	// LoadRadius 加载半径（单位：块）。
	LoadRadius int
	// UnloadRadius 卸载半径（应 >= LoadRadius，默认 LoadRadius+1）。
	UnloadRadius int
	// Loader 分块构建函数（在后台线程调用，禁止触碰 Scene）。
	Loader func(cx, cz int) *Node
	// Async 是否后台加载（false = 加载半径内立即同步构建）。
	Async bool
	// Jobs 后台工作池（Async 时使用）。
	Jobs *Jobs
	// MaxMountPerFrame 每帧最多挂载的分块数（0 = 不限）：
	// 防止一次跨区块给一帧塞进几十个网格而卡顿。
	MaxMountPerFrame int

	chunks map[[2]int]*StreamChunk
	// Loaded / Unloaded 累计计数。
	Loaded, Unloaded int
}

// NewStreaming 创建流式管理器。
func NewStreaming(cellSize float32, loadRadius, unloadRadius int) *Streaming {
	if cellSize <= 0 {
		cellSize = 64
	}
	if loadRadius < 1 {
		loadRadius = 1
	}
	if unloadRadius <= loadRadius {
		unloadRadius = loadRadius + 1
	}
	return &Streaming{
		CellSize:     cellSize,
		LoadRadius:   loadRadius,
		UnloadRadius: unloadRadius,
		chunks:       map[[2]int]*StreamChunk{},
	}
}

// CellOf 世界坐标 → 分块坐标。
func (s *Streaming) CellOf(p Vec3) (int, int) {
	if s == nil || s.CellSize <= 0 {
		return 0, 0
	}
	return int(math.Floor(float64(p.X / s.CellSize))), int(math.Floor(float64(p.Z / s.CellSize)))
}

// ChunkAt 取分块状态（不存在返回 nil）。
func (s *Streaming) ChunkAt(cx, cz int) *StreamChunk {
	if s == nil {
		return nil
	}
	return s.chunks[[2]int{cx, cz}]
}

// Update 按相机位置加载/卸载分块，返回本次加载与卸载数量。
func (s *Streaming) Update(camPos Vec3, sc *Scene) (loaded, unloaded int) {
	if s == nil || s.Loader == nil || sc == nil {
		return 0, 0
	}
	ccx, ccz := s.CellOf(camPos)
	budget := s.MaxMountPerFrame
	mounted := 0

	// 加载：候选分块按「离相机近」优先，受 MaxMountPerFrame 预算限制
	type cand struct{ dx, dz, d2 int }
	cands := make([]cand, 0, (s.LoadRadius*2+1)*(s.LoadRadius*2+1))
	for dz := -s.LoadRadius; dz <= s.LoadRadius; dz++ {
		for dx := -s.LoadRadius; dx <= s.LoadRadius; dx++ {
			if _, ok := s.chunks[[2]int{ccx + dx, ccz + dz}]; ok {
				continue
			}
			cands = append(cands, cand{dx, dz, dx*dx + dz*dz})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].d2 < cands[j].d2 })
	for _, cd := range cands {
		cx, cz := ccx+cd.dx, ccz+cd.dz
		if s.Async && s.Jobs != nil {
			// 异步加载不占挂载预算（后台线程跑），只管提交
			c := &StreamChunk{CX: cx, CZ: cz, Pending: true}
			s.chunks[[2]int{cx, cz}] = c
			x, z := cx, cz
			f := newFuture[*Node]()
			c.Future = f
			s.Jobs.Submit(func() {
				f.val = s.Loader(x, z)
				close(f.done)
			})
			continue
		}
		if budget > 0 && mounted >= budget {
			break // 本帧预算用完，剩下的候选下一帧再处理
		}
		c := &StreamChunk{CX: cx, CZ: cz}
		s.chunks[[2]int{cx, cz}] = c
		c.Node = s.Loader(cx, cz)
		mounted++
		if c.Node != nil {
			sc.Root.Add(c.Node)
			s.Loaded++
			loaded++
		}
	}
	// 回收在途的异步结果（受挂载预算限制，超出的留到下一帧）
	for _, c := range s.chunks {
		if !c.Pending || c.Future == nil || !c.Future.Ready() {
			continue
		}
		n, _, _ := c.Future.TryGet()
		if n == nil {
			c.Pending = false
			continue
		}
		if budget > 0 && mounted >= budget {
			continue
		}
		c.Pending = false
		c.Node = n
		mounted++
		sc.Root.Add(n)
		s.Loaded++
		loaded++
	}
	// 卸载
	for key, c := range s.chunks {
		dx := key[0] - ccx
		dz := key[1] - ccz
		if dx < 0 {
			dx = -dx
		}
		if dz < 0 {
			dz = -dz
		}
		if dx <= s.UnloadRadius && dz <= s.UnloadRadius {
			continue
		}
		if c.Node != nil {
			removeNode(sc.Root, c.Node)
			s.Unloaded++
			unloaded++
		}
		delete(s.chunks, key)
	}
	return loaded, unloaded
}

// StreamingStats 流式加载统计快照。
type StreamingStats struct {
	// Chunks 已登记分块数（含在途）。
	Chunks int
	// Loaded 当前已挂载到场景的分块数。
	Loaded int
	// Pending 在途加载数。
	Pending int
	// TotalLoaded 累计挂载次数。
	TotalLoaded int
	// TotalUnloaded 累计卸载次数。
	TotalUnloaded int
}

// Stats 返回流式加载统计快照。
func (s *Streaming) Stats() StreamingStats {
	st := StreamingStats{}
	if s == nil {
		return st
	}
	st.Chunks = len(s.chunks)
	for _, c := range s.chunks {
		if c.Pending {
			st.Pending++
		}
		if c.Node != nil {
			st.Loaded++
		}
	}
	st.TotalLoaded = s.Loaded
	st.TotalUnloaded = s.Unloaded
	return st
}

// Chunks 已登记分块数。
func (s *Streaming) Chunks() int {
	if s == nil {
		return 0
	}
	return len(s.chunks)
}

// Clear 卸载全部（不触发计数）。
func (s *Streaming) Clear(sc *Scene) {
	if s == nil {
		return
	}
	if sc != nil {
		for _, c := range s.chunks {
			if c.Node != nil {
				removeNode(sc.Root, c.Node)
			}
		}
	}
	s.chunks = map[[2]int]*StreamChunk{}
}

// removeNode 从父节点列表中移除子节点。
func removeNode(parent, child *Node) {
	if parent == nil || child == nil {
		return
	}
	for i, c := range parent.Children {
		if c == child {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}
