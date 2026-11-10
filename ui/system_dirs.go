// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"path/filepath"
	"runtime"
)

// 系统标准目录。
//
// 业务代码里以前只能自己拼 os.UserConfigDir() + "Desktop"，各平台路径规则不同
//（Windows 走已知文件夹 API、Linux 走 XDG、macOS 走 ~/Library），拼错就是
//「下载目录不存在」。这里统一成一个入口，找不到时返回空串（绝不 panic）。
//
//	ui.KnownFolder(ui.DirDownloads)   // "C:\\Users\\me\\Downloads"
//	ui.EnsureFolder(ui.DirCache)      // 不存在就创建
//
// Windows 用 SHGetKnownFolderPath（支持「下载」被重定向到 D 盘这类情况），
// 其它平台按 XDG / Apple 约定回退到用户目录下的同名子目录。
type KnownDir int

const (
	// DirHome 用户主目录。
	DirHome KnownDir = iota
	// DirDesktop 桌面。
	DirDesktop
	// DirDocuments 文档。
	DirDocuments
	// DirDownloads 下载。
	DirDownloads
	// DirPictures 图片。
	DirPictures
	// DirMusic 音乐。
	DirMusic
	// DirVideos 视频。
	DirVideos
	// DirConfig 用户配置目录（Windows: %APPDATA%，Linux: ~/.config，macOS: ~/Library/Application Support）。
	DirConfig
	// DirCache 缓存目录（Windows: %LOCALAPPDATA%，Linux: ~/.cache，macOS: ~/Library/Caches）。
	DirCache
	// DirData 本地数据目录（Windows: %LOCALAPPDATA%，Linux: ~/.local/share，macOS: ~/Library/Application Support）。
	DirData
	// DirTemp 临时目录。
	DirTemp
	// DirPrograms 程序安装目录（Windows: Program Files，其它：/usr 或 /Applications）。
	DirPrograms
	// DirExecutable 当前可执行文件所在目录（配置文件随 exe 走的绿色版常用）。
	DirExecutable
)

var knownDirLabels = [...]string{
	DirHome:       "用户目录",
	DirDesktop:    "桌面",
	DirDocuments:  "文档",
	DirDownloads:  "下载",
	DirPictures:   "图片",
	DirMusic:      "音乐",
	DirVideos:     "视频",
	DirConfig:     "配置",
	DirCache:      "缓存",
	DirData:       "数据",
	DirTemp:       "临时",
	DirPrograms:   "程序",
	DirExecutable: "程序所在目录",
}

// KnownDirName 目录类别的中文名（界面展示 / 日志用）。
func KnownDirName(d KnownDir) string {
	if int(d) >= 0 && int(d) < len(knownDirLabels) {
		return knownDirLabels[d]
	}
	return "未知目录"
}

// knownFolderNative 平台原生实现（Windows 走 SHGetKnownFolderPath）。
// 为空或返回 false 时回退到 portableFolder。
var knownFolderNative func(d KnownDir) (string, bool)

// KnownFolder 返回标准目录的绝对路径；无法确定时返回空串。
func KnownFolder(d KnownDir) string {
	if knownFolderNative != nil {
		if p, ok := knownFolderNative(d); ok && p != "" {
			return filepath.Clean(p)
		}
	}
	if p := portableFolder(d); p != "" {
		return filepath.Clean(p)
	}
	return ""
}

// KnownFolderExists 标准目录是否存在。
func KnownFolderExists(d KnownDir) bool {
	p := KnownFolder(d)
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// EnsureFolder 返回标准目录路径，不存在时按需创建（权限不足时返回错误）。
func EnsureFolder(d KnownDir) (string, error) {
	p := KnownFolder(d)
	if p == "" {
		return "", os.ErrNotExist
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		return p, err
	}
	return p, nil
}

// SubFolder 返回标准目录下的子目录（例如 SubFolder(DirConfig, "MyApp")）。
// 只拼路径，不创建。
func SubFolder(d KnownDir, sub string) string {
	base := KnownFolder(d)
	if base == "" {
		return ""
	}
	if sub == "" {
		return base
	}
	return filepath.Join(base, sub)
}

// portableFolder 跨平台回退：优先环境变量（XDG / 系统约定），再拼用户目录。
//
// 单独抽成函数是为了让非 Windows 端与「Windows 原生 API 失败」两条路径共用
// 同一套规则，避免两处各写一遍容易分叉。
func portableFolder(d KnownDir) string {
	home, _ := os.UserHomeDir()
	sub := func(name string) string {
		if home == "" {
			return ""
		}
		return filepath.Join(home, name)
	}
	switch d {
	case DirHome:
		return home
	case DirDesktop:
		if p := xdgUserDir("DESKTOP"); p != "" {
			return p
		}
		return sub("Desktop")
	case DirDocuments:
		if p := xdgUserDir("DOCUMENTS"); p != "" {
			return p
		}
		return sub("Documents")
	case DirDownloads:
		if p := xdgUserDir("DOWNLOAD"); p != "" {
			return p
		}
		return sub("Downloads")
	case DirPictures:
		if p := xdgUserDir("PICTURES"); p != "" {
			return p
		}
		return sub("Pictures")
	case DirMusic:
		if p := xdgUserDir("MUSIC"); p != "" {
			return p
		}
		return sub("Music")
	case DirVideos:
		if p := xdgUserDir("VIDEOS"); p != "" {
			return p
		}
		if runtime.GOOS == "darwin" {
			return sub("Movies")
		}
		return sub("Videos")
	case DirConfig:
		if p, err := os.UserConfigDir(); err == nil && p != "" {
			return p
		}
		return sub(".config")
	case DirCache:
		if p, err := os.UserCacheDir(); err == nil && p != "" {
			return p
		}
		return sub(".cache")
	case DirData:
		// data 与 config 在 Windows / macOS 上是同一个根（%LOCALAPPDATA% / Application Support），
		// Linux 走 XDG_DATA_HOME。
		if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" {
			if p := os.Getenv("XDG_DATA_HOME"); p != "" {
				return p
			}
			return sub(".local/share")
		}
		if p, err := os.UserConfigDir(); err == nil && p != "" {
			return p
		}
		return sub("Library/Application Support")
	case DirTemp:
		return os.TempDir()
	case DirPrograms:
		switch runtime.GOOS {
		case "windows":
			if p := os.Getenv("ProgramFiles"); p != "" {
				return p
			}
			return `C:\Program Files`
		case "darwin":
			return "/Applications"
		}
		return "/usr"
	case DirExecutable:
		if exe, err := os.Executable(); err == nil {
			return filepath.Dir(exe)
		}
	}
	return ""
}

// xdgUserDir 读 ~/.config/user-dirs.dirs 里的 XDG_xxx_DIR 项（Linux 中文系统的
// 「下载」目录名是本地化的，不能写死 ~/Downloads）。
func xdgUserDir(key string) string {
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		return ""
	}
	if p := os.Getenv("XDG_" + key + "_DIR"); p != "" {
		return p
	}
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(cfg, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	home, _ := os.UserHomeDir()
	prefix := "XDG_" + key + "_DIR="
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if len(line) < len(prefix) || line[:len(prefix)] != prefix {
			continue
		}
		v := trimSpace(line[len(prefix):])
		v = trimQuote(v, '"')
		// 文件里是 "$HOME/下载" 这种形式，需要展开 $HOME
		if len(v) >= 5 && v[:5] == "$HOME" {
			v = home + v[5:]
		}
		if v != "" {
			return v
		}
	}
	return ""
}

// trimSpace 去掉首尾空白（XDG 文件只有几十行，不值得为此引入额外依赖）。
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// trimQuote 去掉首尾成对的引号。
func trimQuote(s string, q byte) string {
	if len(s) >= 2 && s[0] == q && s[len(s)-1] == q {
		return s[1 : len(s)-1]
	}
	return s
}
