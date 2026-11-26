// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"runtime"
	"testing"
	"time"

	"guis/game"
)

// consumerCommand 返回一个「只读 stdin 不做事」的命令，用于验证管道后端。
func consumerCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd /c more"
	}
	return "cat"
}

func TestAudioCmdArgs(t *testing.T) {
	name, args, ok := game.TestAudioCmdArgs("aplay -q -t raw -r %rate% -c 2", 22050)
	if !ok || name != "aplay" {
		t.Fatalf("解析失败: name=%q ok=%v", name, ok)
	}
	if len(args) != 7 || args[4] != "22050" {
		t.Fatalf("参数替换错误: %v", args)
	}
	// 多个占位符都应替换。
	_, args, _ = game.TestAudioCmdArgs("x -r %rate% -b %rate%", 8000)
	if args[1] != "8000" || args[3] != "8000" {
		t.Fatalf("多占位符替换错误: %v", args)
	}
	if _, _, ok := game.TestAudioCmdArgs("   ", 44100); ok {
		t.Fatal("空模板应返回 ok=false")
	}
	if _, _, ok := game.TestAudioCmdArgs("", 44100); ok {
		t.Fatal("空字符串应返回 ok=false")
	}
}

func TestAudioDefaultCommands(t *testing.T) {
	cmds := game.TestAudioDefaultCommands()
	switch runtime.GOOS {
	case "android", "ios":
		if len(cmds) != 0 {
			t.Fatalf("移动端不应有默认外部命令: %v", cmds)
		}
	default:
		if len(cmds) == 0 {
			t.Fatalf("%s 应至少有一个默认命令候选", runtime.GOOS)
		}
		for _, c := range cmds {
			if name, _, ok := game.TestAudioCmdArgs(c, 44100); !ok || name == "" {
				t.Fatalf("默认命令模板无法解析: %q", c)
			}
		}
	}
}

func TestOpenPlayerWithCommandMissing(t *testing.T) {
	p := game.OpenPlayerWithCommand(44100, "definitely-not-a-real-command-xyz -a %rate%")
	if _, ok := p.(game.NopPlayer); !ok {
		t.Fatalf("命令不存在时应降级为 NopPlayer，实际 %T", p)
	}
	if st, ok := p.(game.StatusReporter); ok && st.Status().OK {
		t.Fatal("NopPlayer 的 Status().OK 应为 false")
	}
}

func TestCmdPlayerPipe(t *testing.T) {
	p := game.OpenPlayerWithCommand(44100, consumerCommand())
	rep, ok := p.(game.StatusReporter)
	if !ok {
		t.Fatalf("返回类型未实现 StatusReporter: %T", p)
	}
	st := rep.Status()
	if !st.OK {
		t.Skipf("本机无法启动管道后端（%s），跳过: %s", consumerCommand(), st.Backend)
	}
	if st.Rate != 44100 {
		t.Fatalf("采样率应为 44100，实际 %d", st.Rate)
	}

	id := p.Play(game.Tone(440, 0.05, 44100), game.PlayOpt{Volume: 0.5})
	if id == 0 {
		t.Fatal("Play 应返回有效 VoiceID")
	}
	if n := p.ActiveVoices(); n < 0 {
		t.Fatalf("ActiveVoices 异常: %d", n)
	}
	p.SetVolume(id, 0.2)
	p.SetMasterVolume(0.8)
	time.Sleep(150 * time.Millisecond)
	p.Close()
	p.Close() // 幂等：重复 Close 不应 panic 或死锁
}

func TestCmdPlayerEnvOverride(t *testing.T) {
	t.Setenv("GUIS_AUDIO_CMD", consumerCommand())
	p := game.OpenPlayerWithCommand(22050, "")
	rep, ok := p.(game.StatusReporter)
	if !ok {
		t.Fatalf("返回类型未实现 StatusReporter: %T", p)
	}
	if st := rep.Status(); !st.OK {
		t.Skipf("本机无法启动管道后端，跳过: %s", st.Backend)
	} else if st.Rate != 22050 {
		t.Fatalf("采样率未按参数生效: %d", st.Rate)
	}
	p.Close()
}
