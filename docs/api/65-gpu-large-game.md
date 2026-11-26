# GPU 渲染与大型游戏支撑（game 包）

> 面向大型 2D / 3D 游戏的渲染与运行时能力：跨后端 GPU 抽象、ECS、并行作业、
> 空间分区与遮挡剔除、地形 LOD、实例化合批、流式加载。

## 1. GPU 抽象层

### 1.1 创建与探测

```go
g, err := game.NewGPU(
    game.WithGPUBackend("auto"), // "auto" | "soft" | "gl"
    game.WithGPUFallback(true),  // 硬件不可用时回退软件后端
    game.WithGPUWorkers(0),      // 0 = GOMAXPROCS
)
if err != nil {
    log.Fatal(err)
}
defer g.Close()
log.Println(g.Backend(), g.Caps())
```

| 后端 | 说明 | 依赖 |
|---|---|---|
| `soft` | 纯 Go 多线程 tile 光栅化，可编程着色，任何平台可用 | 无 |
| `gl` | Windows 上通过 `purego` 动态加载 `opengl32.dll`，离屏 FBO 后回读像素 | 显卡驱动（不需要 CGO） |

两个后端对同一场景的输出一致，由 `TestGPUBackendParity` 保证。

### 1.2 把场景渲染成图像

```go
gs, err := game.NewGPUScene(g, 960, 540)
sc := game.NewScene()
sc.Root.Add(game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial()))
img, err := gs.Render(sc, nil) // *image.RGBA
os.WriteFile("frame.png", pngOf(img), 0o644)
```

### 1.3 直接录制命令

```go
color, _ := g.NewTexture(game.TexDesc{W: 960, H: 540, Format: game.PixRGBA8, Usage: game.TexRenderTarget | game.TexSampled})
depth, _ := g.NewTexture(game.TexDesc{W: 960, H: 540, Format: game.PixDepth32, Usage: game.TexDepthStencil})
fb, _ := g.NewFramebuffer([]*game.GPUTexture{color}, depth)
pipe, _ := g.NewPipeline(game.PipelineDesc{
    Program: game.ShaderPhong(), DepthTest: true, DepthWrite: true,
    Cull: game.CullBack, ColorTargets: 1,
})
vb, ib, layout, _ := g.NewMeshBuffers(mesh)

cmd := g.Begin().SetFramebuffer(fb)
cmd.ClearColor(0, game.RGB(18, 20, 26)).ClearDepth(1)
cmd.SetPipeline(pipe)
cmd.SetUniformMat4("u_mvp", cam.ViewProj(16.0/9.0))
cmd.SetVertexBuffer(vb, layout)
cmd.SetIndexBuffer(ib)
cmd.Draw(len(mesh.Indices), 1)
g.Submit(cmd)
```

`RenderGraph` 可把「阴影 → 主相机 → 后处理」串成依赖链，按拓扑序自动提交。

## 2. ECS（实体-组件-系统）

```go
w := game.NewECSWorld()

type Health struct{ HP int }
type Velocity struct{ V game.Vec3 }

e := w.Spawn()
game.Add(w, e, Health{HP: 100})
game.Add(w, e, Velocity{V: game.V3(1, 0, 0)})

w.System("update", "move", func(w *game.ECSWorld, dt float32) {
    game.Query2(w, func(e game.Entity, h *Health, v *Velocity) {
        h.HP--
        _ = v
    })
})
w.Update(0.016)
w.Despawn(e) // 旧句柄立即失效（世代号机制）
```

- 稀疏集存储：`Add/Get/Remove` O(1)，遍历是连续内存
- `Query1/Query2/Query3` 自动选用最小集合驱动
- `Transform + Renderable` + `game.SyncToScene(w, sc, match)` 可把 ECS 同步进渲染场景

## 3. 并行作业系统

```go
j := game.NewJobs(game.DefaultWorkers())
defer j.Close()

// 并行 for
j.Do(1000, func(i int) { particles[i].Update(dt) })

// 并行归约
total := game.ParallelReduce(j, len(hits), func(i int) int { return hits[i].Dmg },
    func(a, b int) int { return a + b })

// 异步队列
j.Submit(func() { bakeLightmap() })
j.Wait()
```

分帧调度（加载不卡帧）：

```go
s := game.NewScheduler(game.Budget{MsPerFrame: 4, MaxItemsPerFrame: 64})
s.Add(items, func() { loaded = true })
// 每帧
s.Tick()
```

## 4. 可见性：空间分区与剔除

```go
oct := game.NewOctree(bounds, 6, 8)
for i, n := range nodes {
    oct.Insert(n.WorldBounds(), i)
}
ids := oct.QueryFrustum(cam.Frustum(aspect), nil)   // 视锥
near := oct.QuerySphere(pos, 12, nil)               // 范围
```

软件遮挡剔除（大遮挡物先写深度，其余对象做保守测试）：

```go
oc := game.NewOcclusionCuller(w, h, 4)
oc.Begin()
oc.AddOccluder(mvp, bigBuilding.WorldBounds(), w, h)
for _, n := range candidates {
    if oc.Visible(mvp, n.WorldBounds(), w, h) {
        draw(n)
    }
}
```

## 5. 地形 LOD

```go
hm := game.NewHeightmap(257, 257, 2)
game.NoiseTerrain(hm, 42, 48, 30, 4) // seed, scale, amplitude, octaves

terr := game.NewTerrain(hm, 16, mat) // 每块 16 顶点跨度
sc.Root.Add(terr.Root)
// 每帧
terr.Update(cam.Pos)

y := terr.HeightAt(x, z)   // 贴地
n := terr.NormalAt(x, z)   // 坡面朝向
```

块内按 `LODSteps{1,2,4,8}` 与 `LODDistances{40,90,180}` 切换，
远处三角形数按 1/4、1/16 递减。

## 6. 实例化与静态合批

```go
batch := game.NewInstanceBatch(grassMesh, mat)
for i := 0; i < 5000; i++ {
    batch.Add(pos[i], 0.8, game.RGB(90, 160, 70))
}
instBuf, instLayout, _ := batch.Upload(g)

cmd.SetVertexBuffer(vb, meshLayout)
cmd.SetIndexBuffer(ib)
cmd.SetInstanceBuffer(instBuf, instLayout)
cmd.Draw(len(grassMesh.Indices), batch.Count()) // 一次画 5000 个
```

静态合批（同材质的小物件烘成一个网格，减少 draw call）：

```go
merged := game.MergeMeshes(parts, transforms)
```

实例流约定：`Location 4 = xyz 偏移 + w 缩放`，`Location 5 = rgba 染色`，
与 `game.ShaderInstanced()`（GLSL 与软件着色器各一份）配套。

## 7. 流式加载与无限世界

```go
loader := game.NewLoader(jobs)
fut := game.LoadAsync[*game.Mesh](loader, "meshes/tree.glb", func(ctx *game.LoadContext) (*game.Mesh, error) {
    return parseTree(ctx) // 后台线程执行
})
// 主线程
if m, ok, err := fut.TryGet(); ok && err == nil {
    scene.Root.Add(game.NewMeshNode("tree", m, nil))
}
```

按相机位置加载/卸载世界分块：

```go
sm := game.NewStreaming(64, 2, 3) // 块边长、加载半径、卸载半径（单位：块）
sm.Async, sm.Jobs = true, jobs
sm.Loader = func(cx, cz int) *game.Node { return buildChunk(cx, cz) }
// 每帧
sm.Update(cam.Pos, scene)
```

## 8. 已知限制

- `gl` 后端当前仅 Windows（WGL）；其他平台自动回退 `soft`
- GPU 纹理在 `soft` 后端是 CPU 采样，超大分辨率贴图会拖慢填充率
- 遮挡剔除为「屏幕矩形 + 平均深度」的保守近似，细长遮挡物建议直接标记为遮挡体
- ECS 组件按类型反射注册，热路径请避免每帧大量 `Add/Remove`（会移动稠密数组）

## 9. 相关测试

```bash
go test ./game/ -run 'TestGPU|TestECS|TestJobs|TestOctree|TestOcclusion|TestTerrain|TestStreaming' -v
```
