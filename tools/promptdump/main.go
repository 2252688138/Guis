// guis tools/promptdump — 打印当前系统提示词，用来核对 rg.exe / aria2c.exe 的路径对不对。
//
//	go run ./tools/promptdump -app "C:\Users\Administrator\AppData\Local\Programs\DeepSeek Harness" -ws "H:\JIMaX\gogui"
//
// 只输出提示词，不发起任何模型请求。
package main

import (
	"flag"
	"fmt"

	"guis/cmd/deepseek/agent"
)

func main() {
	app := flag.String("app", "", "程序目录（exe 所在目录，用来探测 rg.exe / aria2c.exe）")
	ws := flag.String("ws", "", "工作区目录")
	flag.Parse()

	p := agent.SystemPrompt(agent.PromptInput{
		Workspace: *ws,
		AppDir:    *app,
	})
	fmt.Println(p)
}
