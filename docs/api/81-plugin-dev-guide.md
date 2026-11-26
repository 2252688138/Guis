<!-- group: 扩展 -->
# 插件开发指南 · 从 0 到可分发

本文是**动手篇**：怎么写一个能跑起来的插件、怎么调、怎么打包分发。
协议字段与内核 API 的完整参考见 [`80-plugins.md`](80-plugins.md)。

插件写好后由 **guis 应用**加载：工具怎么呈现（按钮 / 命令面板 / 侧栏）由应用自己决定；
路径见 80 篇第 0 节「最小接入（GUI 应用）」。
本文里的 `prompt` / 技能表 / 模型侧工具名等概念属于 **agent 类宿主**（如 deepseek 桌面端），
纯 GUI 应用用不到，看到可跳过。

> 宿主版本 `1.1.0`，支持协议版本 `1` 与 `2`（`2` 才有命令 / 配置 / 进度 / 日志）。

## 0. 先选路线

| 路线 | 适合 | 需要写代码 | 需要编译 | 能力 |
| --- | --- | --- | --- | --- |
| A 声明式 HTTP 工具 | 包一层外部 API / 抓网页 | 否，只写 `plugin.json` | 否 | 工具（宿主代发请求） |
| B 声明式命令工具 | 包一层本机命令（`git`、`ffmpeg`） | 否，只写 `plugin.json` | 否 | 工具（宿主代跑命令） |
| C 进程插件 | 有状态、要访问本地环境、要命令/配置/进度 | 是（任意语言） | 是 | 工具 + 命令 + 配置 + 进度 + 日志 |

三条路线的插件**可以是同一个包**：`plugin.json` 里同时写 `tools`（声明式）与 `runtime`（进程）也合法，
宿主会同时暴露两类工具。

## 1. 目录与清单

```
<dataDir>/plugins/<plugin-id>/
  plugin.json              # 唯一必需文件
  skills/<name>/SKILL.md   # 可选：agent 类宿主注入技能表
  bin/<executable>         # 路线 C 需要
```

`dataDir` 是**宿主应用**的数据目录，由宿主初始化内核时的 `plugins.Options{DataDir: ...}` 决定
（deepseek 桌面端为 `%APPDATA%\guis\deepseek`，你自己的应用填自己的数据目录即可）。
**开发时不要直接往里拷**，用宿主应用的「插件 → 从目录安装」（即 `mgr.Install(ctx, dir)`）装进去，宿主会复制并校验。

最小清单：

```json
{
  "id": "hello",
  "name": "Hello",
  "version": "1.0.0",
  "description": "最小插件",
  "prompt": "需要打招呼时调用 hello__greet。"
}
```

- `id`：`^[a-z0-9][a-z0-9-]{0,63}$`，也是安装目录名；改 id 等于换插件。
- `prompt`：启用后追加进系统提示词，告诉模型**什么时候用你的工具**（仅 agent 类宿主消费，GUI 应用忽略）。
- 工具在模型侧名字是 `<插件ID>__<工具名>`，例如 `web-tools__http_get`（同样只在 agent 宿主侧生效）。

## 2. 路线 A：声明式 HTTP 工具

现成示例：[`cmd/deepseek/examples/plugins/web-tools`](../../cmd/deepseek/examples/plugins/web-tools)
（只是一份 `plugin.json`，与框架、与 agent 均无关，可直接拷进你的项目）。

```json
{
  "id": "weather",
  "name": "天气",
  "version": "1.0.0",
  "description": "查询城市天气",
  "prompt": "用户问天气时调用 weather__get。",
  "tools": [
    {
      "name": "get",
      "description": "查询指定城市的当前天气",
      "parameters": {
        "type": "object",
        "properties": { "city": { "type": "string", "description": "城市名，如 杭州" } },
        "required": ["city"]
      },
      "type": "http",
      "request": {
        "method": "GET",
        "url": "https://api.example.com/weather?city={{city}}&unit=metric",
        "headers": { "User-Agent": "guis-plugin/1.0" }
      },
      "timeoutMs": 20000,
      "safe": true
    },
    {
      "name": "post_demo",
      "description": "POST 一个 JSON（演示 body 模板）",
      "parameters": { "type": "object", "properties": { "text": { "type": "string" } } },
      "type": "http",
      "request": {
        "method": "POST",
        "url": "https://api.example.com/echo",
        "headers": { "Content-Type": "application/json" },
        "body": "{\"text\":\"{{text}}\"}"
      },
      "safe": true
    }
  ]
}
```

模板变量：

- `{{参数名}}`：模型传入的参数，**会做 URL 转义**（放进 JSON body 时注意转义需求）。
- `{{workspace}}`：当前会话工作区路径。
- 返回给模型的内容是「状态码 + 响应体」，响应体截断 64KB。
- `safe: true` 表示只读幂等：免审批，可与其它工具并行。写操作/有副作用的**不要**标 `safe`。
- `timeoutMs` 缺省 30s。
- 声明了 `permissions.net` 时，`url` 的域名必须在白名单内（见第 6 节）。

## 3. 路线 B：声明式命令工具

```json
{
  "id": "git-helpers",
  "version": "1.0.0",
  "prompt": "需要看 git 历史时调用 git_helpers__log。",
  "permissions": { "exec": true },
  "tools": [
    {
      "name": "log",
      "description": "查看最近 20 条提交",
      "parameters": {
        "type": "object",
        "properties": { "path": { "type": "string", "description": "可选路径过滤" } }
      },
      "type": "command",
      "command": "git -C {{workspace}} log --oneline -20 {{path}}",
      "timeoutMs": 15000,
      "safe": true
    }
  ]
}
```

三条前提，缺一不可：

1. `type: "command"` 且 `command` 非空，支持 `{{参数}}` 与 `{{workspace}}`。
2. 宿主调用时 `AllowCommands=true`（用户在应用里允许插件执行命令）。
3. **如果清单里写了 `permissions` 段**，必须 `"exec": true`；不写 `permissions` 的老插件按宽松处理（兼容 v1）。

执行在插件目录下进行，超时（`timeoutMs`，缺省 60s）由宿主强杀进程树；stdout+stderr 合并返回。

## 4. 路线 C：进程插件

现成示例：[`cmd/deepseek/examples/plugins/process-demo`](../../cmd/deepseek/examples/plugins/process-demo)
（Go 手写 NDJSON，不依赖宿主代码，可整体拷到任意项目当模板）。

清单：

```json
{
  "id": "my-plugin",
  "name": "我的插件",
  "version": "1.0.0",
  "prompt": "需要查本机信息时调用 my-plugin__host_info。",
  "minHostVersion": "1.1.0",
  "permissions": { "net": [], "fs": [], "exec": false },
  "configuration": {
    "type": "object",
    "properties": {
      "greeting": { "type": "string", "title": "问候语", "default": "你好" }
    }
  },
  "runtime": {
    "type": "process",
    "command": "bin/my-plugin.exe",
    "protocol": "jsonrpc-stdio",
    "protocolVersion": 2,
    "callTimeoutMs": 15000,
    "startTimeoutMs": 10000,
    "healthIntervalMs": 60000,
    "memoryLimitMB": 256
  }
}
```

约束：`runtime.command` **必须相对插件目录**（禁绝对路径与 `..`）；`env` 只透传显式声明的变量。

传输：stdin/stdout，**每行一条 UTF-8 JSON**；stdout 只能出协议报文，调试信息走 stderr
（宿主保留最近 200 行，插件页可见，也会以 `log` 通知形式进入日志）。

最小骨架（Go，够跑通握手与一个工具）：

```go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type req struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	enc := json.NewEncoder(out)

	for in.Scan() {
		var r req
		if json.Unmarshal(in.Bytes(), &r) != nil {
			continue
		}
		if r.ID == 0 { // 宿主通知：不需要回包
			continue
		}
		switch r.Method {
		case "initialize":
			enc.Encode(map[string]any{"id": r.ID, "result": map[string]any{
				"name": "my-plugin", "version": "1.0.0",
				"capabilities": []string{"tools"},
			}})
		case "tools/list":
			enc.Encode(map[string]any{"id": r.ID, "result": map[string]any{
				"tools": []map[string]any{{
					"name": "host_info", "description": "返回进程信息",
					"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
					"safe":       true,
				}},
			}})
		case "tools/call":
			enc.Encode(map[string]any{"id": r.ID, "result": map[string]any{
				"content": fmt.Sprintf("pid=%d", os.Getpid()),
			}})
		case "ping":
			enc.Encode(map[string]any{"id": r.ID, "result": map[string]any{"ok": true}})
		case "shutdown":
			enc.Encode(map[string]any{"id": r.ID, "result": map[string]any{}})
			return
		default:
			enc.Encode(map[string]any{"id": r.ID,
				"error": map[string]any{"code": -32000, "message": "unknown method " + r.Method}})
		}
		out.Flush()
	}
}
```

编译到插件目录里：

```bash
go build -o my-plugin/bin/my-plugin.exe ./my-plugin   # 插件目录自带 plugin.json 与 bin/
```

然后在宿主应用的插件页里「从目录安装」（或代码里 `mgr.Install(ctx, "./my-plugin")`）选插件目录，打开开关。

## 5. 协议 v2：命令 / 配置 / 进度 / 日志

`initialize` 返回能力集，宿主据此决定调不调相应方法：

```json
{"id":1,"result":{"name":"my-plugin","version":"1.1.0",
 "capabilities":["tools","commands","config","progress","log"]}}
```

追加方法：

| 方法 | 方向 | 参数 | 返回 |
| --- | --- | --- | --- |
| `commands/list` | 宿主→插件 | `{}` | `{commands:[{id,title,description}]}` |
| `commands/exec` | 宿主→插件 | `{id, arguments}` | `{content, isError}` |
| `config/get` | 宿主→插件 | `{}` | `{config:{...}}` |
| `config/set` | 宿主→插件 | `{config:{...}}` | `{config:{...}}` |
| `progress` | 插件→宿主（通知） | `{tool, done, total, message}` | 无需应答 |
| `log` | 插件→宿主（通知） | `{level, message}` | 无需应答 |
| `on_config_change` | 宿主→插件（通知） | `{config:{...}}` | 无需应答 |
| `on_workspace_change` | 宿主→插件（通知） | `{workspace}` | 无需应答 |

- **命令**会出现在宿主的命令面板，标题前缀 `插件：`；命令 ID 建议 `kebab-case`。
- **配置**：`plugin.json` 的 `configuration` 用 JSON Schema 子集（`type` 支持 `string` / `number` / `integer` / `boolean`），
  界面按它生成表单，保存后落盘 `plugins/<id>.config.json` 并给插件推 `on_config_change`。
- **通知**：不带 `id` 的一行 JSON。插件对通知**不要回包**，否则宿主会当成垃圾报文丢弃。
- 进度常用于长任务：`{"method":"progress","params":{"tool":"slow","done":2,"total":5,"message":"第 2 步"}}`。

## 6. 权限与治理

```json
"permissions": {
  "net":  ["api.example.com", "*.trusted.io"],
  "fs":   [],
  "exec": true,
  "env":  ["PATH", "HOME"]
}
```

| 字段 | 作用 | 不写时 |
| --- | --- | --- |
| `net` | 声明式 http 工具的域名白名单（`*.x.com` 匹配子域，不匹配裸域） | 不限制 |
| `fs` | 声明用到的路径（提示与审计，**不阻断**） | 无 |
| `exec` | 是否允许宿主代插件执行命令 | 若声明了 `permissions` 则缺省为拒绝 |
| `env` | 额外透传的环境变量名（进程插件） | 只给最小集合 |

宿主侧还有几层治理，开发时可以直接用：

- **审计**：每次工具/命令调用写 `<dataDir>/plugins_audit.jsonl`（脱敏参数、单行 ≤4KB、文件超 5MB 轮转）。出错先看这里。
- **限流**：单插件默认并发 4 / 每分钟 120 次，超限立刻返回错误（不排队）。长任务请用进度通知而不是并发暴冲。
- **健康检查**：`healthIntervalMs > 0` 时定时 `ping`；连续 2 次失败按重启预算重启，超预算停用并写 `lastError`。
  **你的 `ping` 必须 1 秒内回**，否则会被判不健康。
- **内存上限**：`memoryLimitMB`（Windows 用 Job Object 限制进程内存，超限直接被杀；其他平台暂不支持）。
- **工具缓存**：握手后的工具表写 `<dataDir>/plugins/<id>/.tools.json`，重启宿主后不启动进程也能列出工具；
  插件版本或 `protocolVersion` 变化时自动失效。

> 安全边界：`permissions` 是声明与最小化检查，**不是沙箱**。进程插件以当前用户身份运行，
> 能读能写能联网。只安装可信来源的插件。

## 7. 打包、校验与分发

1. **目录**：把 `plugin.json` 放在 zip **根目录**（不要多包一层文件夹）。

```bash
# Windows
Compress-Archive -Path my-plugin\* -DestinationPath my-plugin.zip
# Linux / macOS
cd my-plugin && zip -r ../my-plugin.zip . -x 'bin/*.pdb'
```

2. **摘要**：

```bash
# Windows
Get-FileHash my-plugin.zip -Algorithm SHA256
# Linux / macOS
shasum -a 256 my-plugin.zip
```

3. **登记到本地 registry**（`<dataDir>/plugins_registry.json`）：

```json
{
  "plugins": [
    {
      "id": "my-plugin",
      "name": "我的插件",
      "version": "1.0.0",
      "description": "示例",
      "url": "https://example.com/my-plugin.zip",
      "sha256": "<上一步得到的 64 位十六进制>"
    }
  ]
}
```

之后调用 `Manager.InstallFromRegistry(ctx, "my-plugin")` 即可「下载 → 校验 → 安装」；
摘要匹配的插件在列表里标记 `trust=verified`（只代表包没被篡改，不代表代码安全）。
`sha256` 不匹配会**直接拒绝且不落盘**。

4. **版本约束**：清单写 `"minHostVersion": "1.1.0"`，宿主版本过低会拒绝安装并提示双方版本。
   新增了需要 v2 协议的能力时，务必同时把 `runtime.protocolVersion` 提到 `2`。

## 8. 调试与排错

| 症状 | 常见原因 | 处理 |
| --- | --- | --- |
| 安装报「缺少 plugin.json」 | zip 多包了一层 / 文件名大小写 | 让 `plugin.json` 在 zip 根目录 |
| 安装报「minHostVersion」 | 清单要求的版本高于宿主 | 降 `minHostVersion` 或升级宿主 |
| 插件卡片显示「错误」 | 清单校验失败（id 不合法、runtime.command 越界） | 看卡片上的错误文案，改 `plugin.json` |
| 启用后没有工具 | 能力没声明 / 进程启动失败 | 看插件页「最近错误」与审计日志；`tools/list` 必须返回数组 |
| 工具名冲突被忽略 | 同一插件内工具重名或名字非法 | 工具名用小写 + 下划线，插件内唯一 |
| 命令面板里没有命令 | 没声明 `commands` 能力 / 进程未运行 | `initialize` 加 `commands`，返回非空 `commands/list` |
| 配置保存了但插件没变 | 没实现 `on_config_change` 或没处理通知 | 见第 5 节；通知不带 id，别回包 |
| 反复重启 / 被停用 | `ping` 超时、崩溃 | 查 stderr（插件页日志）；修 `ping` 时延或启动耗时 |
| 工具调用报「超时」 | 超过 `callTimeoutMs` | 长任务改成分步 + 进度通知 |
| 审计里参数是 `***` | 键名含 token/key/secret/password 等 | 正常脱敏，不是 bug |

自检顺序（进程插件）：`initialize` → `tools/list` → `tools/call` → 插件页错误 → `plugins_audit.jsonl` → stderr。

## 9. 发布前检查清单

- [ ] `id` 合法且未与已装插件冲突；`version` 递增。
- [ ] `prompt` 写清「什么时候用」；工具 `description` 写清「做什么 + 返回什么」。
- [ ] 有副作用的工具**不要**标 `safe`。
- [ ] 进程插件：`ping` < 1s、启动 < `startTimeoutMs`、stdout 只有协议行。
- [ ] 声明了 `permissions` 就把 `net` / `exec` 收窄到实际需要。
- [ ] v2 能力：`capabilities`、`protocolVersion: 2`、`commands/list`、`config/get`、`on_config_change`。
- [ ] 打包后校验 sha256 再登记 registry。
- [ ] 在本机走一遍：安装 → 启用 → 调工具 → 执行命令 → 改配置 → 停用 → 卸载（不留残留进程）。
