//go:build darwin && !ios

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"strings"
)

// init 挂载 macOS 系统语言探测。
//
// Go 在 darwin 上没有直接读「系统首选语言」的标准库接口，这里用系统自带的
// defaults 命令读 AppleLocale（如 "zh_CN"）/ AppleLanguages（plist 数组）。
// 只读不写、失败静默（容器 / 受限环境里没有 defaults 时退回环境变量）。
func init() {
	systemLangProbe = func() []string {
		out := make([]string, 0, 3)
		if raw, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
			if v := envLocaleToTag(strings.TrimSpace(string(raw))); v != "" {
				out = append(out, v)
			}
		}
		if raw, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				line = strings.TrimSpace(line)
				line = strings.Trim(line, "\",(); ")
				if line == "" || line == "AppleLanguages" || strings.HasPrefix(line, "(") {
					continue
				}
				if v := envLocaleToTag(line); v != "" {
					out = append(out, v)
				}
			}
		}
		return out
	}
}
