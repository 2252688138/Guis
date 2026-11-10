//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"errors"
	"strings"
	"sync"
	"time"

	"guis/internal/win32"
)

// 系统解码器回退（Windows Media Foundation）。
//
// 解码优先级：ffmpeg / ffprobe（PATH 或 MediaConfig 指定）→ 系统解码器。
// 没装 ffmpeg 的机器也能播放 mp4 / mkv / mov / mp3 / m4a / wav 等系统支持的格式：
// 一个 IMFSourceReader 同时输出画面（RGB32 / NV12 / YUY2 → 转 RGBA）与声音（PCM → waveOut）。
//
// 与 ffmpeg 路径的差异：
//   - 只支持系统自带解码器（MFT）能解的格式（H.264 / HEVC(需系统扩展) / AAC / MP3 / WMA 等），
//     mkv / flv / m3u8 等容器可能无法打开 → 返回明确的错误提示；
//   - 输出分辨率跟随系统解码器（不做最大尺寸限制）；
//   - 倍速播放暂不支持（保持 1x）。
//
// 可用环境变量 GUIS_DECODER=system 强制走该后端（便于验证回退路径）。

// BackendName 当前可用的解码后端名称（界面展示用）。
func BackendName() string {
	ff := toolPath("", "ffmpeg") != "" && toolPath("", "ffprobe") != ""
	if ff && DecoderPreference() != "system" {
		return "ffmpeg"
	}
	if win32.MFAvailable() {
		return "system"
	}
	return "none"
}

// BackendLabel 解码后端的中文标签。
func BackendLabel(name string) string {
	switch name {
	case "ffmpeg":
		return "ffmpeg 解码"
	case "system":
		return "系统解码（Media Foundation）"
	}
	return "无可用解码器"
}

// FormatBackend 后端名短标签（"ffmpeg" / "系统"）。
func FormatBackend(name string) string {
	switch name {
	case "system":
		return "系统"
	case "ffmpeg":
		return "ffmpeg"
	}
	return "—"
}

// openMediaSystem 用系统解码器打开音视频文件并构造 MediaSource。
func openMediaSystem(path string, cfg MediaConfig) (*MediaSource, error) {
	doneMF := win32.MFThreadInit()
	defer doneMF()

	if !win32.MFAvailable() {
		return nil, errors.New("未找到 ffmpeg，且系统解码器（Media Foundation）不可用")
	}
	r, err := win32.MFOpen(path)
	if err != nil {
		return nil, errors.New("未找到 ffmpeg，系统解码器也无法打开：" + err.Error())
	}
	if !r.HasVideo() {
		r.Close()
		return nil, errors.New("该文件没有视频流（可用 ui.OpenMusic 播放音频）")
	}
	withAudio := !cfg.NoAudio && r.HasAudio()
	r.SelectStreams(true, withAudio)
	if err := r.SetVideoRGB32(); err != nil {
		r.Close()
		return nil, err
	}
	if withAudio {
		if err := r.SetAudioPCM(); err != nil {
			withAudio = false
		}
	}

	w, h := r.Size()
	if w <= 0 || h <= 0 {
		r.Close()
		return nil, errors.New("系统解码器未报告画面尺寸")
	}
	fps := r.FPS()
	if fps <= 0 || fps > 120 {
		fps = 25
	}
	rate, ch, _ := r.AudioFormat()
	if rate <= 0 {
		rate = 48000
	}
	if ch <= 0 {
		ch = 2
	}

	info := &MediaInfo{
		Path:       path,
		Format:     strings.ToUpper(strings.TrimPrefix(extOf(path), ".")),
		Duration:   r.Duration(),
		Width:      w,
		Height:     h,
		FPS:        fps,
		VideoCodec: "系统解码",
		HasVideo:   true,
		HasAudio:   withAudio,
		Backend:    "system",
	}
	if withAudio {
		info.AudioCodec = "pcm"
		info.SampleRate = rate
		info.Channels = ch
	}

	s := &MediaSource{path: path, info: *info, cfg: cfg, speed: 1, sys: r}
	s.cond = sync.NewCond(&s.mu)
	s.outW, s.outH = w, h
	s.frameBytes = w * h * 4
	s.fps = clampFloat(fps, 1, 120)
	byBytes := (48 << 20) / s.frameBytes
	byTime := int(s.fps * 2)
	s.maxFrames = clampInt(minInt(byBytes, byTime), 3, 240)

	if withAudio {
		vol := cfg.Volume
		if vol <= 0 {
			vol = 1
		}
		if a, aerr := win32.NewAudioOut(rate, ch, 8, 2048); aerr == nil {
			s.audio = a
			s.audioDevOn = true
			a.SetVolume(vol)
		}
	}
	registerMedia(s)
	return s, nil
}

// runSysLoop 系统解码器读循环：按流索引交错读视频 / 音频样本，
// 画面进帧环（与 runVideo 一致），声音直接写 waveOut（天然按实时限流）。
func (s *MediaSource) runSysLoop(gen int, pos time.Duration) {
	done := win32.MFThreadInit()
	defer done()

	s.mu.Lock()
	rd := s.sys
	dev := s.audio
	s.mu.Unlock()
	if rd == nil {
		return
	}
	if err := rd.Seek(pos); err != nil {
		s.fail(gen, err)
		return
	}
	vidx, aidx := rd.VideoStreamIndex(), rd.AudioStreamIndex()
	videoDone := vidx < 0
	audioDone := aidx < 0 || dev == nil

	// 循环退出（读完 / 出错 / 被新窗口替换）后设备时钟不再代表播放进度：
	// 必须停用它，否则上层会把播放位置永久停在音频结束处。
	defer func() {
		s.mu.Lock()
		if gen == s.gen && !s.closed && dev != nil {
			s.clockDone = true
		}
		s.mu.Unlock()
	}()

	for {
		s.mu.Lock()
		if gen != s.gen || s.closed || s.sys == nil {
			s.mu.Unlock()
			return
		}
		paused := s.paused
		s.mu.Unlock()
		if dev != nil {
			dev.Pause(paused)
		}
		if paused {
			// 暂停时不读样本（读了就丢帧），稍等再看状态
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// ① 先喂声音：画面帧环满了也必须继续写声音。
		//    播放位置由音频设备时钟给出，声音喂不上 → 时钟停 → UI 不再取帧 →
		//    帧环永远满 → 解码循环卡在等空位：两边互相死等（片头播一两秒后画面定格）。
		if !audioDone {
			kind, data, _, eof, err := rd.ReadStream(aidx)
			if err != nil || eof {
				audioDone = true
				// 声音读完：设备时钟马上会停，通知上层改按墙钟推进（否则位置卡在结尾）
				if dev != nil {
					s.markClockDone()
				}
			} else if kind == win32.PacketAudio && len(data) > 0 && dev != nil {
				if werr := dev.Write(data); werr != nil {
					return
				}
			}
		}

		// ② 画面：只在帧环有空位时读样本，没空位就下一轮再来（绝不阻塞在等空位上）
		if !videoDone {
			if !s.hasRoom(gen) {
				if audioDone {
					// 声音已读完、帧环又满：短暂等待消费端取帧，避免忙等
					time.Sleep(5 * time.Millisecond)
				}
			} else {
				kind, data, _, eof, err := rd.ReadStream(vidx)
				if err != nil {
					if gen == s.gen && !s.isClosed() {
						s.fail(gen, err)
					}
					return
				}
				if eof {
					videoDone = true
				} else if kind == win32.PacketVideo && len(data) > 0 {
					w, h := rd.Size()
					f := &VideoFrame{
						Pix:   data,
						W:     w,
						H:     h,
						Delay: time.Duration(float64(time.Second) / s.fps),
					}
					s.mu.Lock()
					if gen != s.gen || s.closed || s.sys == nil {
						s.mu.Unlock()
						return
					}
					idx := 0
					if n := len(s.frames); n > 0 {
						idx = s.frames[n-1].idx + 1
					}
					s.frames = append(s.frames, mediaFrame{idx: idx, f: f})
					s.cond.Broadcast()
					s.mu.Unlock()
				}
			}
		}

		if videoDone && audioDone {
			s.mu.Lock()
			if gen == s.gen {
				s.eof = true
			}
			s.mu.Unlock()
			s.cond.Broadcast()
			return
		}
	}
}

// hasRoom 帧环是否有空位（非阻塞，供系统解码循环判断能否再读一帧）。
func (s *MediaSource) hasRoom(gen int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return gen == s.gen && !s.closed && s.sys != nil && len(s.frames) < s.maxFrames
}

// isClosed 是否已关闭。
func (s *MediaSource) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// waitTimeout 等待 WaitGroup 完成或超时。
func waitTimeout(wg *sync.WaitGroup, d time.Duration) {
	if wg == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
}

func waitDoneLegacyRemoved() {}

// extOf 取扩展名（小写、含点）。
func extOf(path string) string {
	p := strings.ToLower(path)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if i := strings.LastIndexByte(p, '.'); i >= 0 && i > strings.LastIndexAny(p, "/\\") {
		return p[i:]
	}
	return ""
}

// ---- 音乐（纯音频）系统解码器回退 ----

// openMusicSystem 用系统解码器打开音频文件。
func openMusicSystem(path string, cfg MediaConfig) (*MusicSource, error) {
	doneMF := win32.MFThreadInit()
	defer doneMF()

	if !win32.MFAvailable() {
		return nil, errors.New("未找到 ffmpeg，且系统解码器（Media Foundation）不可用")
	}
	r, err := win32.MFOpen(path)
	if err != nil {
		return nil, errors.New("未找到 ffmpeg，系统解码器也无法打开：" + err.Error())
	}
	if !r.HasAudio() {
		r.Close()
		return nil, errors.New("该文件没有音频流")
	}
	r.SelectStreams(false, true)
	if err := r.SetAudioPCM(); err != nil {
		r.Close()
		return nil, err
	}
	rate, ch, _ := r.AudioFormat()
	if rate <= 0 {
		rate = 48000
	}
	if ch <= 0 {
		ch = 2
	}
	info := MediaInfo{
		Path:       path,
		Format:     strings.ToUpper(strings.TrimPrefix(extOf(path), ".")),
		Duration:   r.Duration(),
		HasAudio:   true,
		AudioCodec: "pcm",
		SampleRate: rate,
		Channels:   ch,
		Backend:    "system",
	}
	s := &MusicSource{path: path, info: info, cfg: cfg, speed: 1, vol: 1, sys: r}
	s.cond = sync.NewCond(&s.mu)
	s.rate, s.ch = rate, ch
	s.an = newSpectrum(SpectrumBands, rate)

	vol := cfg.Volume
	if vol <= 0 {
		vol = 1
	}
	s.vol = vol
	if !cfg.NoAudio {
		if a, aerr := win32.NewAudioOut(rate, ch, 8, 2048); aerr == nil {
			s.audio = a
			s.audioOn = true
			a.SetVolume(vol)
		}
	}
	return s, nil
}

// runMusicSysAudio 音乐系统解码器读 PCM 循环：写 waveOut + 送频谱分析。
func (s *MusicSource) runMusicSysAudio(gen int, pos time.Duration) {
	done := win32.MFThreadInit()
	defer done()

	s.mu.Lock()
	rd := s.sys
	dev := s.audio
	rate, ch := s.rate, s.ch
	s.mu.Unlock()
	if rd == nil {
		return
	}
	if err := rd.Seek(pos); err != nil {
		s.fail(gen, err)
		return
	}
	aidx := rd.AudioStreamIndex()
	if aidx < 0 {
		return
	}

	for {
		s.mu.Lock()
		if gen != s.gen || s.closed || s.sys == nil {
			s.mu.Unlock()
			return
		}
		paused := s.paused
		s.mu.Unlock()
		if dev != nil {
			dev.Pause(paused)
		}

		kind, data, _, eof, err := rd.ReadStream(aidx)
		if err != nil || eof {
			s.mu.Lock()
			if gen == s.gen && err == nil {
				s.eof = true
			}
			s.mu.Unlock()
			RequestRedraw()
			return
		}
		if kind != win32.PacketAudio || len(data) == 0 {
			continue
		}
		if s.an != nil {
			s.an.PushPCM(data, ch, rate)
		}
		if dev != nil {
			if werr := dev.Write(data); werr != nil {
				return
			}
		} else {
			time.Sleep(pcmDuration(len(data), rate, ch))
		}
	}
}

