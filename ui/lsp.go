// guis — LSP 客户端（Language Server Protocol 3.17 子集）
//
// 完整的 JSON-RPC over stdio 客户端：Content-Length 分帧、请求/响应配对、
// 服务端通知（publishDiagnostics / window/logMessage）、生命周期管理。
//
// 接入任意语言服务器（gopls / tsserver / pyright / clangd …）：
//
//	srv := ui.NewLSPClient("gopls", "gopls", "-mode=stdio")
//	srv.Root = "H:/proj"
//	if err := srv.Start(); err == nil {
//	    ed.AttachLSP(srv, "file:///H:/proj/main.go", "go", text)
//	}
//
// 编辑器侧只需要 Editor.AttachLSP；服务进程的启动 / 结束都封装在本文件，
// Web 端（js/wasm）没有子进程，由 lsp_exec_wasm.go 返回可读错误。
package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CodeLocation 定义 / 引用的位置（0 基行列，与编辑器一致）。
type CodeLocation struct {
	Path string
	Line int
	Col  int
}

// LSPClient 一个语言服务器会话。
type LSPClient struct {
	// Name 显示名（gopls / tsserver …）。
	Name string
	// Command 可执行文件（PATH 内或绝对路径）。
	Command string
	Args    []string
	// Root 工作区根目录（go.mod / package.json 所在目录）。
	Root string
	// Env 追加环境变量（KEY=VALUE）。
	Env []string
	// Timeout 单次请求超时（默认 6s）。
	Timeout time.Duration
	// InitOptions initialize 请求里的 initializationOptions。
	// gopls 的 semanticTokens 选项默认关闭，必须在这里（或 workspace/configuration）
	// 显式打开，否则 semanticTokens/full 永远返回空数组。
	InitOptions map[string]any
	// Settings workspace/configuration 的应答内容（对请求里的每个 section 各回一份）。
	Settings map[string]any
	// OnDiagnostics 诊断推送回调（在读取协程里调用，需要自己切回 UI 线程）。
	OnDiagnostics func(uri string, diags []CodeDiagnostic)
	// OnLog 服务端日志（window/logMessage / 进程 stderr）。
	OnLog func(string)
	// OnError 连接层错误（进程退出 / 读取失败），区别于服务端的普通日志。
	OnError func(string)

	mu      sync.Mutex
	stdin   io.WriteCloser
	nextID  int
	pending map[int]chan lspMessage
	opened  map[string]int
	diags   map[string][]CodeDiagnostic
	started  bool
	stopped  bool
	stopping bool
	stopFn   func() error
	capabs  map[string]any
}

type lspMessage struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// defaultLSPSettings 默认用户配置：打开 gopls 的语义高亮。
//
// gopls 只有在 semanticTokens 选项为 true 时才返回语义 token（默认是 false，
// 但初始化结果里照样声明 semanticTokensProvider），所以客户端必须显式下发配置，
// 否则「已连接但语义高亮一直等待服务端」。
func defaultLSPSettings() map[string]any {
	return map[string]any{"semanticTokens": true}
}

// NewLSPClient 新建客户端（尚未启动）。
func NewLSPClient(name, command string, args ...string) *LSPClient {
	return &LSPClient{
		Name:        name,
		Command:     command,
		Args:        args,
		Timeout:     6 * time.Second,
		InitOptions: defaultLSPSettings(),
		Settings:    defaultLSPSettings(),
		pending:     map[int]chan lspMessage{},
		opened:      map[string]int{},
		diags:       map[string][]CodeDiagnostic{},
	}
}

// FileURI 把本地路径转成 file:// URI（盘符、空格、中文都按 RFC 3986 转义）。
func FileURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	return u.String()
}

// URIPath 把 file:// URI 还原成本地路径
// （兼容 gopls 等把盘符冒号转成 %3A 的服务端，旧版只认 %20 / %23 会还原失败）。
func URIPath(uri string) string {
	if u, err := url.Parse(uri); err == nil && strings.EqualFold(u.Scheme, "file") {
		p := u.Path
		if runtime.GOOS == "windows" && len(p) > 2 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		return filepath.FromSlash(p)
	}
	// 兜底：手写解析（非法转义序列时 url.Parse 会失败）
	p := strings.TrimPrefix(uri, "file://")
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	if runtime.GOOS == "windows" && len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// Start 启动服务进程并完成 initialize 握手。
func (c *LSPClient) Start() error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	stdin, stdout, stop, err := startLSPProcess(c.Command, c.Args, c.Root, c.Env)
	if err != nil {
		return fmt.Errorf("启动 %s 失败：%w", c.Name, err)
	}
	c.mu.Lock()
	c.stdin, c.stopFn, c.started = stdin, stop, true
	c.mu.Unlock()
	go c.readLoop(stdout)

	params := map[string]any{
		"processId": nil,
		"clientInfo": map[string]any{
			"name":    "GUIS CodeEditor",
			"version": "1.0",
		},
		"rootUri":  FileURI(c.Root),
		"rootPath": c.Root,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"dynamicRegistration": false,
					"willSave":            false,
					"didSave":             true,
				},
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport":       false,
						"documentationFormat":  []string{"plaintext", "markdown"},
						"insertReplaceSupport": true,
					},
					"contextSupport": true,
				},
				"hover": map[string]any{"contentFormat": []string{"plaintext", "markdown"}},
				"definition":    map[string]any{"linkSupport": true},
				"formatting":    map[string]any{},
				"rename":        map[string]any{"prepareSupport": false},
				"publishDiagnostics": map[string]any{
					"relatedInformation": true,
				},
				"documentSymbol": map[string]any{},
				"signatureHelp":  map[string]any{},
				// 语义高亮：声明客户端支持的 token 类型（空列表会让服务端过滤掉全部 token）
				"semanticTokens": map[string]any{
					"requests": map[string]any{"full": true, "range": true},
					"tokenTypes": []string{
						"namespace", "type", "class", "enum", "interface", "struct",
						"typeParameter", "parameter", "variable", "property", "enumMember",
						"event", "function", "method", "macro", "keyword", "modifier",
						"comment", "string", "number", "regexp", "operator", "decorator",
					},
					"tokenModifiers": []string{
						"declaration", "definition", "readonly", "static", "deprecated",
						"abstract", "async", "modification", "documentation", "defaultLibrary",
					},
					"formats": []string{"relative"},
				},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
				"configuration":    true,
			},
		},
		"workspaceFolders": []map[string]any{{"uri": FileURI(c.Root), "name": filepath.Base(c.Root)}},
	}
	if len(c.InitOptions) > 0 {
		params["initializationOptions"] = c.InitOptions
	}
	res, err := c.request("initialize", params)
	if err != nil {
		c.Stop()
		return err
	}
	c.mu.Lock()
	c.capabs = map[string]any{}
	if m, ok := res["capabilities"].(map[string]any); ok {
		c.capabs = m
	}
	c.mu.Unlock()
	c.notify("initialized", map[string]any{})
	// 再下发一次用户配置：部分服务端（gopls）只在收到 workspace/didChangeConfiguration
	// 后才去读 workspace/configuration，initialize 里给的 options 不一定被采纳。
	if len(c.Settings) > 0 {
		c.notify("workspace/didChangeConfiguration", map[string]any{"settings": c.Settings})
	}
	return nil
}

// Started 是否已完成握手。
func (c *LSPClient) Started() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started && !c.stopped
}

// Capability 服务端能力（initialize 结果里的 capabilities 子项）。
func (c *LSPClient) Capability(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.capabs[name]
	return ok
}

// Stop 关闭会话（先 shutdown 请求，再结束进程）。
//
// 注意顺序：先把 stopped 置位会导致 shutdown 请求被 request 拒发，所以用 stopping
// 单独防重入，协议关闭做完再标记 stopped（shutdown 走 1s 短超时，不阻塞 UI 太久）。
func (c *LSPClient) Stop() {
	c.mu.Lock()
	if c.stopped || c.stopping {
		c.mu.Unlock()
		return
	}
	c.stopping = true
	hasConn := c.stdin != nil && c.stopFn != nil
	stop := c.stopFn
	c.mu.Unlock()

	if hasConn {
		_, _ = c.requestTimeout("shutdown", map[string]any{}, time.Second)
		c.notify("exit", map[string]any{})
	}

	c.mu.Lock()
	c.stopped = true
	c.stopFn = nil
	c.mu.Unlock()
	if stop != nil {
		_ = stop()
	}
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
}

// ---- 文档同步 ----

// DidOpen 打开文档（通知服务端文档内容）。
//
// version 在同一 URI 上只增不减：重新 AttachLSP 会再 didOpen 一次，如果又从 1 开始，
// 部分服务端（gopls）会认为版本回退而丢弃后续 didChange。
func (c *LSPClient) DidOpen(uri, languageID, text string) {
	c.mu.Lock()
	c.opened[uri]++
	ver := c.opened[uri]
	c.mu.Unlock()
	c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    ver,
			"text":       text,
		},
	})
}

// DidChange 全文同步（version 自增；服务端按版本应用）。
func (c *LSPClient) DidChange(uri, text string) {
	c.mu.Lock()
	c.opened[uri]++
	ver := c.opened[uri]
	c.mu.Unlock()
	c.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": ver},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

// DidSave 保存通知。
func (c *LSPClient) DidSave(uri, text string) {
	c.notify("textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"text":         text,
	})
}

// DidClose 关闭文档。
func (c *LSPClient) DidClose(uri string) {
	c.mu.Lock()
	delete(c.opened, uri)
	c.mu.Unlock()
	c.notify("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

// Diagnostics 最近一次收到的诊断。
func (c *LSPClient) Diagnostics(uri string) []CodeDiagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CodeDiagnostic(nil), c.diags[uri]...)
}

// ---- 语言特性 ----

// Completion 请求补全（同步，调用方应在后台协程里跑）。
func (c *LSPClient) Completion(uri, langID string, line, col int, text string) []CodeCompletion {
	if !c.Started() {
		return nil
	}
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     lspPosition(text, line, col),
		"context":      map[string]any{"triggerKind": 1},
	}
	res, err := c.request("textDocument/completion", params)
	if err != nil || res == nil {
		return nil
	}
	raw, _ := res["items"].([]any)
	if raw == nil {
		raw, _ = res["result"].([]any)
	}
	if raw == nil {
		if arr, ok := res[""].([]any); ok {
			raw = arr
		}
	}
	out := make([]CodeCompletion, 0, len(raw))
	for _, it := range raw {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		label, _ := m["label"].(string)
		detail, _ := m["detail"].(string)
		ins := label
		if tf, ok := m["textEdit"].(map[string]any); ok {
			if nt, ok := tf["newText"].(string); ok && nt != "" {
				ins = nt
			}
		} else if it, ok := m["insertText"].(string); ok && it != "" {
			ins = it
		}
		// 过滤掉片段占位符（${1:name}）
		ins = stripSnippet(ins)
		kind := 0
		if k, ok := m["kind"].(float64); ok {
			kind = int(k)
		}
		out = append(out, CodeCompletion{Label: label, Detail: detail, Insert: ins, Kind: kind})
	}
	sortCompletion(out)
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

// Hover 悬停信息（返回去标记的纯文本多行）。
func (c *LSPClient) Hover(uri, langID string, line, col int, text string) string {
	return markdownToText(c.HoverMarkdown(uri, langID, line, col, text))
}

// HoverMarkdown 悬停信息（返回服务端原始 Markdown，编辑器自行排版高亮）。
func (c *LSPClient) HoverMarkdown(uri, langID string, line, col int, text string) string {
	if !c.Started() {
		return ""
	}
	res, err := c.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     lspPosition(text, line, col),
	})
	if err != nil || res == nil {
		return ""
	}
	contents, ok := res["contents"]
	if !ok {
		return ""
	}
	return lspText(contents)
}

// Definition 转到定义（可能有多个位置）。
func (c *LSPClient) Definition(uri, langID string, line, col int, text string) []CodeLocation {
	if !c.Started() {
		return nil
	}
	res, err := c.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     lspPosition(text, line, col),
	})
	if err != nil || res == nil {
		return nil
	}
	var out []CodeLocation
	add := func(m map[string]any) {
		u, _ := m["uri"].(string)
		if u == "" {
			if t, ok := m["targetUri"].(string); ok {
				u = t
			}
		}
		rng, _ := m["range"].(map[string]any)
		if rng == nil {
			rng, _ = m["targetSelectionRange"].(map[string]any)
		}
		line, col := 0, 0
		if rng != nil {
			if sp, ok := rng["start"].(map[string]any); ok {
				if v, ok := sp["line"].(float64); ok {
					line = int(v)
				}
				if v, ok := sp["character"].(float64); ok {
					col = int(v)
				}
			}
		}
		if u != "" {
			out = append(out, CodeLocation{Path: URIPath(u), Line: line, Col: col})
		}
	}
	switch v := res[""].(type) {
	case []any:
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
				add(m)
			}
		}
	case map[string]any:
		add(v)
	}
	// 结果直接是数组时（服务端返回裸数组）
	if len(out) == 0 {
		if arr, ok := res["__array__"].([]any); ok {
			for _, it := range arr {
				if m, ok := it.(map[string]any); ok {
					add(m)
				}
			}
		}
	}
	return out
}

// Format 请求格式化，返回新全文（ok=false 表示服务端不支持或失败）。
func (c *LSPClient) Format(uri, text string, tabSize int) (string, bool) {
	if !c.Started() {
		return "", false
	}
	if tabSize <= 0 {
		tabSize = 4
	}
	res, err := c.request("textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"options":      map[string]any{"tabSize": tabSize, "insertSpaces": true},
	})
	if err != nil || res == nil {
		return "", false
	}
	arr, ok := res[""].([]any)
	if !ok || len(arr) == 0 {
		return "", false
	}
	return applyTextEdits(text, arr), true
}

// Rename 重命名符号，返回「文件 → 新全文」的修改集合。
func (c *LSPClient) Rename(uri, langID string, line, col int, text, newName string) (map[string]string, bool) {
	if !c.Started() || newName == "" {
		return nil, false
	}
	res, err := c.request("textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     lspPosition(text, line, col),
		"newName":      newName,
	})
	if err != nil || res == nil {
		return nil, false
	}
	changes, ok := res["changes"].(map[string]any)
	if !ok {
		if doc, ok := res["documentChanges"].([]any); ok {
			out := map[string]string{}
			for _, d := range doc {
				m, ok := d.(map[string]any)
				if !ok {
					continue
				}
				td, ok := m["textDocument"].(map[string]any)
				if !ok {
					continue
				}
				u, _ := td["uri"].(string)
				edits, _ := m["edits"].([]any)
				if u != "" && len(edits) > 0 {
					p := URIPath(u)
					if sameFilePath(p, uri) {
						out[p] = applyTextEdits(text, edits)
					} else if raw, err := os.ReadFile(p); err == nil {
						// 其它文件：读盘后套用（编辑器侧只拿到当前文档内容）
						out[p] = applyTextEdits(string(raw), edits)
					} else {
						out[p] = ""
					}
				}
			}
			return out, len(out) > 0
		}
		return nil, false
	}
	out := map[string]string{}
	for u, v := range changes {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		out[URIPath(u)] = applyTextEdits(text, arr)
	}
	return out, len(out) > 0
}

// sameFilePath 判断两个路径是否指向同一文件（Windows 大小写 / 分隔符不敏感）。
func sameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// ---- 底层协议 ----

// request 发送请求并等待响应（超时按 Timeout）。
func (c *LSPClient) request(method string, params any) (map[string]any, error) {
	return c.requestTimeout(method, params, c.Timeout)
}

// requestTimeout 同 request，但可指定本次超时（关闭流程 / 语义 token 轮询用）。
func (c *LSPClient) requestTimeout(method string, params any, timeout time.Duration) (map[string]any, error) {
	c.mu.Lock()
	if !c.started || c.stopped || c.stdin == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("%s 未启动", c.Name)
	}
	c.nextID++
	id := c.nextID
	ch := make(chan lspMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	if err := c.write(body); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	select {
	case msg, ok := <-ch:
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("%s 已关闭", c.Name)
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("%s 返回错误：%s", method, msg.Error.Message)
		}
		return decodeResult(msg.Result), nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("%s 超时（%s）", method, timeout)
	}
}

// decodeResult 把 result 解成 map；数组结果放进 "" 键，便于统一取用。
func decodeResult(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil && obj != nil {
		return obj
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return map[string]any{"": arr}
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return map[string]any{"": str}
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return map[string]any{"": num}
	}
	return map[string]any{}
}

func (c *LSPClient) notify(method string, params any) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return
	}
	_ = c.write(body)
}

// write 按 LSP 分帧写出（Content-Length + \r\n\r\n + JSON）。
func (c *LSPClient) write(body []byte) error {
	c.mu.Lock()
	w := c.stdin
	c.mu.Unlock()
	if w == nil {
		return fmt.Errorf("%s 未启动", c.Name)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// readLoop 读取服务端消息：响应交给等待者，通知分发给对应处理器。
func (c *LSPClient) readLoop(r io.Reader) {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		length := 0
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				c.failAll(err)
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				v := strings.TrimSpace(line[len("content-length:"):])
				if n, err := strconv.Atoi(v); err == nil {
					length = n
				}
			}
		}
		if length <= 0 || length > 64<<20 {
			continue
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(br, buf); err != nil {
			c.failAll(err)
			return
		}
		var msg lspMessage
		if err := json.Unmarshal(buf, &msg); err != nil {
			continue
		}
		c.dispatch(msg)
	}
}

func (c *LSPClient) failAll(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = false
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	if err != nil && err != io.EOF {
		msg := fmt.Sprintf("%s 读取结束：%v", c.Name, err)
		if c.OnError != nil {
			c.OnError(msg)
		} else if c.OnLog != nil {
			c.OnLog(msg)
		}
	}
}

func (c *LSPClient) dispatch(msg lspMessage) {
	// 响应
	if msg.ID != 0 && msg.Method == "" {
		c.mu.Lock()
		ch := c.pending[msg.ID]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
		return
	}
	// 请求（服务端 → 客户端）：目前只应答 configuration / 注册能力，保证握手不卡
	if msg.ID != 0 && msg.Method != "" {
		switch msg.Method {
		case "workspace/configuration":
			// 必须按 items 数量逐项应答（少一项 gopls 会当成配置缺失，选项不生效）
			c.reply(msg.ID, c.configForItems(msg.Params))
		case "workspace/workspaceFolders":
			c.reply(msg.ID, []map[string]any{{"uri": FileURI(c.Root), "name": filepath.Base(c.Root)}})
		case "client/registerCapability", "window/workDoneProgress/create":
			c.reply(msg.ID, nil)
		default:
			c.reply(msg.ID, nil)
		}
		return
	}
	switch msg.Method {
	case "textDocument/publishDiagnostics":
		c.onPublishDiagnostics(msg.Params)
	case "window/logMessage", "window/showMessage":
		var p struct {
			Type    int    `json:"type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(msg.Params, &p); err == nil && c.OnLog != nil && p.Message != "" {
			c.OnLog("[" + c.Name + "] " + p.Message)
		}
	}
}

// configForItems 生成 workspace/configuration 的应答：与请求 items 等长，每项都是同一份配置。
func (c *LSPClient) configForItems(raw json.RawMessage) []map[string]any {
	n := 1
	var p struct {
		Items []struct {
			Section string `json:"section"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &p); err == nil && len(p.Items) > 0 {
		n = len(p.Items)
	}
	settings := c.Settings
	if settings == nil {
		settings = defaultLSPSettings()
	}
	out := make([]map[string]any, n)
	for i := range out {
		out[i] = settings
	}
	return out
}

func (c *LSPClient) reply(id int, result any) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	if err == nil {
		_ = c.write(body)
	}
}

func (c *LSPClient) onPublishDiagnostics(raw json.RawMessage) {
	var p struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct{ Line, Character int } `json:"start"`
				End   struct{ Line, Character int } `json:"end"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			Source   string `json:"source"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	out := make([]CodeDiagnostic, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		sev := d.Severity
		if sev == 0 {
			sev = 3
		}
		out = append(out, CodeDiagnostic{
			Line: d.Range.Start.Line, Col: d.Range.Start.Character,
			EndLine: d.Range.End.Line, EndCol: d.Range.End.Character,
			Severity: sev, Message: d.Message, Source: d.Source,
		})
	}
	c.mu.Lock()
	c.diags[p.URI] = out
	c.mu.Unlock()
	if c.OnDiagnostics != nil {
		c.OnDiagnostics(p.URI, out)
	}
}

// ---- 位置换算（LSP 用 UTF-16 code unit 计数，编辑器用 rune）----

// lspPosition rune 列 → LSP position（行 + UTF-16 列）。
func lspPosition(text string, line, col int) map[string]any {
	lines := strings.Split(text, "\n")
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	if line < 0 {
		line = 0
	}
	s := lines[line]
	if col < 0 {
		col = 0
	}
	runes := []rune(s)
	if col > len(runes) {
		col = len(runes)
	}
	u16 := 0
	for _, r := range runes[:col] {
		if r > 0xFFFF {
			u16 += 2
		} else {
			u16++
		}
	}
	return map[string]any{"line": line, "character": u16}
}

// lspText 把 hover contents 统一转成字符串。
func lspText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["value"].(string); ok {
			return s
		}
	case []any:
		var sb strings.Builder
		for _, it := range t {
			if s := lspText(it); s != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(s)
			}
		}
		return sb.String()
	}
	return ""
}

// stripSnippet 去掉 LSP snippet 占位符，返回可落盘文本。
func stripSnippet(s string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '$' && i+1 < len(s) {
			if s[i+1] == '{' {
				end := strings.IndexByte(s[i:], '}')
				if end > 0 {
					inner := s[i+2 : i+end]
					if idx := strings.IndexByte(inner, ':'); idx >= 0 {
						sb.WriteString(inner[idx+1:])
					}
					i += end
					continue
				}
			}
			// $1 / $0
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j > i+1 {
				i = j - 1
				continue
			}
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// markdownToText 粗略把 markdown 悬停内容转成行文本（编辑器自绘不解析 MD）。
func markdownToText(md string) string {
	repl := strings.NewReplacer(
		"```go", "", "```", "",
		"**", "", "`", "", "\r\n", "\n",
	)
	s := repl.Replace(md)
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, ln := range lines {
		out = append(out, strings.TrimRight(ln, " "))
	}
	return strings.Join(out, "\n")
}

// applyTextEdits 按 LSP TextEdit 数组套用到全文（逆序应用避免偏移失效）。
func applyTextEdits(text string, edits []any) string {
	lines := strings.Split(text, "\n")
	type off struct {
		start, end int
		newText    string
	}
	var list []off
	for _, it := range edits {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		nt, _ := m["newText"].(string)
		rng, _ := m["range"].(map[string]any)
		if rng == nil {
			continue
		}
		s0, _ := posInText(lines, rng["start"])
		s1, _ := posInText(lines, rng["end"])
		if s0 < 0 || s1 < 0 {
			continue
		}
		if s1 < s0 {
			s0, s1 = s1, s0
		}
		list = append(list, off{start: s0, end: s1, newText: nt})
	}
	if len(list) == 0 {
		return text
	}
	// 从后往前套用
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].start > list[i].start {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	full := text
	for _, e := range list {
		if e.start < 0 || e.end > len(full) || e.start > e.end {
			continue
		}
		full = full[:e.start] + e.newText + full[e.end:]
	}
	return full
}

// posInText LSP position → 字节偏移。
func posInText(lines []string, v any) (int, int) {
	m, ok := v.(map[string]any)
	if !ok {
		return -1, -1
	}
	line := 0
	ch := 0
	if f, ok := m["line"].(float64); ok {
		line = int(f)
	}
	if f, ok := m["character"].(float64); ok {
		ch = int(f)
	}
	if line < 0 || line >= len(lines) {
		return -1, -1
	}
	// 字节偏移 = 前面所有行 + 换行
	base := 0
	for i := 0; i < line; i++ {
		base += len(lines[i]) + 1
	}
	s := lines[line]
	// UTF-16 列 → 字节偏移
	u16 := 0
	idx := 0
	for _, r := range s {
		if u16 >= ch {
			break
		}
		if r > 0xFFFF {
			u16 += 2
		} else {
			u16++
		}
		idx += len(string(r))
	}
	return base + idx, base + idx
}
