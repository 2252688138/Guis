// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build linux && !android

package game

import (
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// OSS（/dev/dsp）后端：不依赖 cgo 与任何第三方库，直接写 OSS 设备文件。
//
// 现实限制：现代发行版默认用 PipeWire / PulseAudio，通常**没有** /dev/dsp，
// 此时 playerOpen 返回 NopPlayer（Status().OK == false）。需要真出声的 Linux
// 发行版可加载 snd_pcm_oss 模块，或改用 WASM/浏览器版本。
const (
	sndctlDspSetFmt     = 0xC0045004 // _SIOWR('P', 5, int)
	sndctlDspChannels   = 0xC0045005 // _SIOWR('P', 6, int)
	sndctlDspSpeed      = 0xC0045002 // _SIOWR('P', 2, int)
	afmtS16LE           = 0x00000010
	ossWriteChunkFrames = 1024
)

type ossPlayer struct {
	mu     sync.Mutex
	mix    *Mixer
	f      *os.File
	closed bool
	stop   chan struct{}
}

func playerOpen(rate int) Player {
	if rate <= 0 {
		rate = 48000
	}
	for _, name := range []string{"/dev/dsp", "/dev/dsp1"} {
		f, err := os.OpenFile(name, os.O_WRONLY, 0)
		if err != nil {
			continue
		}
		if err := ossConfigure(f, rate); err != nil {
			f.Close()
			continue
		}
		p := &ossPlayer{mix: NewMixer(rate), f: f, stop: make(chan struct{})}
		go p.loop()
		return p
	}
	return NopPlayer{}
}

// ossConfigure 设置 16bit 立体声与采样率（失败说明设备不支持，直接回退）。
func ossConfigure(f *os.File, rate int) error {
	fmtVal := int32(afmtS16LE)
	chVal := int32(2)
	speedVal := int32(rate)
	for _, it := range []struct {
		req uintptr
		v   *int32
	}{
		{sndctlDspSetFmt, &fmtVal},
		{sndctlDspChannels, &chVal},
		{sndctlDspSpeed, &speedVal},
	} {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), it.req, uintptr(unsafe.Pointer(it.v))); errno != 0 {
			return errno
		}
	}
	return nil
}

func (p *ossPlayer) loop() {
	buf := make([]int16, ossWriteChunkFrames*2)
	bytesBuf := make([]byte, len(buf)*2)
	for {
		select {
		case <-p.stop:
			return
		default:
		}
		p.mix.Mix(buf)
		for i, v := range buf {
			bytesBuf[2*i] = byte(uint16(v))
			bytesBuf[2*i+1] = byte(uint16(v) >> 8)
		}
		if _, err := p.f.Write(bytesBuf); err != nil {
			return
		}
		// 写满设备缓冲就会阻塞，这里只是给调度器一个让出点
		time.Sleep(time.Millisecond)
	}
}

func (p *ossPlayer) Play(s *Sound, opt PlayOpt) VoiceID { return p.mix.Play(s, opt) }
func (p *ossPlayer) Stop(id VoiceID)                    { p.mix.Stop(id) }
func (p *ossPlayer) SetVolume(id VoiceID, v float32)    { p.mix.SetVolume(id, v) }
func (p *ossPlayer) SetMasterVolume(v float32)          { p.mix.SetMasterVolume(v) }
func (p *ossPlayer) ActiveVoices() int                  { return p.mix.ActiveVoices() }

func (p *ossPlayer) SetBusVolume(bus string, v float32) { p.mix.SetBusVolume(bus, v) }
func (p *ossPlayer) BusVolume(bus string) float32       { return p.mix.BusVolume(bus) }
func (p *ossPlayer) FadeMusic(to float32, sec float32)  { p.mix.FadeMusic(to, sec) }

func (p *ossPlayer) Status() PlayerStatus {
	p.mu.Lock()
	ok := !p.closed
	p.mu.Unlock()
	return PlayerStatus{Backend: "oss", Rate: p.mix.Rate(), OK: ok}
}

func (p *ossPlayer) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.stop)
	f := p.f
	p.mu.Unlock()
	if f != nil {
		f.Close()
	}
}
