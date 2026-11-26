package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// restartWindow 崩溃重启预算窗口。
	restartWindow = 5 * time.Minute
	// maxRestarts 一个窗口内最多重启次数。
	maxRestarts = 3
	// stderrLines 保留的插件输出行数（stderr + log 通知）。
	stderrLines = 200
	// progressKeep 每个插件保留的进度事件数。
	progressKeep = 50
	// pingFailLimit 连续 ping 失败多少次判定为不健康。
	pingFailLimit = 2
	// protocolEnv 传给插件的协议标记。
	protocolEnv = "GUIS_PLUGIN_PROTOCOL"
)

// pluginProc 一个进程插件子进程的运行状态。
type pluginProc struct {
	id       string
	cmd      *exec.Cmd
	conn     *rpcConn
	stdin    io.WriteCloser
	group    *procGroup
	tools    []ToolDescriptor
	commands []CommandInfo
	caps     []string
	ring     *lineRing
	progress *eventRing
	started  time.Time

	mu        sync.Mutex
	lastErr   string
	pingFails int
	healthy   bool
}

func (p *pluginProc) alive() bool {
	return p.conn != nil && p.conn.Err() == nil
}

func (p *pluginProc) hasCap(cap string) bool {
	for _, c := range p.caps {
		if strings.EqualFold(c, cap) {
			return true
		}
	}
	return false
}

// restartBudget 崩溃重启预算。
type restartBudget struct {
	restarts    int
	windowStart time.Time
}

// Host 进程外插件的运行时宿主：启动、握手、调用、健康检查、崩溃隔离与清理。
type Host struct {
	// OnError 运行期错误回调（pluginID, message）。
	OnError func(id, msg string)
	// OnUnhealthy 健康检查判定不可用且无法恢复时回调（宿主可据此停用插件）。
	OnUnhealthy func(id, msg string)

	logf func(format string, args ...any)

	mu     sync.Mutex
	procs  map[string]*pluginProc
	budget map[string]*restartBudget
	starts sync.Map // id → *sync.Mutex，避免并发 Ensure 重复拉起进程
	closed bool

	// Workspace / DataDir 握手时告知插件（宿主侧可随时更新）。
	Workspace string
	DataDir   string
}

// NewHost 创建宿主。
func NewHost(logf func(format string, args ...any)) *Host {
	return &Host{procs: map[string]*pluginProc{}, budget: map[string]*restartBudget{}, logf: logf}
}

func (h *Host) errorf(id string, err error) {
	if err == nil {
		return
	}
	if h.OnError != nil {
		h.OnError(id, err.Error())
	}
	if h.logf != nil {
		h.logf("插件 %s: %v", id, err)
	}
}

// Running 插件子进程是否存活。
func (h *Host) Running(id string) bool {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	return p != nil && p.alive()
}

// Healthy 上一次健康检查结果（未知视为健康）。
func (h *Host) Healthy(id string) bool {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	if p == nil {
		return true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.healthy
}

// LastError 最近一次运行期错误。
func (h *Host) LastError(id string) string {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

// CachedTools 工具表：活进程优先，其次磁盘缓存（不触发启动）。
func (h *Host) CachedTools(mf *Manifest) []ToolDescriptor {
	h.mu.Lock()
	p := h.procs[mf.ID]
	h.mu.Unlock()
	if p != nil && p.alive() {
		out := make([]ToolDescriptor, len(p.tools))
		copy(out, p.tools)
		if len(out) > 0 {
			return out
		}
	}
	cached := loadToolsCache(mf.Dir, mf.Version, mf.Runtime.ProtocolVersion)
	for i := range cached {
		cached[i].PluginID = mf.ID
	}
	return cached
}

// Logs 最近的 stderr / log 通知输出。
func (h *Host) Logs(id string) []string {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.ring.Lines()
}

// Progress 最近的进度事件。
func (h *Host) Progress(id string) []ProgressEvent {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.progress.Events()
}

// Ensure 确保插件进程已启动并完成握手（幂等）。
func (h *Host) Ensure(ctx context.Context, mf *Manifest) (*pluginProc, error) {
	h.mu.Lock()
	p := h.procs[mf.ID]
	h.mu.Unlock()
	if p != nil && p.alive() {
		return p, nil
	}
	// 同一插件的启动串行化
	muIface, _ := h.starts.LoadOrStore(mf.ID, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	h.mu.Lock()
	p = h.procs[mf.ID]
	h.mu.Unlock()
	if p != nil && p.alive() {
		return p, nil
	}
	return h.start(ctx, mf)
}

// Tools 取插件工具列表（必要时启动）。
func (h *Host) Tools(ctx context.Context, mf *Manifest) ([]ToolDescriptor, error) {
	p, err := h.Ensure(ctx, mf)
	if err != nil {
		return nil, err
	}
	if len(p.tools) == 0 {
		if err := h.fetchTools(ctx, p, mf); err != nil {
			return nil, err
		}
	}
	out := make([]ToolDescriptor, len(p.tools))
	copy(out, p.tools)
	return out, nil
}

// Commands 取插件贡献的命令（需要 capabilities 含 commands）。
func (h *Host) Commands(ctx context.Context, mf *Manifest) ([]CommandInfo, error) {
	p, err := h.Ensure(ctx, mf)
	if err != nil {
		return nil, err
	}
	if !p.hasCap(CapCommands) {
		return nil, nil
	}
	if len(p.commands) > 0 {
		out := make([]CommandInfo, len(p.commands))
		copy(out, p.commands)
		return out, nil
	}
	cctx, cancel := context.WithTimeout(ctx, mf.Runtime.CallTimeout())
	defer cancel()
	raw, err := p.conn.call(cctx, MethodCommandsList, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("取插件命令列表失败: %w", err)
	}
	var list CommandsListResult
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("插件命令列表格式错误: %w", err)
	}
	seen := map[string]bool{}
	var out []CommandInfo
	for _, c := range list.Commands {
		c.ID = strings.TrimSpace(c.ID)
		if c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Title) == "" {
			c.Title = c.ID
		}
		out = append(out, c)
	}
	p.commands = out
	cp := make([]CommandInfo, len(out))
	copy(cp, out)
	return cp, nil
}

// RunCommand 执行插件命令。
func (h *Host) RunCommand(ctx context.Context, mf *Manifest, commandID string, args map[string]any) (CallResult, error) {
	p, err := h.Ensure(ctx, mf)
	if err != nil {
		return CallResult{}, err
	}
	if !p.hasCap(CapCommands) {
		return CallResult{}, fmt.Errorf("插件 %s 未声明 commands 能力", mf.ID)
	}
	cctx, cancel := context.WithTimeout(ctx, mf.Runtime.CallTimeout())
	defer cancel()
	raw, err := p.conn.call(cctx, MethodCommandsExec, CommandsExecParams{ID: commandID, Args: args})
	if err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			h.kill(p)
			return CallResult{}, fmt.Errorf("插件命令 %s 执行超时（%s）", commandID, mf.Runtime.CallTimeout())
		}
		return CallResult{}, fmt.Errorf("执行插件命令 %s 失败: %w", commandID, err)
	}
	var res ToolsCallResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return CallResult{}, fmt.Errorf("插件命令 %s 返回格式错误: %w", commandID, err)
	}
	return CallResult{Text: clampText(res.Content), IsError: res.IsError}, nil
}

// Ping 健康检查。
func (h *Host) Ping(ctx context.Context, mf *Manifest) error {
	h.mu.Lock()
	p := h.procs[mf.ID]
	h.mu.Unlock()
	if p == nil || !p.alive() {
		return nil // 没在跑就不算不健康
	}
	cctx, cancel := context.WithTimeout(ctx, mf.Runtime.PingTimeout())
	defer cancel()
	_, err := p.conn.call(cctx, MethodPing, map[string]any{})
	return err
}

// PingAll 对一组插件做健康检查：连续失败达到阈值则重启，超预算则回调 OnUnhealthy。
func (h *Host) PingAll(ctx context.Context, mfs []*Manifest) {
	for _, mf := range mfs {
		err := h.Ping(ctx, mf)
		h.mu.Lock()
		p := h.procs[mf.ID]
		h.mu.Unlock()
		if p == nil {
			continue
		}
		p.mu.Lock()
		if err == nil {
			p.pingFails = 0
			p.healthy = true
			p.mu.Unlock()
			continue
		}
		p.pingFails++
		fails := p.pingFails
		p.healthy = false
		p.lastErr = fmt.Sprintf("健康检查失败（连续 %d 次）：%v", fails, err)
		p.mu.Unlock()
		if fails < pingFailLimit {
			continue
		}
		if !mf.Runtime.AutoRestartEnabled() || !h.consumeRestartBudget(mf.ID) {
			msg := fmt.Sprintf("健康检查连续失败 %d 次且无法重启，已停止插件：%v", fails, err)
			h.kill(p)
			h.mu.Lock()
			delete(h.procs, mf.ID)
			h.mu.Unlock()
			if h.OnUnhealthy != nil {
				h.OnUnhealthy(mf.ID, msg)
			}
			h.errorf(mf.ID, errors.New(msg))
			continue
		}
		h.kill(p)
		h.mu.Lock()
		delete(h.procs, mf.ID)
		h.mu.Unlock()
		if _, err := h.start(ctx, mf); err != nil {
			h.errorf(mf.ID, fmt.Errorf("健康检查后重启失败: %w", err))
		}
	}
}

// SetWorkspace 把新工作区通知给所有存活插件。
func (h *Host) SetWorkspace(workspace string) {
	h.mu.Lock()
	h.Workspace = workspace
	procs := make([]*pluginProc, 0, len(h.procs))
	for _, p := range h.procs {
		procs = append(procs, p)
	}
	h.mu.Unlock()
	for _, p := range procs {
		if p.alive() && p.hasCap(CapConfig) {
			_ = p.conn.notify(NotifyWorkspaceChange, map[string]any{"workspace": workspace})
		}
	}
}

// NotifyConfig 把配置变更推给插件（v2 且声明 config 能力时）。
func (h *Host) NotifyConfig(id string, cfg map[string]any) {
	h.mu.Lock()
	p := h.procs[id]
	h.mu.Unlock()
	if p == nil || !p.alive() || !p.hasCap(CapConfig) {
		return
	}
	_ = p.conn.notify(NotifyConfigChange, map[string]any{"config": cfg})
}

// Call 调用插件工具；连接已断开且预算允许时重启后重试一次。
func (h *Host) Call(ctx context.Context, mf *Manifest, env Env, tool string, args map[string]any) (CallResult, error) {
	p, err := h.Ensure(ctx, mf)
	if err != nil {
		return CallResult{}, err
	}
		res, callErr := h.callOnce(ctx, p, mf, env, tool, args)
	if callErr == nil {
		return res, nil
	}
	if p.alive() {
		return CallResult{}, callErr
	}
	if !mf.Runtime.AutoRestartEnabled() {
		h.errorf(mf.ID, callErr)
		return CallResult{}, fmt.Errorf("插件进程已退出: %v", callErr)
	}
	if !h.consumeRestartBudget(mf.ID) {
		h.errorf(mf.ID, fmt.Errorf("插件反复崩溃，已停止重启: %v", callErr))
		return CallResult{}, fmt.Errorf("插件 %s 重复崩溃且已超过重启上限（%d 次 / %s）", mf.ID, maxRestarts, restartWindow)
	}
	p2, err2 := h.start(ctx, mf)
	if err2 != nil {
		return CallResult{}, err2
	}
	return h.callOnce(ctx, p2, mf, env, tool, args)
}

func (h *Host) callOnce(ctx context.Context, p *pluginProc, mf *Manifest, env Env, tool string, args map[string]any) (CallResult, error) {
	timeout := mf.Runtime.CallTimeout()
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, err := p.conn.call(cctx, MethodToolsCall, ToolsCallParams{Name: tool, Arguments: args})
	if err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			h.kill(p)
			return CallResult{}, fmt.Errorf("插件工具 %s 调用超时（%s）", tool, timeout)
		}
		return CallResult{}, fmt.Errorf("调用插件工具 %s 失败: %w", tool, err)
	}
	var res ToolsCallResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return CallResult{}, fmt.Errorf("插件工具 %s 返回格式错误: %w", tool, err)
	}
	return CallResult{Text: clampText(res.Content), IsError: res.IsError}, nil
}

func clampText(s string) string {
	if len(s) > MaxResultBytes {
		return s[:MaxResultBytes] + "\n…（结果过长，已截断）"
	}
	return s
}

func (h *Host) consumeRestartBudget(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	b := h.budget[id]
	if b == nil {
		b = &restartBudget{}
		h.budget[id] = b
	}
	now := time.Now()
	if b.windowStart.IsZero() || now.Sub(b.windowStart) > restartWindow {
		b.windowStart = now
		b.restarts = 0
	}
	if b.restarts >= maxRestarts {
		return false
	}
	b.restarts++
	return true
}

func (h *Host) fetchTools(ctx context.Context, p *pluginProc, mf *Manifest) error {
	cctx, cancel := context.WithTimeout(ctx, mf.Runtime.StartTimeout())
	defer cancel()
	raw, err := p.conn.call(cctx, MethodToolsList, map[string]any{})
	if err != nil {
		return fmt.Errorf("取插件工具列表失败: %w", err)
	}
	var list ToolsListResult
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("插件工具列表格式错误: %w", err)
	}
	declared := map[string]bool{}
	for _, t := range mf.Tools {
		declared[strings.ToLower(t.Name)] = true
	}
	seen := map[string]bool{}
	var out []ToolDescriptor
	for _, t := range list.Tools {
		name := strings.TrimSpace(t.Name)
		if !validToolName(name) {
			continue
		}
		if declared[strings.ToLower(name)] || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		params := t.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, ToolDescriptor{
			PluginID:    mf.ID,
			Name:        name,
			Description: t.Description,
			Parameters:  params,
			Safe:        t.Safe,
			Kind:        RuntimeProcess,
		})
	}
	p.tools = out
	_ = saveToolsCache(mf.Dir, mf.Version, mf.Runtime.ProtocolVersion, out)
	return nil
}

func (h *Host) start(ctx context.Context, mf *Manifest) (*pluginProc, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, ErrHostClosed
	}
	h.mu.Unlock()
	abs, err := resolvePluginPath(mf.Dir, mf.Runtime.Command)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(abs, mf.Runtime.Args...)
	cmd.Dir = mf.Dir
	cmd.Env = pluginEnv(mf)
	prepareProc(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("连接插件 stdout 失败: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("连接插件 stdin 失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("连接插件 stderr 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动插件进程失败: %w", err)
	}
	group := attachProc(cmd)
	if mf.Runtime.MemoryLimitMB > 0 && group != nil {
		if err := group.SetMemoryLimit(int64(mf.Runtime.MemoryLimitMB) << 20); err != nil && h.logf != nil {
			h.logf("插件 %s 设置内存上限失败: %v", mf.ID, err)
		}
	}

	p := &pluginProc{
		id:       mf.ID,
		cmd:      cmd,
		stdin:    stdin,
		group:    group,
		ring:     newLineRing(stderrLines),
		progress: newEventRing(progressKeep),
		started:  time.Now(),
		healthy:  true,
	}
	p.conn = newRPCConn(stdout, stdin, func(method string, params json.RawMessage) {
		switch method {
		case NotifyProgress:
			var pr ProgressParams
			if json.Unmarshal(params, &pr) == nil {
				p.progress.Add(ProgressEvent{
					PluginID: mf.ID,
					Tool:     pr.Tool,
					Done:     pr.Done,
					Total:    pr.Total,
					Message:  pr.Message,
					At:       time.Now(),
				})
			}
		case NotifyLog:
			var lg LogParams
			if json.Unmarshal(params, &lg) == nil {
				level := strings.ToUpper(strings.TrimSpace(lg.Level))
				if level == "" {
					level = "INFO"
				}
				p.ring.Add(fmt.Sprintf("[%s] %s", level, lg.Message))
			}
		}
	})
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 8<<10), maxLineBytes)
		for sc.Scan() {
			p.ring.Add(sc.Text())
		}
	}()

	startTimeout := mf.Runtime.StartTimeout()
	sctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	h.mu.Lock()
	workspace, dataDir := h.Workspace, h.DataDir
	h.mu.Unlock()
	raw, err := p.conn.call(sctx, MethodInitialize, InitializeParams{
		ProtocolVersion:  mf.Runtime.ProtocolVersion,
		PluginID:         mf.ID,
		Workspace:        workspace,
		DataDir:          dataDir,
		Host:             map[string]string{"name": "guis-plugins", "version": HostVersion},
		HostVersion:      HostVersion,
		HostCapabilities: HostCapabilitiesV2,
	})
	if err != nil {
		h.kill(p)
		return nil, fmt.Errorf("插件握手失败: %w", err)
	}
	var info InitializeResult
	_ = json.Unmarshal(raw, &info)
	if len(info.Capabilities) == 0 {
		p.caps = []string{CapTools}
	} else {
		p.caps = info.Capabilities
	}
	if err := h.fetchTools(sctx, p, mf); err != nil {
		h.kill(p)
		return nil, err
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.kill(p)
		return nil, ErrHostClosed
	}
	if old := h.procs[mf.ID]; old != nil && old != p {
		go h.kill(old)
	}
	h.procs[mf.ID] = p
	h.mu.Unlock()
	return p, nil
}

func pluginEnv(mf *Manifest) []string {
	allow := map[string]bool{
		"PATH": true, "HOME": true, "USERPROFILE": true, "TMP": true, "TEMP": true,
		"TMPDIR": true, "SYSTEMROOT": true, "SYSTEMDRIVE": true, "WINDIR": true,
		"USERNAME": true, "USER": true, "LANG": true, "LC_ALL": true,
	}
	for _, name := range mf.Permissions.Env {
		n := strings.ToUpper(strings.TrimSpace(name))
		if n != "" {
			allow[n] = true
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		if allow[strings.ToUpper(kv[:i])] {
			env = append(env, kv)
		}
	}
	env = append(env, protocolEnv+"="+ProtocolJSONRPCStdio)
	for k, v := range mf.Runtime.Env {
		env = append(env, k+"="+expandTemplate(v, nil, "", false))
	}
	return env
}

func (h *Host) kill(p *pluginProc) {
	if p == nil {
		return
	}
	if p.conn != nil {
		p.conn.Close()
	}
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.group != nil {
		_ = p.group.Kill()
		p.group.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
}

// Stop 停止某个插件的子进程。
func (h *Host) Stop(id string) {
	h.mu.Lock()
	p := h.procs[id]
	delete(h.procs, id)
	h.mu.Unlock()
	h.kill(p)
}

// Close 停止所有插件子进程（先尝试 graceful shutdown 通知）。
func (h *Host) Close() {
	h.mu.Lock()
	h.closed = true
	all := make([]*pluginProc, 0, len(h.procs))
	for id, p := range h.procs {
		all = append(all, p)
		delete(h.procs, id)
	}
	h.mu.Unlock()
	for _, p := range all {
		if p.alive() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, _ = p.conn.call(ctx, MethodShutdown, map[string]any{})
			cancel()
		}
		h.kill(p)
	}
}

// lineRing 环形保存最近 N 行文本。
type lineRing struct {
	mu    sync.Mutex
	max   int
	lines []string
}

func newLineRing(max int) *lineRing { return &lineRing{max: max} }

func (r *lineRing) Add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.max <= 0 {
		return
	}
	if len(r.lines) >= r.max {
		r.lines = append(r.lines[1:], s)
		return
	}
	r.lines = append(r.lines, s)
}

func (r *lineRing) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// eventRing 环形保存最近 N 条进度事件。
type eventRing struct {
	mu     sync.Mutex
	max    int
	events []ProgressEvent
}

func newEventRing(max int) *eventRing { return &eventRing{max: max} }

func (r *eventRing) Add(e ProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.max <= 0 {
		return
	}
	if len(r.events) >= r.max {
		r.events = append(r.events[1:], e)
		return
	}
	r.events = append(r.events, e)
}

func (r *eventRing) Events() []ProgressEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ProgressEvent, len(r.events))
	copy(out, r.events)
	return out
}

// ErrUnsupported 当前平台不支持进程外插件。
var ErrUnsupported = errors.New("当前平台不支持进程外插件")

// ErrHostClosed 宿主已关闭，不再接受新进程。
var ErrHostClosed = errors.New("插件宿主已关闭")

// pluginExecutable 返回插件可执行文件绝对路径（校验不越界）。
func pluginExecutable(mf *Manifest) (string, error) {
	if strings.TrimSpace(mf.Runtime.Command) == "" {
		return "", errors.New("进程插件缺少 runtime.command")
	}
	abs := filepath.Join(mf.Dir, filepath.FromSlash(mf.Runtime.Command))
	if !insideDir(mf.Dir, abs) {
		return "", fmt.Errorf("runtime.command 越出插件目录：%s", mf.Runtime.Command)
	}
	return abs, nil
}
