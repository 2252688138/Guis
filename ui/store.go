// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "sync"

// 应用级原语：Store（可订阅状态）/ Resource（异步资源）/ Router（页面栈）。
//
// 框架本身只到「组件库」这一层：每帧重建节点树，交互状态按 Key 继承。
// 真实应用还需要三样东西，之前每个应用都在自己重复实现，这里收敛成一套：
//
//	var theme = ui.NewStore("dark")        // 全局状态，改了就重绘
//	var profile = ui.NewResource[Profile]() // 异步加载 + 加载/错误/重试
//	var nav = ui.NewRouter()                // 页面栈 + 返回
//
// 约定：三者都只在「UI 线程」（view 回调 / 事件回调）里读，写入可以从任意
// goroutine 发起，内部会自行 RequestRedraw。

// ---- Store ----

// Store 单值可订阅状态。
//
// 写入在任意 goroutine 安全；订阅回调在写入方 goroutine 内同步执行，
// 因此回调里只做「置状态 / 请求重绘」，不要直接跑长任务。
type Store[T any] struct {
	mu   sync.RWMutex
	v    T
	rev  uint64
	subs []func(T)
}

// NewStore 创建状态容器。
func NewStore[T any](init T) *Store[T] { return &Store[T]{v: init} }

// Get 读取当前值。
func (s *Store[T]) Get() T {
	s.mu.RLock()
	v := s.v
	s.mu.RUnlock()
	return v
}

// Rev 变更代号（每次实际变化 +1）；用于判断「这帧要不要重算」。
func (s *Store[T]) Rev() uint64 {
	s.mu.RLock()
	r := s.rev
	s.mu.RUnlock()
	return r
}

// Set 设置新值；值未变时不通知、不重绘（返回 false）。
func (s *Store[T]) Set(v T) bool {
	s.mu.Lock()
	if storeEqual(s.v, v) {
		s.mu.Unlock()
		return false
	}
	s.v = v
	s.rev++
	// 复制一份非 nil 订阅者再解锁回调：Subscribe 的取消只把槽位置空，
	// 这里跳过 nil；回调在锁外执行，避免回调里再读 Store 造成死锁。
	subs := make([]func(T), 0, len(s.subs))
	for _, fn := range s.subs {
		if fn != nil {
			subs = append(subs, fn)
		}
	}
	s.mu.Unlock()

	for _, fn := range subs {
		fn(v)
	}
	RequestRedraw()
	return true
}

// Update 基于旧值计算新值（读改写在一个锁里完成）。
func (s *Store[T]) Update(fn func(T) T) bool {
	if fn == nil {
		return false
	}
	s.mu.RLock()
	old := s.v
	s.mu.RUnlock()
	return s.Set(fn(old))
}

// Subscribe 订阅后续变化（不会立即回调）；返回取消函数。
func (s *Store[T]) Subscribe(fn func(T)) func() {
	if fn == nil {
		return func() {}
	}
	s.mu.Lock()
	s.subs = append(s.subs, fn)
	idx := len(s.subs) - 1
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			// 用置空而不是切片删除：保持其它订阅者的下标稳定。
			if idx < len(s.subs) {
				s.subs[idx] = nil
			}
			s.mu.Unlock()
		})
	}
}

// storeEqual 比较两个值是否相等；不可比较类型（切片 / map / 函数）一律视为「不等」。
func storeEqual[T any](a, b T) (eq bool) {
	defer func() {
		if recover() != nil {
			eq = false
		}
	}()
	return any(a) == any(b)
}

// ---- Resource ----

// Resource 异步资源（一次加载 + 手动重载），自带「加载中 / 错误 / 数据」三态。
//
//	var list = ui.NewResource[[]Item]()
//
//	func view() *ui.Node {
//	    list.Load(func() ([]Item, error) { return api.List() }) // 幂等，重复调用不会重复拉
//	    return list.Section(func(items []Item) *ui.Node { /* 有数据时 */ })
//	}
type Resource[T any] struct {
	mu      sync.Mutex
	data    T
	err     error
	loading bool
	loaded  bool
	gen     int
}

// NewResource 创建资源容器。
func NewResource[T any]() *Resource[T] { return &Resource[T]{} }

// Load 首次调用时发起异步加载；已加载 / 加载中时直接返回（幂等）。
func (r *Resource[T]) Load(fetch func() (T, error)) *Resource[T] {
	if fetch == nil {
		return r
	}
	r.mu.Lock()
	if r.loaded || r.loading {
		r.mu.Unlock()
		return r
	}
	r.loading = true
	r.gen++
	gen := r.gen
	r.mu.Unlock()

	go r.run(fetch, gen)
	return r
}

// Reload 强制重新加载（保留旧数据直到新数据到达，界面不会闪空）。
func (r *Resource[T]) Reload(fetch func() (T, error)) *Resource[T] {
	if fetch == nil {
		return r
	}
	r.mu.Lock()
	r.loading = true
	r.gen++
	gen := r.gen
	r.mu.Unlock()

	go r.run(fetch, gen)
	return r
}

// Invalidate 丢弃已加载标记，下次 Load 会重新发起。
func (r *Resource[T]) Invalidate() {
	r.mu.Lock()
	r.loaded = false
	r.mu.Unlock()
}

func (r *Resource[T]) run(fetch func() (T, error), gen int) {
	data, err := fetch()

	r.mu.Lock()
	if gen != r.gen { // 被新一次加载取代，结果作废
		r.mu.Unlock()
		return
	}
	r.loading = false
	r.loaded = err == nil
	r.data, r.err = data, err
	r.mu.Unlock()

	RequestRedraw()
}

// State 读取当前三态。
func (r *Resource[T]) State() (data T, loading bool, err error) {
	r.mu.Lock()
	data, loading, err = r.data, r.loading, r.err
	r.mu.Unlock()
	return
}

// Section 按状态渲染：加载中 / 错误（可重试）/ 有数据。
//
// retry 为 nil 时不显示重试按钮；empty 由调用方在 body 里自行判断（例如列表为空时
// 直接返回 EmptyState），保持 Section 只关心「异步三态」。
func (r *Resource[T]) Section(retry func() (T, error), body func(T) *Node) *Node {
	data, loading, err := r.State()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	var onRetry func()
	if retry != nil {
		onRetry = func() { r.Reload(retry) }
	}
	return AsyncSection(loading && !hasResourceData(r, data), errMsg, false, onRetry, func() *Node {
		if body == nil {
			return nil
		}
		return body(data)
	})
}

// hasResourceData 判断是否已有可用数据（有数据时重载不退回骨架屏）。
func hasResourceData[T any](r *Resource[T], data T) bool {
	r.mu.Lock()
	loaded := r.loaded
	r.mu.Unlock()
	if loaded {
		return true
	}
	// 零值兜底：T 为指针/slice/map 时非 nil 也算有数据。
	if !storeEqual(data, *new(T)) {
		return true
	}
	return false
}

// ---- Router ----

// Route 一个页面。
type Route struct {
	Key string
	// Build 构建页面节点；args 为 Push 时传入的参数。
	Build func(args any) *Node
}

// Router 页面栈（Push / Pop / Replace），配合 view() 返回 Router.View()。
//
//	var nav = ui.NewRouter()
//
//	func init() {
//	    nav.Register("home", homePage).Register("detail", detailPage)
//	    nav.Push("home", nil)
//	}
//
//	func view() *ui.Node { return nav.View() }
//
// 页面栈变化会自行 RequestRedraw；转场动画可在外层包 ui.Anim。
type Router struct {
	mu     sync.Mutex
	routes map[string]Route
	stack  []routeFrame
}

type routeFrame struct {
	key  string
	args any
}

// NewRouter 创建页面栈。
func NewRouter() *Router { return &Router{routes: make(map[string]Route, 8)} }

// Register 注册页面（链式）。
func (r *Router) Register(key string, build func(args any) *Node) *Router {
	if key == "" || build == nil {
		return r
	}
	r.mu.Lock()
	r.routes[key] = Route{Key: key, Build: build}
	r.mu.Unlock()
	return r
}

// Push 入栈并切换过去；页面未注册时返回 false。
func (r *Router) Push(key string, args any) bool {
	r.mu.Lock()
	if _, ok := r.routes[key]; !ok {
		r.mu.Unlock()
		return false
	}
	r.stack = append(r.stack, routeFrame{key: key, args: args})
	r.mu.Unlock()
	RequestRedraw()
	return true
}

// Replace 替换栈顶（用于「登录后替换首页」这类不保留返回的场景）。
func (r *Router) Replace(key string, args any) bool {
	r.mu.Lock()
	if _, ok := r.routes[key]; !ok {
		r.mu.Unlock()
		return false
	}
	if len(r.stack) == 0 {
		r.stack = append(r.stack, routeFrame{key: key, args: args})
	} else {
		r.stack[len(r.stack)-1] = routeFrame{key: key, args: args}
	}
	r.mu.Unlock()
	RequestRedraw()
	return true
}

// Pop 出栈；已是最后一页时返回 false（不把栈清空）。
func (r *Router) Pop() bool {
	r.mu.Lock()
	if len(r.stack) <= 1 {
		r.mu.Unlock()
		return false
	}
	r.stack = r.stack[:len(r.stack)-1]
	r.mu.Unlock()
	RequestRedraw()
	return true
}

// PopTo 回退到指定页面（栈里第一次出现的位置）。
func (r *Router) PopTo(key string) bool {
	r.mu.Lock()
	for i := len(r.stack) - 1; i >= 0; i-- {
		if r.stack[i].key == key {
			if i == len(r.stack)-1 {
				r.mu.Unlock()
				return false
			}
			r.stack = r.stack[:i+1]
			r.mu.Unlock()
			RequestRedraw()
			return true
		}
	}
	r.mu.Unlock()
	return false
}

// Len 栈深度。
func (r *Router) Len() int {
	r.mu.Lock()
	n := len(r.stack)
	r.mu.Unlock()
	return n
}

// CanPop 是否可返回（页面据此决定是否显示返回按钮）。
func (r *Router) CanPop() bool { return r.Len() > 1 }

// Current 当前页面 key 与参数。
func (r *Router) Current() (string, any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stack) == 0 {
		return "", nil, false
	}
	top := r.stack[len(r.stack)-1]
	return top.key, top.args, true
}

// View 当前栈顶页面的节点树；栈空或页面未注册时返回 nil。
func (r *Router) View() *Node {
	r.mu.Lock()
	if len(r.stack) == 0 {
		r.mu.Unlock()
		return nil
	}
	top := r.stack[len(r.stack)-1]
	route, ok := r.routes[top.key]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return route.Build(top.args)
}
