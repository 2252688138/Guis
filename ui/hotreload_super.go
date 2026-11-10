//go:build !js && !android && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 热重载守护进程侧：编译、监听源码、重启子进程。
//
// 流程：go build 出新 exe → 以子进程方式启动（GUIS_HOT_CHILD=1）→
// 轮询源码变化 → 等源码树「静默」→ 编译 → 通知旧子进程保存现场并退出 → 起新子进程。
// 编译失败时保留旧窗口继续运行，只在控制台打印错误。
//
// 防抖（窗口不再频繁闪断重启）：
//   - 改动后必须连续 hotQuietTime 没有任何新改动才开始编译：一次保存通常伴随
//     多文件 / 多阶段写入（先写临时文件再改名、agent 连打多次保存），
//     原先「检测到就编译」会为同一个动作重启好几轮；
//   - 两次重启之间至少间隔 hotMinRestartGap；
//   - 子进程刚起来就退出（起不来）时不跟着退出，等下次源码改动再试。
//
// 退出（不再出现 exit status 0xc000013a 后留下孤儿窗口）：
//   - 拦截 Ctrl+C / 关控制台，先把子进程干净收掉再退出。

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// hotStateFile 落在临时目录里的现场文件名。
	hotStateFile = "state.json"
	// hotWatchInterval 源码轮询间隔（不引入 fsnotify 依赖，轮询足够省）。
	hotWatchInterval = 300 * time.Millisecond
	// hotQuietTime 源码树需要连续这么久没有新改动，才认为「这次保存结束」。
	// 一次保存往往是多文件 / 多阶段写入，等静默可以把它们合并成一次编译。
	hotQuietTime = 800 * time.Millisecond
	// hotMinRestartGap 两次重启之间的最小间隔，编译很快时也给界面留出稳定时间。
	hotMinRestartGap = 1200 * time.Millisecond
	// hotStopTimeout 通知子进程保存现场后等待其退出的上限。
	hotStopTimeout = 3 * time.Second
	// hotCrashWindow 子进程启动后这么短时间内就退出，视为「起不来」而不是用户关窗。
	hotCrashWindow = 2500 * time.Millisecond
)

// hotReloadSupervise 守护进程主循环：编译 → 起子进程 → 监听源码 → 重启。
func hotReloadSupervise() error {
	root := hotModuleDir()
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("guis-hot-%d", os.Getpid()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stateFile := filepath.Join(dir, hotStateFile)

	hotLog("热重载已开启：监听 %s（GUIS_HOT=0 可关闭）", root)
	w := newHotWatcher(root)
	go w.run()
	tick := time.NewTicker(hotWatchInterval)
	defer tick.Stop()

	// cur 会被信号处理 goroutine 读取，统一走锁访问。
	var (
		mu  sync.Mutex
		cur *hotChild
	)
	getCur := func() *hotChild {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	setCur := func(c *hotChild) {
		mu.Lock()
		cur = c
		mu.Unlock()
	}

	// Ctrl+C / 关控制台：先通知子进程保存现场退出，再自己退出，
	// 否则控制台一断子进程就成了关不掉的孤儿窗口（exit status 0xc000013a）。
	hotWatchInterrupt(func() {
		if c := getCur(); c != nil {
			hotLog("收到退出信号，正在关闭子进程…")
			c.stop()
		}
	})

	var (
		gen         int
		lastRestart time.Time
		retry       bool // 上次编译 / 启动失败：等下一次改动再试，避免空转
	)

	// waitChange 阻塞到下一次源码改动，并继续等它静默下来。
	waitChange := func() {
		for !w.take() {
			<-tick.C
		}
		w.settle(hotQuietTime)
	}

	// compile 编译下一代 exe（成功时返回新路径、编译开始时刻与耗时）。
	compile := func() (string, time.Time, time.Duration, error) {
		next := hotExePath(dir, gen+1)
		t0 := time.Now()
		if err := hotBuild(next); err != nil {
			return "", t0, time.Since(t0), err
		}
		gen++
		hotRemoveOld(dir, gen)
		return next, t0, time.Since(t0), nil
	}

	// launch 用新编译的 exe 替换当前窗口。buildStart 为本次编译开始的时刻：
	// 编译期间又到的改动没有被这次编译包含，要保留 dirty 再编一轮，否则会「丢掉一次修改」。
	launch := func(exe string, buildStart time.Time) bool {
		if old := getCur(); old != nil {
			if w.changedAfter(buildStart) {
				hotLog("编译期间源码又有改动，将再编译一次")
			} else {
				// 这批保存已经被编进新 exe，清掉标记，免得空转重启。
				w.take()
			}
			if gap := hotMinRestartGap - time.Since(lastRestart); gap > 0 {
				time.Sleep(gap)
			}
			old.stop()
		}
		c := hotSpawn(exe, stateFile)
		setCur(c)
		if c == nil {
			return false
		}
		lastRestart = time.Now()
		return true
	}

	for {
		c := getCur()
		if c == nil {
			if retry {
				waitChange()
				retry = false
			}
			exe, t0, _, err := compile()
			if err != nil {
				hotLog("编译失败，等待下一次修改：\n%v", err)
				retry = true
				continue
			}
			if !launch(exe, t0) {
				retry = true // 起不来：等下次改动重试，避免死循环
			}
			continue
		}

		select {
		case <-tick.C:
			if !w.take() {
				continue
			}
			if paths := w.takeChanged(); len(paths) > 0 {
				hotLog("检测到源码变化：%s（等待静默 %dms）", hotShortPaths(root, paths), hotQuietTime.Milliseconds())
			}
			// 等这次保存彻底结束，把多文件 / 多阶段写入合并成一次编译。
			w.settle(hotQuietTime)
			exe, t0, dur, err := compile()
			if err != nil {
				hotLog("编译失败，保留当前界面继续运行：\n%v", err)
				continue
			}
			if !launch(exe, t0) {
				hotLog("新进程启动失败，保留当前界面继续运行")
				continue
			}
			hotLog("已重新加载（编译 %dms / 合计 %dms）", dur.Milliseconds(), time.Since(t0).Milliseconds())
		case <-c.done:
			if getCur() != c {
				continue // 已被替换的旧进程，忽略它的退出
			}
			hotRemoveOld(dir, gen)
			if time.Since(c.start) < hotCrashWindow && c.waitErr != nil {
				// 刚起来就没了：多半是配置 / 资源 / 端口占用之类的问题。
				// 这里不跟着退出，否则用户看到的就是「热重载把程序关了」，而且连日志都来不及看。
				hotLog("子进程启动后立刻退出（%v），等待源码修改后重试", c.waitErr)
				setCur(nil)
				retry = true
				continue
			}
			hotLog("窗口已关闭，退出热重载")
			return nil
		}
	}
}

// hotChild 一个被守护的应用子进程。
type hotChild struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	// start 子进程启动时刻（判断「刚起来就退出」用）。
	start time.Time
	// waitErr cmd.Wait 的返回（进程退出后才可读）。
	waitErr error
}

// hotSpawn 启动一个子进程（继承当前工作目录，输出直通终端）。
func hotSpawn(exe, stateFile string) *hotChild {
	cmd := exec.Command(exe)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		hotLog("创建子进程控制管道失败：%v", err)
		return nil
	}
	cmd.Env = append(os.Environ(), hotEnvChild+"=1", hotEnvState+"="+stateFile)
	hotCmdAttrs(cmd)
	if err := cmd.Start(); err != nil {
		hotLog("启动子进程失败：%v", err)
		_ = in.Close()
		return nil
	}
	c := &hotChild{cmd: cmd, stdin: in, done: make(chan struct{}), start: time.Now()}
	go func() {
		c.waitErr = cmd.Wait()
		close(c.done)
	}()
	return c
}

// stop 通知子进程保存现场并退出；超时则强杀。
func (c *hotChild) stop() {
	if c == nil {
		return
	}
	if c.stdin != nil {
		_, _ = io.WriteString(c.stdin, "save\n")
		_ = c.stdin.Close()
	}
	select {
	case <-c.done:
		return
	case <-time.After(hotStopTimeout):
	}
	hotKillProcess(c.cmd)
	<-c.done
}

// hotWatchInterrupt 拦截 Ctrl+C / 控制台关闭：先收掉子进程再退出。
func hotWatchInterrupt(stop func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		stop()
		os.Exit(0)
	}()
}

// hotWatcher 轮询源码变化（只关心 .go / go.mod / go.sum，忽略测试文件与构建目录）。
type hotWatcher struct {
	root  string
	state map[string]string
	mu    sync.Mutex
	// dirty 有未处理改动；last 最近一次观察到改动的时刻（静默判断用）。
	dirty bool
	last  time.Time
	// paths 最近一批改动的文件（只用于日志，帮助定位「谁在反复触发重启」）。
	paths []string
}

func newHotWatcher(root string) *hotWatcher {
	return &hotWatcher{root: root, state: map[string]string{}, last: time.Now()}
}

func (w *hotWatcher) run() {
	w.scan(false) // 基线快照
	t := time.NewTicker(hotWatchInterval)
	defer t.Stop()
	for range t.C {
		w.scan(true)
	}
}

// take 取走「源码已变化」标记：连续多次保存只会合并成一次重启
// （编译本身要花时间，中间的改动不需要逐个重放）。
func (w *hotWatcher) take() bool {
	w.mu.Lock()
	d := w.dirty
	w.dirty = false
	w.mu.Unlock()
	return d
}

// takeChanged 取走最近一批改动路径（日志用）。
func (w *hotWatcher) takeChanged() []string {
	w.mu.Lock()
	p := w.paths
	w.paths = nil
	w.mu.Unlock()
	return p
}

// changedAfter 编译开始后是否又检测到新的改动（这些改动没被本次编译包含）。
func (w *hotWatcher) changedAfter(t time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dirty && w.last.After(t)
}

// settle 阻塞到源码树连续 quiet 时间没有新改动，期间到达的改动会把等待顺延，
// 从而把「一次保存」引发的多文件 / 多阶段写入合并成一次编译。
func (w *hotWatcher) settle(quiet time.Duration) {
	for {
		w.take()
		w.mu.Lock()
		last := w.last
		w.mu.Unlock()
		d := quiet - time.Since(last)
		if d <= 0 {
			return
		}
		if d > 60*time.Millisecond {
			d = 60 * time.Millisecond
		}
		time.Sleep(d)
	}
}

// scan 重新扫描一遍源码树，返回相对上次是否有变化。
//
// record=false 用于建立基线（首次扫描不算改动）；新出现与被删除的源码文件
// 同样算改动——否则新增一个 .go 文件不会触发重编译。
func (w *hotWatcher) scan(record bool) bool {
	next := make(map[string]string, len(w.state)+16)
	var paths []string
	_ = filepath.WalkDir(w.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != w.root && hotSkipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			return nil
		}
		if strings.HasSuffix(name, "_test.go") {
			return nil // 测试文件不参与构建，改了不该重启窗口
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		stamp := fmt.Sprintf("%d/%d", info.Size(), info.ModTime().UnixNano())
		if old, ok := w.state[p]; ok {
			if old != stamp {
				paths = append(paths, p)
			}
		} else if record {
			paths = append(paths, p) // 新增文件
		}
		next[p] = stamp
		return nil
	})
	// 被删除的文件 / 目录同样要重新编译。
	for p := range w.state {
		if _, ok := next[p]; !ok && record {
			paths = append(paths, p)
		}
	}
	w.state = next
	if !record || len(paths) == 0 {
		return false
	}
	w.mu.Lock()
	w.dirty = true
	w.last = time.Now()
	if len(w.paths) < 16 {
		w.paths = append(w.paths, paths...)
	}
	w.mu.Unlock()
	return true
}

// hotShortPaths 把改动路径压成一行日志（去重，最多列 4 个，其余只报数量）。
func hotShortPaths(root string, paths []string) string {
	const max = 4
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, max+1)
	for _, p := range paths {
		if rel, err := filepath.Rel(root, p); err == nil {
			p = rel
		}
		p = filepath.ToSlash(p)
		if seen[p] {
			continue
		}
		seen[p] = true
		if len(out) >= max {
			out = append(out, fmt.Sprintf("…共 %d 处", len(seen)))
			break
		}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

// hotSkipDir 监听时直接跳过的目录（构建产物、编辑器缓存、依赖目录）。
func hotSkipDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".zym", ".workflow", ".idea", ".vscode",
		"node_modules", "vendor", "bin", "dist", "build", "out", "release":
		return true
	}
	return false
}

// hotBuild 把当前包编译到 dst（失败时依次尝试其它包选择方式）。
func hotBuild(dst string) error {
	dir := hotModuleDir()
	var lastErr error
	for _, pkg := range hotBuildPackages() {
		args := []string{"build", "-o", dst}
		if tags := hotBuildTags(); tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, pkg)
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%v\n%s", err, strings.TrimSpace(string(out)))
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可编译的包")
	}
	return lastErr
}

// hotBuildPackages 编译目标候选：先用 go run 记录下来的包路径，失败再退回当前目录。
func hotBuildPackages() []string {
	var out []string
	if p := hotMainPackage(); p != "" {
		out = append(out, p)
	}
	return append(out, ".")
}

// hotMainPackage 从构建信息里取当前 main 包的导入路径。
func hotMainPackage() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	p := strings.TrimSpace(bi.Path)
	if p == "" || p == "command-line-arguments" || strings.ContainsAny(p, " \t\"'") {
		return ""
	}
	return p
}

// hotBuildTags 还原 go run 时使用的 -tags（保证与手动运行一致）。
func hotBuildTags() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == "-tags" {
			return s.Value
		}
	}
	return ""
}

// hotModuleDir 从当前目录向上找 go.mod，找不到就留在当前目录。
func hotModuleDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}

// hotExePath 每次编译用新文件名：Windows 下正在运行的 exe 不能被覆盖。
func hotExePath(dir string, gen int) string {
	return filepath.Join(dir, fmt.Sprintf("app-%d%s", gen, hotExeSuffix()))
}

// hotRemoveOld 清理除最新两代之外的旧产物。
func hotRemoveOld(dir string, gen int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	keep := map[string]bool{
		fmt.Sprintf("app-%d%s", gen, hotExeSuffix()):   true,
		fmt.Sprintf("app-%d%s", gen-1, hotExeSuffix()): true,
	}
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] || !strings.HasPrefix(e.Name(), "app-") {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// hotLog 守护进程日志：统一前缀，方便在 go run 的控制台里辨认。
func hotLog(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[guis-hot] "+format+"\n", args...)
}
