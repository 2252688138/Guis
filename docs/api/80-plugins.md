<!-- group: 扩展 -->
# 插件内核 · internal/plugins

GUIS 框架的插件扩展能力：`internal/plugins` 是一个**与 UI 无关**的插件内核，
guis 应用（跑在 `ui` 之上）用它加载「声明式工具 / 进程外插件」，再把插件暴露的能力挂到
自己的按钮、命令面板或侧栏上——插件只声明能力，界面怎么呈现由应用决定。

内核**不依赖** `ui`、`cmd/*`，只依赖标准库 + `golang.org/x/sys`（进程树强杀），
所以任何宿主（包括非 GUI 程序）都能直接复用同一份代码。
deepseek 桌面端用它接 agent，那部分用法收在文末「附：在 agent 中复用」。

> 动手流程（选型、完整示例、打包分发、排错清单）见 [`81-plugin-dev-guide.md`](81-plugin-dev-guide.md)；本文只讲字段与 API。

> [!IMPORTANT]
> 进程外插件会执行**真实代码**。内核的安全底线：
> 安装后**默认停用**、启用需用户显式操作、非 `safe` 工具默认**需要审批**、
> `runtime.command` 必须落在插件目录内、不给插件透传宿主环境变量。

## 0. 最小接入（GUI 应用）

```go
mgr := plugins.New(plugins.Options{DataDir: dataDir}) // dataDir：应用自己的数据目录
defer mgr.Close()                                     // 结束所有插件子进程
mgr.StartAll(ctx)                                     // 后台拉起已启用的进程插件

// 可用工具 = 声明式工具 + 已启动进程插件的缓存工具
tools := append(mgr.DeclarativeTools(), mgr.RunningProcessTools("demo")...)

// 挂到自己的界面：一个插件工具就是一个按钮 / 一条命令
var lastResult string
for _, td := range tools {
    td := td
    row = append(row, ui.Button(td.Description, func() {
        res, err := mgr.Call(ctx, td.PluginID, td.Name, map[string]any{"q": "guis"},
            plugins.Env{Workspace: ws, DataDir: dataDir, AllowCommands: false})
        if err != nil {
            lastResult = "调用失败：" + err.Error()
            return
        }
        if res.IsError {
            lastResult = "插件报错：" + res.Text
            return
        }
        lastResult = res.Text // 结果怎么渲染（文本 / 表格 / 弹窗）由应用决定
    }))
}
```

- `Options.DataDir`：插件清单、二进制与 `plugins.json` 的根，通常和应用其它本地状态放一起。
- `Call` 的 `Env`：`Workspace` 填当前项目目录，模板里的 `{{workspace}}` 用它；`AllowCommands` 为 false 时命令类工具直接报错。
- 启停状态持久化在 `<dataDir>/plugins.json`，用户下次打开应用沿用。
- 安全默认值：安装后停用、`safe` 之外的工具要审批（见第 5 节）。

> 清单字段看第 2 节，内核方法看第 3 节，进程协议看第 4 节，治理与审计看第 9 节。

## 1. 目录结构

```
<dataDir>/plugins/<plugin-id>/
  plugin.json                 # 清单（必需）
  skills/<name>/SKILL.md      # 技能（可选，agent 类宿主会注入技能表）
  bin/<executable>            # 进程插件的可执行文件（可选）
<dataDir>/plugins.json        # 启停状态（与旧版本兼容，只增字段）
```

## 2. plugin.json 全字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 必填，kebab-case（`^[a-z0-9][a-z0-9-]{0,63}$`） |
| `name` / `version` / `description` | string | 展示信息，缺省时 `name=id`、`version=0.0.0` |
| `prompt` | string | 启用后追加到系统提示词（agent 类宿主消费，GUI 应用可忽略） |
| `skills` | []string | 技能目录（相对插件目录或绝对路径），缺省 `["skills"]`（同上） |
| `tools` | []Tool | 声明式工具，见下 |
| `mcpServers` | []MCP | 由宿主 MCP 管理器拉起 `{id, command, args, env, cwd, timeoutMs}` |
| `runtime` | Runtime | 进程外插件声明，见第 4 节 |
| `permissions` | Permissions | `{net, fs, exec, env}`，见第 9 节 |
| `configuration` | object | 配置项声明（JSON Schema 子集），UI 据此生成表单并持久化到 `plugins/<id>.config.json` |
| `minHostVersion` | string | 宿主要求的最低版本，见第 10 节 |
| `trust` | string | `unverified`（默认）/ `verified`（摘要校验通过后由安装流程标记） |

声明式工具：

```jsonc
{
  "name": "http_get",
  "description": "抓网页文本",
  "parameters": {"type":"object","properties":{"q":{"type":"string"}}},
  "type": "http",                       // http | command
  "request": {"method":"GET","url":"https://example.com/?q={{q}}","headers":{},"body":""},
  "command": "Get-Date",                // type=command 时必填，支持 {{arg}} / {{workspace}}
  "timeoutMs": 30000,
  "safe": true                          // 只读幂等：免审批、可与其它工具并行
}
```

## 3. 内核 API

```go
m := plugins.New(plugins.Options{
    DataDir: dataDir,
    Runner:  plugins.CommandFunc(myRunner), // 可选：替换 command 执行器（接自己的进程树管控）
    Logf:    logf,                          // 可选
})
defer m.Close()                            // 结束所有插件子进程

m.List()                                   // []PluginInfo（含 Runtime / Running / LastError）
m.Install(ctx, "/path/to/plugin")          // 目录或 zip（含 zip-slip 防护）
m.SetEnabled(id, true)                     // 进程插件：这里才会拉起子进程
m.Uninstall(id)                            // 先停进程再删目录
m.EnabledManifests()                       // 已启用清单副本（构建工具/技能/提示词用）
m.DeclarativeTools()                       // 声明式工具描述
m.ProcessTools(ctx, id)                    // 进程插件工具（必要时启动并握手）
m.RunningProcessTools(id)                  // 已启动插件缓存的工具（不阻塞）
m.Call(ctx, id, tool, args, env)           // 统一调用入口，返回 CallResult{Text, IsError}
m.ProcessLogs(id)                          // 最近 200 行 stderr
m.StartAll(ctx)                            // 启动所有已启用的进程插件
```

`plugins.Env{Workspace, DataDir, AllowCommands}` 决定模板里的 `{{workspace}}` 与 command 工具是否放行。

## 4. 进程插件运行时

```jsonc
"runtime": {
  "type": "process",              // 唯一支持的运行时
  "command": "bin/demo.exe",      // 必须相对插件目录，禁止绝对路径与 ..
  "args": ["--stdio"],
  "env": {"FOO": "bar"},          // 只透传这些变量（外加 PATH/HOME/TEMP/SYSTEMROOT 等最小集合）
  "protocol": "jsonrpc-stdio",    // 目前唯一取值
  "protocolVersion": 1,
  "callTimeoutMs": 30000,
  "startTimeoutMs": 10000,
  "autoRestart": true             // 崩溃后自动重启（3 次 / 5 分钟预算）
  "healthIntervalMs": 60000,     // >0 时按此间隔 ping（0 = 不做健康检查）
  "memoryLimitMB": 256            // >0 时限内存（Windows 用 Job Object）
}
```

传输：stdin/stdout，**每行一条 UTF-8 JSON**（NDJSON）。stdout 只能输出协议报文，调试信息走 stderr。

| 方法 | 方向 | 参数 | 返回 |
| --- | --- | --- | --- |
| `initialize` | 宿主→插件 | `{protocolVersion, pluginId, workspace, dataDir, host}` | `{name, version, capabilities}` |
| `tools/list` | 宿主→插件 | `{}` | `{tools:[{name, description, parameters, safe}]}` |
| `tools/call` | 宿主→插件 | `{name, arguments}` | `{content, isError}` |
| `ping` | 宿主→插件 | `{}` | `{ok:true}` |
| `shutdown` | 宿主→插件 | `{}` | `{}` |

v2 追加方法见第 8 节（`commands/*`、`config/*`）与插件→宿主通知（`progress` / `log`）。

示例报文：

```jsonc
// 宿主 → 插件
{"id":1,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}
// 插件 → 宿主
{"id":1,"result":{"content":"hi"}}
```

约束：单行上限 1MB；`tools/call` 结果截断 64KB；工具名需匹配 `^[a-zA-Z0-9_-]{1,64}$`，
与声明式工具同名时进程工具会被丢弃；调用超时会强杀该插件进程；崩溃只影响自身工具。

## 5. GUI 应用接入要点

最小代码见第 0 节；这一节讲接进去之后要自己补的几件事。

```go
// 启停：用户点开关时同步持久化
mgr.SetEnabled("demo", on)   // 进程插件在这一步才拉起子进程；false 会先停进程

// 刷新：安装 / 卸载 / 启停后重拉一次列表
mgr.List()                   // []PluginInfo：Enabled / Running / Healthy / Tools / LastError

// 审批：非 safe 工具在受控权限下必须由用户确认
for _, td := range mgr.DeclarativeTools() {
    if !td.Safe { /* 先弹确认框，再调 Call */ }
}

// 排错：把插件 stderr 尾部显示在插件页
mgr.ProcessLogs("demo")      // 最近 200 行
```

推荐的 UI 落点（gallery 与各类 guis 应用通用）：

| 位置 | 内容 |
| --- | --- |
| 设置 → 插件 | `List()` 列表、启用开关、`Install` / `Uninstall`、`ProcessLogs` |
| 命令面板 / 菜单 | 每个 `ToolDescriptor` 一条命令，标题用 `td.Description` 或自定义映射 |
| 主界面按钮 | 高频工具做成按钮；`td.Safe` 为 true 的可以静默执行 |
| 结果区 | `CallResult.Text` 直接渲染；`IsError` 为 true 时按错误样式展示 |

> `prompt`、`skills`、`mcpServers` 是给 agent 类宿主消费的字段；纯 GUI 应用忽略它们即可，不影响安装、启停与调用。

## 6. 示例

参考实现放在 `cmd/deepseek/examples/plugins/` 下，但**与框架、与 agent 都无关**，可以整目录拷到任何项目：

- `process-demo/`：可编译的进程插件（`echo` / `host_info` / 慢任务进度），只依赖标准库，
  用 `bufio.Scanner` 读 stdin、`json.Encoder` 写 stdout 手写 NDJSON；编译与安装见其 `README.md`。
- `web-tools/`：纯 `plugin.json` 的声明式 HTTP 工具，没有一行代码。

跑起一个进程插件的最小路径：实现 `initialize` / `tools/list` / `tools/call` / `shutdown`
四个方法（报文格式见第 4 节），`process-demo/main.go` 可直接当模板复制。

## 7. 常见问题

| 现象 | 原因 / 处理 |
| --- | --- |
| 安装后列表里 `enabled=false` | 进程插件默认停用，需在插件页/`SetEnabled` 显式启用 |
| 工具没出现 | 进程未启动或握手失败，看 `List()[i].LastError` 与 `ProcessLogs(id)` |
| 提示「插件工具需要审批」 | 非 `safe` 工具在非完整权限下必须确认，或给插件工具声明 `"safe": true` |
| 反复崩溃 | 超过 3 次 / 5 分钟重启预算后停止重启，需排查插件自身或先停用 |
| Windows 有残留进程 | 内核用 Job Object 绑定进程树，`Close()` 时整组终止；自建执行器请复用同一套管控 |


## 8. 协议 v2：命令 / 配置 / 进度 / 日志

插件在 `initialize` 里返回 `capabilities` 声明能力；缺省视为 `["tools"]`（即 v1 插件）。
能力集：`tools` / `commands` / `progress` / `log` / `config`。

| 方法 | 方向 | 参数 | 返回 |
| --- | --- | --- | --- |
| `commands/list` | 宿主→插件 | `{}` | `{commands:[{id, title, description}]}` |
| `commands/exec` | 宿主→插件 | `{id, arguments}` | `{content, isError}` |
| `config/get` | 宿主→插件 | `{}` | `{config:{...}}` |
| `config/set` | 宿主→插件 | `{config:{...}}` | `{config:{...}}` |
| `progress` | 插件→宿主通知 | `{tool, done, total, message}` | 无需应答 |
| `log` | 插件→宿主通知 | `{level, message}` | 无需应答 |
| `on_config_change` | 宿主→插件通知 | `{config:{...}}` | 无需应答 |
| `on_workspace_change` | 宿主→插件通知 | `{workspace}` | 无需应答 |

通知是**不带 id** 的一行 JSON，宿主不会回包；插件不要对通知回复。
`progress` / `log` 会进入宿主的事件环，UI 与日志可读（`Manager.Progress` / `Manager.ProcessLogs`）。

## 9. 治理：缓存 / 审计 / 限流 / 权限 / 健康检查

- **工具表缓存**：握手成功后写 `<plugin>/.tools.json`（记录插件版本与协议版本）。
  插件版本或协议版本变化、停用或卸载时缓存自动失效；宿主重启后即使不拉起进程也能列出工具。
- **审计**：每次插件工具/命令调用写 `<dataDir>/plugins_audit.jsonl` 一行（≤4KB），
  含 `plugin`、`tool`、`kind`、`argsDigest`、`durationMs`、`ok`、`isError`、`resultBytes`、`error`；
  参数里名字含 token / key / secret / password / authorization / cookie 的值会被打成 `***`，长值截断；文件超 5MB 自动轮转为 `.1.jsonl`。
- **限流**：默认单插件并发 4、每分钟 120 次，超限直接返回可读错误（不排队）；
  `plugins.Options{MaxConcurrent, MaxPerMinute}` 可调。
- **权限**：`permissions.net` 非空时，http 型工具只能访问白名单域名（支持 `*.example.com`）；
  `permissions.exec` 控制宿主要不要代插件执行命令；`permissions.env` 额外透传环境变量。
  未声明 `permissions` 段的旧插件保持宽松行为（只对显式声明者收紧）。
- **健康检查**：`runtime.healthIntervalMs` 非 0 时定时 `ping`；连续 2 次失败按重启预算重启，
  超预算则停用并把原因写进 `lastError`（同时回调宿主 `OnUnhealthy`）。
- **内存限额**：`runtime.memoryLimitMB` 在 Windows 上通过 Job Object 的进程内存上限实现；
  其他平台暂不支持（不报错，仅记录日志）。

## 10. 版本约束、registry 与安装校验

- `minHostVersion`：清单里声明宿主要求的最低版本，低于 `HostVersion` 直接拒绝安装（语义化版本比较，支持 `v1.2`、`1.2` 等写法）。
- 本地 registry：`<dataDir>/plugins_registry.json`
  ```jsonc
  {"plugins":[{"id":"demo","url":"https://example.com/demo.zip","sha256":"<64 hex>"}]}
  ```
  `Manager.Registry()` 读清单，`Manager.InstallFromRegistry(ctx, id)` 下载 → 校验 → 安装；
  摘要校验通过后插件在 `List()` 里标记为 `trust=verified`（否则 `unverified`）。
- `InstallWithOptions(ctx, src, InstallOptions{ExpectedSHA256})` 可对任意 zip 做摘要校验；目录安装源不支持摘要（会报错）。

## 11. 兼容与降级

| 场景 | 行为 |
| --- | --- |
| v1 插件（不返回 capabilities） | 只暴露 tools；`Commands` 返回空，不报错 |
| 插件不声明 `commands` 能力 | 宿主不调用 `commands/list`，命令面板无该插件条目 |
| 插件不声明 `config` 能力 | 配置仍存文件，但不推 `on_config_change` |
| 缓存版本不匹配 / 文件损坏 | 视为无缓存，下次握手重新拉取 |
| 进程崩溃 | `autoRestart` 预算内重启；超预算返回错误并写 `lastError` |
| 插件反复不健康且 `autoRestart=false` | 停用插件，原因写 `lastError`，不无限重启 |

## 12. 安全边界（重要）

`permissions` 是**声明与最小化检查**，不是沙箱：

- 进程插件运行在**用户同一账号**下，可读写该账号能访问的文件、可联网。`permissions.fs` 只做提示与审计，不阻断系统调用。
- `permissions.net` 只约束**声明式 http 工具**（宿主代发请求时检查域名）；插件自己发起的网络请求不受限。
- `permissions.exec` 只约束**声明式 command 工具**；插件进程内自行 `exec` 不受限。
- `env` 只影响透传的环境变量集合；插件仍能通过文件系统读到宿主可读的一切。
- 结论：**只安装可信来源的插件**。`trust=verified` 只表示安装包摘要与 registry 一致，不表示代码安全。
- 可用的收口手段：默认停用 + 显式启用、`memoryLimitMB` 内存上限、健康检查自动停用、审计日志留痕。

### v1 → v2 迁移清单

1. `initialize` 里返回 `capabilities: ["tools","commands","progress","log","config"]`（按需子集）。
2. `runtime.protocolVersion` 改为 `2`（不写也能跑，但 v2 方法不会被调用）。
3. 需要命令：实现 `commands/list` + `commands/exec`；命令 ID 会在 UI 里显示为 `插件：<title>`。
4. 需要配置：`plugin.json` 增加 `configuration`（JSON Schema 子集），实现 `config/get`，并处理宿主推送的 `on_config_change` 通知。
5. 进度与日志：用**不带 id** 的 `{"method":"progress",...}` / `{"method":"log",...}` 通知。
6. 建议补 `permissions`、`minHostVersion`、`runtime.healthIntervalMs`、`runtime.memoryLimitMB`。
7. 兼容性：宿主对 v1 插件保持原行为（只调 `tools/*`、`ping`、`shutdown`）。

## 附：在 agent 中复用（deepseek 桌面端）

同一份内核也被 **agent 类宿主**复用：`agent.NewPluginManager(dataDir)`（`cmd/deepseek/agent`）
把内核的工具适配成 `agent.Tool`，并把插件的 `prompt`、`skills/`、`mcpServers` 接进对话循环——
插件在这里不只是「一组函数」，还能携带技能文档与 MCP 服务器。

因此下面几项只在 agent 宿主上有意义（纯 GUI 应用忽略即可）：

| 字段 | agent 侧用途 |
| --- | --- |
| `prompt` | 启用后追加到系统提示词 |
| `skills` | 技能目录，`SKILL.md` 注入技能表 |
| `mcpServers` | 由宿主的 MCP 管理器拉起子进程 |

审批语义两边一致：agent 里是「调用前的确认提示」，GUI 应用里是你自己的确认弹窗，
判断依据相同——`ToolDescriptor.Safe` 与 `Env.AllowCommands`。

