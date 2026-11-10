// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// 文件与 Shell 集成：文件图标 / 缩略图 / 拖出 / 快捷方式 / 打开终端 / 前台窗口。
//
//	icon, _ := ui.FileIcon(path, 32)            // 32×32 文件类型图标
//	thumb, _ := ui.FileThumbnail(path, 128, 128) // 缩略图（图片 / 视频 / 文档）
//	ui.DragFiles([]string{path}, -1, -1)         // 拖到资源管理器 / 聊天窗口
//	ui.OpenTerminal(dir)                          // 在该目录打开终端
//
// Windows 走 shell32 / OLE（SHGetFileInfo / IImageList / DoDragDrop / IShellLink），
// 其它平台尽量用系统命令（xdg-open / open / gio）实现同等效果，做不到的返回
// 「不支持」错误，业务侧用 HasCap 判断后隐藏入口。

// fileIconNative 文件类型图标（Windows: SHGetFileInfo + SHGetImageList）。
var fileIconNative func(path string, size int) (*image.NRGBA, error)

// fileThumbNative 缩略图（Windows: IThumbnailProvider / SHGetFileInfo 回退到图标）。
var fileThumbNative func(path string, w, h int) (*image.NRGBA, error)

// fileTypeNative 文件类型友好名（Windows: SHGetFileInfo.szTypeName）。
var fileTypeNative func(path string) string

// FileIcon 返回文件的类型图标（size 为像素边长，建议 16 / 24 / 32 / 48 / 256）。
// 文件不存在时返回「未知文件」图标；平台不支持时返回错误。
func FileIcon(path string, size int) (*image.NRGBA, error) {
	if size <= 0 {
		size = 32
	}
	if fileIconNative == nil {
		return nil, errUnsupported("文件图标")
	}
	return fileIconNative(path, size)
}

// FileThumbnail 返回文件缩略图（图片 / 视频 / PDF / Office 由系统缩略图提供者生成）。
// 生成失败时回退到 FileIcon 的放大版本，保证列表里总有图可画。
func FileThumbnail(path string, w, h int) (*image.NRGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, errUnsupported("缩略图")
	}
	if fileThumbNative != nil {
		if img, err := fileThumbNative(path, w, h); err == nil && img != nil {
			return img, nil
		}
	}
	return FileIcon(path, maxIntOf(w, h))
}

// FileTypeName 返回文件类型的友好名称（如「文本文档」「文件夹」）；未知返回空串。
func FileTypeName(path string) string {
	if fileTypeNative != nil {
		return fileTypeNative(path)
	}
	return ""
}

// dragFilesNative 开始 OLE 拖放（Windows: DoDragDrop）。
var dragFilesNative func(hwnd uintptr, paths []string, x, y int32) error

// DragFiles 用系统拖放把文件拖出去（阻塞到用户放下或按 ESC 取消）。
//
// x, y 为窗口内逻辑像素坐标（通常传触发拖动的指针位置）；传 -1, -1 表示用当前
// 鼠标位置。必须由一次真实鼠标按下/移动驱动 —— 系统拖放需要捕获鼠标。
func DragFiles(paths []string, x, y float32) error {
	if len(paths) == 0 {
		return errUnsupported("拖出文件")
	}
	if dragFilesNative == nil {
		return errUnsupported("拖出文件")
	}
	var hwnd uintptr
	if a := currentApp.Load(); a != nil {
		hwnd = a.HWND()
	}
	// 逻辑像素 → 物理像素（系统拖放用屏幕坐标）
	s := float32(1)
	if a := currentApp.Load(); a != nil {
		s = a.effScale()
	}
	px, py := int32(-1), int32(-1)
	if x >= 0 && y >= 0 {
		px, py = int32(x*s), int32(y*s)
	}
	return dragFilesNative(hwnd, paths, px, py)
}

// ShellLink Windows 快捷方式（.lnk）参数。Linux 上写 .desktop，macOS 写 .webloc / 软链。
type ShellLink struct {
	// Target 目标程序或文件（绝对路径）。
	Target string
	// Args 启动参数。
	Args []string
	// WorkDir 工作目录（空 = 目标所在目录）。
	WorkDir string
	// IconPath / IconIndex 图标（.ico / .exe / .dll 的索引）。
	IconPath  string
	IconIndex int
	// Description 描述 / 备注。
	Description string
	// Hotkey 快捷键描述（如 "Ctrl+Alt+K"，仅 Windows 生效）。
	Hotkey string
	// Minimized 启动时最小化。
	Minimized bool
}

// shortcutNative 平台原生实现（Windows: IShellLink + IPersistFile）。
var shortcutNative func(lnkPath string, sc ShellLink) error

// CreateShortcut 在 lnkPath 创建快捷方式（父目录不存在时自动创建）。
func CreateShortcut(lnkPath string, sc ShellLink) error {
	if lnkPath == "" || sc.Target == "" {
		return errUnsupported("创建快捷方式")
	}
	if dir := filepath.Dir(lnkPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if shortcutNative != nil {
		return shortcutNative(lnkPath, sc)
	}
	// 非 Windows：写 freedesktop .desktop（Linux）或 macOS .webloc / 符号链接
	if strings.HasSuffix(strings.ToLower(lnkPath), ".desktop") {
		return writeDesktopEntry(lnkPath, sc)
	}
	if runtime.GOOS == "darwin" && strings.HasSuffix(strings.ToLower(lnkPath), ".webloc") {
		return writeWebloc(lnkPath, sc.Target)
	}
	return errUnsupported("创建快捷方式")
}

// writeDesktopEntry 生成 freedesktop 桌面入口（Linux / BSD）。
func writeDesktopEntry(path string, sc ShellLink) error {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Version=1.0\n")
	name := sc.Description
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sc.Target), filepath.Ext(sc.Target))
	}
	b.WriteString("Name=" + name + "\n")
	b.WriteString("Exec=" + quoteExec(sc.Target, sc.Args) + "\n")
	if sc.WorkDir != "" {
		b.WriteString("Path=" + sc.WorkDir + "\n")
	}
	if sc.IconPath != "" {
		b.WriteString("Icon=" + sc.IconPath + "\n")
	}
	if sc.Description != "" {
		b.WriteString("Comment=" + sc.Description + "\n")
	}
	b.WriteString("Terminal=false\n")
	if sc.Minimized {
		b.WriteString("StartupNotify=false\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeWebloc 生成 macOS .webloc（本质是 plist）。
func writeWebloc(path, target string) error {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>URL</key><string>` + xmlEscapeText(target) + `</string></dict></plist>`
	return os.WriteFile(path, []byte(body), 0o644)
}

func xmlEscapeText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// quoteExec 拼 Exec= 行（含空格的参数加引号）。
func quoteExec(target string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteIfSpace(target))
	for _, a := range args {
		parts = append(parts, quoteIfSpace(a))
	}
	return strings.Join(parts, " ")
}

func quoteIfSpace(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

// SetFileAssociation 把扩展名关联到指定程序（仅 Windows 生效）。
//
//	ui.SetFileAssociation(".myapp", "MyApp.Document", `"C:\App\app.exe" "%1"`)
//
// 写的是 HKCU\Software\Classes（当前用户级，无需管理员），并在资源管理器里
// 通知刷新。非 Windows 返回不支持错误。
func SetFileAssociation(ext, progID, command string) error {
	if assocNative == nil {
		return errUnsupported("文件关联")
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return assocNative(ext, progID, command)
}

var assocNative func(ext, progID, command string) error

// WindowInfo 前台窗口信息。
type WindowInfo struct {
	// Handle 原生窗口句柄（Windows: HWND）。
	Handle uintptr
	// Title 窗口标题。
	Title string
	// Process 进程可执行文件名（如 "Code.exe"）。
	Process string
	// PID 进程 ID。
	PID uint32
}

// foregroundInfoNative 前台窗口查询（Windows 实现）。
var foregroundInfoNative func() (WindowInfo, bool)

// ForegroundWindowInfo 返回当前前台窗口信息（不支持时零值）。
func ForegroundWindowInfo() WindowInfo {
	if foregroundInfoNative != nil {
		if wi, ok := foregroundInfoNative(); ok {
			return wi
		}
	}
	return WindowInfo{}
}

// Activate 把本窗口拉到前台并抢焦点（Windows 有前台锁，必要时临时置顶绕过）。
func (a *App) Activate() {
	if a == nil || a.win == nil {
		return
	}
	a.win.Restore()
	a.win.SetVisible(true)
	if f, ok := a.win.(interface{ Activate() }); ok {
		f.Activate()
	}
	a.dirty = true
	a.Invalidate()
}

// OpenTerminal 在指定目录打开系统终端（目录为空时用用户主目录）。
// 找不到可用终端时返回错误。
func OpenTerminal(dir string) error {
	if dir == "" {
		dir = KnownFolder(DirHome)
	}
	if dir == "" {
		return errUnsupported("打开终端")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Windows Terminal 优先，回退到 cmd
		if p, err := exec.LookPath("wt.exe"); err == nil {
			cmd = exec.Command(p, "-d", dir)
		} else {
			cmd = exec.Command("cmd.exe", "/c", "start", "", "cmd.exe", "/k", "cd", "/d", dir)
		}
	case "darwin":
		cmd = exec.Command("open", "-a", "Terminal", dir)
	default:
		for _, name := range []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xfce4-terminal", "xterm"} {
			if p, err := exec.LookPath(name); err == nil {
				cmd = exec.Command(p, "--working-directory", dir)
				if name == "xterm" {
					cmd = exec.Command(p, "-e", "sh", "-c", `cd "`+dir+`" && exec $SHELL`)
				}
				break
			}
		}
	}
	if cmd == nil {
		return errUnsupported("打开终端")
	}
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func maxIntOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}
