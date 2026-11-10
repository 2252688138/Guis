<!-- group: 入门 -->
# 热重载 · Hot Reload（go run 改代码自动重编译）

`go run ./cmd/gallery` 起的窗口，改完 `.go` 源码后**自动重新编译并重新出图**，不用 Ctrl+C 再跑一遍。
热重载由框架自身托管，不依赖 air / realize 之类的第三方工具。

## 1. 怎么触发

| 条件 | 行为 |
| --- | --- |
| 可执行文件位于 go 的临时构建目录（路径含 `go-build`，即 `go run` 的产物） | 自动开启 |
| `GUIS_HOT=1`（`on` / `true` / `yes` 同义） | 强制开启（普通 `go build` 的 exe 也能开） |
| `GUIS_HOT=0`（`off` / `false` / `no` 同义） | 关闭 |

```bash
go run ./cmd/gallery            # 开箱即用的热重载
GUIS_HOT=0 go run ./cmd/gallery  # 临时关闭
```

> 打包后的正式 exe 本来就不会命中（路径里没有 `go-build`），无需特殊处理。

## 2. 工作原理

Go 不支持在同一进程内替换已编译的代码（剪掉 plugin：Windows 不可用且无法卸载），
所以采用与 air 同构、但由框架托管的**双进程模型**：

```text
父进程（go run 出来的临时 exe）：守护进程，不建窗口
  ├─ 轮询源码（默认 300ms）→ 变化后再等 220ms 合并多文件保存
  ├─ go build 出新 exe（失败时保留旧窗口继续运行，只在控制台打印错误）
  ├─ 通过 stdin 给旧子进程发 "save" → 旧进程写现场后退出（最多等 3s）
  └─ 用新 exe 起新子进程（GUIS_HOT_CHILD=1），恢复现场

子进程：真正的应用
  └─ 正常跑消息泵；收到 save 时把窗口几何 + 业务状态写入 state.json
```

视觉上等价于「同一扇窗口的内容被刷新」，代价是窗口会重建一次（通常 < 1 秒）。

控制台输出形如：

```text
热重载已开启：监听 H:\JIMaX\guis（GUIS_HOT=0 可关闭）
编译完成（820ms）
已重新加载（860ms）
```

## 3. 状态保留：SetHotState

默认只保留**窗口位置 / 尺寸 / 最大化**。业务状态（当前页面、滚动位置、输入框内容……）需要自己接：

```go
func init() {
    ui.SetHotState(
        func() []byte { return []byte(currentPage + "\n" + searchText) },
        func(b []byte) { /* 新进程建好窗口后调用一次 */ },
    )
}
```

* `save` 在**后台 goroutine** 里执行（收到守护进程指令时）：涉及 UI 状态请自行加锁，或用 `ui.Post` 投递后尽快返回；
* `restore` 在新进程窗口建好后调用一次，入参即上次 `save` 的返回值；
* 两者都可传 `nil`，也可只传其中一个；
* 回调里 panic 会被框架 `recover`，不会连累重启；
* 非热重载运行时两个回调**永远不会被调用**。

## 4. 判断当前是否处于热重载

```go
if ui.HotReloadActive() {
    ui.Toast("热重载模式：保存 .go 文件即重新加载")
}
```

`HotReloadActive()` 只在「本进程由守护进程启动」时返回 true，
用 `go run` 被守护的父进程（不建窗口）和打包后的正式 exe 都返回 false。

## 5. 环境变量一览

| 变量 | 作用 |
| --- | --- |
| `GUIS_HOT` | `1` 强制开启 / `0` 关闭，留空按「是否 go run」自动判断 |
| `GUIS_HOT_CHILD` | 由守护进程设置，标记「我是子进程」，业务不要手动设置 |
| `GUIS_HOT_STATE` | 现场文件路径（默认 `<临时目录>/guis-hot-<pid>/state.json`），由守护进程下发 |

## 6. 注意事项

* **仅桌面端**：`js/wasm`、Android、iOS 上 `SetHotState` 是空操作，`HotReloadActive()` 恒为 false；
* 改的是**源码**才生效，改资源文件（图片 / 字体 / 数据）不会触发重编译，需要自己刷新；
* 每次重载都会**新建窗口**：不要指望保留 GL 资源、打开的文件句柄、WebView 会话；
* 编译失败不会白屏：旧窗口继续运行，控制台打印 `编译失败，保留当前界面继续运行：…`；
* 子进程退出（关窗口）时守护进程也一起退出，不会留下后台进程。
