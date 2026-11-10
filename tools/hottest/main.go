package main

import (
	"fmt"
	"os"

	"guis/ui"
)

// version 改这一行即可验证热重载（第 2 次运行应打印 v2 并恢复上次状态）。
var version = "v1"

func init() {
	ui.SetEntry(func() (ui.Options, func() *ui.Node) {
		opts := ui.Options{Title: "guis hot reload test", Width: 420, Height: 260, Hidden: true}
		return opts, func() *ui.Node {
			return ui.Box(ui.Text("hot reload " + version))
		}
	})
	ui.SetHotState(
		func() []byte { return []byte(version) },
		func(b []byte) { fmt.Fprintf(os.Stderr, "[hotest] 恢复状态: %s\n", string(b)) },
	)
}

func main() {
	fmt.Fprintf(os.Stderr, "[hotest] 启动 %s hot=%v pid=%d\n", version, ui.HotReloadActive(), os.Getpid())
	ui.Main()
}
