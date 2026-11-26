package plugins

import "encoding/json"

// 协议方法名。
const (
	MethodInitialize   = "initialize"
	MethodToolsList    = "tools/list"
	MethodToolsCall    = "tools/call"
	MethodPing         = "ping"
	MethodShutdown     = "shutdown"
	MethodCommandsList = "commands/list"
	MethodCommandsExec = "commands/exec"
	MethodConfigGet    = "config/get"
	MethodConfigSet    = "config/set"
)

// 插件 → 宿主的通知方法名（无 id，不期待响应）。
const (
	NotifyProgress        = "progress"
	NotifyLog             = "log"
	NotifyConfigChange    = "on_config_change"
	NotifyWorkspaceChange = "on_workspace_change"
)

// HostCapabilitiesV2 协议 v2 宿主支持的能力集合。
var HostCapabilitiesV2 = []string{CapTools, CapCommands, CapProgress, CapLog, CapConfig}

// rpcRequest 宿主发往插件的一行 JSON。
type rpcRequest struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// rpcError 插件返回的错误体。
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// rpcResponse 插件返回的一行 JSON（也可能是不带 id 的通知）。
type rpcResponse struct {
	ID     int64           `json:"id"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// InitializeParams initialize 请求参数。
type InitializeParams struct {
	ProtocolVersion  int               `json:"protocolVersion"`
	PluginID         string            `json:"pluginId"`
	Workspace        string            `json:"workspace,omitempty"`
	DataDir          string            `json:"dataDir,omitempty"`
	Locale           string            `json:"locale,omitempty"`
	Host             map[string]string `json:"host,omitempty"`
	HostVersion      string            `json:"hostVersion,omitempty"`
	HostCapabilities []string          `json:"hostCapabilities,omitempty"`
	Config           map[string]any    `json:"config,omitempty"`
}

// InitializeResult initialize 响应结果。
type InitializeResult struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// ProtoTool 插件上报的工具声明。
type ProtoTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Safe        bool            `json:"safe,omitempty"`
}

// ToolsListResult tools/list 响应结果。
type ToolsListResult struct {
	Tools []ProtoTool `json:"tools"`
}

// ToolsCallParams tools/call 请求参数。
type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolsCallResult tools/call 响应结果。
type ToolsCallResult struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// CommandInfo 插件贡献的命令。
type CommandInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// CommandsListResult commands/list 响应结果。
type CommandsListResult struct {
	Commands []CommandInfo `json:"commands"`
}

// CommandsExecParams commands/exec 请求参数。
type CommandsExecParams struct {
	ID   string         `json:"id"`
	Args map[string]any `json:"arguments,omitempty"`
}

// ConfigResult config/get 响应结果；config/set 用同样的包体回传生效值。
type ConfigResult struct {
	Config map[string]any `json:"config"`
}

// ProgressParams progress 通知参数。
type ProgressParams struct {
	Tool    string `json:"tool,omitempty"`
	Done    int    `json:"done,omitempty"`
	Total   int    `json:"total,omitempty"`
	Message string `json:"message,omitempty"`
}

// LogParams log 通知参数。
type LogParams struct {
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
}
