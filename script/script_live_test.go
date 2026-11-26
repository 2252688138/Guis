// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 真实环境测试：video（需 ffmpeg）/ webview（需窗口后端）/ 3D 视口（程序化源）。
// 环境不具备时一律 t.Skip 并打印原因，不算失败。

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"guis/ui"
	"guis/ui/uidsl"
)

// 3D 视口：用程序化源走完整注入链路，应当得到 KViewport3D（而不是降级文本）。
func TestViewportWithDemoSource(t *testing.T) {
	uidsl.ResetExtraStates()
	defer uidsl.RegisterViewport3DProvider(nil)

	src := ui.NewDemoViewportSource()
	e := newEngine(t, Options{OnViewport3D: func(key string) *ui.Viewport3DOpts {
		if key != "main" {
			return nil
		}
		return &ui.Viewport3DOpts{Src: src, Key: key, Resolution: [2]int{64, 64}}
	}})
	if err := e.DoString(`function view() return ui.viewport{ key = "main" } end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatalf("视口构树失败: %v", err)
	}
	if n := findKind(root, ui.KViewport3D); n == nil || n.Viewport == nil {
		t.Fatalf("未得到 KViewport3D 节点，文本=%q", firstText(root))
	}
	// 视口源本身能出帧
	pix, tw, th := src.RenderViewport(32, 32, 0.016)
	if len(pix) != tw*th*4 {
		t.Fatalf("视口源输出异常: len=%d %dx%d", len(pix), tw, th)
	}
}

// video 成功路径：需要 ffmpeg / ffprobe。
func TestVideoLive(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("未安装 %s，跳过 video 成功路径测试", bin)
		}
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=10",
		"-pix_fmt", "yuv420p", clip)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg 生成测试视频失败（不算失败）: %v\n%s", err, out)
	}

	uidsl.ResetExtraStates()
	defer uidsl.ResetExtraStates() // Close 掉 ffmpeg 进程
	e := newEngine(t, Options{})
	if err := e.DoString(fmt.Sprintf(`function view() return ui.video{ key = "v", src = %q } end`, clip)); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatalf("video 打开失败: %v", err)
	}
	n := findKind(root, ui.KVideo)
	if n == nil || n.Video == nil {
		t.Fatalf("未构出 video 节点，界面文本=%q", firstText(root))
	}
}

// webview 成功路径：需要窗口 + 平台后端。
func TestWebviewLive(t *testing.T) {
	probe, err := ui.NewWebView(ui.WebViewOptions{URL: "about:blank"})
	if err != nil {
		t.Skipf("当前环境无法创建 WebView（跳过，不算失败）: %v", err)
	}
	probe.Close()

	uidsl.ResetExtraStates()
	defer uidsl.ResetExtraStates()
	e := newEngine(t, Options{})
	if err := e.DoString(`function view() return ui.webview{ key = "w", src = "about:blank" } end`); err != nil {
		t.Fatal(err)
	}
	root := e.Root()
	if err := e.LastError(); err != nil {
		t.Fatalf("webview 构树失败: %v", err)
	}
	if n := findKind(root, ui.KWebView); n == nil || n.WebView == nil {
		t.Fatalf("未构出 webview 节点，界面文本=%q", firstText(root))
	}
}
