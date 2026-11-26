// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// 外部命令音频后端：把 Mixer 混出的 S16LE 立体声裸流写进某个系统播放器的 stdin。
//
// 它不需要 cgo，也不绑定具体平台：只要机器上有 aplay / paplay / sox / ffplay 之类的
// 命令就能出声（macOS 默认用 sox 或 ffplay，Linux 默认 aplay → paplay → ffplay 依次探测，
// Windows 默认 ffplay）。
//
//	switch p := OpenPlayerWithCommand(44100, "").(type) {
//	case NopPlayer:
//	    // 没找到可用命令，静音运行
//	default:
//	    if st, ok := p.(StatusReporter); ok {
//	        log.Println("音频后端:", st.Status().Backend)
//	    }
//	}
//
// 注意：管道后端延迟取决于外部命令的缓冲（通常 50~200ms），适合背景音乐与
// 节奏类音效；需要低延迟请用平台原生后端（Windows waveOut / 浏览器 WebAudio）。

// audioCmdChunkFrames 每次写入外部命令的帧数（单声道帧数，立体声写两倍 int16）。
const audioCmdChunkFrames = 1024

// audioDefaultCommands 返回当前平台候选的裸流播放命令（%rate% 会被替换成采样率）。
func audioDefaultCommands() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"sox -q -t raw -r %rate% -c 2 -e signed-integer -b 16 - -d",
			"ffplay -nodisp -autoexit -loglevel quiet -f s16le -ar %rate% -ac 2 -",
		}
	case "windows":
		return []string{
			"ffplay -nodisp -autoexit -loglevel quiet -f s16le -ar %rate% -ac 2 -",
		}
	case "android", "ios":
		// 移动端沙箱内没有可执行的外部播放器，交给上层改用原生后端。
		return nil
	default:
		return []string{
			"aplay -q -t raw -f S16_LE -c 2 -r %rate%",
			"paplay --raw --format=s16le --rate=%rate% --channels=2",
			"ffplay -nodisp -autoexit -loglevel quiet -f s16le -ar %rate% -ac 2 -",
		}
	}
}

// audioCmdArgs 把命令模板拆成「可执行文件 + 参数」，并替换 %rate% 占位符。
//
// 模板为空或只有空白时返回 ok=false；不做 shell 解析，参数按空格切分，
// 因此带空格的路径需要用不带空格的可执行文件名（或换成原生后端）。
func audioCmdArgs(tmpl string, rate int) (name string, args []string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(tmpl))
	if len(fields) == 0 {
		return "", nil, false
	}
	rs := strconv.Itoa(rate)
	name = fields[0]
	args = make([]string, 0, len(fields)-1)
	for _, f := range fields[1:] {
		args = append(args, strings.ReplaceAll(f, "%rate%", rs))
	}
	return name, args, true
}

// OpenPlayerWithCommand 用外部命令作音频后端；command 为空时按平台自动探测。
//
// command 支持 %rate% 占位符，例如 "aplay -q -t raw -f S16_LE -c 2 -r %rate%"。
// 命令不存在或启动失败时返回 NopPlayer（静音，不报错），可用 StatusReporter 判定。
func OpenPlayerWithCommand(rate int, command string) Player {
	if rate <= 0 {
		rate = 44100
	}
	var candidates []string
	switch {
	case strings.TrimSpace(command) != "":
		candidates = []string{command}
	case strings.TrimSpace(os.Getenv("GUIS_AUDIO_CMD")) != "":
		// 环境变量优先：没有原生后端的平台可借此接入 aplay/ffplay/sox。
		candidates = []string{os.Getenv("GUIS_AUDIO_CMD")}
	default:
		candidates = audioDefaultCommands()
	}
	for _, tmpl := range candidates {
		name, args, ok := audioCmdArgs(tmpl, rate)
		if !ok {
			continue
		}
		if _, err := exec.LookPath(name); err != nil {
			continue
		}
		if p := newCmdPlayer(rate, name, args); p != nil {
			return p
		}
	}
	return NopPlayer{}
}

// cmdPlayer 外部命令管道播放器。
type cmdPlayer struct {
	mix     *Mixer
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	buf     []int16
	rate    int
	backend string

	mu     sync.Mutex
	done   chan struct{}
	closed bool
	wg     sync.WaitGroup
}

// 编译期断言：cmdPlayer 同时提供总线音量与音乐淡入淡出。
var (
	_ Player          = (*cmdPlayer)(nil)
	_ StatusReporter  = (*cmdPlayer)(nil)
	_ BusControllable = (*cmdPlayer)(nil)
	_ MusicFader      = (*cmdPlayer)(nil)
)

// newCmdPlayer 启动外部命令并开始泵送混音数据；失败返回 nil。
func newCmdPlayer(rate int, name string, args []string) *cmdPlayer {
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil
	}
	p := &cmdPlayer{
		mix:     NewMixer(rate),
		cmd:     cmd,
		stdin:   stdin,
		buf:     make([]int16, audioCmdChunkFrames*2),
		rate:    rate,
		backend: "cmd(" + name + ")",
		done:    make(chan struct{}),
	}
	p.wg.Add(1)
	go p.pump()
	return p
}

// pump 持续混音并写入进程 stdin；写入阻塞天然起到限速作用。
func (p *cmdPlayer) pump() {
	defer p.wg.Done()
	raw := make([]byte, len(p.buf)*2)
	for {
		select {
		case <-p.done:
			return
		default:
		}
		p.mix.Mix(p.buf)
		for i, v := range p.buf {
			binary.LittleEndian.PutUint16(raw[i*2:], uint16(v))
		}
		if _, err := p.stdin.Write(raw); err != nil {
			return
		}
	}
}

// Play 实现 Player。
func (p *cmdPlayer) Play(s *Sound, opt PlayOpt) VoiceID { return p.mix.Play(s, opt) }

// Stop 实现 Player。
func (p *cmdPlayer) Stop(id VoiceID) { p.mix.Stop(id) }

// SetVolume 实现 Player。
func (p *cmdPlayer) SetVolume(id VoiceID, v float32) { p.mix.SetVolume(id, v) }

// SetMasterVolume 实现 Player。
func (p *cmdPlayer) SetMasterVolume(v float32) { p.mix.SetMasterVolume(v) }

// ActiveVoices 实现 Player。
func (p *cmdPlayer) ActiveVoices() int { return p.mix.ActiveVoices() }

// SetBusVolume 实现 BusControllable。
func (p *cmdPlayer) SetBusVolume(bus string, v float32) { p.mix.SetBusVolume(bus, v) }

// BusVolume 实现 BusControllable。
func (p *cmdPlayer) BusVolume(bus string) float32 { return p.mix.BusVolume(bus) }

// FadeMusic 实现 MusicFader。
func (p *cmdPlayer) FadeMusic(to float32, seconds float32) { p.mix.FadeMusic(to, seconds) }

// Status 实现 StatusReporter。
func (p *cmdPlayer) Status() PlayerStatus {
	if p == nil {
		return PlayerStatus{Backend: "nop"}
	}
	return PlayerStatus{Backend: p.backend, Rate: p.rate, OK: true}
}

// Close 停止泵送、关闭管道并回收子进程（可重复调用）。
func (p *cmdPlayer) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.done)
	p.mu.Unlock()

	p.mix.Close()
	_ = p.stdin.Close()
	p.wg.Wait()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
}
