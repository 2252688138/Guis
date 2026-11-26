// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Rumble 触发手柄震动（left/right 为 0~1 强度）；后端不支持时返回 false。
func (g *Gamepad) Rumble(left, right float32) bool {
	if g == nil || !g.Connected {
		return false
	}
	return padRumble(g.Index, left, right)
}
