//go:build windows

// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"image"

	"guis/internal/win32"
)

// Windows 端把 internal/win32 的实现挂到跨平台钩子上。
//
// 这一层只做「类型映射」，不含业务逻辑：ui 侧保持与平台无关的签名，
// 平台文件负责把参数翻译成本地结构体。

func init() {
	// ---- 标准目录 ----
	knownFolderNative = func(d KnownDir) (string, bool) {
		var k win32.KnownFolderKind
		switch d {
		case DirHome:
			k = win32.KFHome
		case DirDesktop:
			k = win32.KFDesktop
		case DirDocuments:
			k = win32.KFDocuments
		case DirDownloads:
			k = win32.KFDownloads
		case DirPictures:
			k = win32.KFPictures
		case DirMusic:
			k = win32.KFMusic
		case DirVideos:
			k = win32.KFVideos
		case DirConfig:
			k = win32.KFConfig
		case DirCache:
			k = win32.KFCache
		case DirData:
			k = win32.KFData
		case DirTemp:
			k = win32.KFTemp
		case DirPrograms:
			k = win32.KFPrograms
		default:
			return "", false
		}
		p := win32.KnownFolder(k)
		if p == "" {
			return "", false
		}
		return p, true
	}

	// ---- 原生消息框 ----
	messageBoxNative = func(title, body string, kind MsgKind, buttons MsgButtons) (MsgResult, bool) {
		var hwnd uintptr
		if a := currentApp.Load(); a != nil {
			hwnd = a.HWND()
		}
		r := win32.MessageBox(hwnd, title, body,
			win32.MessageBoxKind(kind), win32.MessageBoxButtons(buttons))
		switch win32.MessageBoxResult(r) {
		case win32.MBResultOK:
			return MsgResOK, true
		case win32.MBResultCancel:
			return MsgResCancel, true
		case win32.MBResultYes:
			return MsgResYes, true
		case win32.MBResultNo:
			return MsgResNo, true
		}
		return MsgResCancel, true
	}

	// ---- 系统信息 / 电源 ----
	sysInfoNative = func() (SysInfo, bool) {
		r := win32.SysInfo()
		return SysInfo{
			Locale:     r.Locale,
			LocaleName: r.LocaleName,
			UserName:   r.UserName,
			HostName:   r.HostName,
			OS:         r.OS,
			OSVersion:  r.OSVersion,
			Arch:       r.Arch,
			Elevated:   r.Elevated,
		}, true
	}
	powerStateNative = func() (PowerStatus, bool) {
		onBattery, percent, charging, acOnline, ok := win32.PowerState()
		if !ok {
			return PowerStatus{}, false
		}
		return PowerStatus{
			OnBattery: onBattery,
			Percent:   percent,
			Charging:  charging,
			ACOnline:  acOnline,
		}, true
	}

	// ---- 会话控制 ----
	lockScreenNative = func() error { return win32.LockScreen() }
	suspendNative = func(hibernate bool) error { return win32.SuspendSystem(hibernate) }
	powerOffNative = func(reboot bool, delay int, force bool) error {
		return win32.ShutdownSystem(reboot, delay, force)
	}
	signOutNative = func(force bool) error { return win32.SignOut(force) }
	cancelShutdownNative = func() error { return win32.CancelShutdown() }

	// ---- 文件监听 ----
	watchDirNative = func(dir string, opts WatchOptions, fn func([]FsEvent)) func() {
		return win32.WatchDirectory(dir, opts.Recursive, func(evs []win32.WatchEvent) {
			out := make([]FsEvent, 0, len(evs))
			for _, e := range evs {
				out = append(out, FsEvent{
					Path:    e.Path,
					OldPath: e.OldPath,
					Change:  FsChange(e.Kind),
					IsDir:   e.IsDir,
				})
			}
			fn(out)
		})
	}

	// ---- 文件图标 ----
	fileIconNative = func(path string, size int) (*image.NRGBA, error) {
		return win32.FileIcon(path, size)
	}
	fileThumbNative = func(path string, w, h int) (*image.NRGBA, error) {
		return win32.FileThumbnail(path, w, h)
	}
	fileTypeNative = func(path string) string { return win32.FileTypeName(path) }

	// ---- 拖出 / 快捷方式 / 关联 ----
	dragFilesNative = func(hwnd uintptr, paths []string, x, y int32) error {
		return win32.DragFiles(hwnd, paths, x, y)
	}
	shortcutNative = func(lnkPath string, sc ShellLink) error {
		return win32.CreateShortcut(lnkPath, sc.Target, sc.Args, sc.WorkDir,
			sc.IconPath, sc.IconIndex, sc.Description, sc.Hotkey, sc.Minimized)
	}
	assocNative = func(ext, progID, command string) error {
		return win32.SetFileAssociation(ext, progID, command)
	}

	// ---- 富文本剪贴板 ----
	clipboardRichGetNative = func(format string) (string, bool) {
		name := "HTML Format"
		if format == ClipRTF {
			name = "Rich Text Format"
		}
		data, err := win32.ClipboardGetFormat(name)
		if err != nil || len(data) == 0 {
			return "", false
		}
		if format == ClipRTF {
			return string(data), true
		}
		return win32.ParseCF_HTML(data), true
	}
	clipboardRichSetNative = func(format, data string) bool {
		if format == ClipRTF {
			err := win32.ClipboardSetFormat("Rich Text Format", []byte(data), HTMLToPlainText(data))
			return err == nil
		}
		err := win32.ClipboardSetFormat("HTML Format",
			win32.CF_HTMLFragment(data), HTMLToPlainText(data))
		return err == nil
	}

	// ---- 键鼠模拟 ----
	sendKeyNative = func(mods uint32, vk uint32) error { return win32.SendKey(mods, vk) }
	sendTextNative = func(s string) error { return win32.SendText(s) }
	mouseMoveNative = func(x, y int) error { return win32.MouseMoveTo(x, y) }
	mouseClickNative = func(x, y int, button int) error { return win32.MouseClickAt(x, y, button) }
	mouseWheelNative = func(delta int) error { return win32.MouseWheel(delta) }

	// ---- 前台窗口 ----
	foregroundInfoNative = func() (WindowInfo, bool) {
		wi, ok := win32.ForegroundWindowInfo()
		if !ok {
			return WindowInfo{}, false
		}
		return WindowInfo{Handle: wi.Handle, Title: wi.Title, Process: wi.Process, PID: wi.PID}, true
	}

	// ---- 系统音量 ----
	volumeGetNative = func() (float32, bool) { return win32.MasterVolume() }
	volumeSetNative = func(v float32) error {
		if !win32.SetMasterVolume(v) {
			return errUnsupported("设置系统音量")
		}
		return nil
	}
	muteToggleNative = func() (bool, error) {
		muted, ok := win32.ToggleMasterMute()
		if !ok {
			return false, errUnsupported("系统静音")
		}
		return muted, nil
	}
}
