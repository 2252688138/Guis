// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"io"
	"os"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// MusicStream 流式背景音乐源：分块解码，长曲不会整首驻留内存。
//
// 支持 MP3 / OGG Vorbis 真流式；WAV 因为通常较短，整读后当作内存流。
type MusicStream struct {
	rate     int
	ch       int
	frames   int
	closer   io.Closer
	readFn   func(p []int16) (int, error)
	seekFn   func(frame int) error
	tempPath string // 非空时 Close 会删除临时文件
}

// OpenMusicStream 打开流式音乐文件（按内容嗅探格式）。
func OpenMusicStream(path string) (*MusicStream, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}
	switch {
	case n >= 4 && string(head) == "OggS":
		r, err := oggvorbis.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("game: OGG 流打开失败: %w", err)
		}
		ch := r.Channels()
		if ch <= 0 {
			ch = 2
		}
		ms := &MusicStream{rate: r.SampleRate(), ch: ch, frames: int(r.Length()), closer: f}
		buf := make([]float32, 4096)
		ms.readFn = func(p []int16) (int, error) {
			want := len(p)
			if want > len(buf) {
				buf = make([]float32, want)
			}
			n, err := r.Read(buf[:want])
			for i := 0; i < n; i++ {
				p[i] = floatToI16(buf[i])
			}
			if err == io.EOF {
				return n, io.EOF
			}
			return n, err
		}
		ms.seekFn = func(frame int) error { return r.SetPosition(int64(frame)) }
		return ms, nil
	case n >= 4 && string(head) == "RIFF":
		b, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		snd, err := LoadWAVBytes(b)
		if err != nil {
			return nil, err
		}
		if snd.Channels < 2 {
			// 单声道补齐为立体声，方便与 Mixer 的立体声输出对齐
			stereo := make([]int16, len(snd.Data)*2)
			for i, v := range snd.Data {
				stereo[2*i], stereo[2*i+1] = v, v
			}
			snd.Data = stereo
			snd.Channels = 2
		}
		ms := &MusicStream{rate: snd.SampleRate, ch: snd.Channels, frames: snd.Frames()}
		pos := 0
		ms.readFn = func(p []int16) (int, error) {
			if pos >= len(snd.Data) {
				return 0, io.EOF
			}
			n := copy(p, snd.Data[pos:])
			pos += n
			return n, nil
		}
		ms.seekFn = func(frame int) error {
			pos = frame * snd.Channels
			if pos < 0 || pos > len(snd.Data) {
				return fmt.Errorf("game: seek 越界")
			}
			return nil
		}
		return ms, nil
	default:
		d, err := mp3.NewDecoder(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("game: 不支持的音频格式（或 MP3 打开失败: %v）", err)
		}
		ms := &MusicStream{rate: d.SampleRate(), ch: 2, frames: int(d.Length()) / 4, closer: f}
		ms.readFn = func(p []int16) (int, error) {
			buf := make([]byte, len(p)*2)
			n, err := d.Read(buf)
			cnt := n / 2
			for i := 0; i < cnt; i++ {
				p[i] = int16(uint16(buf[2*i]) | uint16(buf[2*i+1])<<8)
			}
			if err == io.EOF {
				return cnt, io.EOF
			}
			return cnt, err
		}
		ms.seekFn = func(frame int) error {
			_, err := d.Seek(int64(frame)*4, io.SeekStart)
			return err
		}
		return ms, nil
	}
}

// Read 读入交错 int16 样本（返回样本数，非帧数）。到末尾返回 io.EOF。
func (m *MusicStream) Read(p []int16) (int, error) {
	if m == nil || m.readFn == nil {
		return 0, io.EOF
	}
	return m.readFn(p)
}

// Rate 采样率。
func (m *MusicStream) Rate() int {
	if m == nil {
		return 0
	}
	return m.rate
}

// Channels 声道数。
func (m *MusicStream) Channels() int {
	if m == nil {
		return 0
	}
	return m.ch
}

// Frames 总帧数（-1 表示未知）。
func (m *MusicStream) Frames() int {
	if m == nil {
		return 0
	}
	return m.frames
}

// SeekFrames 跳到指定帧（单位：帧，非秒）。
func (m *MusicStream) SeekFrames(frame int) error {
	if m == nil || m.seekFn == nil {
		return fmt.Errorf("game: 该流不支持定位")
	}
	return m.seekFn(frame)
}

// Close 释放底层文件（内存流创建的临时文件一并删除）。
func (m *MusicStream) Close() error {
	if m == nil {
		return nil
	}
	var err error
	if m.closer != nil {
		err = m.closer.Close()
		m.closer = nil
	}
	if m.tempPath != "" {
		_ = os.Remove(m.tempPath)
		m.tempPath = ""
	}
	return err
}

// MusicStreamFromBytes 用内存字节创建流（测试 / 内嵌资源用）。
func MusicStreamFromBytes(b []byte) (*MusicStream, error) {
	tmp, err := os.CreateTemp("", "guis-music-*")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return nil, err
	}
	tmp.Close()
	ms, err := OpenMusicStream(name)
	if err != nil {
		os.Remove(name)
		return nil, err
	}
	ms.tempPath = name
	return ms, nil
}
