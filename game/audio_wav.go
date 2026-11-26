// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/binary"
	"errors"
	"os"
)

// ErrBadWAV WAV 格式非法或不支持。
var ErrBadWAV = errors.New("game: 不支持的 WAV 格式（只支持 PCM 8/16bit）")

// LoadWAV 读取 WAV 文件。
func LoadWAV(path string) (*Sound, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadWAVBytes(b)
}

// LoadWAVBytes 解码 WAV 数据（RIFF/WAVE，PCM 8bit 无符号或 16bit 有符号）。
func LoadWAVBytes(b []byte) (*Sound, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, ErrBadWAV
	}
	channels, bits, rate := 0, 0, 0
	off := 12
	var (
		fmtOK bool
		data  []byte
	)
	for off+8 <= len(b) {
		id := string(b[off : off+4])
		sz := int(binary.LittleEndian.Uint32(b[off+4 : off+8]))
		body := off + 8
		if body+sz > len(b) {
			sz = len(b) - body
		}
		switch id {
		case "fmt ":
			if sz < 16 {
				return nil, ErrBadWAV
			}
			format := binary.LittleEndian.Uint16(b[body : body+2])
			if format != 1 { // 1 = PCM
				return nil, ErrBadWAV
			}
			channels = int(binary.LittleEndian.Uint16(b[body+2 : body+4]))
			rate = int(binary.LittleEndian.Uint32(b[body+4 : body+8]))
			bits = int(binary.LittleEndian.Uint16(b[body+14 : body+16]))
			fmtOK = true
		case "data":
			data = b[body : body+sz]
		}
		off = body + sz
		if off%2 == 1 {
			off++
		}
	}
	if !fmtOK || data == nil || channels <= 0 || rate <= 0 {
		return nil, ErrBadWAV
	}
	s := &Sound{SampleRate: rate, Channels: channels}
	switch bits {
	case 16:
		n := len(data) / 2
		s.Data = make([]int16, n)
		for i := 0; i < n; i++ {
			s.Data[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
		}
	case 8:
		s.Data = make([]int16, len(data))
		for i, v := range data {
			s.Data[i] = int16((int(v) - 128) << 8)
		}
	default:
		return nil, ErrBadWAV
	}
	return s, nil
}

// Tone 生成指定频率/时长的正弦音效（无音频素材时占位用）。
func Tone(freq, seconds float32, sampleRate int) *Sound {
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	n := int(seconds * float32(sampleRate))
	if n <= 0 {
		n = 1
	}
	s := &Sound{SampleRate: sampleRate, Channels: 1, Data: make([]int16, n)}
	w := 2 * 3.14159265 * float64(freq) / float64(sampleRate)
	for i := 0; i < n; i++ {
		// 线性衰减包络，避免爆音
		env := 1 - float64(i)/float64(n)
		s.Data[i] = int16(12000 * env * sinApprox(w*float64(i)))
	}
	return s
}

// sinApprox 用泰勒级数近似 sin（避免 math 调用开销，精度足够做占位音效）。
func sinApprox(x float64) float64 {
	const pi = 3.141592653589793
	// 归一化到 [-pi, pi]
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	x2 := x * x
	return x * (1 - x2/6 + x2*x2/120 - x2*x2*x2/5040)
}
