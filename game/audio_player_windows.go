// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build windows

package game

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// waveFormatEx WAVEFORMATEX（18 字节）。
type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

// waveHdr WAVEHDR（64 位下 48 字节）。
type waveHdr struct {
	Data      unsafe.Pointer
	BufferLen uint32
	BytesRec  uint32
	User      uintptr
	Flags     uint32
	Loops     uint32
	Next      unsafe.Pointer
	Reserved  uintptr
}

const (
	wdrDone     = 0x00000001
	wdrPrepared = 0x00000002
)

var (
	winmm            = syscall.NewLazyDLL("winmm.dll")
	waveOutOpen      = winmm.NewProc("waveOutOpen")
	waveOutWrite     = winmm.NewProc("waveOutWrite")
	waveOutReset     = winmm.NewProc("waveOutReset")
	waveOutClose     = winmm.NewProc("waveOutClose")
	waveOutPrepare   = winmm.NewProc("waveOutPrepareHeader")
	waveOutUnprepare = winmm.NewProc("waveOutUnprepareHeader")
)

// wavePlayer waveOut 双缓冲播放器（每缓冲 2048 帧）。
type wavePlayer struct {
	mu     sync.Mutex
	mix    *Mixer
	hwo    uintptr
	hdr    [2]waveHdr
	buf    [2][]int16
	closed bool
	done   chan struct{}
}

// SetBusVolume 转发到内部 Mixer：总线音量（sfx / music / voice）。
func (p *wavePlayer) SetBusVolume(bus string, v float32) {
	if p == nil {
		return
	}
	p.mix.SetBusVolume(bus, v)
}

// BusVolume 读总线音量。
func (p *wavePlayer) BusVolume(bus string) float32 {
	if p == nil {
		return 1
	}
	return p.mix.BusVolume(bus)
}

// FadeMusic 音乐总线淡入淡出（按混音帧推进）。
func (p *wavePlayer) FadeMusic(to float32, seconds float32) {
	if p == nil {
		return
	}
	p.mix.FadeMusic(to, seconds)
}

// Status 后端自述：winmm + 实际采样率 + 是否还能输出。
func (p *wavePlayer) Status() PlayerStatus {
	if p == nil {
		return PlayerStatus{Backend: "nop", OK: false}
	}
	p.mu.Lock()
	ok := !p.closed
	p.mu.Unlock()
	return PlayerStatus{Backend: "winmm", Rate: p.mix.Rate(), OK: ok}
}

func playerOpen(rate int) Player {
	p := newWavePlayer(rate)
	if p == nil {
		return NopPlayer{}
	}
	return p
}

func newWavePlayer(rate int) *wavePlayer {
	if rate <= 0 {
		rate = 48000
	}
	fmtEx := waveFormatEx{
		FormatTag: 1, Channels: 2, SamplesPerSec: uint32(rate),
		BlockAlign: 4, BitsPerSample: 16, CbSize: 0,
	}
	fmtEx.AvgBytesPerSec = fmtEx.SamplesPerSec * 4

	p := &wavePlayer{mix: NewMixer(rate), done: make(chan struct{})}
	var h uintptr
	r, _, _ := waveOutOpen.Call(
		uintptr(unsafe.Pointer(&h)),
		uintptr(0xFFFFFFFF), // WAVE_MAPPER
		uintptr(unsafe.Pointer(&fmtEx)),
		0, 0, 0, // CALLBACK_NULL
	)
	if r != 0 || h == 0 {
		return nil
	}
	p.hwo = h
	const frames = 2048
	for i := 0; i < 2; i++ {
		p.buf[i] = make([]int16, frames*2)
		p.hdr[i].Data = unsafe.Pointer(&p.buf[i][0])
		p.hdr[i].BufferLen = uint32(len(p.buf[i]) * 2)
		waveOutPrepare.Call(p.hwo, uintptr(unsafe.Pointer(&p.hdr[i])), unsafe.Sizeof(p.hdr[i]))
	}
	go p.loop()
	return p
}

func (p *wavePlayer) loop() {
	i := 0
	for {
		select {
		case <-p.done:
			return
		default:
		}
		hdr := &p.hdr[i]
		hdr.Flags &^= wdrDone
		p.mix.Mix(p.buf[i])
		r, _, _ := waveOutWrite.Call(p.hwo, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		if r != 0 {
			return
		}
		// 等待该缓冲播完（4ms 粒度轮询，2048 帧 @48k ≈ 42.6ms）
		for hdr.Flags&wdrDone == 0 {
			select {
			case <-p.done:
				return
			default:
			}
			time.Sleep(2 * time.Millisecond)
		}
		i ^= 1
	}
}

func (p *wavePlayer) Play(s *Sound, opt PlayOpt) VoiceID { return p.mix.Play(s, opt) }

func (p *wavePlayer) Stop(id VoiceID) { p.mix.Stop(id) }

func (p *wavePlayer) SetVolume(id VoiceID, v float32) { p.mix.SetVolume(id, v) }

func (p *wavePlayer) SetMasterVolume(v float32) { p.mix.SetMasterVolume(v) }

func (p *wavePlayer) ActiveVoices() int { return p.mix.ActiveVoices() }

func (p *wavePlayer) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	close(p.done)
	waveOutReset.Call(p.hwo)
	for i := 0; i < 2; i++ {
		waveOutUnprepare.Call(p.hwo, uintptr(unsafe.Pointer(&p.hdr[i])), unsafe.Sizeof(p.hdr[i]))
	}
	waveOutClose.Call(p.hwo)
}
