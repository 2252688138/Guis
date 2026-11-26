// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// 本文件为 game/test 外部测试包（package game_test）提供必要的桥接：
// 外部测试包只能访问导出标识符，而摇杆死区、手柄位映射属于内部实现细节，
// 故以 Test 前缀导出，仅供测试使用，不属于稳定公开 API。

// TestApplyDeadzone 暴露摇杆死区计算逻辑（测试用）。
func TestApplyDeadzone(v, dz float32) float32 { return applyDeadzone(v, dz) }

// TestPadButtonBits 暴露手柄按钮到 XInput 位掩码的映射（测试用，只读）。
func TestPadButtonBits() map[PadButton]uint16 { return padButtonBits }

// TestRendererBufferSize 返回渲染器内部帧缓冲尺寸（测试用，受 Supersample 影响）。
func TestRendererBufferSize(r *Renderer) (int, int) {
	if r == nil || r.fb == nil {
		return 0, 0
	}
	return r.fb.w, r.fb.h
}

// TestSceneManagerSetFade 直接设置场景切换的遮罩强度（测试用，0 透明 1 全遮）。
func TestSceneManagerSetFade(m *SceneManager, v float32) {
	if m != nil {
		m.fade = v
	}
}

// TestGamepadSetState 注入手柄当前帧/上一帧的按钮状态（测试用）。
func TestGamepadSetState(g *Gamepad, cur, prev map[PadButton]bool) {
	if g == nil {
		return
	}
	g.cur, g.prev = cur, prev
}

// TestGamepadPrev 暴露手柄上一帧按钮状态表（测试用，可写）。
func TestGamepadPrev(g *Gamepad) map[PadButton]bool {
	if g == nil {
		return nil
	}
	return g.prev
}

// TestAudioCmdArgs 暴露外部命令后端的命令模板解析（测试用）。
func TestAudioCmdArgs(tmpl string, rate int) (string, []string, bool) {
	return audioCmdArgs(tmpl, rate)
}

// TestAudioDefaultCommands 返回当前平台的默认外部播放命令候选（测试用）。
func TestAudioDefaultCommands() []string { return audioDefaultCommands() }
