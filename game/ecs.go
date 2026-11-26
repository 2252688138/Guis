// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"reflect"
)

// 轻量 ECS（实体-组件-系统）。
//
// 定位：大型游戏里成千上万个实体（小怪 / 子弹 / 掉落物 / 粒子宿主），
// 用「结构体数组 + 稀疏索引」代替成千上万个 *Node，遍历更省内存、更快。
//
// 渲染侧仍可把 ECS 数据同步到 Scene（见 SyncToScene），碰撞用 Collide 系列，
// 寻路用 NavGrid，二者正交。
//
// 用法：
//
//	type Health struct{ HP int }
//	e := w.Spawn()
//	game.Add(&w, e, Health{HP: 100})
//	game.Query1(&w, func(e game.Entity, h *Health) { h.HP-- })

// Entity 实体句柄（索引 + 代号，删除后代号自增，避免悬空引用）。
type Entity struct {
	index uint32
	gen   uint32
}

// Valid 句柄是否已初始化。
func (e Entity) Valid() bool { return e.gen != 0 }

// Index 实体索引（调试 / 序列化用）。
func (e Entity) Index() int { return int(e.index) }

// ComponentStore 组件存储通用接口（内部用）。
type ComponentStore interface {
	// Len 组件数量。
	Len() int
	// Entities 实体索引列表（稠密数组，可直接遍历）。
	Entities() []uint32
	// RemoveEntity 删除某实体的组件。
	RemoveEntity(id uint32)
}

// Store 单一组件类型的稠密存储（稀疏索引 + 稠密数组）。
type Store[T any] struct {
	sparse map[uint32]int
	dense  []uint32
	data   []T
}

// NewStore 创建组件存储。
func NewStore[T any]() *Store[T] {
	return &Store[T]{sparse: make(map[uint32]int, 64)}
}

// Len 组件数量。
func (s *Store[T]) Len() int { return len(s.dense) }

// Entities 拥有该组件的实体索引。
func (s *Store[T]) Entities() []uint32 { return s.dense }

// Add 写入（覆盖）。
func (s *Store[T]) Add(id uint32, v T) {
	if idx, ok := s.sparse[id]; ok {
		s.data[idx] = v
		return
	}
	s.sparse[id] = len(s.dense)
	s.dense = append(s.dense, id)
	s.data = append(s.data, v)
}

// Get 读取。
func (s *Store[T]) Get(id uint32) (T, bool) {
	if idx, ok := s.sparse[id]; ok {
		return s.data[idx], true
	}
	var zero T
	return zero, false
}

// GetPtr 取指针（允许原地修改）。
func (s *Store[T]) GetPtr(id uint32) *T {
	if idx, ok := s.sparse[id]; ok {
		return &s.data[idx]
	}
	return nil
}

// Remove 删除（末位交换，O(1)）。
func (s *Store[T]) Remove(id uint32) {
	idx, ok := s.sparse[id]
	if !ok {
		return
	}
	last := len(s.dense) - 1
	if idx != last {
		s.dense[idx] = s.dense[last]
		s.data[idx] = s.data[last]
		s.sparse[s.dense[idx]] = idx
	}
	s.dense = s.dense[:last]
	s.data = s.data[:last]
	delete(s.sparse, id)
}

// RemoveEntity 实现 ComponentStore。
func (s *Store[T]) RemoveEntity(id uint32) { s.Remove(id) }

// ForEach 遍历（顺序为稠密数组顺序）。
func (s *Store[T]) ForEach(fn func(id uint32, v *T)) {
	for i := range s.dense {
		fn(s.dense[i], &s.data[i])
	}
}

// ECSWorld ECS 世界。
type ECSWorld struct {
	gen     []uint32
	alive   []bool
	freeIDs []uint32
	stores  map[reflect.Type]ComponentStore
	// order 系统执行顺序（PreUpdate / Update / PostUpdate 三个阶段）。
	pre, upd, post []SystemFn
}

// SystemFn 系统更新函数。
type SystemFn struct {
	// Name 系统名（调试）。
	Name string
	// Fn 更新回调（dt 为秒）。
	Fn func(w *ECSWorld, dt float32)
}

// NewWorld 创建 ECS 世界。
func NewECSWorld() *ECSWorld {
	return &ECSWorld{stores: map[reflect.Type]ComponentStore{}}
}

// Spawn 创建实体。
func (w *ECSWorld) Spawn() Entity {
	if n := len(w.freeIDs); n > 0 {
		id := w.freeIDs[n-1]
		w.freeIDs = w.freeIDs[:n-1]
		w.alive[id] = true
		w.gen[id]++
		return Entity{index: id, gen: w.gen[id]}
	}
	id := uint32(len(w.gen))
	w.gen = append(w.gen, 1)
	w.alive = append(w.alive, true)
	return Entity{index: id, gen: 1}
}

// Alive 实体是否存活。
func (w *ECSWorld) Alive(e Entity) bool {
	i := e.index
	return int(i) < len(w.gen) && w.alive[i] && w.gen[i] == e.gen
}

// Despawn 销毁实体并移除其全部组件。
func (w *ECSWorld) Despawn(e Entity) {
	if !w.Alive(e) {
		return
	}
	for _, s := range w.stores {
		s.RemoveEntity(e.index)
	}
	w.alive[e.index] = false
	w.freeIDs = append(w.freeIDs, e.index)
}

// Entity 由索引还原句柄（遍历 ComponentStore 时用）。
func (w *ECSWorld) Entity(index uint32) Entity {
	if int(index) >= len(w.gen) || !w.alive[index] {
		return Entity{}
	}
	return Entity{index: index, gen: w.gen[index]}
}

// Entities 当前存活实体数。
func (w *ECSWorld) Entities() int {
	n := 0
	for _, a := range w.alive {
		if a {
			n++
		}
	}
	return n
}

// RegisterStore 注册组件存储（可选；StoreOf 会惰性注册）。
func RegisterStore[T any](w *ECSWorld) *Store[T] {
	return StoreOf[T](w)
}

// StoreOf 取组件存储（不存在则创建）。
func StoreOf[T any](w *ECSWorld) *Store[T] {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if s, ok := w.stores[t]; ok {
		if st, ok2 := s.(*Store[T]); ok2 {
			return st
		}
	}
	st := NewStore[T]()
	w.stores[t] = st
	return st
}

// Add 给实体挂组件。
func Add[T any](w *ECSWorld, e Entity, v T) {
	if !w.Alive(e) {
		return
	}
	StoreOf[T](w).Add(e.index, v)
}

// Get 读组件。
func Get[T any](w *ECSWorld, e Entity) (T, bool) {
	if !w.Alive(e) {
		var zero T
		return zero, false
	}
	return StoreOf[T](w).Get(e.index)
}

// GetPtr 取组件指针（原地修改）。
func GetPtr[T any](w *ECSWorld, e Entity) *T {
	if !w.Alive(e) {
		return nil
	}
	return StoreOf[T](w).GetPtr(e.index)
}

// Remove 移除组件。
func Remove[T any](w *ECSWorld, e Entity) {
	StoreOf[T](w).Remove(e.index)
}

// Has 是否拥有组件。
func Has[T any](w *ECSWorld, e Entity) bool {
	_, ok := Get[T](w, e)
	return ok
}

// Query1 遍历拥有组件 T 的实体。
func Query1[T any](w *ECSWorld, fn func(e Entity, c *T)) {
	s := StoreOf[T](w)
	s.ForEach(func(id uint32, v *T) {
		if e := w.Entity(id); e.Valid() {
			fn(e, v)
		}
	})
}

// Query2 遍历同时拥有 A、B 的实体（以较小集合为主）。
func Query2[A, B any](w *ECSWorld, fn func(e Entity, a *A, b *B)) {
	sa, sb := StoreOf[A](w), StoreOf[B](w)
	ids := sa.Entities()
	if sb.Len() < sa.Len() {
		ids = sb.Entities()
	}
	for _, id := range ids {
		pa := sa.GetPtr(id)
		pb := sb.GetPtr(id)
		if pa == nil || pb == nil {
			continue
		}
		if e := w.Entity(id); e.Valid() {
			fn(e, pa, pb)
		}
	}
}

// Query3 遍历同时拥有 A、B、C 的实体。
func Query3[A, B, C any](w *ECSWorld, fn func(e Entity, a *A, b *B, c *C)) {
	sa, sb, sc := StoreOf[A](w), StoreOf[B](w), StoreOf[C](w)
	ids := sa.Entities()
	if sb.Len() < len(ids) {
		ids = sb.Entities()
	}
	if sc.Len() < len(ids) {
		ids = sc.Entities()
	}
	for _, id := range ids {
		pa := sa.GetPtr(id)
		pb := sb.GetPtr(id)
		pc := sc.GetPtr(id)
		if pa == nil || pb == nil || pc == nil {
			continue
		}
		if e := w.Entity(id); e.Valid() {
			fn(e, pa, pb, pc)
		}
	}
}

// System 注册系统：phase 为 "pre" / "update"（默认） / "post"。
func (w *ECSWorld) System(phase, name string, fn func(w *ECSWorld, dt float32)) {
	s := SystemFn{Name: name, Fn: fn}
	switch phase {
	case "pre":
		w.pre = append(w.pre, s)
	case "post":
		w.post = append(w.post, s)
	default:
		w.upd = append(w.upd, s)
	}
}

// Update 推进一帧：依次执行 pre → update → post 系统。
func (w *ECSWorld) Update(dt float32) {
	for _, s := range w.pre {
		s.Fn(w, dt)
	}
	for _, s := range w.upd {
		s.Fn(w, dt)
	}
	for _, s := range w.post {
		s.Fn(w, dt)
	}
}

// Clear 清空世界（保留系统注册）。
func (w *ECSWorld) Clear() {
	w.gen = w.gen[:0]
	w.alive = w.alive[:0]
	w.freeIDs = w.freeIDs[:0]
	for k := range w.stores {
		delete(w.stores, k)
	}
}

// Transform 常用组件：世界变换（位置 / 旋转 / 缩放）。
type Transform struct {
	Pos Vec3
	Rot Vec3
	Scl Vec3
}

// NewTransform 默认变换（缩放 1）。
func NewTransform(pos Vec3) Transform {
	return Transform{Pos: pos, Scl: V3(1, 1, 1)}
}

// Matrix 组合变换矩阵。
func (t Transform) Matrix() Mat4 {
	scl := t.Scl
	if scl == (Vec3{}) {
		scl = V3(1, 1, 1)
	}
	m := Translate(t.Pos).Mul(RotateEuler(t.Rot.X, t.Rot.Y, t.Rot.Z)).Mul(Scale(scl))
	return m
}

// SyncToScene 把 ECS 中「Transform + Renderable」的实体同步到 Scene。
//
// 大型游戏里通常每帧同步相机附近的实体，这样渲染侧仍是熟悉的 Scene/Node。
// 传入的 match 用于过滤（例如只同步相机周围 N 米），可为 nil。
func SyncToScene(w *ECSWorld, sc *Scene, match func(e Entity, t *Transform) bool) {
	if w == nil || sc == nil {
		return
	}
	// 复用已有节点：以实体索引命名的节点
	existing := map[string]*Node{}
	sc.Root.Each(func(n *Node) {
		if n.Name != "" {
			existing[n.Name] = n
		}
	})
	Query2(w, func(e Entity, t *Transform, r *Renderable) {
		_ = r
		if match != nil && !match(e, t) {
			return
		}
		name := "ecs:" + entityKey(e)
		node, ok := existing[name]
		if !ok {
			node = NewMeshNode(name, r.Mesh, r.Material)
			sc.Root.Add(node)
		}
		node.Pos, node.Rot, node.Scl = t.Pos, t.Rot, t.Scl
		node.Mesh, node.Material = r.Mesh, r.Material
	})
}

// Renderable 常用组件：可渲染（网格 + 材质）。
type Renderable struct {
	Mesh     *Mesh
	Material *Material
	// Visible 为 false 时不同步。
	Visible bool
}

// entityKey 实体稳定键（索引十进制串）。
func entityKey(e Entity) string {
	if e.index == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	v := e.index
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}