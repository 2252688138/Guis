//go:build darwin

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os/exec"
	"strconv"
	"strings"
)

// 显示器枚举（macOS）。
//
// 系统没有内置的多屏命令行工具，这里优先用 displayplacer（brew 可装），
// 其次用 AppleScript 取桌面总区域（只能给出并集，当作单屏使用）。
// 都拿不到时返回空，业务侧 HasCap(CapMultiMonitor) 依旧为假。

func init() {
	monitorsNative = readDarwinMonitors
}

// readDarwinMonitors 先试 displayplacer，再退回 AppleScript 桌面区域。
func readDarwinMonitors() []Monitor {
	if p, err := exec.LookPath("displayplacer"); err == nil {
		c := exec.Command(p, "list")
		hideConsole(c)
		if out, err := c.Output(); err == nil {
			if ms := parseDisplayPlacer(string(out)); len(ms) > 0 {
				return ms
			}
		}
	}
	// AppleScript：桌面窗口 bounds 给出所有屏幕的并集
	c := exec.Command("osascript", "-e", `tell application "Finder" to get bounds of window of desktop`)
	hideConsole(c)
	out, err := c.Output()
	if err != nil {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 4 {
		return nil
	}
	vals := make([]int, 4)
	for i, s := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil
		}
		vals[i] = n
	}
	// bounds 是 {left, top, right, bottom}
	w, h := vals[2]-vals[0], vals[3]-vals[1]
	if w <= 0 || h <= 0 {
		return nil
	}
	return []Monitor{{
		Index: 0, Name: "main", X: vals[0], Y: vals[1], W: w, H: h,
		WorkX: vals[0], WorkY: vals[1], WorkW: w, WorkH: h,
		Scale: 1, Primary: true,
	}}
}

// parseDisplayPlacer 解析 displayplacer list 的 "Resolution: 1920x1080" 与
// "Origin: (0,0)" 行（只取每个屏幕的第一组）。
func parseDisplayPlacer(s string) []Monitor {
	var ms []Monitor
	var cur Monitor
	has := false
	flush := func() {
		if has && cur.W > 0 && cur.H > 0 {
			ms = append(ms, cur)
		}
		cur = Monitor{Scale: 1}
		has = false
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Persistent screen id"):
			flush()
			has = true
		case strings.HasPrefix(line, "Resolution:"):
			f := strings.Fields(line)
			if len(f) >= 2 {
				if w, h, ok := parseWxH(f[1]); ok {
					cur.W, cur.H = w, h
				}
			}
		case strings.HasPrefix(line, "Origin:"):
			v := strings.TrimSpace(strings.TrimPrefix(line, "Origin:"))
			v = strings.Trim(v, "()")
			parts := strings.Split(v, ",")
			if len(parts) == 2 {
				x, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
				y, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				cur.X, cur.Y = x, y
			}
		case strings.Contains(line, "main display"):
			cur.Primary = true
		}
	}
	flush()
	for i := range ms {
		ms[i].Index = i
		ms[i].WorkX, ms[i].WorkY = ms[i].X, ms[i].Y
		ms[i].WorkW, ms[i].WorkH = ms[i].W, ms[i].H
	}
	return ms
}
