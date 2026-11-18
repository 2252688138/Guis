// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// uidemo 演示 uidsl：界面写在 demo.json / demo.dsl 里，改文件即时刷新界面，
// 不重新编译、不重启进程。
//
//	go run ./cmd/uidemo                  → 读 cmd/uidemo/demo.json
//	go run ./cmd/uidemo -f demo.dsl      → 读 DSL 版（同一份界面）
//	go run ./cmd/uidemo -f 任意路径
//
// 事件回调仍然写在 Go 里（uidsl.Actions）：描述文件负责「长什么样」，
// Go 负责「点下去做什么」，两者可以各自改各自的。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"guis/ui"
	"guis/ui/uidsl"
)

var (
	live    *uidsl.Live
	accent  = "#6366f1"
	status  = "改 cmd/uidemo/demo.json，保存即生效"
	touched time.Time
)

func main() {
	path := pickFile()
	l, err := uidsl.Open(path, uidsl.Actions{
		"say-hi": func() {
			ui.ToastShow("按钮回调在 Go 里：uidsl.Actions[\"say-hi\"]")
		},
		"accent": func() {
			if accent == "#6366f1" {
				accent = "#22c55e"
			} else {
				accent = "#6366f1"
			}
			ui.RequestRedraw()
		},
		"reload": func() {
			if err := live.Reload(); err != nil {
				status = "重载失败：" + err.Error()
			}
			ui.RequestRedraw()
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
	defer l.Close()
	live = l
	l.OnLoad(func(uidsl.Node) {
		status = "已重新加载 " + time.Now().Format("15:04:05")
		touched = time.Now()
	})

	ui.SetEntry(func() (ui.Options, func() *ui.Node) {
		return ui.Options{
			Title:        "uidsl · 改文件即时刷新",
			Width:        920,
			Height:       660,
			CornerRadius: 8,
		}, view
	})
	ui.Main()
}

func view() *ui.Node {
	head := ui.Row(
		ui.Icon("palette").St(ui.St().Text(0, 0, accent)),
		ui.Text("uidsl 实时界面").St(ui.St().Text(15, 700, "#e6edf7")),
		ui.Spacer(),
		ui.Text(filepath.Base(live.Path())).St(ui.St().Text(12, 400, "#8b97ab")),
	).St(ui.St().PadXY(16, 12).GapOf(10).BgC("#141b2b"))

	body := ui.Box(live.Root()).St(ui.St().GrowTo(1).Padding(18).BgC("#0f1626"))

	// 底部状态条：解析成功显示时间，失败显示错误（界面保持上一版，不会白）。
	footIcon := ui.Icon("check")
	footColor := "#22c55e"
	msg := status
	if err := live.LastError(); err != nil {
		footIcon = ui.Icon("info")
		footColor = "#f97316"
		msg = err.Error()
	}
	foot := ui.Row(
		footIcon.St(ui.St().Text(0, 0, footColor)),
		ui.Text(msg).St(ui.St().Text(12, 400, "#8b97ab")),
	).St(ui.St().PadXY(16, 10).GapOf(8).BgC("#111826"))

	return ui.Column(head, body, foot).St(ui.St().BgC("#0f1626"))
}

// pickFile 找描述文件：-f 指定 → 当前目录 → 可执行文件同目录 → cmd/uidemo。
func pickFile() string {
	name := "demo.json"
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		if (a == "-f" || a == "--file") && i+1 < len(os.Args) {
			name = os.Args[i+1]
			i++
		} else if strings.HasPrefix(a, "-f=") || strings.HasPrefix(a, "--file=") {
			name = a[strings.IndexByte(a, '=')+1:]
		}
	}
	if filepath.IsAbs(name) {
		return name
	}
	candidates := []string{name, filepath.Join("cmd", "uidemo", name)}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return candidates[0]
}
