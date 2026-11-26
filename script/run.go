// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 启动入口：一行跑一个脚本文文件，或把引擎挂到已有应用上。

import (
	"errors"
	"path/filepath"
	"time"

	"guis/ui"
)

// RunFile 用脚本启动应用（阻塞到窗口关闭）。
//
//	script.RunFile("ui.lua", ui.Options{Title: "脚本", Width: 900, Height: 620}, script.Options{})
func RunFile(path string, opts ui.Options, so Options) error {
	e, err := Attach(path, so)
	if err != nil {
		return err
	}
	defer e.Close()
	err = ui.Run(opts, e.Root)
	e.shutdown() // 先跑脚本的 on_exit()，再走宿主钩子
	return err
}

// Attach 装载脚本并（可选）开启热重载，返回可传给 SetEntry 的引擎。
//
//	e, err := script.Attach("ui.lua", script.Options{})
//	if err != nil { log.Fatal(err) }
//	defer e.Close()
//	ui.SetEntry(func() (ui.Options, func() *ui.Node) {
//	  return ui.Options{Title: "脚本"}, e.Root
//	})
func Attach(path string, so Options) (*Engine, error) {
	if path == "" {
		return nil, errors.New("script: 脚本路径为空")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if so.WorkDir == "" {
		so.WorkDir = filepath.Dir(abs)
	}
	e, err := New(so)
	if err != nil {
		return nil, err
	}
	e.SetArgs(append([]string{abs}, so.Args...))
	if err := e.DoFile(abs); err != nil {
		e.Close()
		return nil, err
	}
	e.startWatch()
	return e, nil
}

// DefaultInterval 默认热重载轮询间隔。
const DefaultInterval = 200 * time.Millisecond
