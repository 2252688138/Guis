// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"os"
	"sort"
)

// FrameInput 单帧输入快照（用于回放）。
type FrameInput struct {
	Keys    []string
	Pressed []string
	MouseX  float32
	MouseY  float32
	DX      float32
	DY      float32
	Wheel   float32
	Buttons [3]bool
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Recorder 输入录制 / 回放（确定性回放，便于复现 bug 与做 demo 回放）。
type Recorder struct {
	Frames    []FrameInput `json:"frames"`
	recording bool
	playing   bool
	idx       int
}

// NewRecorder 创建空录制器。
func NewRecorder() *Recorder { return &Recorder{} }

// StartRecord 开始录制（清空旧帧）。
func (r *Recorder) StartRecord() {
	r.Frames = r.Frames[:0]
	r.recording = true
	r.playing = false
}

// Record 录制当前帧输入。
func (r *Recorder) Record(in *Input) {
	if r == nil || !r.recording || in == nil {
		return
	}
	r.Frames = append(r.Frames, FrameInput{
		Keys:    sortedKeys(in.Keys),
		Pressed: sortedKeys(in.Pressed),
		MouseX:  in.MouseX, MouseY: in.MouseY,
		DX: in.MouseDX, DY: in.MouseDY, Wheel: in.Wheel,
		Buttons: in.Buttons,
	})
}

// StopRecord 结束录制，返回帧数。
func (r *Recorder) StopRecord() int {
	if r == nil {
		return 0
	}
	r.recording = false
	return len(r.Frames)
}

// StartPlayback 从头开始回放。
func (r *Recorder) StartPlayback() {
	if r == nil {
		return
	}
	r.playing = true
	r.recording = false
	r.idx = 0
}

// Playback 把下一帧快照写入 in，返回是否还有帧（结束后自动停止）。
func (r *Recorder) Playback(in *Input) bool {
	if r == nil || !r.playing || in == nil {
		return false
	}
	if r.idx >= len(r.Frames) {
		r.playing = false
		return false
	}
	f := r.Frames[r.idx]
	r.idx++
	in.Keys = map[string]bool{}
	for _, k := range f.Keys {
		in.Keys[k] = true
	}
	in.Pressed = map[string]bool{}
	for _, k := range f.Pressed {
		in.Pressed[k] = true
	}
	in.Released = map[string]bool{}
	in.MouseX, in.MouseY = f.MouseX, f.MouseY
	in.MouseDX, in.MouseDY = f.DX, f.DY
	in.Wheel = f.Wheel
	in.Buttons = f.Buttons
	in.BtnPressed = [3]bool{}
	in.Touches = nil
	if r.idx >= len(r.Frames) {
		r.playing = false
	}
	return true
}

// StopPlayback 停止回放。
func (r *Recorder) StopPlayback() {
	if r == nil {
		return
	}
	r.playing = false
}

// FrameCount 已录制帧数。
func (r *Recorder) FrameCount() int {
	if r == nil {
		return 0
	}
	return len(r.Frames)
}

// Recording / Playing 当前状态。
func (r *Recorder) Recording() bool    { return r != nil && r.recording }
func (r *Recorder) PlayingState() bool { return r != nil && r.playing }

// Save 保存录制到文件（JSON）。
func (r *Recorder) Save(path string) error {
	if r == nil {
		return os.ErrInvalid
	}
	b, err := json.Marshal(r.Frames)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

// Load 从文件读取录制。
func (r *Recorder) Load(path string) error {
	if r == nil {
		return os.ErrInvalid
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &r.Frames)
}
