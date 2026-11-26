// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

// Audio 音效后端接口（按名字播放，对接业务自己的资源表）。
//
// 需要直接播 PCM 数据时用 Player（见 OpenPlayer / Mixer）。
type Audio interface {
	// PlaySound 播放短音效（可叠加）。
	PlaySound(name string, volume float32)
	// PlayMusic 播放背景音乐（循环）。
	PlayMusic(name string)
	// StopMusic 停止背景音乐。
	StopMusic()
}

// NopAudio 空实现（不会报错，方便先跑通画面）。
type NopAudio struct{}

// PlaySound 空实现。
func (NopAudio) PlaySound(string, float32) {}

// PlayMusic 空实现。
func (NopAudio) PlayMusic(string) {}

// StopMusic 空实现。
func (NopAudio) StopMusic() {}

// VoiceID 播放实例编号（0 表示无效）。
type VoiceID int

// PlayOpt 播放参数。
type PlayOpt struct {
	Loop   bool
	Volume float32 // 0~1，0 时按 1
	Pitch  float32 // 音高倍率（1 = 原调）
	Pan    float32 // -1 左 / 0 中 / 1 右
	// Bus 音量总线（"sfx"|"music"|"voice"|"master"），默认 "sfx"。
	Bus string
}

// Sound 解码后的 PCM 音频（16bit 交错样本）。
type Sound struct {
	Data       []int16
	SampleRate int
	Channels   int
}

// Frames 帧数（每帧含 Channels 个样本）。
func (s *Sound) Frames() int {
	if s == nil || s.Channels <= 0 {
		return 0
	}
	return len(s.Data) / s.Channels
}

// Duration 时长（秒）。
func (s *Sound) Duration() float32 {
	if s == nil || s.SampleRate <= 0 {
		return 0
	}
	return float32(s.Frames()) / float32(s.SampleRate)
}

// Player 低层播放器（直接播 Sound，支持多实例与音量/声像控制）。
type Player interface {
	// Play 开始播放，返回实例编号（失败返回 0）。
	Play(s *Sound, opt PlayOpt) VoiceID
	// Stop 停止指定实例。
	Stop(id VoiceID)
	// SetVolume 调整实例音量。
	SetVolume(id VoiceID, v float32)
	// SetMasterVolume 调整总音量。
	SetMasterVolume(v float32)
	// ActiveVoices 当前活跃实例数。
	ActiveVoices() int
	// Close 释放设备。
	Close()
}

// NopPlayer 空播放器（无设备时降级使用）。
type NopPlayer struct{}

// Play 空实现。
func (NopPlayer) Play(s *Sound, opt PlayOpt) VoiceID { return 0 }

// Stop 空实现。
func (NopPlayer) Stop(id VoiceID) {}

// SetVolume 空实现。
func (NopPlayer) SetVolume(id VoiceID, v float32) {}

// SetMasterVolume 空实现。
func (NopPlayer) SetMasterVolume(v float32) {}

// ActiveVoices 空实现。
func (NopPlayer) ActiveVoices() int { return 0 }

// Close 空实现。
func (NopPlayer) Close() {}
