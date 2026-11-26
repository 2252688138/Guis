// Package plugins 是框架级插件内核：清单校验、安装启停、声明式工具执行，
// 以及进程外插件（stdio + NDJSON JSON-RPC）的运行时宿主。
//
// 本包不依赖 ui / cmd/*，只依赖标准库与 golang.org/x/sys（进程树强杀），
// 因此 GUI 应用与 agent 都能直接复用。
//
// 插件目录结构：
//
//	<plugin>/
//	  plugin.json
//	  skills/<name>/SKILL.md      （可选）
//	  bin/<executable>            （进程插件，可选）
package plugins

import (
	"encoding/json"
	"strings"
	"time"
)

// 运行时类型与协议常量。
const (
	// RuntimeProcess 进程外插件（独立可执行文件 + stdio JSON-RPC）。
	RuntimeProcess = "process"
	// ProtocolJSONRPCStdio 每行一条 JSON 的 JSON-RPC 1.x 协议。
	ProtocolJSONRPCStdio = "jsonrpc-stdio"
	// ProtocolCurrent 宿主支持的最高协议版本。
	ProtocolCurrent = 2
	// HostVersion 宿主版本（用于 minHostVersion 约束）。
	HostVersion = "1.1.0"
)

// 插件可声明的能力（P2 协议）。
const (
	CapTools    = "tools"
	CapCommands = "commands"
	CapProgress = "progress"
	CapLog      = "log"
	CapConfig   = "config"
)

// 信任等级。
const (
	// TrustUnverified 未验证（默认）。
	TrustUnverified = "unverified"
	// TrustVerified 已验证（签名为预留能力，当前仅由 registry 摘要校验置位）。
	TrustVerified = "verified"
)

const (
	// DefaultCallTimeout 单次工具调用默认超时。
	DefaultCallTimeout = 30 * time.Second
	// DefaultStartTimeout 握手（启动 + initialize）默认超时。
	DefaultStartTimeout = 10 * time.Second
	// DefaultPingTimeout 健康检查 ping 超时。
	DefaultPingTimeout = 5 * time.Second
	// DefaultHealthInterval 健康检查间隔，0 表示不检查。
	DefaultHealthInterval = 60 * time.Second
	// MaxResultBytes 单次调用回灌文本上限。
	MaxResultBytes = 64 << 10
	// DefaultMaxConcurrent 单个插件默认并发上限。
	DefaultMaxConcurrent = 4
	// DefaultMaxPerMinute 单个插件默认每分钟调用上限。
	DefaultMaxPerMinute = 120
)

// Env 一次工具调用的宿主上下文。
type Env struct {
	// Workspace 工作区绝对路径（模板里的 {{workspace}}）。
	Workspace string
	// DataDir 宿主数据目录（进程插件的 dataDir 参数）。
	DataDir string
	// AllowCommands 是否允许执行 command 型声明工具。
	AllowCommands bool
}

// Permissions 插件声明自己需要的能力。
//
// 注意：这只约束「经宿主执行」的 http / command 工具；进程插件自身是普通进程，
// 内核无法限制它读写文件，真隔离要靠后续的 WASM 运行时与签名机制。
type Permissions struct {
	// FS 需要访问的路径（仅展示与审计）。
	FS []string `json:"fs,omitempty"`
	// Net 允许访问的域名白名单；为空表示不限制。
	Net []string `json:"net,omitempty"`
	// Exec 是否允许通过宿主执行命令（declarative command 工具）。
	Exec bool `json:"exec,omitempty"`
	// Env 需要透传给插件进程的额外环境变量名。
	Env []string `json:"env,omitempty"`

	// declared 清单里是否显式写了 permissions 段（未写则保持旧插件的宽松行为）。
	declared bool
}

// Declared 清单是否显式声明了 permissions。
func (p Permissions) Declared() bool { return p.declared }

// RuntimeSpec plugin.json 里的 runtime 段。
type RuntimeSpec struct {
	// Type "" 或 "process"。
	Type string `json:"type,omitempty"`
	// Command 可执行文件，相对插件目录（禁止绝对路径与 ..）。
	Command string `json:"command,omitempty"`
	// Args 启动参数。
	Args []string `json:"args,omitempty"`
	// Env 追加给插件进程的环境变量（不透传宿主其余变量）。
	Env map[string]string `json:"env,omitempty"`
	// Protocol 目前只支持 jsonrpc-stdio。
	Protocol string `json:"protocol,omitempty"`
	// ProtocolVersion 1 或 2。
	ProtocolVersion int `json:"protocolVersion,omitempty"`
	// CallTimeoutMs 单次工具调用超时，0 用默认值。
	CallTimeoutMs int `json:"callTimeoutMs,omitempty"`
	// StartTimeoutMs 启动 + 握手超时，0 用默认值。
	StartTimeoutMs int `json:"startTimeoutMs,omitempty"`
	// AutoRestart 崩溃后是否自动重启（默认 true）。
	AutoRestart *bool `json:"autoRestart,omitempty"`
	// MemoryLimitMB 子进程内存上限（Windows Job Object），0 = 不限。
	MemoryLimitMB int `json:"memoryLimitMB,omitempty"`
	// HealthIntervalMs 健康检查间隔，0 = 默认 60s，负数 = 关闭。
	HealthIntervalMs int `json:"healthIntervalMs,omitempty"`
	// PingTimeoutMs 健康检查 ping 超时，0 用默认值。
	PingTimeoutMs int `json:"pingTimeoutMs,omitempty"`
}

// IsProcess 是否声明为进程外插件。
func (r RuntimeSpec) IsProcess() bool {
	return strings.EqualFold(strings.TrimSpace(r.Type), RuntimeProcess)
}

// CallTimeout 单次调用超时。
func (r RuntimeSpec) CallTimeout() time.Duration {
	if r.CallTimeoutMs <= 0 {
		return DefaultCallTimeout
	}
	return time.Duration(r.CallTimeoutMs) * time.Millisecond
}

// StartTimeout 启动握手超时。
func (r RuntimeSpec) StartTimeout() time.Duration {
	if r.StartTimeoutMs <= 0 {
		return DefaultStartTimeout
	}
	return time.Duration(r.StartTimeoutMs) * time.Millisecond
}

// AutoRestartEnabled 是否允许崩溃后重启。
func (r RuntimeSpec) AutoRestartEnabled() bool {
	if r.AutoRestart == nil {
		return true
	}
	return *r.AutoRestart
}

// HealthInterval 健康检查间隔（0 = 关闭）。
func (r RuntimeSpec) HealthInterval() time.Duration {
	if r.HealthIntervalMs < 0 {
		return 0
	}
	if r.HealthIntervalMs == 0 {
		return DefaultHealthInterval
	}
	return time.Duration(r.HealthIntervalMs) * time.Millisecond
}

// PingTimeout 健康检查超时。
func (r RuntimeSpec) PingTimeout() time.Duration {
	if r.PingTimeoutMs <= 0 {
		return DefaultPingTimeout
	}
	return time.Duration(r.PingTimeoutMs) * time.Millisecond
}

// PluginTool 插件声明的工具（声明式，type 为 http / command）。
type PluginTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  json.RawMessage   `json:"parameters"`
	Type        string            `json:"type"` // http / command
	Request     *PluginHTTPAction `json:"request,omitempty"`
	Command     string            `json:"command,omitempty"`
	TimeoutMs   int               `json:"timeoutMs,omitempty"`
	Safe        bool              `json:"safe,omitempty"`
}

// PluginHTTPAction HTTP 型工具的动作定义。
type PluginHTTPAction struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// MCPServer 插件声明的 MCP 服务器（由宿主侧 MCP 管理器消费）。
type MCPServer struct {
	ID        string            `json:"id"`
	Command   string            `json:"command"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	TimeoutMs int               `json:"timeoutMs,omitempty"`
	// WaitPort 启动后先等该端口监听成功再握手（0 = 不等），宿主内部使用。
	WaitPort int `json:"-"`
}

// Manifest 插件清单（plugin.json）。
type Manifest struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Prompt      string       `json:"prompt,omitempty"`
	Skills      []string     `json:"skills,omitempty"`
	Tools       []PluginTool `json:"tools,omitempty"`
	MCPServers  []MCPServer  `json:"mcpServers,omitempty"`
	Runtime     RuntimeSpec  `json:"runtime,omitempty"`
	// Permissions 插件声明的权限（约束宿主代执行的工具）。
	Permissions Permissions `json:"permissions,omitempty"`
	// Configuration 配置项 JSON Schema 子集，供 UI 生成表单。
	Configuration json.RawMessage `json:"configuration,omitempty"`
	// MinHostVersion 最低宿主版本要求（如 "1.1.0"）。
	MinHostVersion string `json:"minHostVersion,omitempty"`
	// Trust 信任等级（unverified / verified）。
	Trust string `json:"trust,omitempty"`
	// Dir 插件安装目录（不进清单 JSON）。
	Dir string `json:"-"`
}

// PluginInfo 插件页展示信息。
type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Dir         string `json:"dir"`
	Enabled     bool   `json:"enabled"`
	Tools       int    `json:"tools"`
	Skills      int    `json:"skills"`
	// Runtime 运行时类型（"" 表示纯声明式，process 表示进程外）。
	Runtime string `json:"runtime,omitempty"`
	// Running 进程插件当前是否有活着的子进程。
	Running bool `json:"running,omitempty"`
	// Healthy 上一次健康检查是否通过（未知为 true）。
	Healthy bool `json:"healthy"`
	// CachedTools 缓存的进程工具数量。
	CachedTools int `json:"cachedTools,omitempty"`
	// Trust 信任等级。
	Trust string `json:"trust,omitempty"`
	// Permissions 声明的权限。
	Permissions Permissions `json:"permissions,omitempty"`
	// Configuration 配置项声明（JSON Schema 子集）。
	Configuration json.RawMessage `json:"configuration,omitempty"`
	// LastError 运行期错误（启动失败 / 崩溃 / 超预算 / 健康检查失败）。
	LastError string `json:"lastError,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ToolDescriptor 与宿主无关的工具描述。
type ToolDescriptor struct {
	// PluginID 所属插件。
	PluginID string
	// Name 插件内部原始工具名（未经宿主规范化）。
	Name string
	// Description 描述。
	Description string
	// Parameters 参数 JSON Schema。
	Parameters json.RawMessage
	// Safe 只读且幂等，可与其它工具并行且免审批。
	Safe bool
	// Kind http / command / process。
	Kind string
}

// CallResult 工具调用结果。
type CallResult struct {
	// Text 回灌给调用方的文本。
	Text string
	// IsError 插件明确报告失败（仍属正常协议响应）。
	IsError bool
}

// CommandDescriptor 插件贡献的命令。
type CommandDescriptor struct {
	PluginID    string `json:"pluginId"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ProgressEvent 插件上报的进度。
type ProgressEvent struct {
	PluginID string
	Tool     string
	Done     int
	Total    int
	Message  string
	At       time.Time
}

// LogEvent 插件主动上报的结构化日志。
type LogEvent struct {
	PluginID string
	Level    string
	Message  string
	At       time.Time
}
