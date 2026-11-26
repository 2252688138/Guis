// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"guis/game"
)

// wavBytes 生成 16bit 单声道 PCM 的 WAV 字节。
func wavBytes(t *testing.T, rate int, samples []int16) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	dataLen := len(samples) * 2
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVEfmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(rate))
	binary.Write(buf, binary.LittleEndian, uint32(rate*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataLen))
	for _, s := range samples {
		binary.Write(buf, binary.LittleEndian, s)
	}
	return buf.Bytes()
}

func TestDecodeSoundWAV(t *testing.T) {
	samples := make([]int16, 800)
	for i := range samples {
		samples[i] = int16(i * 7)
	}
	b := wavBytes(t, 8000, samples)
	s, err := game.DecodeSound(b)
	if err != nil {
		t.Fatalf("解码 WAV 失败: %v", err)
	}
	if s.SampleRate != 8000 || s.Channels != 1 {
		t.Fatalf("格式 = %dHz/%dch", s.SampleRate, s.Channels)
	}
	if s.Frames() != len(samples) {
		t.Fatalf("帧数 = %d，期望 %d", s.Frames(), len(samples))
	}
	if got := s.Duration(); math.Abs(float64(got-0.1)) > 1e-4 {
		t.Fatalf("时长 = %v，期望 0.1", got)
	}
}

func TestDecodeSoundUnknown(t *testing.T) {
	if _, err := game.DecodeSound([]byte("not-audio-data")); err == nil {
		t.Fatal("无法识别的格式应返回错误")
	}
	if _, err := game.DecodeSound(nil); err == nil {
		t.Fatal("空数据应返回错误")
	}
}

func TestSpatialGain(t *testing.T) {
	if g := game.SpatialGain(0.5, 1, 10); g != 1 {
		t.Fatalf("最小距离内增益 = %v，期望 1", g)
	}
	if g := game.SpatialGain(20, 1, 10); g != 0 {
		t.Fatalf("超出最大距离增益 = %v，期望 0", g)
	}
	prev := float32(2)
	for d := float32(1); d <= 10; d += 0.5 {
		g := game.SpatialGain(d, 1, 10)
		if g > prev {
			t.Fatalf("增益应随距离单调递减: d=%v g=%v prev=%v", d, g, prev)
		}
		prev = g
	}
}

func TestMixerSpatialParams(t *testing.T) {
	m := game.NewMixer(48000)
	m.SetListener(game.Listener{
		Pos: game.Vec3{}, Forward: game.Vec3{Z: -1}, Up: game.Vec3{Y: 1},
		MinDist: 1, MaxDist: 20,
	})
	// 右方声源 → pan > 0
	pan, gain := m.SpatialParams(game.Vec3{X: 5})
	if pan <= 0 {
		t.Fatalf("右侧声源 pan = %v，期望 > 0", pan)
	}
	if gain >= 1 {
		t.Fatalf("5 米处增益 = %v，应已衰减", gain)
	}
	// 左方声源 → pan < 0
	if pan, _ := m.SpatialParams(game.Vec3{X: -5}); pan >= 0 {
		t.Fatalf("左侧声源 pan = %v，期望 < 0", pan)
	}
	// 正前方 → pan ≈ 0
	if pan, _ := m.SpatialParams(game.Vec3{Z: -5}); math.Abs(float64(pan)) > 1e-3 {
		t.Fatalf("正前方声源 pan = %v，期望 0", pan)
	}
	// 超出最大距离 → PlayAt 不创建声音
	snd := game.Tone(440, 0.05, 8000)
	if id := m.PlayAt(snd, game.Vec3{X: 50}, game.PlayOpt{}); id != 0 {
		t.Fatalf("超出距离仍返回 VoiceID=%v", id)
	}
	if id := m.PlayAt(snd, game.Vec3{X: 2}, game.PlayOpt{}); id == 0 {
		t.Fatal("范围内 PlayAt 应返回有效 VoiceID")
	}
}

func TestMusicStreamWAV(t *testing.T) {
	samples := make([]int16, 1600)
	for i := range samples {
		samples[i] = int16(i % 100)
	}
	b := wavBytes(t, 8000, samples)
	ms, err := game.MusicStreamFromBytes(b)
	if err != nil {
		t.Fatalf("打开内存流失败: %v", err)
	}
	defer ms.Close()
	if ms.Rate() != 8000 || ms.Channels() != 2 {
		t.Fatalf("流格式 = %dHz/%dch（WAV 单声道应补成 2ch）", ms.Rate(), ms.Channels())
	}
	var total int
	buf := make([]int16, 256)
	for {
		n, err := ms.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if n == 0 {
			break
		}
	}
	if total < len(samples)*2-4 {
		t.Fatalf("流读取样本数 = %d，期望约 %d", total, len(samples)*2)
	}
	if err := ms.SeekFrames(0); err != nil {
		t.Fatalf("定位失败: %v", err)
	}
	if n, _ := ms.Read(buf); n == 0 {
		t.Fatal("定位回首帧后应能继续读")
	}
}

func TestMusicStreamInvalid(t *testing.T) {
	if _, err := game.MusicStreamFromBytes([]byte("definitely-not-audio")); err == nil {
		t.Fatal("非音频数据应返回错误")
	}
}
