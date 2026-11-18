// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// toolPath 定位外部工具（ffmpeg / ffprobe 等）：显式路径 > 程序目录 > PATH。
//
// 顺序刻意把「程序目录」放在 PATH 前面：安装版会把 ffmpeg 随安装包解到
// <安装目录>\ffmpeg（见 installer/guis.nsi），这样即使系统里没装 FFmpeg、
// 也没配 PATH，在线音乐 / 视频解码依然可用；开发机装了系统 FFmpeg 时，
// exe 同级的 dist\ffmpeg 优先，行为与发布版一致，便于复现问题。
func toolPath(explicit, name string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}
	// 程序目录：<exe 同级>、<exe>\bin、<exe>\ffmpeg、<exe>\tools
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, sub := range []string{"", "bin", "ffmpeg", "tools"} {
			for _, cand := range toolCandidates(name) {
				p := filepath.Join(dir, sub, cand)
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					return p
				}
			}
		}
	}
	// PATH（LookPath 已按 PATHEXT 处理扩展名）
	for _, cand := range toolCandidates(name) {
		if p, err := exec.LookPath(cand); err == nil {
			return p
		}
	}
	return ""
}

// toolCandidates 工具名的候选文件名（带/不带 .exe，Windows 下都试一次）。
func toolCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		return []string{name}
	}
	return []string{name, name + ".exe"}
}
