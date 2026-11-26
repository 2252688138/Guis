// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bytes"
	"fmt"
	"io"
	"os"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// DecodeSound 按内容嗅探解码音频：WAV / MP3 / OGG Vorbis（统一输出 int16 交错 PCM）。
//
// 格式识别靠文件头（RIFF / OggS / MP3 帧同步或 ID3），不靠扩展名。
func DecodeSound(data []byte) (*Sound, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("game: 音频数据过短")
	}
	switch {
	case bytes.HasPrefix(data, []byte("RIFF")):
		return LoadWAVBytes(data)
	case bytes.HasPrefix(data, []byte("OggS")):
		return decodeOGG(data)
	case bytes.HasPrefix(data, []byte("ID3")), isMP3Sync(data):
		return decodeMP3(data)
	}
	return nil, fmt.Errorf("game: 无法识别的音频格式（支持 wav/mp3/ogg）")
}

// DecodeSoundFile 读取并解码音频文件。
func DecodeSoundFile(path string) (*Sound, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeSound(b)
}

// LoadSound 加载音频（任意已支持格式），失败时返回静音声音而非错误？
//
// 不：与 LoadWAV 一致，失败返回错误。
func LoadSound(path string) (*Sound, error) { return DecodeSoundFile(path) }

// LoadSoundBytes 从内存解码音频。
func LoadSoundBytes(b []byte) (*Sound, error) { return DecodeSound(b) }

func isMP3Sync(b []byte) bool {
	return len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0
}

// decodeMP3 用纯 Go 解码器解码 MP3（输出固定为 16bit 立体声）。
func decodeMP3(data []byte) (*Sound, error) {
	d, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("game: MP3 解码失败: %w", err)
	}
	var out []int16
	buf := make([]byte, 8192)
	for {
		n, rerr := d.Read(buf)
		if n > 0 {
			// go-mp3 输出 16bit little-endian 立体声交错样本
			for i := 0; i+1 < n; i += 2 {
				out = append(out, int16(uint16(buf[i])|uint16(buf[i+1])<<8))
			}
		}
		if rerr == io.EOF || (rerr == nil && n == 0) {
			break
		}
		if rerr != nil {
			return nil, fmt.Errorf("game: MP3 解码中断: %w", rerr)
		}
	}
	return &Sound{Data: out, SampleRate: d.SampleRate(), Channels: 2}, nil
}

// decodeOGG 解码 OGG Vorbis。
func decodeOGG(data []byte) (*Sound, error) {
	r, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("game: OGG 解码失败: %w", err)
	}
	ch := r.Channels()
	if ch <= 0 {
		ch = 2
	}
	var floats []float32
	tmp := make([]float32, 8192)
	for {
		n, rerr := r.Read(tmp)
		if n > 0 {
			floats = append(floats, tmp[:n]...)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, fmt.Errorf("game: OGG 解码中断: %w", rerr)
		}
	}
	out := make([]int16, len(floats))
	for i, v := range floats {
		out[i] = floatToI16(v)
	}
	return &Sound{Data: out, SampleRate: r.SampleRate(), Channels: ch}, nil
}

func floatToI16(v float32) int16 {
	if v > 1 {
		v = 1
	} else if v < -1 {
		v = -1
	}
	return int16(v * 32767)
}
