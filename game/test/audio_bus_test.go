// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"math"
	"testing"

	"guis/game"
)

func mixPeak(m *game.Mixer, samples int) int32 {
	buf := make([]int16, samples)
	m.Mix(buf)
	var peak int32
	for _, v := range buf {
		if int32(v) > peak {
			peak = int32(v)
		} else if -int32(v) > peak {
			peak = -int32(v)
		}
	}
	return peak
}

func TestMixerBusVolume(t *testing.T) {
	m := game.NewMixer(8000)
	snd := game.Tone(440, 0.1, 8000)
	if id := m.Play(snd, game.PlayOpt{Bus: "sfx", Volume: 1, Loop: true}); id == 0 {
		t.Fatal("Play 返回无效 ID")
	}
	base := mixPeak(m, 1600)
	if base == 0 {
		t.Fatal("基线静音，无法验证总线音量")
	}

	m.SetBusVolume("sfx", 0.5)
	half := mixPeak(m, 1600)
	if ratio := float64(half) / float64(base); math.Abs(ratio-0.5) > 0.1 {
		t.Fatalf("总线 0.5 后幅度比 = %.2f，期望 ≈0.5（base=%d half=%d）", ratio, base, half)
	}

	// 另一条总线不受影响
	m.SetBusVolume("music", 1)
	if v := m.BusVolume("music"); v != 1 {
		t.Fatalf("music 总线音量 = %v，期望 1", v)
	}
	if v := m.BusVolume("未登记总线"); v != 1 {
		t.Fatalf("未登记总线应默认 1，得到 %v", v)
	}
}

func TestMixerBusDefaultIsSfx(t *testing.T) {
	m := game.NewMixer(8000)
	m.Play(game.Tone(440, 0.1, 8000), game.PlayOpt{Volume: 1, Loop: true})
	base := mixPeak(m, 1600)
	m.SetBusVolume("sfx", 0)
	if peak := mixPeak(m, 1600); peak != 0 {
		t.Fatalf("默认总线应为 sfx（静音后 peak=%d，base=%d）", peak, base)
	}
}

func TestMixerFadeMusic(t *testing.T) {
	m := game.NewMixer(8000)
	m.SetBusVolume("music", 1)
	m.FadeMusic(0, 0.5) // 0.5 s 内到 0
	// 推进 0.5s（4000 帧 × 2 = 8000 样本）
	m.Mix(make([]int16, 8000))
	if v := m.BusVolume("music"); v > 0.05 {
		t.Fatalf("渐变后 music 总线 = %v，期望 ≈0", v)
	}
	// 手动设值会覆盖渐变终点
	m.FadeMusic(1, 0)
	if v := m.BusVolume("music"); v != 1 {
		t.Fatalf("立即渐变失败，得到 %v", v)
	}
}

func TestPlayMusicStreamLoop(t *testing.T) {
	samples := make([]int16, 800) // 0.1s @ 8000Hz
	for i := range samples {
		samples[i] = int16(12000)
	}
	wav := wavBytes(t, 8000, samples)
	ms, err := game.MusicStreamFromBytes(wav)
	if err != nil {
		t.Fatalf("打开内存流失败: %v", err)
	}
	defer ms.Close()

	m := game.NewMixer(8000)
	if m.Rate() != 8000 {
		t.Fatalf("Mixer 采样率 = %d", m.Rate())
	}
	if id := m.PlayMusicStream(ms, game.PlayOpt{Loop: true, Volume: 1}); id == 0 {
		t.Fatal("PlayMusicStream 返回无效 ID")
	}
	if m.ActiveMusic() != 1 {
		t.Fatalf("ActiveMusic = %d，期望 1", m.ActiveMusic())
	}

	// 混 0.5 秒（4000 帧）→ WAV 只 0.1 秒，循环后应持续有声
	buf := make([]int16, 4000*2)
	m.Mix(buf)
	nonZero := 0
	for _, v := range buf {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero < 4000 {
		t.Fatalf("循环播放非零样本 = %d，期望接近 8000（5 个循环）", nonZero)
	}

	m.StopMusicStream(0)
	if m.ActiveMusic() != 0 {
		t.Fatalf("停止后 ActiveMusic = %d", m.ActiveMusic())
	}
}
