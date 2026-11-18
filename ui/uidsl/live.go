// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 跟着文件走的界面源：轮询文件修改时间，一变就重新解析并请求重绘。
//
// 这正是「不重启进程、不重新编译就能改界面」的那条路：view() 返回 l.Root()，
// 描述文件改了 → 重新 Build 出一棵新树 → ui.RequestRedraw() → 下一帧就是新界面。
//
//	col pad=16
//	  text "改我试试" size=20
//
// 解析失败时保留上一版（界面不会白掉），错误记在 LastError()，可显示在界面上。

import (
	"os"
	"sync"
	"time"

	"guis/ui"
)

// DefaultInterval 默认的文件轮询间隔。
const DefaultInterval = 200 * time.Millisecond

// Live 一个跟随描述文件的界面源（并发安全）。
type Live struct {
	path     string
	format   Format
	actions  Actions
	interval time.Duration

	mu      sync.RWMutex
	decl    *Node
	err     error
	modTime time.Time
	size    int64
	onLoad  func(Node)

	stopOnce sync.Once
	stop     chan struct{}
}

// Open 打开描述文件（按扩展名判断格式）并开始监听变化。
//
// 首次解析失败会直接返回错误；之后的失败只记在 LastError() 里。
func Open(path string, actions Actions) (*Live, error) {
	return OpenAs(path, FormatOf(path), actions)
}

// OpenAs 指定格式打开（扩展名不合心意时用，例如把 DSL 存成 .txt）。
func OpenAs(path string, f Format, actions Actions) (*Live, error) {
	decl, err := readFileAs(path, f)
	if err != nil {
		return nil, err
	}
	if actions == nil {
		actions = Actions{}
	}
	l := &Live{
		path:     path,
		format:   f,
		actions:  actions,
		interval: DefaultInterval,
		decl:     &decl,
		stop:     make(chan struct{}),
	}
	if st, err := os.Stat(path); err == nil {
		l.modTime, l.size = st.ModTime(), st.Size()
	}
	go l.watch()
	return l, nil
}

// SetInterval 调整轮询间隔（越小越灵敏，代价是文件系统调用更频繁）。
func (l *Live) SetInterval(d time.Duration) {
	if l == nil || d <= 0 {
		return
	}
	l.mu.Lock()
	l.interval = d
	l.mu.Unlock()
}

// OnLoad 注册每次成功加载后的回调（可用于日志或联动其它状态）。
func (l *Live) OnLoad(fn func(Node)) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.onLoad = fn
	l.mu.Unlock()
}

// Root 每次调用都按当前声明新建一棵节点树（框架要求每帧一棵新树）。
func (l *Live) Root() *ui.Node {
	if l == nil {
		return ui.Box()
	}
	l.mu.RLock()
	decl, actions := l.decl, l.actions
	l.mu.RUnlock()
	if decl == nil {
		return ui.Box()
	}
	return Build(*decl, actions)
}

// LastError 最近一次解析 / 构建失败的原因（成功后为 nil）。
func (l *Live) LastError() error {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.err
}

// Path 描述文件路径。
func (l *Live) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Reload 立刻重读文件（不等轮询）；返回本次是否成功。
func (l *Live) Reload() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	path, f := l.path, l.format
	l.mu.Unlock()
	return l.apply(path, f)
}

// Close 停止监听。
func (l *Live) Close() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.stop) })
}

func (l *Live) watch() {
	for {
		l.mu.RLock()
		d := l.interval
		l.mu.RUnlock()
		if d <= 0 {
			d = DefaultInterval
		}
		select {
		case <-l.stop:
			return
		case <-time.After(d):
		}
		st, err := os.Stat(l.path)
		if err != nil {
			continue
		}
		l.mu.RLock()
		same := st.ModTime().Equal(l.modTime) && st.Size() == l.size
		l.mu.RUnlock()
		if same {
			continue
		}
		l.mu.Lock()
		l.modTime, l.size = st.ModTime(), st.Size()
		l.mu.Unlock()
		_ = l.apply(l.path, l.format)
	}
}

// apply 重新解析并替换当前声明；失败时保留旧树，只记录错误。
func (l *Live) apply(path string, f Format) error {
	decl, err := readFileAs(path, f)
	l.mu.Lock()
	if err != nil {
		l.err = err
		l.mu.Unlock()
		ui.RequestRedraw() // 让界面有机会把错误显示出来
		return err
	}
	l.decl = &decl
	l.err = nil
	cb := l.onLoad
	l.mu.Unlock()
	ui.RequestRedraw()
	if cb != nil {
		cb(decl)
	}
	return nil
}

func readFileAs(path string, f Format) (Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Node{}, err
	}
	return Parse(b, f)
}
