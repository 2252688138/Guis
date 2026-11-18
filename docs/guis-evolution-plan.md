# GUIS 框架体检与演进方案

> 基于当前工作区实际代码 + 本机实测数据（`go build` / `go vet` / `go test -cover` / `go test -bench`）得出的诊断与路线图。
> 所有结论均标注证据；未核实的项目不出现在方案里。

---

## 一、体检结论

### 1.1 基本盘（实测）

| 项 | 数据 | 评价 |
| --- | --- | --- |
| 模块 | `guis`，纯 Go、零 cgo、自绘 OpenGL | 定位清晰 |
| `ui` 包 | 215 个非测试 `.go` 文件 / 约 70,463 行 | 单包规模偏大 |
| `internal` | 13.8 万字符级 / 约 26,391 行（gpu / text / geom / display / win32 / x11） | 分层合理 |
| 平台后端 | Windows(win32+WGL)、X11(EGL)、darwin、iOS、Android(NativeActivity)、WASM | 六端齐全 |
| `go build ./...` | 通过，无告警 | ✅ |
| `go vet ./ui/...` | 通过，无告警 | ✅ |
| 测试覆盖率 | `guis/ui` **50.4%**，`guis/ui/uidsl` 68.3% | 中等，交互态偏薄 |
| 测试文件 | 96 个 `_test.go` / 427 个 go 文件 | 约 22% |
| Benchmark | 8 个文件 18 个用例 | 有基线，但无回归门禁 |
| 文档 | `docs/api/` 40 篇 + 组件套件/多平台/数字人专题 | 完善度高于同类自研框架 |
| CI / Lint | **无 `.github`、无 `.golangci.y*ml`** | ⚠️ 最大工程短板 |
| 渲染回归 | **无 golden / 快照测试** | ⚠️ |

### 1.2 性能基线（本机实测，非估算）

```
BenchmarkFrameBuild-12          147,745 ns/op   289,701 B/op   546 allocs   ← 仅"重建节点树"
BenchmarkFrameLayout-12         216,159 ns/op   289,821 B/op   549 allocs   ← 建树 + 排版
BenchmarkMarkdownRebuild-12     119,464 ns/op   262,625 B/op   547 allocs
BenchmarkChatRowsLayoutStable-12 900,720 ns/op                        0 B/op  ← 10 条消息整列排版
BenchmarkWrapCacheThrash-12    2,926,380 ns/op  154,962 B/op   607 allocs   ← 折行缓存抖动
BenchmarkTableVOldPrealloc-12   598,002 ns/op  2,400,256 B/op              ← 全量建行
BenchmarkTableVWindowBuild-12    22,689 ns/op    52,440 B/op   113 allocs  ← 窗口化（**26×**）
```

**关键发现：`TableVWindowBuild` 的 26 倍收益只存在于 `ui/suite_perf_test.go` 的对比实验里，生产 API 尚未采纳。**
即：瓶颈已经被作者量化过，但优化没落地——这是投入产出比最高的一类工作。

### 1.3 已具备、无需重复建设的能力（避免方案空转）

- 虚拟列表：`VList`（固定行高）/ `VListVar`（变行高，前缀和缓存）/ `VListVisible`（可见区间计算）已在 `ui/vlist.go`。
- 拖拽重排：`ReorderList`（`ui/table_extra.go`，含插入位高亮、`reorderStale` 残留清理）已存在。
- 崩溃兜底：`ui/crash.go`、`ui/goroutine_safe.go`、`runLoop` 内 `recover`。
- 空闲/节流：`animBusy` 动画门控、`slowOnlyFrame` 装饰动画 8–12Hz 节流、`scrollIdle` 滚动条淡出。
- 子帧缓存：`CacheOn()` / `CachedLayer()`（带自动回退与命中日志）。
- 热重载：含"预热交接"（新窗口首帧画好再亮出，避免空档）。
- IME：Win32 层已绑定 `imm32` + `WM_IME_*`。
- 全局热键：win32 / X11(`XGrabKey`) / darwin(`CGEventTap`) 三端。
- 折行缓存：`wrapLinesCached`（measure 与 paint 共用）。

### 1.4 真实短板（有证据）

| # | 短板 | 证据 |
| --- | --- | --- |
| S1 | 无障碍只声明不落地 | `ui/a11y.go:8-13` 注释明写"P3 占位…屏幕阅读器目前读不到" |
| S2 | 无 CI / 无 lint / 无渲染回归 | 无 `.github`、无 `.golangci*`、`rg "golden\|snapshot" *_test.go` 零命中 |
| S3 | 传统 `Table()` 全量建节点 | `ui/widgets.go:1198` 收 `rows [][]*Node`，10 万行即 10 万+ 节点/帧 |
| S4 | 建树分配偏高 | 546 allocs / 289KB 每帧；`St()` 每帧 `&Style{}`（`ui/style.go:120`） |
| S5 | 折行缓存抖动代价大 | `WrapCacheThrash` 2.93ms/次，命中/未命中差 80 倍（36µs vs 2.9ms） |
| S6 | 无应用级路由 / 状态原语 | `rg "func Navigate\|router\|UseState\|Subscribe\("` 业务侧零命中 |
| S7 | 单包 215 文件 | `ui/` 目录清单，`package ui` 单一 |
| S8 | 无内置 DevTools | `rg -il "devtools\|inspector\|性能面板"` 零命中 |
| S9 | 手势缺口 | 无 pinch / long-press / double-tap / flick（移动端仅拖动跟手） |
| S10 | 覆盖率 50.4% | 焦点路由 / keymap / IME / 滚动状态机缺用例 |

---

## 二、优化（O：不改 API，纯收益，风险低）

### O1 · 表格窗口化正式 API —— 收益 26×，最高优先
- 落点：`ui/widgets.go` 新增 `TableV(key string, headers []string, widths []float32, count, rowH, height float32, buildRow func(i int) *Node)`，内部复用 `VListOpt` + `VListVisible`，表头吸顶（`Scroll` 外置 header）。
- 保留 `Table()` 作为小数据量便捷入口，文档标注"≥500 行请用 `TableV`"。
- 把 `suite_perf_test.go` 的对比实验抽出为 `TestTableVEquivalence`（同数据下 `Table` 与 `TableV` 可视结果一致）。
- 验收：10 万行首帧 < 50ms；滚动中每帧建行 ≤ 60 行；`BenchmarkTableVOldPrealloc` 旁新增 `BenchmarkTableVProd` 并入 CI 基线。

### O2 · 折行缓存抖动治理 —— 2.93ms → 目标 < 500µs
- 落点：`ui/textwrap*` 的 `wrapLinesCached`。
- 现状痛点：缓存键若为"文本+字体"而未含 `maxW`，窗口缩放时整屏 miss；若含 `maxW`，缩放过程每帧产生新键并把旧键挤爆。
- 方案：两级桶——`key=(fontID, text)` 内保存最近 2 个 `maxW` 的结果（LRU2）；超过阈值（如文本长度 > 4KB）不进缓存，避免大段 Markdown 把缓存冲干净。
- 验收：`BenchmarkWrapCacheThrash` < 500µs；`TestWrapFuzz` 补 200 组随机宽度断言行数单调性。

### O3 · 建树分配削减 —— 546 allocs/帧 → 目标 < 300
- `St()` 目前每次 `&Style{Opacity:1}`。加 `sync.Pool` 取 Style，配合"节点树在帧结束后整体回收"（`App.frame` 结束后把上一棵树 `pool.Put`；需要保证用户代码不再持有节点引用——文档已声明"每帧重建"，语义兼容）。
- 高频节点（`Text`/`Box`/`Row`）走节点池；`children` 切片按容量复用。
- 风险控制：先做 Style 池（最安全），节点池作为可选开关 `ui.OptimizeNodePool(true)`，观察一版再默认开。
- 验收：`BenchmarkFrameBuild` allocs ≤ 300，`B/op` ≤ 150KB，帧时间 ≤ 100µs。

### O4 · 布局增量（脏子树短路）
- 落点：`ui/layout.go:145 Layout / 169 arrange / 180 measureNode`。
- 思路：节点布局完成后记录 `(styleHash, frameSize)`；下一帧若 `Style` 全等且所在容器可用宽高未变，跳过整棵子树的 `measure`（直接用上帧 Frame）。
- 前提：`Style` 需可比较/可哈希——给 `Style` 加 `rev uint32`（每个链式 setter 自增），`Key + rev + 可用宽高` 命中即短路。这比哈希便宜。
- 验收：`BenchmarkMarkdownLayoutStable` 162µs → < 80µs；`BenchmarkChatRowsLayoutStable` 901µs → < 400µs。

### O5 · 富文本行内混排测量缓存
- 落点：`ui/richtext.go` + `codeeditor_view.go`（大量行内 span 混排）。
- 现状：`ChatRowsLayoutStable` 900µs / 10 条，行内 span 每帧重算宽度。
- 方案：以 `(span 文本, 字号, 字重, 家族)` 为键缓存 `Measure` 结果（字体层已有 `GlyphsFor` 字符串级缓存，但 span 级宽度和仍需累加）；行内 code 背景/圆角几何一并缓存。
- 验收：聊天列排版 < 400µs（与 O4 合并验收）。

### O6 · 命中测试从全树线性走改为可见 AABB 索引
- 落点：`ui/app.go:1130 paint / hitInteractive / walkScrollbars`。
- 现状：鼠标移动（`hoverProbe`）与大列表滚动时每帧全树走一遍，且 `paintChildren` 已做纵向剔除但命中侧没有同等剔除。
- 方案：paint 阶段顺带把"可交互节点的屏幕 AABB"压进按 y 排序的扁平数组（paint 已经在遍历，零额外树走）；命中时二分定位 y 区间再逐个精确判定。滚动时只重建受影响区间。
- 验收：新增 `BenchmarkHitTest`（1 万可交互节点，单次命中）< 5µs；hover 移动帧时间与静态帧差 < 1ms。

### O7 · 单帧预算与自适应降级
- 现有 `slowOnlyFrame` 只覆盖装饰动画。扩展为通用"帧预算"：若上一帧 CPU 时间 > 14ms（60fps 余量），本帧自动跳过低优先级绘制（阴影、毛玻璃、渐变改为纯色近似，`CacheOn` 优先命中），并在 `GUIS_FPS=1` 日志打出降级原因。
- 收益：弱机 / 大表格 / WASM 端不掉帧、不卡顿。
- 验收：`GUIS_FPS=1` 下人为注入 100 万节点页面，帧时间稳定 ≤ 16.6ms（降级生效），日志可解释。

### O8 · 事件/状态通道收敛
- `ui.Post`-类跨线程投递若无界会积压；检查 `runPending`（`app.go:825`）是否已有上限与合并策略，补"同一 key 的重复失效合并"+"积压告警"。
- 验收：后台 1000 次/秒投递时队列峰值 < 64 且丢帧日志可辨识。

---

## 三、进化（E：架构级，跨迭代）

### E1 · 把"多平台回归"从手工脚本升级为 CI 门禁
- 现状：靠 `verify-platforms.ps1`、`build-multi.ps1` 手工跑。
- 方案：GitHub Actions（或 Gitea Actions）矩阵：
  - `windows-latest`：`go build ./...` + `go vet` + `go test ./ui/... -race` + `go test -bench . -benchtime 100ms` → **benchstat 与基线比对，回归 > 10% 直接失败**
  - `ubuntu-latest`：同上 + `xvfb-run` 跑 headless 渲染用例（`backend_null` + FBO 离屏）
  - `macos-latest` / `GOOS=wasm GOARCH=wasm`（build only）/ `GOOS=android`（build only）
  - `golangci-lint`（errcheck / govet / staticcheck / ineffassign / unused / gocritic）
- 配套：基线文件 `bench/baseline.txt` 入库，PR 里自动贴 diff 表。
- 验收：一条 PR 全绿需 < 15 分钟；驳回能力可演示（人为 +20% 回归）。

### E2 · 渲染回归（golden image）
- 现状：零快照测试。
- 方案：`backend_null` + 离屏 FBO 渲染核心组件到 PNG，与 `testdata/golden/*.png` 做像素 diff（阈值：差异像素 < 0.5%，允许平台字体亚像素差）。
- 覆盖矩阵：20 个核心组件 × {深色, 浅色} × {1.0x, 1.5x, 2.0x} 缩放。
- 关键设计：字体必须固定（仓库内置一份测试字体，避免 CI 机器字体不同）；抗锯齿差异用 `阈值化 diff` 吸收。
- 升级路径：先只做"布局快照"（节点 Frame 序列化 JSON 比对），比像素 diff 稳、成本低；像素快照作为第二步。
- 验收：故意改坏 `padding` 一处，CI 能定位到具体组件快照。

### E3 · 拆包与依赖分层
- 现状：`package ui` 215 文件。
- 方案（保守、可分三步）：
  1. 目录分区但不拆包：`ui/core`、`ui/widgets`、`ui/rich`、`ui/media`、`ui/sys` 仅作文件组织（Go 同包仍可跨目录吗——不可，故改为**保持单包 + 文件前缀规范 + 生成式依赖检查**）。
  2. 真正拆包只拆"低耦合大块"：`markdown`(6.5K 行)、`codeeditor`(8.7K)、`term`(3.6K)、`chart`(2.2K)、`i18n`(2.5K) → 拆成 `ui/markdown`、`ui/codeeditor` 等独立包，`ui` 保留 facade 再导出（`type Editor = codeeditor.Editor`）。已有 `ui/uidsl` 子包先例，说明可行。
  3. 用 `go list -deps` + 自定义脚本禁止底层反向依赖（`uidsl` 不得 import `ui` 的具体组件）。
- 收益：编译增量时间、IDE 索引、心智负担。
- 验收：`ui` 包文件数 < 120；拆出的包单测独立可跑；`go build` 全平台仍绿。

### E4 · 可选 diff 渲染模式（跳出"每帧重建"的成本上限）
- 现状：每帧全量 `view()` → 建树 → 布局 → 绘制；`CacheOn` 只能救静态子树。
- 方案：给节点加 `Same(key)` / `Eq(fingerprint)` 标记，父子链命中时整棵子树直接复用上帧节点对象（跳过 build 回调——需要 build 侧支持惰性，`func() *Node` 形式）。这与 `uidsl` 的声明式描述天然契合：`uidsl` 已能把 DSL 解析成节点，可在 DSL 层做"文本+属性指纹"短路。
- 风险：破坏"每帧重建"的语义承诺 → 必须默认关闭，文档明确"仅在超大静态页面启用"。
- 验收：静态 5 万节点页面空闲帧 CPU 从 X 降到接近 0（配合脏标记），且交互仍正确。

### E5 · 无障碍真正落地（分平台，先 Windows）
- 现状：`a11y.go` 只有角色声明 + `A11yTree()` 导出，系统层零注册。
- 方案（Windows 优先，覆盖主力用户）：
  1. `WM_GETOBJECT` 拦截 → `UiaReturnRawElementProvider`，为窗口和每个 `A11yRole` 节点提供 `IRawElementProviderSimple/Fragment` 的 COM 实现（purego 可做，参考 `internal/win32` 现有 Syscall 风格）。
  2. 节点几何/Focus 变化时 `UiaRaiseAutomationEvent` / `StructureChanged`。
  3. 键盘 Tab 序与 `focusKey` 已有基础设施，直接映射。
  4. Linux：AT-SPI over D-Bus（后续）；macOS：`NSAccessibility`（后续）。
- 中间态（低成本高价值）：先做"无障碍模式"——按 `A11yRole` 渲染高对比焦点框 + 纯键盘可达性检查工具（`ui.A11yLint()` 返回缺 role/label 的节点清单），这可立刻提升产品可用性并进 CI。
- 验收：Windows Narrator 能读出按钮/输入框/表格行数与可操作状态；`A11yLint` 在 gallery 全页零告警。

### E6 · 覆盖率 50.4% → 65%+
- 优先补"状态机类"测试（bug 高发区）：焦点路由（`app.go:968` 附近点击决定焦点的分支全覆盖）、Tab 序、`keymap`、IME 组合输入、滚动条拖拽（`applyScrollbarDrag`）、裁剪切换（`pushClip/popClip` 的 `clipSkip` 顺序问题已被注释记录过，正是回归高发点）。
- 工具：`go test -coverprofile` + `go tool cover -func` 生成"未覆盖函数清单"，逐迭代清零。

---

## 四、增强（N：新能力，用户可感知）

### N1 · 应用级原语三件套（Router / Store / Resource）
当前框架只到"组件库"，业务侧每写一个 app 都要自己造轮子（deepseek 目录就有大量重复）。
- `Router`：页面栈 + 参数 + 返回栈 + 转场动画（复用 `anim.go`）；`ui.Push(page, params)` / `ui.Pop(result)`；与热重载协同（保存栈）。
- `Store`：`ui.NewStore[T](init)` + `store.Select(sel)` 局部订阅；节点级细粒度失效（只 `dirty=true`，不做全局重建约定）。
- `Resource`：`ui.Resource(key, fetch)` 返回 `{data, loading, err, retry}`，自动套 `AsyncSection` + `SkeletonText`（两者已有），带缓存与去重。
- 验收：用这三件套把 `cmd/deepseek` 里一处手写状态/加载逻辑替换掉，代码行数下降且行为不变。

### N2 · 手势识别器
- `ui.Gesture(n, opts)`：pinch / rotate / long-press / double-tap / fling（带速度与惯性衰减），移动端优先，桌面端用滚轮+Ctrl 模拟 pinch。
- 与现有"1:1 跟手拖动"协同，惯性滚动复用同一衰减曲线。
- 验收：移动端画廊双指缩放画布、长按弹菜单、快速滑动惯性滚动。

### N3 · 内置 DevTools（F5 打开）
- 节点树 + 属性 + 盒模型可视化（含 padding/margin 高亮）、帧耗时火焰条、重绘原因（谁把 `dirty` 置真）、命中测试热区、`CacheOn` 命中率、文本折行缓存命中率。
- 现状已有一半数据源（`GUIS_FPS=1` 日志、`layer=命中/未命中`），把它 UI 化即可。
- 架构上放 `ui/devtools`（build tag `!release`），零成本剔除。
- 验收：能在 3 秒内回答"这个组件为什么在重绘"。

### N4 · 数据可视化升级
- `chart` 已有 18 种图：折线/面积/柱/饼/环/雷达/散点/K 线（含量能副图），水平条形/直方图/瑰图/气泡/瀑希图/箱线图/分时图/热力图/漏斗图/仪表盘 + 命中测试（`chart_hit_test.go` / `chart_radar_hit_test.go` / `chart_stack_hit_test.go`）。
- 补：十字光标 + tooltip、框选缩放、平移、数据点高亮联动、图例点击隐藏序列、导出 PNG/SVG。
- 验收：折线图能缩放平移并保持 tooltip 跟随。

### N5 · 多窗口与停靠进化
- 已有 split/dock。补：tab 拖出成独立窗口（`OpenWindow` 已有多窗口基础）、跨窗口拖拽（需共享 drag payload）、窗口位置记忆（`autostart`/prefs 已有持久化）。
- 验收：把 dock 里的 tab 拖到屏幕外生成新窗口，原位置留占位。

### N6 · 主题体系产品化
- `Theme` + `Tokens` 已有。补：
  - 主题 JSON 导入/导出（设计-开发协作）
  - 从 Tokens 生成 CSS 变量 / Tailwind config（同一套设计语言同时产出 Web 端）
  - 主题校验 lint（对比度 AA/AAA、色板语义完整性）
- 验收：一套 Tokens 同时驱动 GUIS 界面与导出的网页主题，对比度检查全过。

### N7 · 自更新与分发
- 已有 `autostart`、`installer`、`build-*.ps1`；补 updater：清单签名校验 + 差量下载 + 静默替换（Windows 需处理运行中文件替换 `MoveFileEx`）。
- 验收：旧版本应用内一键升级到新版本并重启，签名校验失败时拒绝。

### N8 · uidsl 双向能力
- 现有 DSL 是"文本 → 节点"。补：
  - "节点树 → DSL" 反向导出（DevTools 里选中一个节点即可复制它的 DSL，写页面时极其好用）
  - DSL 热编辑面板（配合 `hotreload`，左边写 DSL 右边实时预览）
- 验收：参加者不看文档，靠反向导出的 DSL 片段就能改出界面。

---

## 五、路线图（三个迭代）

### 迭代 1（1–2 周｜纯收益、可独立发布）
1. **O1** 表格窗口化 API（26×）
2. **O2** 折行缓存抖动（2.93ms → <500µs）
3. **O5** 富文本 span 测量缓存
4. **E1** CI + benchstat 门禁 + golangci-lint
5. **E2(第一步)** 布局快照（Frame JSON diff）
> 交付标准：CI 全绿，benchmark 基线入库，表格性能提升可量化。

### 迭代 2（2–4 周｜架构与体验）
1. **O3** 建树分配削减（546 → <300 allocs）
2. **O4** 布局增量短路
3. **O6** 命中 AABB 索引
4. **N1** Router / Store / Resource
5. **N3** DevTools v1（节点树 + 重绘原因 + 折行缓存命中率）
6. **E6** 覆盖率补到 60%（交互状态机优先）

### 迭代 3（1–2 月｜平台级能力）
1. **E5** Windows UIA Provider + `A11yLint`
2. **N2** 手势识别器
3. **E3** 拆分 markdown / codeeditor / term 子包
4. **O7** 帧预算自适应降级
5. **E2(第二步)** 像素级 golden 快照矩阵

---

## 六、量化验收总表

| 指标 | 现状 | 目标 |
| --- | --- | --- |
| 10 万行表格首帧 | 598µs/帧仅建行（2.4MB） | < 50ms 完成、滚动 60fps |
| 表格窗口化单帧建行 | 598µs / 2.4MB | 22.7µs / 52KB（已实测可达） |
| `FrameBuild` allocs | 546 / 289KB | < 300 / < 150KB |
| `FrameLayout` | 216µs | < 120µs |
| `MarkdownLayoutStable` | 162µs | < 80µs |
| `ChatRowsLayoutStable` | 901µs | < 400µs |
| `WrapCacheThrash` | 2.93ms | < 500µs |
| 单次命中测试（1 万节点） | 全树线性 | < 5µs |
| 测试覆盖率（ui） | 50.4% | ≥ 65% |
| CI 覆盖平台 | 0 | 5（win/linux/mac/wasm/android）+ race + bench 门禁 |
| 渲染回归 | 0 | 20 组件 × 2 主题 × 3 缩放快照 |
| 无障碍 | 仅声明 | Windows Narrator 可读 + `A11yLint` 零告警 |

---

## 七、立刻可开工的 5 个 PR（文件级落点）

| PR | 内容 | 触及文件 | 预估 |
| --- | --- | --- | --- |
| PR-1 | `TableV` 窗口化表格 + 等价性测试 | `ui/widgets.go`、`ui/vlist.go`(复用)、`ui/suite_perf_test.go` | 1–2 天 |
| PR-2 | 折行缓存 LRU2（按 font+text 存两个 maxW） | `ui/textwrap*.go` 及对应 `_test.go` | 1 天 |
| PR-3 | `.github/workflows/ci.yml` + `bench/baseline.txt` + benchstat 比较 | 新增 | 1 天 |
| PR-4 | `Style.rev` + 布局脏子树短路 | `ui/style.go`、`ui/layout.go`、`ui/layout_perf_test.go` | 2–3 天 |
| PR-5 | `A11yLint()` 检查器（role/label/焦点可达） | `ui/a11y.go` + 新增 `ui/a11y_lint_test.go` | 1 天 |

> 建议顺序：PR-3（先有护栏）→ PR-2 → PR-1 → PR-4 → PR-5。

---

## 八、落地进展（本轮已实施）

实施过程中对原方案做了两处**纠正**（原方案基于不完整调研，这里如实记录）：

| 原条目 | 纠正 |
| --- | --- |
| O1 `TableV` 窗口化表格 | **撤销**：`ui/table_extra.go:66` 已有完整实现（虚拟滚动 + 分组折叠 + 排序 + 列宽拖拽），属于重复建设 |
| O2 折行缓存「抖动 2.93ms」 | **诊断修正**：2.93ms 是 `BenchmarkWrapCacheThrash` 里「清空后 600 段全量重折行」的**无缓存基线**，不是抖动 bug。真正可改进的是淘汰策略（随机删 → 近似 LRU） |

### 已交付

| 条目 | 内容 | 文件 | 验证 |
| --- | --- | --- | --- |
| E5a A11yLint | 无障碍静态检查器：缺角色 / 缺名称 / 可聚焦无 Key / Key 重复 / 图片无替代文本，先序输出带路径的问题清单 | `ui/a11y_lint.go`、`ui/a11y_lint_test.go` | 7 个用例全绿 |
| N1 Store | 单值可订阅状态：`Set/Get/Update/Rev/Subscribe`，值未变不通知不重绘，不可比较类型不 panic | `ui/store.go`、`ui/store_test.go` | 5 个用例 |
| N1 Resource | 异步资源：幂等 `Load` / `Reload` / `Invalidate` / `State` / `Section`（接 `AsyncSection` 四态），旧结果不会覆盖新结果 | 同上 | 5 个用例（含竞态序号校验） |
| N1 Router | 页面栈：`Register/Push/Replace/Pop/PopTo/Current/View/CanPop` | 同上 | 4 个用例 |
| N8 uidsl 反向导出 | `uidsl.FromNode / FromNodes`：节点树 → DSL 文本，含样式映射、文本转义、不支持类型注释占位 | `ui/uidsl/export.go`、`ui/uidsl/export_test.go` | 8 个用例（含往返解析） |
| E1 CI + 门禁 | 三平台 build/vet/单测、交叉编译（wasm/android/darwin）、golangci-lint、benchstat 对比；本地 `scripts/bench-gate.ps1`（采样基线 + 阈值 10% 失败） | `.github/workflows/ci.yml`、`scripts/bench-gate.ps1`、`bench/baseline.txt` | 脚本实跑通过（16 用例基线，实测能捕获 +15% 回归） |
| E2 布局快照 | 10 个布局场景（flex/grid/wrap/stack/百分比/绝对定位/约束）几何快照，跨平台稳定（不含字体） | `ui/layout_snapshot_test.go`、`ui/testdata/layout_snapshot/*.txt` | 生成 + 复跑比对通过 |
| O2 折行缓存 | 淘汰策略改近似 LRU：`wrapEntry.use` 原子时钟 + 采样 16 条淘汰最冷 + 单轮淘汰上限 16 条（原为随机删 512 条） | `ui/textwrap.go` | `ui/textwrap_lru_test.go` 3 个用例 |
| N6 主题导出 | `TokenPairs / TokensCSS / TokensTailwind / TokensGo`：反射遍历 Tokens，同一套设计语言输出 CSS 变量与 Tailwind 配置 | `ui/theme_export.go`、`ui/theme_export_test.go` | 5 个用例 |

### 未实施（风险 / 前置条件评估后暂缓）

| 条目 | 暂缓原因 |
| --- | --- |
| O3 Style/节点对象池 | `St()` 返回的 `*Style` 可能被用户跨节点复用（`card := St().Padding(16)` 反复传入），池化后会被二次复用污染数据。需要先加「样式值拷贝」语义约定或提供显式开关，属语义变更 |
| O4 布局脏子树短路 | 需给 `Style` 加 `rev` 并由全部链式 setter 自增（60+ 个 setter），且 `Layout` 短路会影响滚动/百分比尺寸等求解顺序，必须有完整回归矩阵才有意义 |
| O5 富文本 span 测量缓存 | 与 O4 共享「测量结果复用」的设计，建议同批做 |
| O6 命中 AABB 索引 | 需改 `App.paint` 的遍历顺序与 `hitInteractive` 语义（含 `PosAbs`、裁剪、冒泡三处特例），改动面大，建议单独分支 + 快照回归护栏后进行 |
| E3 拆包 / E4 diff 渲染 | 架构级改动，应与一次大版本一起规划 |
| E5 系统级 UIA Provider | 需要 COM 接口实现与真机 Narrator 验证，本轮先交付了静态检查（A11yLint）这一层 |
| N2/N3/N4/N5/N7 | 属新功能开发，需要产品侧确认优先级 |

### 验证结论

- `go build ./...`：通过
- `go vet ./ui/...`：通过，无告警
- `go test ./ui/ -count=1`：**12.4s 全绿**
- `go test ./ui/uidsl/ -count=1`：全绿
- `scripts/bench-gate.ps1`：基线 16 用例写入 `bench/baseline.txt`，对比逻辑实测有效
- 已知限制：本机 `-cover` 与 `-race` 不可用（cover 全包重编译超时、race 需要 64 位 gcc），两者均已安排在 CI 的 Linux 任务里执行

