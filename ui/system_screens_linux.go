//go:build linux || freebsd

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

// 显示器枚举（X11）：解析 `xrandr --query` 的 connected 行。
//
//	xrandr 输出示例：
//	  DP-1 connected primary 2560x1440+0+0 (normal left inverted right x axis y axis) 597mm x 336mm
//	  HDMI-1 connected 1920x1080+2560+0 ...
//
// 不依赖 XRandR 扩展的协议细节，也不需要 cgo；xrandr 不在时返回空（业务侧用
// HasCap(CapMultiMonitor) 判断）。坐标是 X11 的根窗口坐标，与 ui.CaptureScreen
// 的坐标系一致。

func init() {
	monitorsNative = readXrandrMonitors
}

// readXrandrMonitors 解析 xrandr 输出。
func readXrandrMonitors() []Monitor {
	p, err := exec.LookPath("xrandr")
	if err != nil {
		return nil
	}
	c := exec.Command(p, "--query")
	hideConsole(c)
	out, err := c.Output()
	if err != nil {
		return nil
	}
	var ms []Monitor
	primarySeen := false
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "connected" {
			continue
		}
		name := fields[0]
		primary := false
		idx := 2
		if fields[idx] == "primary" {
			primary = true
			idx++
		}
		if idx >= len(fields) {
			continue
		}
		geom := fields[idx]
		// 形如 2560x1440+0+0 或 2560x1440（未定位时可忽略）
		at := strings.IndexByte(geom, '+')
		if at < 0 {
			continue
		}
		size := geom[:at]
		pos := geom[at+1:]
		w, h, ok := parseWxH(size)
		if !ok {
			continue
		}
		x, y, ok := parsePlusPos(pos)
		if !ok {
			continue
		}
		if primary {
			primarySeen = true
		}
		ms = append(ms, Monitor{
			Name: name, X: x, Y: y, W: w, H: h,
			// X11 没有「工作区」概念（面板由各自窗口管理器摆放），
			// 这里让工作区等于整屏，业务侧自行扣掉面板高度即可。
			WorkX: x, WorkY: y, WorkW: w, WorkH: h,
			Scale:   1,
			Primary: primary,
		})
	}
	if len(ms) == 0 {
		return nil
	}
	// 没有 primary 标记时把第一个当主屏（与 Windows 端「主屏在前」的约定一致）
	if !primarySeen {
		ms[0].Primary = true
	}
	// 主屏排最前
	for i := 1; i < len(ms); i++ {
		if ms[i].Primary {
			ms[0], ms[i] = ms[i], ms[0]
			break
		}
	}
	for i := range ms {
		ms[i].Index = i
	}
	return ms
}

// parsePlusPos 解析 "0+0" / "2560+0" 形式的位置。
func parsePlusPos(s string) (int, int, bool) {
	i := strings.IndexByte(s, '+')
	if i <= 0 {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(s[:i])
	y, err2 := strconv.Atoi(s[i+1:])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return x, y, true
}
