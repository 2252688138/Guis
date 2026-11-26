package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options 构造插件管理器。
type Options struct {
	// DataDir 数据目录（插件装在 <DataDir>/plugins，状态在 <DataDir>/plugins.json）。
	DataDir string
	// Runner 声明式 command 工具的执行器；nil 用内核自带实现。
	Runner CommandRunner
	// Logf 日志输出；nil 丢弃。
	Logf func(format string, args ...any)
	// AuditPath 审计文件路径；空则用 <DataDir>/plugins_audit.jsonl。
	AuditPath string
	// DisableAudit 关闭审计（测试用）。
	DisableAudit bool
	// MaxConcurrent 单插件并发上限，0 用默认 4。
	MaxConcurrent int
	// MaxPerMinute 单插件每分钟调用上限，0 用默认 120。
	MaxPerMinute int
}

// InstallOptions 安装选项。
type InstallOptions struct {
	// ExpectedSHA256 期望的安装包摘要（仅 zip / 单文件安装源支持）。
	ExpectedSHA256 string
}

type pluginState struct {
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
	InstalledAt int64  `json:"installedAt"`
	EnabledAt   int64  `json:"enabledAt,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	Verified    bool   `json:"verified,omitempty"`
}

type stateFile struct {
	Plugins []pluginState `json:"plugins"`
}

// Manager 插件管理器：清单、状态、声明式执行、进程插件宿主、审计与限流。
type Manager struct {
	DataDir string
	Runner  CommandRunner
	Logf    func(format string, args ...any)

	mu       sync.RWMutex
	states   map[string]pluginState
	order    []string
	manifest map[string]*Manifest
	errs     map[string]string

	hostMu sync.Mutex
	host   *Host

	limiter *limiter
	audit   *auditor

	healthOnce   sync.Once
	healthCtx    context.Context
	healthCancel context.CancelFunc
	lastHealth   map[string]time.Time
	closed       bool
}

// New 读取状态与清单（不启动任何进程）。
func New(opts Options) *Manager {
	m := &Manager{
		DataDir:    opts.DataDir,
		Runner:     opts.Runner,
		Logf:       opts.Logf,
		states:     map[string]pluginState{},
		manifest:   map[string]*Manifest{},
		errs:       map[string]string{},
		limiter:    newLimiter(opts.MaxConcurrent, opts.MaxPerMinute),
		lastHealth: map[string]time.Time{},
	}
	if !opts.DisableAudit {
		if opts.AuditPath != "" {
			m.audit = &auditor{path: opts.AuditPath}
		} else {
			m.audit = newAuditor(opts.DataDir)
		}
	}
	m.load()
	return m
}

// PluginsDir 插件安装根目录。
func (m *Manager) PluginsDir() string { return filepath.Join(m.DataDir, "plugins") }

// StatePath 状态文件路径。
func (m *Manager) StatePath() string { return filepath.Join(m.DataDir, "plugins.json") }

// AuditPath 审计文件路径（关闭审计时为空）。
func (m *Manager) AuditPath() string { return m.audit.Path() }

func (m *Manager) logf(format string, args ...any) {
	if m.Logf == nil {
		return
	}
	m.Logf(format, args...)
}

func (m *Manager) load() {
	raw, err := os.ReadFile(m.StatePath())
	if err == nil {
		var sf stateFile
		if json.Unmarshal(raw, &sf) == nil {
			for _, st := range sf.Plugins {
				if st.ID == "" {
					continue
				}
				m.states[st.ID] = st
				m.order = append(m.order, st.ID)
			}
		}
	}
	// 即使状态文件丢失，也扫描目录恢复可用插件
	entries, err := os.ReadDir(m.PluginsDir())
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			id := e.Name()
			if _, ok := m.states[id]; !ok {
				m.states[id] = pluginState{ID: id, Enabled: false, InstalledAt: nowMilli()}
				m.order = append(m.order, id)
			}
		}
	}
	for _, id := range m.order {
		dir := filepath.Join(m.PluginsDir(), id)
		mf, err := ReadManifest(dir)
		if err != nil {
			m.errs[id] = err.Error()
			continue
		}
		m.manifest[id] = mf
	}
	m.saveLocked()
}

func (m *Manager) saveLocked() {
	if m.closed {
		return
	}
	sf := stateFile{}
	for _, id := range m.order {
		sf.Plugins = append(sf.Plugins, m.states[id])
	}
	raw, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(m.DataDir, 0o755); err != nil {
		return
	}
	if err := writeFileAtomic(m.StatePath(), raw); err != nil {
		m.logf("写插件状态失败: %v", err)
	}
}

// List 插件列表（含运行期错误、健康状态与声明）。
func (m *Manager) List() []PluginInfo {
	host := m.hostIfStarted()
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PluginInfo, 0, len(m.order))
	for _, id := range m.order {
		st := m.states[id]
		info := PluginInfo{ID: id, Enabled: st.Enabled, LastError: st.LastError, Healthy: true}
		if mf, ok := m.manifest[id]; ok {
			info.Name = mf.Name
			info.Version = mf.Version
			info.Description = mf.Description
			info.Dir = mf.Dir
			info.Tools = len(mf.Tools)
			info.Skills = len(mf.Skills)
			info.Permissions = mf.Permissions
			info.Configuration = mf.Configuration
			info.Trust = mf.Trust
			if st.Verified {
				info.Trust = TrustVerified
			}
			if mf.Runtime.IsProcess() {
				info.Runtime = RuntimeProcess
				info.CachedTools = len(loadToolsCache(mf.Dir, mf.Version, mf.Runtime.ProtocolVersion))
			}
		} else {
			info.Name = id
			info.Dir = filepath.Join(m.PluginsDir(), id)
		}
		if e := m.errs[id]; e != "" {
			info.Error = e
		}
		if host != nil {
			info.Running = host.Running(id)
			info.Healthy = host.Healthy(id)
			if e := host.LastError(id); e != "" {
				info.LastError = e
			}
			if cached := host.CachedTools(&Manifest{ID: id, Dir: info.Dir}); len(cached) > 0 {
				info.CachedTools = len(cached)
			}
		}
		out = append(out, info)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get 取清单。
func (m *Manager) Get(id string) (*Manifest, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mf, ok := m.manifest[id]
	return mf, ok
}

func (m *Manager) manifestOf(id string) (*Manifest, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mf, ok := m.manifest[id]
	return mf, ok
}

func (m *Manager) enabledOf(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[id].Enabled
}

// EnabledManifests 取已启用插件的清单副本（按 id 排序）。
func (m *Manager) EnabledManifests() []*Manifest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Manifest, 0, len(m.order))
	for _, id := range m.order {
		if !m.states[id].Enabled {
			continue
		}
		mf, ok := m.manifest[id]
		if !ok {
			continue
		}
		cp := *mf
		out = append(out, &cp)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Install 从本地目录或 zip 安装（已存在则覆盖）。
//
// 声明式插件沿用「安装即启用」；进程插件安装后默认停用，需显式 SetEnabled 才拉起子进程。
func (m *Manager) Install(ctx context.Context, src string) (*Manifest, error) {
	return m.InstallWithOptions(ctx, src, InstallOptions{})
}

// InstallWithOptions 带摘要校验的安装。
func (m *Manager) InstallWithOptions(ctx context.Context, src string, opts InstallOptions) (*Manifest, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, errors.New("安装源不能为空")
	}
	st, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("安装源不存在: %w", err)
	}
	verified := false
	if want := strings.ToLower(strings.TrimSpace(opts.ExpectedSHA256)); want != "" {
		if st.IsDir() {
			return nil, errors.New("目录安装源不支持摘要校验，请用 zip")
		}
		got, err := fileSHA256(src)
		if err != nil {
			return nil, err
		}
		if got != want {
			return nil, fmt.Errorf("安装包校验失败：期望 %s，实际 %s", want, got)
		}
		verified = true
	}
	if err := os.MkdirAll(m.PluginsDir(), 0o755); err != nil {
		return nil, err
	}

		var (
		mf     *Manifest
		target string
	)
	if st.IsDir() {
		srcMf, err := ReadManifest(src)
		if err != nil {
			return nil, err
		}
		target = filepath.Join(m.PluginsDir(), srcMf.ID)
		if !samePath(src, target) {
			if err := os.RemoveAll(target); err != nil {
				return nil, err
			}
			if err := copyTree(src, target); err != nil {
				return nil, fmt.Errorf("复制插件失败: %w", err)
			}
		}
	} else {
		installed, err := installZip(src, m.PluginsDir())
		if err != nil {
			return nil, err
		}
		target = installed.Dir
	}

	// 以落盘后的目录为准重新校验（Dir 必须指向安装目录，而不是安装源）
	mf, err = ReadManifest(target)
	if err != nil {
		return nil, err
	}
	enabled := !mf.Runtime.IsProcess()
	m.mu.Lock()
	if _, ok := m.states[mf.ID]; !ok {
		m.order = append(m.order, mf.ID)
	}
	m.states[mf.ID] = pluginState{
		ID: mf.ID, Enabled: enabled, InstalledAt: nowMilli(), EnabledAt: nowMilli(), Verified: verified,
	}
	m.manifest[mf.ID] = mf
	delete(m.errs, mf.ID)
	m.saveLocked()
	m.mu.Unlock()

	if enabled {
		m.startIfProcess(ctx, mf.ID)
	}
	return mf, nil
}

// SetEnabled 启用 / 停用插件。
func (m *Manager) SetEnabled(id string, on bool) error {
	m.mu.Lock()
	st, ok := m.states[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("插件不存在：%s", id)
	}
	if st.Enabled == on {
		m.mu.Unlock()
		return nil
	}
	st.Enabled = on
	st.EnabledAt = nowMilli()
	if !on {
		st.LastError = ""
	}
	m.states[id] = st
	m.saveLocked()
	m.mu.Unlock()

	if on {
		m.startIfProcess(context.Background(), id)
		return nil
	}
	m.stopProcess(id)
	if mf, ok := m.manifestOf(id); ok {
		clearToolsCache(mf.Dir)
	}
	return nil
}

// Uninstall 卸载：先停进程，再删目录与状态。
func (m *Manager) Uninstall(id string) error {
	m.stopProcess(id)
	dir := filepath.Join(m.PluginsDir(), id)
	if mf, ok := m.manifestOf(id); ok {
		dir = mf.Dir
	}
	clearToolsCache(dir)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("删除插件目录失败: %w", err)
	}
	_ = os.Remove(m.configPath(id))
	m.mu.Lock()
	delete(m.states, id)
	delete(m.manifest, id)
	delete(m.errs, id)
	delete(m.lastHealth, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.saveLocked()
	m.mu.Unlock()
	return nil
}

// Close 停止健康检查与所有插件子进程。
func (m *Manager) Close() {
	m.mu.Lock()
	alreadyClosed := m.closed
	m.closed = true
	m.mu.Unlock()
	if alreadyClosed {
		return
	}
	if m.healthCancel != nil {
		m.healthCancel()
	}
	m.hostMu.Lock()
	h := m.host
	m.hostMu.Unlock()
	if h != nil {
		h.Close()
	}
}

// StartAll 启动所有已启用的进程插件，并开启健康检查循环（幂等）。
func (m *Manager) StartAll(ctx context.Context) {
	m.healthOnce.Do(func() {
		m.healthCtx, m.healthCancel = context.WithCancel(context.Background())
		go m.healthLoop()
	})
	for _, mf := range m.EnabledManifests() {
		if !mf.Runtime.IsProcess() {
			continue
		}
		m.startIfProcess(ctx, mf.ID)
	}
}

func (m *Manager) healthLoop() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-m.healthCtx.Done():
			return
		case <-t.C:
			m.sweepHealth()
		}
	}
}

func (m *Manager) sweepHealth() {
	h := m.hostIfStarted()
	if h == nil {
		return
	}
	now := time.Now()
	for _, mf := range m.EnabledManifests() {
		if !mf.Runtime.IsProcess() {
			continue
		}
		interval := mf.Runtime.HealthInterval()
		if interval <= 0 {
			continue
		}
		m.mu.Lock()
		last := m.lastHealth[mf.ID]
		if now.Sub(last) < interval {
			m.mu.Unlock()
			continue
		}
		m.lastHealth[mf.ID] = now
		m.mu.Unlock()
		if !h.Running(mf.ID) {
			continue
		}
		h.PingAll(m.healthCtx, []*Manifest{mf})
	}
}

func (m *Manager) startIfProcess(ctx context.Context, id string) {
	m.mu.RLock()
	mf, ok := m.manifest[id]
	enabled := m.states[id].Enabled
	m.mu.RUnlock()
	if !ok || !enabled || !mf.Runtime.IsProcess() {
		return
	}
	h := m.hostFor()
	go func() {
		if _, err := h.Ensure(ctx, mf); err != nil {
			m.noteError(id, err.Error())
		}
	}()
}

func (m *Manager) stopProcess(id string) {
	m.hostMu.Lock()
	h := m.host
	m.hostMu.Unlock()
	if h != nil {
		h.Stop(id)
	}
}

func (m *Manager) hostIfStarted() *Host {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	return m.host
}

func (m *Manager) hostFor() *Host {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	if m.host == nil {
		h := NewHost(m.logf)
		h.OnError = m.noteError
		h.OnUnhealthy = m.noteUnhealthy
		h.DataDir = m.DataDir
		m.host = h
	}
	return m.host
}

// noteError 记录运行期错误（出现在 List 的 lastError 上）。
func (m *Manager) noteError(id, msg string) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	st := m.states[id]
	st.LastError = msg
	m.states[id] = st
	m.errs[id] = ""
	m.saveLocked()
	m.mu.Unlock()
}

// noteUnhealthy 健康检查判定插件不可恢复时自动停用。
func (m *Manager) noteUnhealthy(id, msg string) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	st := m.states[id]
	st.Enabled = false
	st.LastError = msg
	m.states[id] = st
	m.saveLocked()
	m.mu.Unlock()
}

// SetWorkspace 工作区切换时通知所有存活插件。
func (m *Manager) SetWorkspace(workspace string) {
	if h := m.hostIfStarted(); h != nil {
		h.SetWorkspace(workspace)
	}
}

// ProcessLogs 取进程插件最近的输出（stderr + log 通知，最多 200 行）。
func (m *Manager) ProcessLogs(id string) []string {
	h := m.hostIfStarted()
	if h == nil {
		return nil
	}
	return h.Logs(id)
}

// Progress 取插件最近的进度事件。
func (m *Manager) Progress(id string) []ProgressEvent {
	h := m.hostIfStarted()
	if h == nil {
		return nil
	}
	return h.Progress(id)
}

// ProcessPing 对已启用的进程插件做一次健康检查（不启动新进程）。
func (m *Manager) ProcessPing(ctx context.Context, id string) error {
	mf, ok := m.manifestOf(id)
	if !ok {
		return fmt.Errorf("插件不存在：%s", id)
	}
	if !mf.Runtime.IsProcess() || !m.enabledOf(id) {
		return nil
	}
	h := m.hostIfStarted()
	if h == nil {
		return nil
	}
	return h.Ping(ctx, mf)
}

// ProcessTools 取进程插件上报的工具（必要时拉起并握手）。
func (m *Manager) ProcessTools(ctx context.Context, id string) ([]ToolDescriptor, error) {
	m.mu.RLock()
	mf, ok := m.manifest[id]
	enabled := m.states[id].Enabled
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("插件不存在：%s", id)
	}
	if !mf.Runtime.IsProcess() {
		return nil, nil
	}
	if !enabled {
		return nil, fmt.Errorf("插件未启用：%s", id)
	}
	tools, err := m.hostFor().Tools(ctx, mf)
	if err != nil {
		m.noteError(id, err.Error())
		return nil, err
	}
	return tools, nil
}

// RunningProcessTools 已启用进程插件的工具：活进程优先，其次磁盘缓存（不会拉起进程）。
func (m *Manager) RunningProcessTools(id string) []ToolDescriptor {
	m.mu.RLock()
	mf, ok := m.manifest[id]
	enabled := m.states[id].Enabled
	m.mu.RUnlock()
	if !ok || !enabled || !mf.Runtime.IsProcess() {
		return nil
	}
	if h := m.hostIfStarted(); h != nil {
		if tools := h.CachedTools(mf); len(tools) > 0 {
			return tools
		}
	}
	cached := loadToolsCache(mf.Dir, mf.Version, mf.Runtime.ProtocolVersion)
	for i := range cached {
		cached[i].PluginID = mf.ID
	}
	return cached
}

// Commands 已启用插件贡献的命令。
func (m *Manager) Commands(ctx context.Context, id string) ([]CommandDescriptor, error) {
	m.mu.RLock()
	mf, ok := m.manifest[id]
	enabled := m.states[id].Enabled
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("插件不存在：%s", id)
	}
	if !enabled {
		return nil, fmt.Errorf("插件未启用：%s", id)
	}
	if !mf.Runtime.IsProcess() {
		return nil, nil
	}
	list, err := m.hostFor().Commands(ctx, mf)
	if err != nil {
		m.noteError(id, err.Error())
		return nil, err
	}
	out := make([]CommandDescriptor, 0, len(list))
	for _, c := range list {
		out = append(out, CommandDescriptor{PluginID: id, ID: c.ID, Title: c.Title, Description: c.Description})
	}
	return out, nil
}

// RunCommand 执行插件命令（带限流与审计）。
func (m *Manager) RunCommand(ctx context.Context, id, commandID string, args map[string]any) (CallResult, error) {
	start := time.Now()
	mf, ok := m.manifestOf(id)
	if !ok {
		err := fmt.Errorf("插件不存在：%s", id)
		m.recordAudit(id, commandID, "plugin-command", args, start, CallResult{}, err)
		return CallResult{}, err
	}
	if !m.enabledOf(id) {
		err := fmt.Errorf("插件未启用：%s", id)
		m.recordAudit(id, commandID, "plugin-command", args, start, CallResult{}, err)
		return CallResult{}, err
	}
	release, err := m.limiter.acquire(id)
	if err != nil {
		m.recordAudit(id, commandID, "plugin-command", args, start, CallResult{}, err)
		return CallResult{}, err
	}
	defer release()
	res, err := m.hostFor().RunCommand(ctx, mf, commandID, args)
	if err != nil {
		m.noteError(id, err.Error())
	}
	m.recordAudit(id, commandID, "plugin-command", args, start, res, err)
	return res, err
}

// DeclarativeTools 已启用插件的声明式工具（http / command）。
func (m *Manager) DeclarativeTools() []ToolDescriptor {
	var out []ToolDescriptor
	for _, mf := range m.EnabledManifests() {
		for _, t := range mf.Tools {
			out = append(out, ToolDescriptor{
				PluginID:    mf.ID,
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
				Safe:        t.Safe,
				Kind:        t.Type,
			})
		}
	}
	return out
}

// Call 调用插件的某个工具：声明式工具就地执行，进程插件转发到子进程。
func (m *Manager) Call(ctx context.Context, pluginID, tool string, args map[string]any, env Env) (CallResult, error) {
	start := time.Now()
	mf, ok := m.manifestOf(pluginID)
	if !ok {
		err := fmt.Errorf("插件不存在：%s", pluginID)
		m.recordAudit(pluginID, tool, "", args, start, CallResult{}, err)
		return CallResult{}, err
	}
	if !m.enabledOf(pluginID) {
		err := fmt.Errorf("插件未启用：%s", pluginID)
		m.recordAudit(pluginID, tool, mf.toolKind(tool), args, start, CallResult{}, err)
		return CallResult{}, err
	}
	release, err := m.limiter.acquire(pluginID)
	if err != nil {
		m.recordAudit(pluginID, tool, mf.toolKind(tool), args, start, CallResult{}, err)
		return CallResult{}, err
	}
	defer release()

	kind := mf.toolKind(tool)
	var res CallResult
	if mf.Runtime.IsProcess() {
		if env.Workspace != "" {
			if h := m.hostIfStarted(); h != nil {
				h.Workspace = env.Workspace
			}
		}
		res, err = m.hostFor().Call(ctx, mf, env, tool, args)
		if err != nil {
			m.noteError(pluginID, err.Error())
		}
		m.recordAudit(pluginID, tool, kind, args, start, res, err)
		return res, err
	}
	for _, t := range mf.Tools {
		if !strings.EqualFold(t.Name, tool) {
			continue
		}
		if t.Type == "http" {
			res, err = m.runHTTP(ctx, mf, t, args, env)
		} else {
			res, err = m.runCommand(ctx, mf, t, args, env)
		}
		m.recordAudit(pluginID, tool, kind, args, start, res, err)
		return res, err
	}
	err = fmt.Errorf("插件 %s 没有工具 %s", pluginID, tool)
	m.recordAudit(pluginID, tool, kind, args, start, CallResult{}, err)
	return CallResult{}, err
}

// toolKind 判断工具类型（审计用）。
func (mf *Manifest) toolKind(tool string) string {
	if mf.Runtime.IsProcess() {
		return RuntimeProcess
	}
	for _, t := range mf.Tools {
		if strings.EqualFold(t.Name, tool) {
			return t.Type
		}
	}
	return ""
}

func (m *Manager) recordAudit(pluginID, tool, kind string, args map[string]any, start time.Time, res CallResult, err error) {
	if m.audit == nil {
		return
	}
	e := auditEntry{
		Plugin:      pluginID,
		Tool:        tool,
		Kind:        kind,
		ArgsDigest:  sanitizeArgs(args),
		DurationMs:  time.Since(start).Milliseconds(),
		OK:          err == nil,
		IsError:     res.IsError,
		ResultBytes: len(res.Text),
	}
	if err != nil {
		msg := err.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		e.Error = msg
	}
	m.audit.write(e)
}

// ---- 插件配置 ----

func (m *Manager) configPath(id string) string {
	return filepath.Join(m.PluginsDir(), id+".config.json")
}

// ConfigPath 插件配置文件路径。
func (m *Manager) ConfigPath(id string) string { return m.configPath(id) }

// GetConfig 读取插件配置（已保存值优先，缺省值取 configuration 里的 default）。
func (m *Manager) GetConfig(id string) (map[string]any, error) {
	mf, ok := m.manifestOf(id)
	if !ok {
		return nil, fmt.Errorf("插件不存在：%s", id)
	}
	cfg := map[string]any{}
	if len(mf.Configuration) > 0 {
		var schema struct {
			Properties map[string]struct {
				Default any `json:"default"`
			} `json:"properties"`
		}
		if json.Unmarshal(mf.Configuration, &schema) == nil {
			for k, v := range schema.Properties {
				if v.Default != nil {
					cfg[k] = v.Default
				}
			}
		}
	}
	raw, err := os.ReadFile(m.configPath(id))
	if err == nil {
		var saved map[string]any
		if json.Unmarshal(raw, &saved) == nil {
			for k, v := range saved {
				cfg[k] = v
			}
		}
	}
	return cfg, nil
}

// SetConfig 保存插件配置并通知插件（v2 且声明 config 能力时）。
func (m *Manager) SetConfig(id string, cfg map[string]any) error {
	if _, ok := m.manifestOf(id); !ok {
		return fmt.Errorf("插件不存在：%s", id)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.PluginsDir(), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(m.configPath(id), raw); err != nil {
		return err
	}
	if h := m.hostIfStarted(); h != nil {
		h.NotifyConfig(id, cfg)
	}
	return nil
}

// fileSHA256 计算文件摘要。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
