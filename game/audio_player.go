// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// 播放器的可选能力：平台后端与 Mixer 按需实现，调用方用类型断言探测，
// 不实现时保持兼容（NopPlayer 只满足 Player）。

// PlayerStatus 后端自述状态。
type PlayerStatus struct {
	// Backend 后端名（"winmm" / "pulse" / "alsa" / "webaudio" / "nop" …）。
	Backend string
	// Rate 实际输出采样率。
	Rate int
	// OK 是否真的能出声（false = 静音回退）。
	OK bool
}

// BusControllable 按音量总线控制（sfx / music / voice）。
type BusControllable interface {
	SetBusVolume(bus string, v float32)
	BusVolume(bus string) float32
}

// MusicFader 音乐淡入淡出。
type MusicFader interface {
	FadeMusic(to float32, seconds float32)
}

// StatusReporter 报告后端状态（用于诊断「为什么没声音」）。
type StatusReporter interface {
	Status() PlayerStatus
}

// Status NopPlayer 的实现：始终静音。
func (NopPlayer) Status() PlayerStatus {
	return PlayerStatus{Backend: "nop", Rate: 0, OK: false}
}

// 编译期断言：Mixer 本身就是可用的播放器与总线控制器。
var (
	_ Player          = (*Mixer)(nil)
	_ BusControllable = (*Mixer)(nil)
	_ MusicFader      = (*Mixer)(nil)
)

// Play 实现 Player：把单次音效混入 dst 的语义下，Mixer.Play 返回声部编号。
// （Mixer 已通过 Play/Stop/SetVolume/SetMasterVolume/ActiveVoices/Close 满足 Player，
// 这里的 Close 只停掉全部声部，不关闭底层设备。）
func (m *Mixer) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	for i := range m.voices {
		m.voices[i].alive = false
	}
	for _, mv := range m.music {
		mv.alive = false
	}
	m.voices = nil
	m.music = nil
	m.mu.Unlock()
}
