// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// guislua 用 Lua 脚本直接跑一个 guis 窗口，不需要写 Go 代码。
//
//	guislua ui.lua
//	guislua -f ui.lua -w 1000 -h 700 -title "我的脚本"
//	guislua ui.lua -- 参数1 参数2      # 脚本里用 arg[1] / arg[2] 取
package main

import (
	"flag"
	"fmt"
	"os"

	"guis/script"
	"guis/ui"
)

func main() {
	fs := flag.NewFlagSet("guislua", flag.ExitOnError)
	file := fs.String("f", "", "Lua 脚本路径（也可直接跟位置参数）")
	title := fs.String("title", "guis 脚本", "窗口标题")
	w := fs.Int("w", 900, "窗口宽度")
	h := fs.Int("h", 620, "窗口高度")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: guislua [-f] 脚本.lua [-w 900 -h 620 -title 标题] [-- 脚本参数...]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])

	path := *file
	args := fs.Args()
	if path == "" && len(args) > 0 {
		path, args = args[0], args[1:]
	}
	if path == "" {
		fs.Usage()
		os.Exit(2)
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintln(os.Stderr, "脚本不存在:", path)
		os.Exit(2)
	}

	opts := ui.Options{Title: *title, Width: *w, Height: *h}
	if err := script.RunFile(path, opts, script.Options{Args: args}); err != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", err)
		os.Exit(1)
	}
}
