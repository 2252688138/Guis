// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// 文件 / 目录变更监听。
//
//	stop := ui.WatchDir(dir, ui.WatchOptions{Recursive: true}, func(evs []ui.FsEvent) {
//	    for _, e := range evs { reload(e.Path) }
//	})
//	defer stop()
//
//	stop = ui.WatchFile(cfgPath, func(e ui.FsEvent) { applyConfig(e.Path) })
//
// Windows 走 ReadDirectoryChangesW（事件驱动、零轮询）；其它平台用「定时扫描 + 差异
// 比对」兜底（默认 400ms 一次）。回调统一在 UI 线程执行，可以直接改状态并 RequestRedraw。
//
// 注意：扫描兜底只比较「大小 + 修改时间 + 类型」，同一时间点内被改回原大小的快速写入
// 可能漏报；配置文件热加载这类场景足够，强一致需求请自行校验内容哈希。

// FsChange 变更类型。
type FsChange int

const (
	// FsCreated 新建（含从外部移入）。
	FsCreated FsChange = iota
	// FsModified 内容或属性变化。
	FsModified
	// FsRemoved 删除（含移出到别处）。
	FsRemoved
	// FsRenamed 重命名（扫描兜底会拆成 Removed + Created）。
	FsRenamed
)

// FsChangeName 变更类型的中文名（日志 / 界面展示用）。
func FsChangeName(c FsChange) string {
	switch c {
	case FsCreated:
		return "新建"
	case FsModified:
		return "修改"
	case FsRemoved:
		return "删除"
	case FsRenamed:
		return "重命名"
	}
	return "未知"
}

// FsEvent 一条文件变更。
type FsEvent struct {
	// Path 变更的绝对路径。
	Path string
	// OldPath 重命名前的路径（仅 FsRenamed 且平台提供时非空）。
	OldPath string
	// Change 变更类型。
	Change FsChange
	// IsDir 目标是否为目录。
	IsDir bool
}

// String 便于日志输出的简短描述。
func (e FsEvent) String() string { return FsChangeName(e.Change) + " " + e.Path }

// WatchOptions 监听参数。
type WatchOptions struct {
	// Recursive 是否递归监听子目录。
	Recursive bool
	// Debounce 变更合并窗口（同一窗口内的事件一次性回调）。0 = 150ms。
	Debounce time.Duration
	// PollInterval 扫描兜底间隔（仅非 Windows 生效）。0 = 400ms。
	PollInterval time.Duration
	// Filter 路径过滤（返回 false 的变更不上报）。nil = 全部上报。
	Filter func(path string) bool
}

func (o WatchOptions) debounce() time.Duration {
	if o.Debounce == 0 {
		return 150 * time.Millisecond
	}
	return o.Debounce
}

func (o WatchOptions) pollInterval() time.Duration {
	if o.PollInterval <= 0 {
		return 400 * time.Millisecond
	}
	return o.PollInterval
}

// watchDirNative 平台原生实现（Windows: ReadDirectoryChangesW）。
// 为空时统一走 watchDirPolling。
var watchDirNative func(dir string, opts WatchOptions, fn func([]FsEvent)) func()

// WatchDir 监听目录变更，返回停止函数（可重复调用，第二次是空操作）。
func WatchDir(dir string, opts WatchOptions, fn func([]FsEvent)) func() {
	if dir == "" || fn == nil {
		return func() {}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	// 回调统一切到 UI 线程：业务在回调里改界面状态是常态。
	wrapped := func(evs []FsEvent) {
		if len(evs) == 0 {
			return
		}
		if opts.Filter != nil {
			kept := make([]FsEvent, 0, len(evs))
			for _, e := range evs {
				if opts.Filter(e.Path) {
					kept = append(kept, e)
				}
			}
			if len(kept) == 0 {
				return
			}
			evs = kept
		}
		cp := append([]FsEvent(nil), evs...)
		postToUI(func() {
			fn(cp)
			RequestRedraw()
		})
	}
	if watchDirNative != nil {
		if stop := watchDirNative(dir, opts, wrapped); stop != nil {
			return stop
		}
	}
	return watchDirPolling(dir, opts, wrapped)
}

// WatchFile 监听单个文件（内部监听父目录再过滤，文件被删除后重建同样能收到）。
func WatchFile(path string, fn func(FsEvent)) func() {
	if path == "" || fn == nil {
		return func() {}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	dir, base := filepath.Split(abs)
	if dir == "" {
		dir = "."
	}
	return WatchDir(dir, WatchOptions{
		Filter: func(p string) bool { return filepath.Base(p) == base },
	}, func(evs []FsEvent) {
		for _, e := range evs {
			fn(e)
		}
	})
}

// ---- 扫描兜底实现 ----

// fsSig 一个文件的可比较签名。
type fsSig struct {
	size  int64
	mtime int64
	dir   bool
}

// scanTree 扫描目录树，返回 path → 签名。
func scanTree(root string, recursive bool, filter func(string) bool) map[string]fsSig {
	out := map[string]fsSig{}
	record := func(p string, d os.DirEntry) {
		if filter != nil && !filter(p) {
			return
		}
		info, err := d.Info()
		if err != nil {
			return
		}
		out[p] = fsSig{size: info.Size(), mtime: info.ModTime().UnixNano(), dir: d.IsDir()}
	}
	if !recursive {
		ents, err := os.ReadDir(root)
		if err != nil {
			return out
		}
		for _, d := range ents {
			record(filepath.Join(root, d.Name()), d)
		}
		return out
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p != root {
			record(p, d)
		}
		return nil
	})
	return out
}

// diffTree 对比两次快照，产出变更列表（按路径排序保证顺序稳定）。
func diffTree(prev, cur map[string]fsSig) []FsEvent {
	var evs []FsEvent
	collect := func(m map[string]fsSig) []string {
		paths := make([]string, 0, len(m))
		for p := range m {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		return paths
	}
	for _, p := range collect(cur) {
		if _, ok := prev[p]; !ok {
			evs = append(evs, FsEvent{Path: p, Change: FsCreated, IsDir: cur[p].dir})
		}
	}
	for _, p := range collect(prev) {
		if _, ok := cur[p]; !ok {
			evs = append(evs, FsEvent{Path: p, Change: FsRemoved, IsDir: prev[p].dir})
		}
	}
	for _, p := range collect(prev) {
		if b, ok := cur[p]; ok && (prev[p].size != b.size || prev[p].mtime != b.mtime) {
			evs = append(evs, FsEvent{Path: p, Change: FsModified, IsDir: b.dir})
		}
	}
	return evs
}

// watchDirPolling 跨平台兜底：定时扫描 + 差异比对。
func watchDirPolling(dir string, opts WatchOptions, fn func([]FsEvent)) func() {
	var once sync.Once
	quit := make(chan struct{})
	stop := func() { once.Do(func() { close(quit) }) }
	if runtime.GOOS == "js" {
		// 浏览器端没有文件系统，保持「符号存在、行为为空」。
		return stop
	}
	prev := scanTree(dir, opts.Recursive, opts.Filter)
	go func() {
		defer recoverReport("watch-dir")
		tk := time.NewTicker(opts.pollInterval())
		defer tk.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tk.C:
				cur := scanTree(dir, opts.Recursive, opts.Filter)
				evs := diffTree(prev, cur)
				prev = cur
				fn(evs)
			}
		}
	}()
	return stop
}
