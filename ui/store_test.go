// guis — Store / Resource / Router 回归用例

package ui

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// ---- Store ----

func TestStoreSetGetRev(t *testing.T) {
	s := NewStore(1)
	if s.Get() != 1 {
		t.Fatalf("初值 = %d", s.Get())
	}
	rev0 := s.Rev()
	if !s.Set(2) {
		t.Fatal("值变化时 Set 应返回 true")
	}
	if s.Get() != 2 || s.Rev() != rev0+1 {
		t.Fatalf("Set 后 Get=%d Rev=%d", s.Get(), s.Rev())
	}
}

// 值未变化时不通知、不递增 rev。
func TestStoreSetSameValueNoNotify(t *testing.T) {
	s := NewStore("dark")
	var calls int32
	s.Subscribe(func(string) { atomic.AddInt32(&calls, 1) })

	if s.Set("dark") {
		t.Fatal("相同值 Set 应返回 false")
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Fatalf("相同值不应通知，实际 %d 次", n)
	}
	if s.Rev() != 0 {
		t.Fatalf("rev 不应变化：%d", s.Rev())
	}
}

func TestStoreSubscribeCancel(t *testing.T) {
	s := NewStore(0)
	var got int32
	cancel := s.Subscribe(func(v int) { atomic.AddInt32(&got, int32(v)) })
	s.Set(3)
	if atomic.LoadInt32(&got) != 3 {
		t.Fatalf("回调未收到值：%d", got)
	}
	cancel()
	cancel() // 幂等，不应 panic
	s.Set(4)
	if atomic.LoadInt32(&got) != 3 {
		t.Fatalf("取消后不应再收到回调：%d", got)
	}
}

func TestStoreUpdate(t *testing.T) {
	s := NewStore(10)
	if !s.Update(func(v int) int { return v + 5 }) {
		t.Fatal("Update 应返回 true")
	}
	if s.Get() != 15 {
		t.Fatalf("Update 后 = %d", s.Get())
	}
	if s.Update(nil) {
		t.Fatal("nil Updater 应返回 false")
	}
}

// 不可比较类型（切片）不应 panic。
func TestStoreUncomparableValue(t *testing.T) {
	s := NewStore([]int{1, 2})
	if got := s.Get(); len(got) != 2 {
		t.Fatalf("初值 = %v", got)
	}
	if !s.Set([]int{1, 2, 3}) {
		t.Fatal("切片 Set 应视为变化")
	}
	if got := s.Get(); len(got) != 3 {
		t.Fatalf("Set 后 = %v", got)
	}
}

// ---- Resource ----

func TestResourceLoadAsync(t *testing.T) {
	r := NewResource[string]()
	r.Load(func() (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "ok", nil
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, loading, err := r.State()
		if !loading {
			if err != nil || data != "ok" {
				t.Fatalf("结果 data=%q err=%v", data, err)
			}
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("加载超时")
}

// Load 幂等：重复调用只拉一次。
func TestResourceLoadIdempotent(t *testing.T) {
	r := NewResource[int]()
	var calls int32
	fetch := func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 7, nil
	}
	r.Load(fetch)
	r.Load(fetch)
	r.Load(fetch)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, loading, _ := r.State(); !loading {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("Load 应只拉一次，实际 %d 次", n)
	}
}

func TestResourceErrorAndReload(t *testing.T) {
	r := NewResource[string]()
	r.Load(func() (string, error) { return "", errors.New("boom") })

	waitResource(t, r)
	if _, _, err := r.State(); err == nil || err.Error() != "boom" {
		t.Fatalf("应记录错误：%v", err)
	}

	r.Reload(func() (string, error) { return "fixed", nil })
	waitResource(t, r)
	data, _, err := r.State()
	if err != nil || data != "fixed" {
		t.Fatalf("重载后 data=%q err=%v", data, err)
	}
}

// 旧一次加载的结果不应覆盖新一次。
func TestResourceStaleResultIgnored(t *testing.T) {
	r := NewResource[string]()
	r.Load(func() (string, error) {
		time.Sleep(20 * time.Millisecond)
		return "old", nil
	})
	r.Reload(func() (string, error) { return "new", nil })

	waitResource(t, r)
	// 等足够久，确保「慢的旧加载」也有机会写回
	time.Sleep(40 * time.Millisecond)
	if data, _, _ := r.State(); data != "new" {
		t.Fatalf("慢结果覆盖了新结果：%q", data)
	}
}

// Section 三态：加载中 → 骨架；错误 → 错误态；有数据 → body。
func TestResourceSection(t *testing.T) {
	r := NewResource[int]()
	r.Load(func() (int, error) { return 42, nil })
	waitResource(t, r)

	called := false
	n := r.Section(nil, func(v int) *Node {
		called = true
		if v != 42 {
			t.Fatalf("body 收到 %d", v)
		}
		return Text("data")
	})
	if !called || n == nil {
		t.Fatal("有数据时应渲染 body")
	}
}

func waitResource[T any](t *testing.T, r *Resource[T]) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, loading, _ := r.State(); !loading {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("资源加载超时")
}

// ---- Router ----

func TestRouterBasic(t *testing.T) {
	nav := NewRouter()
	nav.Register("home", func(any) *Node { return Text("home") }).
		Register("detail", func(args any) *Node { return Text(args.(string)) })

	if nav.Len() != 0 || nav.CanPop() || nav.View() != nil {
		t.Fatal("空栈状态不对")
	}
	if !nav.Push("home", nil) {
		t.Fatal("Push home 失败")
	}
	if nav.CanPop() {
		t.Fatal("单页不应可返回")
	}
	if !nav.Push("detail", "d1") {
		t.Fatal("Push detail 失败")
	}
	if key, args, ok := nav.Current(); !ok || key != "detail" || args != "d1" {
		t.Fatalf("Current = %q %v %v", key, args, ok)
	}
	if nav.View() == nil || nav.View().Text != "d1" {
		t.Fatal("View 应构建 detail 页")
	}
	if !nav.Pop() || nav.Current2() != "home" {
		t.Fatal("Pop 后应回到 home")
	}
	if nav.Pop() {
		t.Fatal("最后一页不应被 Pop")
	}
}

// Current2 测试辅助：当前页面 key。
func (r *Router) Current2() string {
	k, _, _ := r.Current()
	return k
}

func TestRouterUnregistered(t *testing.T) {
	nav := NewRouter()
	if nav.Push("nope", nil) {
		t.Fatal("未注册页面 Push 应失败")
	}
	if nav.Replace("nope", nil) {
		t.Fatal("未注册页面 Replace 应失败")
	}
	if nav.Len() != 0 {
		t.Fatal("失败的 Push 不应改变栈")
	}
}

func TestRouterReplace(t *testing.T) {
	nav := NewRouter()
	nav.Register("a", func(any) *Node { return Text("a") }).
		Register("b", func(any) *Node { return Text("b") })
	nav.Push("a", nil)
	if !nav.Replace("b", nil) {
		t.Fatal("Replace 失败")
	}
	if nav.Len() != 1 || nav.Current2() != "b" {
		t.Fatalf("Replace 后 len=%d key=%q", nav.Len(), nav.Current2())
	}
}

func TestRouterPopTo(t *testing.T) {
	nav := NewRouter()
	for _, k := range []string{"a", "b", "c"} {
		nav.Register(k, func(any) *Node { return Text(k) })
	}
	nav.Push("a", nil)
	nav.Push("b", nil)
	nav.Push("c", nil)

	if !nav.PopTo("a") {
		t.Fatal("PopTo a 失败")
	}
	if nav.Len() != 1 || nav.Current2() != "a" {
		t.Fatalf("PopTo 后 len=%d key=%q", nav.Len(), nav.Current2())
	}
	if nav.PopTo("nope") {
		t.Fatal("不存在的目标应返回 false")
	}
}

// 并发读写状态不应触发 race（配合 -race 运行）。
func TestStoreConcurrent(t *testing.T) {
	s := NewStore(0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			s.Update(func(v int) int { return v + 1 })
			_ = s.Get()
		}
	}()
	for i := 0; i < 100; i++ {
		_ = s.Get()
		_ = s.Rev()
	}
	<-done
	if s.Get() != 100 {
		t.Fatalf("并发 Update 结果 = %d", s.Get())
	}
}
