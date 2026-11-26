// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build !windows

package game

// padPoll 非 Windows 平台暂无手柄后端，恒返回未连接。
func padPoll(idx int) (padState, bool) { return padState{}, false }

func padRumble(idx int, left, right float32) bool { return false }
