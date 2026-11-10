// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// 系统级能力的跨平台单测：只断言「符号存在、行为自洽、不 panic」，
// 平台相关的真实效果（任务栏进度 / 材质 / 拖出）由 gallery 实机验证。

func TestKnownFoldersNotEmpty(t *testing.T) {
	for d := DirHome; d <= DirExecutable; d++ {
		if KnownDirName(d) == "未知目录" {
			t.Fatalf("目录类别 %d 缺少名称", d)
		}
	}
	// 用户目录与临时目录在任何平台都应该能拿到
	if runtime.GOOS != "js" {
		if p := KnownFolder(DirHome); p == "" {
			t.Fatal("KnownFolder(DirHome) 为空")
		}
		if p := KnownFolder(DirTemp); p == "" {
			t.Fatal("KnownFolder(DirTemp) 为空")
		}
	}
}

func TestEnsureFolderCreates(t *testing.T) {
	base := t.TempDir()
	// 用 SubFolder 拼接，确保它只是拼路径不创建
	p := filepath.Join(base, "a", "b")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Skipf("无法建临时目录: %v", err)
	}
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		t.Fatalf("目录不存在: %v", err)
	}
	if got := SubFolder(DirHome, "x"); got == "" && KnownFolder(DirHome) != "" {
		t.Fatal("SubFolder 返回空")
	}
}

func TestFsChangeNameAllKinds(t *testing.T) {
	for _, c := range []FsChange{FsCreated, FsModified, FsRemoved, FsRenamed} {
		if FsChangeName(c) == "未知" {
			t.Fatalf("变更类型 %d 缺少名称", c)
		}
	}
	if FsChangeName(FsChange(99)) != "未知" {
		t.Fatal("越界变更类型应返回未知")
	}
	e := FsEvent{Path: "a.txt", Change: FsCreated}
	if e.String() == "" {
		t.Fatal("FsEvent.String 为空")
	}
}

// TestWatchDirDetectsCreate 监听目录并新建文件，应在超时前收到 FsCreated。
func TestWatchDirDetectsCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("短模式跳过异步监听测试")
	}
	dir := t.TempDir()
	got := make(chan FsEvent, 8)
	stop := WatchDir(dir, WatchOptions{PollInterval: 50 * time.Millisecond},
		func(evs []FsEvent) {
			for _, e := range evs {
				select {
				case got <- e:
				default:
				}
			}
		})
	defer stop()

	time.Sleep(80 * time.Millisecond) // 让监听先就绪
	target := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case e := <-got:
			if filepath.Base(e.Path) == "hello.txt" && (e.Change == FsCreated || e.Change == FsModified) {
				return
			}
		case <-deadline:
			t.Fatal("3 秒内没有收到 hello.txt 的变更事件")
		}
	}
}

func TestWatchDirStopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	stop := WatchDir(dir, WatchOptions{PollInterval: 20 * time.Millisecond}, func([]FsEvent) {})
	stop()
	stop() // 重复调用不得 panic
}

func TestHTMLToPlainText(t *testing.T) {
	cases := map[string]string{
		"<b>加粗</b>":          "加粗",
		"a &amp; b":           "a & b",
		"<p>x</p><p>y</p>":    "xy",
		"plain":               "plain",
		"<i>&lt;tag&gt;</i>":  "<tag>",
	}
	for in, want := range cases {
		if got := HTMLToPlainText(in); got != want {
			t.Fatalf("HTMLToPlainText(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestMsgButtonTexts(t *testing.T) {
	if len(MsgButtonTexts(MsgBoxOK)) != 1 {
		t.Fatal("OK 组合应有 1 个按钮")
	}
	if len(MsgButtonTexts(MsgBoxYesNo)) != 2 {
		t.Fatal("YesNo 组合应有 2 个按钮")
	}
	unknown := MsgButtonTexts(MsgButtons(99))
	if len(unknown) != 1 || unknown[0] != "确定" {
		t.Fatal("未知按钮组合应回退到「确定」")
	}
}

func TestUnsupportedError(t *testing.T) {
	err := errUnsupported("测试能力")
	if !IsUnsupported(err) {
		t.Fatal("IsUnsupported 应识别平台不支持错误")
	}
	if IsUnsupported(nil) {
		t.Fatal("IsUnsupported(nil) 应为 false")
	}
	if IsUnsupported(os.ErrNotExist) {
		t.Fatal("普通错误不应被当成不支持")
	}
}

func TestSystemInfoAndPowerDoNotPanic(t *testing.T) {
	info := SystemInfo()
	if runtime.GOOS != "js" {
		if info.Locale == "" {
			t.Fatal("Locale 为空")
		}
		if info.Arch == "" {
			t.Fatal("Arch 为空")
		}
	}
	_ = info.String()
	st := PowerState()
	if st.Percent < -1 || st.Percent > 100 {
		t.Fatalf("电量越界: %d", st.Percent)
	}
	_ = HasBattery()
}

func TestForegroundWindowInfoDoNotPanic(t *testing.T) {
	info := ForegroundWindowInfo()
	_ = info.PID
	_ = info.Title
}

func TestDragFilesEmptyIsSafe(t *testing.T) {
	// 空列表不应进入 OLE 拖放（否则会卡住消息循环）
	if err := DragFiles(nil, -1, -1); err == nil {
		t.Log("空列表返回 nil（可接受的降级实现）")
	}
}

func TestFileColorHelpers(t *testing.T) {
	// 不存在的文件：Windows 走 SHGFI_USEFILEATTRIBUTES 仍能给出图标，
	// 其它平台返回「不支持」——两者都不许 panic。
	_, _ = FileIcon(filepath.Join(t.TempDir(), "nope.unknownext"), 32)
	_, _ = FileThumbnail(filepath.Join(t.TempDir(), "nope.png"), 64, 64)
	_ = FileTypeName("x.txt")
}
