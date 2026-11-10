// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package text

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// 字体文件发现与挑选（平台无关部分）。
//
// 类 Unix 端没有 GDI 枚举、也没有浏览器端那种注册表，因此统一走「递归扫目录 +
// 按文件名关键词匹配」：与 Windows 端「按文件名猜 msyh.ttc」的策略一致，
// 零依赖、零 cgo，且逻辑纯 Go 可单测（见 fontpick_test.go）。
// 各平台的目录列表与 mmap 实现在 fontplat_<os>.go / fontscan.go 里。

var (
	scanMu    sync.Mutex
	scanCache = map[string][]string{} // 目录集合 -> 字体文件列表
)

// scanFontFiles 递归扫描字体目录，返回 .ttf/.otf/.ttc/.otc 文件路径。
// 不存在的目录与权限错误静默跳过；结果排序，保证候选顺序稳定可测。
func scanFontFiles(dirs []string) []string {
	key := strings.Join(dirs, string(os.PathListSeparator))
	scanMu.Lock()
	if v, ok := scanCache[key]; ok {
		scanMu.Unlock()
		return v
	}
	scanMu.Unlock()

	var out []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // 权限不足等一律跳过，不影响其它目录
			}
			if d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".otc":
				out = append(out, path)
			}
			return nil
		})
	}
	sort.Strings(out)

	scanMu.Lock()
	scanCache[key] = out
	scanMu.Unlock()
	return out
}

// pickFonts 在已扫描的文件里按关键词顺序挑选：第 1 个关键词的全部命中排前面，
// 第 2 个关键词次之……（同一关键词内保持路径字典序）。
//
// 关键词对**文件名（不含目录）**做不区分大小写的子串匹配，例如
// "NotoSansCJK" 能命中 NotoSansCJK-Regular.ttc。
func pickFonts(files []string, keywords ...string) []string {
	var out []string
	seen := make(map[string]bool, len(files))
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		low := strings.ToLower(kw)
		for _, p := range files {
			if seen[p] {
				continue
			}
			if strings.Contains(strings.ToLower(filepath.Base(p)), low) {
				out = append(out, p)
				seen[p] = true
			}
		}
	}
	return out
}

// concat 串联两组候选（Go 的切片不能用 + 拼接）。
// 返回新切片，不改动入参，便于测试比较顺序。
func concat(a, b []string) []string {
	switch {
	case len(a) == 0:
		return b
	case len(b) == 0:
		return a
	}
	out := make([]string, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// withBold 粗体优先：**在每个关键词分组内部**把文件名含 Bold/Heavy/Semibold 的提前，
// 组与组之间仍保持关键词优先级。
//
// 旧实现把所有 bold 文件统一提到最前，结果 Roboto-Bold（只有拉丁字形）会排到
// NotoSansCJK-Regular 前面：粗体中文全部命中不到字形，只画出 0.9×字号的空白宽度
// （表现就是「胶囊有宽度但没有文字」）。
func withBold(files []string, bold bool, keywords []string) []string {
	if !bold {
		return pickFonts(files, keywords...)
	}
	out := make([]string, 0, len(files))
	seen := make(map[string]bool, len(files))
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		low := strings.ToLower(kw)
		var regular, bolds []string
		for _, p := range files {
			if seen[p] {
				continue
			}
			name := strings.ToLower(filepath.Base(p))
			if !strings.Contains(name, low) {
				continue
			}
			seen[p] = true
			if strings.Contains(name, "bold") || strings.Contains(name, "heavy") || strings.Contains(name, "semibold") {
				bolds = append(bolds, p)
			} else {
				regular = append(regular, p)
			}
		}
		out = append(out, bolds...)
		out = append(out, regular...)
	}
	return out
}

// 常用字族关键词（按优先级），供各平台候选复用。
var (
	sansCJKKeywords = []string{
		"NotoSansCJK", "NotoSansSC", "NotoSansTC", "SourceHanSans",
		"WenQuanYiZenHei", "WenQuanYiMicroHei", "DroidSansFallback",
		"PingFang", "HiraginoSansGB", "STHeiti", "HeitiSC", "MicrosoftYaHei",
	}
	sansLatinKeywords = []string{
		"NotoSans-Regular", "DejaVuSans", "LiberationSans", "FreeSans", "Arial",
	}
	monoKeywords = []string{
		"NotoSansMono", "DejaVuSansMono", "LiberationMono", "UbuntuMono",
		"SourceCodePro", "Menlo", "Monaco", "Consolas",
	}
	serifKeywords = []string{
		"NotoSerifCJK", "SourceHanSerif", "NotoSerif", "DejaVuSerif",
		"LiberationSerif", "TimesNewRoman", "Times",
	}
	emojiKeywords = []string{"NotoColorEmoji", "AppleColorEmoji", "EmojiOne", "Twemoji"}
)
