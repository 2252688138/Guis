//go:build linux || freebsd

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 电源状态（Linux / BSD）：读 /sys/class/power_supply。
//
// 台式机没有 BAT* 目录，此时返回「接交流电、电量未知」，接口语义保持一致。

func powerCaps() []Cap {
	// 有 sysfs 就认为可读；容器里可能没有，PowerState 会自然回退
	if _, err := os.Stat("/sys/class/power_supply"); err == nil {
		return []Cap{CapPowerStatus}
	}
	return nil
}

func init() {
	powerStateNative = func() (PowerStatus, bool) {
		entries, err := os.ReadDir("/sys/class/power_supply")
		if err != nil {
			return PowerStatus{}, false
		}
		st := PowerStatus{Percent: -1, ACOnline: true}
		found := false
		for _, e := range entries {
			name := e.Name()
			base := filepath.Join("/sys/class/power_supply", name)
			switch {
			case strings.HasPrefix(name, "BAT"):
				if p, err := readIntFile(filepath.Join(base, "capacity")); err == nil {
					st.Percent = p
					found = true
				}
				if s, err := os.ReadFile(filepath.Join(base, "status")); err == nil {
					switch strings.TrimSpace(string(s)) {
					case "Charging":
						st.Charging = true
						st.ACOnline = true
					case "Discharging":
						st.OnBattery = true
						st.ACOnline = false
					case "Full":
						st.ACOnline = true
					}
				}
			case strings.HasPrefix(name, "AC"), strings.HasPrefix(name, "ADP"):
				if v, err := readIntFile(filepath.Join(base, "online")); err == nil {
					st.ACOnline = v != 0
					if !st.ACOnline {
						st.OnBattery = true
					}
					found = true
				}
			}
		}
		return st, found
	}
}

// readIntFile 读取单值文本文件（sysfs 里的数字）。
func readIntFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
