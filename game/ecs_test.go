// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"sync"
	"testing"
	"time"
)

type testHealth struct{ HP int }
type testVel struct{ V Vec3 }

func TestECSBasic(t *testing.T) {
	w := NewECSWorld()
	e1 := w.Spawn()
	e2 := w.Spawn()
	if !w.Alive(e1) || !w.Alive(e2) {
		t.Fatalf("实体应存活")
	}
	if w.Entities() != 2 {
		t.Fatalf("实体数 = %d", w.Entities())
	}
	Add(w, e1, testHealth{HP: 10})
	Add(w, e2, testHealth{HP: 20})
	Add(w, e2, testVel{V: V3(1, 0, 0)})

	if h, ok := Get[testHealth](w, e1); !ok || h.HP != 10 {
		t.Fatalf("Get 失败: %+v %v", h, ok)
	}
	if p := GetPtr[testHealth](w, e2); p == nil || p.HP != 20 {
		t.Fatalf("GetPtr 失败")
	} else {
		p.HP = 99
	}
	if h, _ := Get[testHealth](w, e2); h.HP != 99 {
		t.Fatalf("GetPtr 未原地修改: %d", h.HP)
	}
	// 查询
	sum := 0
	Query1(w, func(e Entity, h *testHealth) { sum += h.HP })
	if sum != 109 {
		t.Fatalf("Query1 求和 = %d", sum)
	}
	n2 := 0
	Query2(w, func(e Entity, h *testHealth, v *testVel) { n2++ })
	if n2 != 1 {
		t.Fatalf("Query2 命中 = %d", n2)
	}
	// 销毁 + 复用
	w.Despawn(e1)
	if w.Alive(e1) {
		t.Fatalf("销毁后仍存活")
	}
	if _, ok := Get[testHealth](w, e1); ok {
		t.Fatalf("销毁后组件应被移除")
	}
	e3 := w.Spawn()
	if e3 == e1 {
		t.Fatalf("复用索引后代号应变化")
	}
	Add(w, e3, testHealth{HP: 1})
	if h, ok := Get[testHealth](w, e3); !ok || h.HP != 1 {
		t.Fatalf("复用实体组件异常: %+v", h)
	}
	if _, ok := Get[testHealth](w, e1); ok {
		t.Fatalf("旧句柄不应读到新实体的组件")
	}
}

func TestECSSystems(t *testing.T) {
	w := NewECSWorld()
	var order []string
	w.System("pre", "a", func(w *ECSWorld, dt float32) { order = append(order, "pre") })
	w.System("update", "b", func(w *ECSWorld, dt float32) { order = append(order, "update") })
	w.System("post", "c", func(w *ECSWorld, dt float32) { order = append(order, "post") })
	w.Update(0.016)
	if len(order) != 3 || order[0] != "pre" || order[1] != "update" || order[2] != "post" {
		t.Fatalf("系统顺序错误: %v", order)
	}
}

func TestJobsDo(t *testing.T) {
	j := NewJobs(4)
	defer j.Close()
	const n = 1000
	results := make([]int, n)
	j.Do(n, func(i int) { results[i] = i * 2 })
	for i := 0; i < n; i++ {
		if results[i] != i*2 {
			t.Fatalf("第 %d 项 = %d", i, results[i])
		}
	}
	// 去重检查：每个下标恰好执行一次
	var mu sync.Mutex
	counts := make(map[int]int, n)
	j.Do(n, func(i int) {
		mu.Lock()
		counts[i]++
		mu.Unlock()
	})
	for i := 0; i < n; i++ {
		if counts[i] != 1 {
			t.Fatalf("第 %d 项执行 %d 次", i, counts[i])
		}
	}
}

func TestJobsReduceAndScheduler(t *testing.T) {
	j := NewJobs(3)
	defer j.Close()
	total := ParallelReduce(j, 100, func(i int) int { return i }, func(a, b int) int { return a + b })
	if total != 4950 {
		t.Fatalf("并行归约 = %d", total)
	}
	// 异步任务
	var sum int64
	var mu sync.Mutex
	for i := 0; i < 32; i++ {
		j.Submit(func() {
			mu.Lock()
			sum++
			mu.Unlock()
		})
	}
	j.Wait()
	if sum != 32 {
		t.Fatalf("异步任务数 = %d", sum)
	}
	// 分帧调度
	s := NewScheduler(Budget{MsPerFrame: 0.001, MaxItemsPerFrame: 5})
	var ran int
	s.Add([]func(){
		func() { ran++ }, func() { ran++ }, func() { ran++ },
		func() { ran++ }, func() { ran++ }, func() { ran++ }, func() { ran++ },
	}, func() { ran += 100 })
	n := s.Tick()
	if n != 5 || ran != 5 {
		t.Fatalf("首帧应受 MaxItemsPerFrame 限制: n=%d ran=%d", n, ran)
	}
	s.Flush()
	if ran != 107 {
		t.Fatalf("Flush 后 ran = %d", ran)
	}
	if s.Pending() != 0 {
		t.Fatalf("任务未清空: %d", s.Pending())
	}
}

func TestOctree(t *testing.T) {
	o := NewOctree(AABB{Min: V3(-100, -100, -100), Max: V3(100, 100, 100)}, 5, 2)
	for i := 0; i < 200; i++ {
		x := float32(i%20) * 5
		z := float32(i/20) * 5
		box := AABB{Min: V3(x, 0, z), Max: V3(x+2, 2, z+2)}
		o.Insert(box, i)
	}
	if o.Count != 200 {
		t.Fatalf("条目数 = %d", o.Count)
	}
	// 范围查询
	q := AABB{Min: V3(0, -1, 0), Max: V3(12, 3, 12)}
	ids := o.QueryAABB(q, nil)
	if len(ids) == 0 {
		t.Fatalf("范围查询无结果")
	}
	// 视锥查询
	cam := NewScene().Camera
	f := cam.Frustum(1.5)
	vis := o.QueryFrustum(f, nil)
	if len(vis) > 200 {
		t.Fatalf("视锥查询结果异常多: %d", len(vis))
	}
	// 球查询
	sph := o.QuerySphere(V3(5, 1, 5), 8, nil)
	for _, id := range sph {
		if id < 0 || id >= 200 {
			t.Fatalf("球查询返回非法 ID: %d", id)
		}
	}
	o.Clear()
	if o.Count != 0 {
		t.Fatalf("Clear 后条目数 = %d", o.Count)
	}
}

func TestOcclusionCuller(t *testing.T) {
	oc := NewOcclusionCuller(320, 240, 4)
	cam := NewScene().Camera
	mvp := cam.ViewProj(320.0 / 240.0)
	oc.Begin()
	// 相机正前方的大遮挡板
	front := AABB{Min: V3(-4, -4, -1), Max: V3(4, 4, -0.5)}
	oc.AddOccluder(mvp, front, 320, 240)
	if oc.Occluders() != 1 {
		t.Fatalf("遮挡物计数 = %d", oc.Occluders())
	}
	// 板后面的小物体应被剔除
	behind := AABB{Min: V3(-1, -1, -8), Max: V3(1, 1, -7)}
	if oc.Visible(mvp, behind, 320, 240) {
		t.Fatalf("被遮挡的物体没有被剔除")
	}
	// 相机与板之间的物体应可见
	between := AABB{Min: V3(-1, -1, 3), Max: V3(1, 1, 4)}
	if !oc.Visible(mvp, between, 320, 240) {
		t.Fatalf("近处物体被错误剔除")
	}
	// 屏幕外的物体应不可见
	outside := AABB{Min: V3(50, 50, 0), Max: V3(52, 52, 1)}
	if oc.Visible(mvp, outside, 320, 240) {
		t.Fatalf("屏幕外物体没有被剔除")
	}
}

func TestTerrain(t *testing.T) {
	hm := NewHeightmap(33, 33, 2)
	NoiseTerrain(hm, 42, 12, 6, 3)
	// 高度查询应落在合理区间
	h := hm.HeightAt(10, 10)
	if h < -1000 || h > 1000 {
		t.Fatalf("高度异常: %f", h)
	}
	n := hm.NormalAt(10, 10)
	if n.Y < 0 {
		t.Fatalf("法线应朝上: %+v", n)
	}
	// chunk 网格
	m := hm.ChunkMesh(0, 0, 8, 1)
	if m == nil || len(m.Positions) == 0 {
		t.Fatalf("chunk 网格为空")
	}
	if len(m.Indices)%3 != 0 {
		t.Fatalf("三角形索引数不是 3 的倍数: %d", len(m.Indices))
	}
	// LOD 网格顶点更少
	hi := hm.ChunkMesh(0, 0, 16, 1)
	lo := hm.ChunkMesh(0, 0, 16, 4)
	if hi == nil || lo == nil {
		t.Fatalf("LOD 网格生成失败")
	}
	if len(lo.Positions) >= len(hi.Positions) {
		t.Fatalf("低 LOD 顶点数应更少: %d vs %d", len(lo.Positions), len(hi.Positions))
	}
	// Terrain 与 LOD 切换
	tr := NewTerrain(hm, 16, nil)
	if tr == nil || len(tr.Chunks) == 0 {
		t.Fatalf("地形块为空")
	}
	near := tr.Update(V3(0, 50, 0))
	if near == 0 {
		t.Logf("提示: 近处 LOD 已是 0，无需切换")
	}
	far := tr.Update(V3(1000, 50, 1000))
	if far == 0 {
		t.Fatalf("远处应触发 LOD 切换")
	}
	chunks, perLOD, tris := tr.Stats()
	if chunks == 0 || tris == 0 {
		t.Fatalf("地形统计异常: %d %v %d", chunks, perLOD, tris)
	}
}

func TestMergeMeshesAndInstances(t *testing.T) {
	a := NewBox(1, 1, 1)
	b := NewBox(1, 1, 1)
	merged := MergeMeshes([]*Mesh{a, b}, []Mat4{Translate(V3(0, 0, 0)), Translate(V3(5, 0, 0))})
	if merged == nil {
		t.Fatalf("合并失败")
	}
	if len(merged.Positions) != len(a.Positions)+len(b.Positions) {
		t.Fatalf("合并顶点数 = %d", len(merged.Positions))
	}
	if merged.Bounds.Max.X < 5 {
		t.Fatalf("合并包围盒未覆盖偏移: %+v", merged.Bounds)
	}
	// 实例批次
	batch := NewInstanceBatch(a, nil)
	for i := 0; i < 10; i++ {
		batch.Add(V3(float32(i), 0, 0), 1, RGB(255, 0, 0))
	}
	if batch.Count() != 10 {
		t.Fatalf("实例数 = %d", batch.Count())
	}
	data, layout := batch.Stream()
	if len(data) != 10*32 || layout.Stride != 32 || !layout.Instanced {
		t.Fatalf("实例流异常: len=%d stride=%d", len(data), layout.Stride)
	}
	// 渲染一遍（软件后端）确认实例化路径可用
	g := newSoftGPU(t)
	_, fb := newTestTarget(t, g, 32, 32)
	vb, ib, vlayout, err := g.NewMeshBuffers(a)
	if err != nil {
		t.Fatalf("网格: %v", err)
	}
	instBuf, instLayout, err := batch.Upload(g)
	if err != nil {
		t.Fatalf("实例缓冲: %v", err)
	}
	p, err := g.NewPipeline(PipelineDesc{
		Program: ShaderInstanced(), DepthTest: true, DepthWrite: true,
		Cull: CullNone, ColorTargets: 1,
	})
	if err != nil {
		t.Fatalf("管线: %v", err)
	}
	cam := NewScene().Camera
	cmd := g.Begin().SetFramebuffer(fb)
	cmd.ClearColor(0, RGB(0, 0, 0))
	cmd.ClearDepth(1)
	cmd.SetPipeline(p)
	cmd.SetUniformMat4("u_mvp", cam.ViewProj(1))
	cmd.SetVertexBuffer(vb, vlayout)
	cmd.SetIndexBuffer(ib)
	cmd.SetInstanceBuffer(instBuf, instLayout)
	cmd.Draw(len(a.Indices), batch.Count())
	if err := g.Submit(cmd); err != nil {
		t.Fatalf("提交: %v", err)
	}
	st := g.Stats()
	if st.Instances != 10 {
		t.Fatalf("实例统计 = %d", st.Instances)
	}
	img, _ := g.ReadPixels(fb, 0)
	if _, _, n := gpuCentroid(img); n == 0 {
		t.Fatalf("实例化渲染没有产生像素")
	}
}

func TestStreaming(t *testing.T) {
	s := NewStreaming(50, 1, 2)
	sc := NewScene()
	built := 0
	s.Loader = func(cx, cz int) *Node {
		built++
		return NewMeshNode("chunk", NewBox(1, 1, 1), nil)
	}
	loaded, _ := s.Update(V3(0, 0, 0), sc)
	if loaded != 9 || built != 9 {
		t.Fatalf("首帧应加载 3x3=9 块: loaded=%d built=%d", loaded, built)
	}
	if s.Chunks() != 9 {
		t.Fatalf("分块数 = %d", s.Chunks())
	}
	// 移到远处：旧块应卸载，新块加载
	loaded2, unloaded := s.Update(V3(500, 0, 0), sc)
	if unloaded == 0 {
		t.Fatalf("远处应卸载旧分块")
	}
	if loaded2 == 0 {
		t.Fatalf("远处应加载新分块")
	}
	if s.Loader == nil {
		t.Fatalf("Loader 被清空")
	}
	// 异步模式
	s2 := NewStreaming(50, 1, 2)
	j := NewJobs(2)
	defer j.Close()
	s2.Jobs = j
	s2.Async = true
	s2.Loader = func(cx, cz int) *Node { return NewMeshNode("c", NewBox(1, 1, 1), nil) }
	s2.Update(V3(0, 0, 0), sc)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, loaded := s2.Update(V3(0, 0, 0), sc); loaded > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s2.Loaded == 0 {
		t.Fatalf("异步流式加载未完成")
	}
	s2.Clear(sc)
	if s2.Chunks() != 0 {
		t.Fatalf("Clear 后仍有分块")
	}
}

func TestCollectVisible(t *testing.T) {
	sc := NewScene()
	near := NewMeshNode("near", NewBox(1, 1, 1), nil)
	near.Pos = V3(0, 0, -3)
	far := NewMeshNode("far", NewBox(1, 1, 1), nil)
	far.Pos = V3(0, 0, -5000)
	behind := NewMeshNode("behind", NewBox(1, 1, 1), nil)
	behind.Pos = V3(0, 0, 5000)
	sc.Root.Add(near, far, behind)
	vis := CollectVisible(sc, sc.Camera, 1.5)
	names := map[string]bool{}
	for _, n := range vis {
		names[n.Name] = true
	}
	if !names["near"] {
		t.Fatalf("近处物体应可见: %v", names)
	}
	if names["behind"] {
		t.Fatalf("相机背后的物体不应可见: %v", names)
	}
}
