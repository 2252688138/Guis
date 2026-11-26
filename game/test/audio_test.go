// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"encoding/binary"
	"testing"

	"guis/game"
)

// makeWAV 构造 16bit PCM WAV 字节。
func makeWAV(samples []int16, channels, rate int) []byte {
	dataLen := len(samples) * 2
	b := make([]byte, 0, 44+dataLen)
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(36+dataLen))
	b = append(b, "WAVE"...)
	b = append(b, "fmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = binary.LittleEndian.AppendUint16(b, uint16(channels))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate*channels*2))
	b = binary.LittleEndian.AppendUint16(b, uint16(channels*2))
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(dataLen))
	for _, s := range samples {
		b = binary.LittleEndian.AppendUint16(b, uint16(s))
	}
	return b
}

func TestLoadWAVBytes(t *testing.T) {
	samples := []int16{0, 1000, -1000, 2000, -2000, 3000}
	s, err := game.LoadWAVBytes(makeWAV(samples, 2, 22050))
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if s.SampleRate != 22050 || s.Channels != 2 {
		t.Fatalf("格式 = %dHz %dch", s.SampleRate, s.Channels)
	}
	if s.Frames() != 3 {
		t.Fatalf("帧数 = %d，期望 3", s.Frames())
	}
	if d := s.Duration(); d < 0.0001 || d > 0.001 {
		t.Fatalf("时长 = %v", d)
	}
	if s.Data[1] != 1000 || s.Data[2] != -1000 {
		t.Fatalf("样本解码错误: %v", s.Data[:3])
	}
}

func TestLoadWAVBytesBad(t *testing.T) {
	if _, err := game.LoadWAVBytes([]byte("not a wav")); err != game.ErrBadWAV {
		t.Fatalf("非 WAV 应返回 ErrBadWAV，实际 %v", err)
	}
}

func TestTone(t *testing.T) {
	s := game.Tone(440, 0.1, 44100)
	if s.Frames() != 4410 {
		t.Fatalf("帧数 = %d，期望 4410", s.Frames())
	}
	nonZero := 0
	for _, v := range s.Data {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < len(s.Data)/2 {
		t.Fatal("音调应有大量非零样本")
	}
}

func TestMixerPlayStop(t *testing.T) {
	m := game.NewMixer(48000)
	s := game.Tone(440, 0.05, 48000)
	id := m.Play(s, game.PlayOpt{Volume: 1})
	if id == 0 || m.ActiveVoices() != 1 {
		t.Fatalf("播放实例 = %d，活跃 = %d", id, m.ActiveVoices())
	}
	buf := make([]int16, 480)
	m.Mix(buf)
	peak := int16(0)
	for _, v := range buf {
		if v > peak {
			peak = v
		}
	}
	if peak == 0 {
		t.Fatal("混音输出应非静音")
	}
	m.Stop(id)
	if m.ActiveVoices() != 0 {
		t.Fatalf("停止后活跃 = %d", m.ActiveVoices())
	}
	buf2 := make([]int16, 64)
	m.Mix(buf2)
	for _, v := range buf2 {
		if v != 0 {
			t.Fatal("停止后应静音")
		}
	}
}

func TestMixerNoClipAndLoop(t *testing.T) {
	m := game.NewMixer(48000)
	s := game.Tone(220, 0.01, 48000) // 480 帧
	for i := 0; i < 8; i++ {
		m.Play(s, game.PlayOpt{Volume: 1, Loop: true})
	}
	buf := make([]int16, 4800)
	m.Mix(buf)
	// 8 个声部叠加必然超过 16bit 范围：应饱和到极值而不是溢出翻转
	max, min := int16(-32768), int16(32767)
	for _, v := range buf {
		if v > max {
			max = v
		}
		if v < min {
			min = v
		}
	}
	if max != 32767 || min != -32768 {
		t.Fatalf("多声部叠加应饱和到极值，实际 [%d, %d]", min, max)
	}
	// 循环播放应保持活跃
	if m.ActiveVoices() != 8 {
		t.Fatalf("循环播放活跃数 = %d，期望 8", m.ActiveVoices())
	}
}

func TestMixerNonLoopFinishes(t *testing.T) {
	m := game.NewMixer(48000)
	s := game.Tone(220, 0.01, 48000)
	m.Play(s, game.PlayOpt{})
	buf := make([]int16, 4800)
	m.Mix(buf)
	if m.ActiveVoices() != 0 {
		t.Fatalf("非循环播完应回收，剩余 %d", m.ActiveVoices())
	}
}

func TestMixerSingleVoiceNoClip(t *testing.T) {
	m := game.NewMixer(48000)
	m.Play(game.Tone(440, 0.05, 48000), game.PlayOpt{Volume: 1})
	buf := make([]int16, 2400)
	m.Mix(buf)
	for _, v := range buf {
		if v >= 32767 || v <= -32768 {
			t.Fatal("单声部不应削顶")
		}
	}
}

func TestMixerPan(t *testing.T) {
	m := game.NewMixer(48000)
	s := game.Tone(440, 0.02, 48000)
	m.Play(s, game.PlayOpt{Pan: -1}) // 全左
	buf := make([]int16, 200)
	m.Mix(buf)
	var l, r int
	for i := 0; i < len(buf); i += 2 {
		l += int(buf[i] & 0x7fff)
		r += int(buf[i+1] & 0x7fff)
	}
	if l == 0 || r != 0 {
		t.Fatalf("全左声像左右能量 = %d/%d", l, r)
	}
}

func TestOpenPlayerFallback(t *testing.T) {
	p := game.OpenPlayer()
	if p == nil {
		t.Fatal("OpenPlayer 不应返回 nil")
	}
	defer p.Close()
	p.SetMasterVolume(0.5)
}
