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

// 电源状态（macOS）：解析 `pmset -g batt`。
//
//	Now drawing from 'Battery Power'
//	 -InternalBattery-0 (id=4653155)	85%; discharging; 4:12 remaining present: true
//
// 台式机会输出 "AC Power" 且没有 InternalBattery 行，此时返回「接交流电、电量未知」。

func powerCaps() []Cap {
	if _, err := exec.LookPath("pmset"); err == nil {
		return []Cap{CapPowerStatus}
	}
	return nil
}

func init() {
	powerStateNative = func() (PowerStatus, bool) {
		c := exec.Command("pmset", "-g", "batt")
		hideConsole(c)
		out, err := c.Output()
		if err != nil {
			return PowerStatus{}, false
		}
		s := string(out)
		st := PowerStatus{Percent: -1, ACOnline: true}
		if strings.Contains(s, "Battery Power") {
			st.OnBattery = true
			st.ACOnline = false
		}
		// 电量：第一处 "NN%"
		if i := strings.Index(s, "%"); i > 0 {
			j := i - 1
			for j >= 0 && s[j] >= '0' && s[j] <= '9' {
				j--
			}
			if n, err := strconv.Atoi(s[j+1 : i]); err == nil {
				st.Percent = n
			}
		}
		if strings.Contains(s, "charging") || strings.Contains(s, "AC attached") {
			st.Charging = true
		}
		return st, true
	}
}
