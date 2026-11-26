// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

//go:build js && wasm

package game

import (
	"sync"
	"syscall/js"
)

// WebAudio 后端：浏览器里用 AudioContext + ScriptProcessorNode 拉流。
//
// ScriptProcessorNode 已被规范标记为废弃，但兼容性最好、实现最简；
// 需要更低延迟时可换成 AudioWorklet（需额外 .js 模块，暂不做）。
const wasmChunkFrames = 2048

type wasmPlayer struct {
	mu     sync.Mutex
	mix    *Mixer
	ctx    js.Value
	node   js.Value
	fn     js.Func
	closed bool
}

func playerOpen(rate int) Player {
	if rate <= 0 {
		rate = 48000
	}
	ctor := js.Global().Get("AudioContext")
	if ctor.IsUndefined() {
		ctor = js.Global().Get("webkitAudioContext")
		if ctor.IsUndefined() {
			return NopPlayer{}
		}
	}
	ctx := ctor.New(map[string]any{"sampleRate": rate})
	if ctx.IsUndefined() || ctx.IsNull() {
		return NopPlayer{}
	}
	node := ctx.Call("createScriptProcessor", wasmChunkFrames, 2, 2)
	if node.IsUndefined() {
		return NopPlayer{}
	}
	p := &wasmPlayer{mix: NewMixer(rate), ctx: ctx, node: node}
	p.fn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		ob := args[0].Get("outputBuffer")
		if ob.IsUndefined() {
			return nil
		}
		n := ob.Get("length").Int()
		ch0 := ob.Call("getChannelData", 0)
		ch1 := ob.Call("getChannelData", 1)
		buf := make([]int16, n*2)
		p.mix.Mix(buf)
		for i := 0; i < n; i++ {
			ch0.SetIndex(i, float64(buf[2*i])/32768)
			ch1.SetIndex(i, float64(buf[2*i+1])/32768)
		}
		return nil
	})
	node.Set("onaudioprocess", p.fn)
	node.Call("connect", ctx.Get("destination"))
	ctx.Call("resume")
	return p
}

func (p *wasmPlayer) Play(s *Sound, opt PlayOpt) VoiceID { return p.mix.Play(s, opt) }
func (p *wasmPlayer) Stop(id VoiceID)                    { p.mix.Stop(id) }
func (p *wasmPlayer) SetVolume(id VoiceID, v float32)    { p.mix.SetVolume(id, v) }
func (p *wasmPlayer) SetMasterVolume(v float32)          { p.mix.SetMasterVolume(v) }
func (p *wasmPlayer) ActiveVoices() int                  { return p.mix.ActiveVoices() }

func (p *wasmPlayer) SetBusVolume(bus string, v float32) { p.mix.SetBusVolume(bus, v) }
func (p *wasmPlayer) BusVolume(bus string) float32       { return p.mix.BusVolume(bus) }
func (p *wasmPlayer) FadeMusic(to float32, sec float32)  { p.mix.FadeMusic(to, sec) }

func (p *wasmPlayer) Status() PlayerStatus {
	p.mu.Lock()
	ok := !p.closed
	p.mu.Unlock()
	return PlayerStatus{Backend: "webaudio", Rate: p.mix.Rate(), OK: ok}
}

func (p *wasmPlayer) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	node, ctx, fn := p.node, p.ctx, p.fn
	p.mu.Unlock()
	if !node.IsUndefined() {
		node.Set("onaudioprocess", js.Null())
	}
	if !ctx.IsUndefined() {
		ctx.Call("close")
	}
	fn.Release()
}
