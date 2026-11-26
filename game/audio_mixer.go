// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"math"
	"sync"
)

// voice 混音实例。
type voice struct {
	id    VoiceID
	s     *Sound
	pos   float64
	vol   float32
	pan   float32
	pitch float32
	bus   string
	loop  bool
	alive bool
}

// Mixer 软件混音器（输出 16bit 立体声交错样本，并发安全）。
type Mixer struct {
	mu       sync.Mutex
	rate     int
	master   float32
	voices   []voice
	next     VoiceID
	listener Listener
	// buses 总线音量（sfx / music / voice …；未设置的总线按 1）
	buses map[string]float32
	// music 流式音乐声部（不占 voices，独立拉流）
	music []*musicVoice
	// fades 总线音量渐变队列
	fades []busFade
}

// NewMixer 创建混音器（rate ≤ 0 时按 48000）。
func NewMixer(rate int) *Mixer {
	if rate <= 0 {
		rate = 48000
	}
	return &Mixer{rate: rate, master: 1}
}

// Rate 输出采样率。
func (m *Mixer) Rate() int { return m.rate }

// Play 开始播放，返回实例编号。
func (m *Mixer) Play(s *Sound, opt PlayOpt) VoiceID {
	if m == nil || s == nil || len(s.Data) == 0 || s.SampleRate <= 0 {
		return 0
	}
	if opt.Volume <= 0 {
		opt.Volume = 1
	}
	if opt.Pitch <= 0 {
		opt.Pitch = 1
	}
	if opt.Pan < -1 {
		opt.Pan = -1
	}
	if opt.Pan > 1 {
		opt.Pan = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	id := m.next
	bus := opt.Bus
	if bus == "" {
		bus = "sfx"
	}
	m.voices = append(m.voices, voice{
		id: id, s: s, vol: opt.Volume, pan: opt.Pan, pitch: opt.Pitch, bus: bus, loop: opt.Loop, alive: true,
	})
	return id
}

// Stop 停止实例。
func (m *Mixer) Stop(id VoiceID) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.voices {
		if m.voices[i].id == id {
			m.voices[i].alive = false
		}
	}
}

// SetVolume 调整实例音量。
func (m *Mixer) SetVolume(id VoiceID, v float32) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.voices {
		if m.voices[i].id == id {
			m.voices[i].vol = v
		}
	}
}

// SetMasterVolume 调整总音量。
func (m *Mixer) SetMasterVolume(v float32) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.master = v
}

// ActiveVoices 活跃实例数。
func (m *Mixer) ActiveVoices() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for i := range m.voices {
		if m.voices[i].alive {
			n++
		}
	}
	return n
}

// Mix 填充 dst（16bit 立体声交错），不足部分补静音。
func (m *Mixer) Mix(dst []int16) {
	for i := range dst {
		dst[i] = 0
	}
	if m == nil || len(dst) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.master <= 0 {
		m.master = 1
	}
	frames := len(dst) / 2
	m.mixMusicLocked(dst, frames)
	for vi := range m.voices {
		v := &m.voices[vi]
		if !v.alive || v.s == nil {
			continue
		}
		src := v.s
		step := float64(src.SampleRate) / float64(m.rate) * float64(v.pitch)
		total := src.Frames()
		if total <= 0 {
			v.alive = false
			continue
		}
		// 声像：-1 全左、0 居中、+1 全右；再乘上所属总线音量
		bus := m.busGain(v.bus)
		lg := v.vol * m.master * bus * min32(1, 1-v.pan)
		rg := v.vol * m.master * bus * min32(1, 1+v.pan)
		for f := 0; f < frames; f++ {
			idx := int(v.pos)
			if idx >= total {
				if v.loop {
					v.pos -= float64(total)
					idx = int(v.pos)
				} else {
					v.alive = false
					break
				}
			}
			var l, r int16
			base := idx * src.Channels
			if src.Channels == 1 {
				l, r = src.Data[base], src.Data[base]
			} else {
				l = src.Data[base]
				r = src.Data[base+1]
			}
			acc := int32(dst[f*2]) + int32(float32(l)*lg)
			dst[f*2] = clampI16(acc)
			acc = int32(dst[f*2+1]) + int32(float32(r)*rg)
			dst[f*2+1] = clampI16(acc)
			v.pos += step
		}
	}
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func clampI16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

// OpenPlayer 打开系统音频设备（Windows 用 waveOut；其它平台或无设备时返回 NopPlayer）。
// musicVoice 流式音乐声部（从 MusicStream 拉样本，支持重采样 / 循环 / 总线音量）。
type musicVoice struct {
	id     VoiceID
	stream *MusicStream
	vol    float32
	bus    string
	loop   bool
	alive  bool
	phase  float64 // 源帧位置（浮点，重采样用）
	base   int64   // buf[0] 对应的源帧号
	buf    []int16 // 交错样本缓存
}

// busFade 总线音量渐变状态（按帧推进，避免开 goroutine）。
type busFade struct {
	bus           string
	from, to      float32
	remain, total float32
}

// busGain 读总线增益（调用方必须已持锁；master 与未登记总线按 1）。
func (m *Mixer) busGain(bus string) float32 {
	if m == nil || bus == "" || bus == "master" {
		return 1
	}
	if v, ok := m.buses[bus]; ok {
		if v < 0 {
			return 0
		}
		return v
	}
	return 1
}

// SetBusVolume 设置音量总线（"sfx" / "music" / "voice" …，1 = 原音量）。
func (m *Mixer) SetBusVolume(bus string, v float32) {
	if m == nil || bus == "" || bus == "master" {
		return
	}
	if v < 0 {
		v = 0
	}
	m.mu.Lock()
	if m.buses == nil {
		m.buses = map[string]float32{}
	}
	m.buses[bus] = v
	m.mu.Unlock()
}

// BusVolume 读总线音量（未设置返回 1）。
func (m *Mixer) BusVolume(bus string) float32 {
	if m == nil {
		return 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.busGain(bus)
}

// FadeMusic 在 seconds 秒内把音乐总线音量渐变到 to（≤ 0 立即生效）。
func (m *Mixer) FadeMusic(to float32, seconds float32) {
	if m == nil {
		return
	}
	if to < 0 {
		to = 0
	}
	m.mu.Lock()
	if m.buses == nil {
		m.buses = map[string]float32{}
	}
	from := m.busGain("music")
	if seconds <= 0 || from == to {
		m.buses["music"] = to
		m.mu.Unlock()
		return
	}
	m.fades = append(m.fades, busFade{bus: "music", from: from, to: to, remain: seconds, total: seconds})
	m.mu.Unlock()
}

// FadeBus 任意总线渐变（voice / sfx 等）。
func (m *Mixer) FadeBus(bus string, to float32, seconds float32) {
	if m == nil || bus == "" || bus == "master" {
		return
	}
	if to < 0 {
		to = 0
	}
	m.mu.Lock()
	if m.buses == nil {
		m.buses = map[string]float32{}
	}
	from := m.busGain(bus)
	if seconds <= 0 || from == to {
		m.buses[bus] = to
		m.mu.Unlock()
		return
	}
	m.fades = append(m.fades, busFade{bus: bus, from: from, to: to, remain: seconds, total: seconds})
	m.mu.Unlock()
}

// updateFadesLocked 按本次混音的帧数推进总线渐变。
func (m *Mixer) updateFadesLocked(frames int) {
	if len(m.fades) == 0 || m.rate <= 0 {
		return
	}
	dt := float32(frames) / float32(m.rate)
	for i := 0; i < len(m.fades); i++ {
		f := &m.fades[i]
		f.remain -= dt
		if f.remain <= 0 || f.total <= 0 {
			m.buses[f.bus] = f.to
			m.fades = append(m.fades[:i], m.fades[i+1:]...)
			i--
			continue
		}
		t := 1 - f.remain/f.total
		m.buses[f.bus] = f.from + (f.to-f.from)*t
	}
}

// PlayMusicStream 播放流式音乐（独立于 Play 的一次性音效，可多条并行）。
func (m *Mixer) PlayMusicStream(ms *MusicStream, opt PlayOpt) VoiceID {
	if m == nil || ms == nil {
		return 0
	}
	bus := opt.Bus
	if bus == "" {
		bus = "music"
	}
	vol := opt.Volume
	if vol <= 0 {
		vol = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	id := m.next
	m.music = append(m.music, &musicVoice{
		id: id, stream: ms, vol: vol, bus: bus, loop: opt.Loop, alive: true,
		buf: make([]int16, 8192), base: -1, // base=-1 强制首次读缓冲
	})
	return id
}

// StopMusicStream 停止流式音乐（id = 0 时停全部）。
func (m *Mixer) StopMusicStream(id VoiceID) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.music[:0]
	for _, mv := range m.music {
		if id != 0 && mv.id != id {
			kept = append(kept, mv)
			continue
		}
		mv.alive = false
	}
	m.music = kept
}

// ActiveMusic 当前存活音乐声部数。
func (m *Mixer) ActiveMusic() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, mv := range m.music {
		if mv.alive {
			n++
		}
	}
	return n
}

// mixMusicLocked 把流式音乐混入 dst（调用方必须已持锁）。
func (m *Mixer) mixMusicLocked(dst []int16, frames int) {
	m.updateFadesLocked(frames)
	if len(m.music) == 0 {
		return
	}
	for _, mv := range m.music {
		if !mv.alive || mv.stream == nil {
			continue
		}
		sr := mv.stream.Rate()
		ch := mv.stream.Channels()
		if sr <= 0 || ch <= 0 || sr == 0 {
			mv.alive = false
			continue
		}
		gain := mv.vol * m.master * m.busGain(mv.bus)
		if gain <= 0 {
			continue
		}
		step := float64(sr) / float64(m.rate)
		if step <= 0 {
			step = 1
		}
		total := float64(mv.stream.Frames())
		for f := 0; f < frames; f++ {
			if total > 0 && mv.phase >= total {
				if mv.loop {
					mv.phase = math.Mod(mv.phase, total)
					mv.base = -1 // 强制重新定位
				} else {
					mv.alive = false
					break
				}
			}
			if !m.musicEnsureLocked(mv, int64(mv.phase)) {
				break
			}
			i := int(int64(mv.phase)-mv.base) * ch
			if i < 0 || i >= len(mv.buf) || (ch > 1 && i+1 >= len(mv.buf)) {
				mv.alive = false
				break
			}
			var l, r int16
			if ch == 1 {
				l = mv.buf[i]
				r = l
			} else {
				l = mv.buf[i]
				r = mv.buf[i+1]
			}
			dst[2*f] = clampI16(int32(dst[2*f]) + int32(float32(l)*gain))
			dst[2*f+1] = clampI16(int32(dst[2*f+1]) + int32(float32(r)*gain))
			mv.phase += step
		}
	}
}

// musicEnsureLocked 确保缓存覆盖绝对帧 abs；必要时重新定位并读一块。
func (m *Mixer) musicEnsureLocked(mv *musicVoice, abs int64) bool {
	ch := mv.stream.Channels()
	if ch <= 0 {
		return false
	}
	if len(mv.buf) == 0 {
		mv.buf = make([]int16, 8192)
	}
	held := int64(len(mv.buf) / ch)
	if mv.base >= 0 && abs >= mv.base && abs < mv.base+held {
		return true
	}
	_ = mv.stream.SeekFrames(int(abs)) // 不可定位的流忽略错误，继续顺序读
	n, _ := mv.stream.Read(mv.buf)
	mv.base = abs
	if n <= 0 {
		if mv.loop {
			_ = mv.stream.SeekFrames(0)
			mv.base = 0
			mv.phase = 0
			n, _ = mv.stream.Read(mv.buf)
			if n <= 0 {
				mv.alive = false
				return false
			}
			return true
		}
		mv.alive = false
		return false
	}
	return true
}

func OpenPlayer() Player { return playerOpen(48000) }
