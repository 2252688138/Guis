// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 热重载：轮询脚本文件的修改时间 / 大小，一变就重新执行。
// 重载前会清掉上一版脚本的全局状态（含定时器、匿名回调、ui.set 覆盖、模块缓存），
// 只保留标准库与注入的命名空间；与 ui/uidsl 的 Live 同思路：解析失败保留上一版，界面不会白掉。

import (
	"os"
	"time"

	"guis/ui"
)

// startWatch 启动文件轮询（Interval 为负时不起协程）。
func (e *Engine) startWatch() {
	path := e.Path()
	if path == "" || e.opt.Interval < 0 {
		return
	}
	interval := e.opt.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	go func() {
		var lastMod time.Time
		var lastSize int64
		if st, err := os.Stat(path); err == nil {
			lastMod, lastSize = st.ModTime(), st.Size()
		}
		tk := time.NewTicker(interval)
		defer tk.Stop()
		for {
			select {
			case <-e.stopWatch:
				return
			case <-tk.C:
				st, err := os.Stat(path)
				if err != nil {
					continue
				}
				if st.ModTime().Equal(lastMod) && st.Size() == lastSize {
					continue
				}
				lastMod, lastSize = st.ModTime(), st.Size()
				if err := e.DoFile(path); err != nil {
					continue // 错误已进 LastError()，保留上一版界面
				}
				ui.RequestRedraw()
			}
		}
	}()
}
