// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 平台能力查询：同一套 UI 代码要在 Windows / Linux / macOS / Web / 移动端编译运行，
// 各端能力必然不齐（内嵌浏览器、托盘、媒体解码、全局热键…）。
//
// 约定：**符号永不缺失**（没有实现就给空实现），能力用 HasCap 查询，
// 业务代码据此决定是否显示入口或走降级分支：
//
//	if ui.HasCap(ui.CapVideo) {
//		p, _ := ui.NewMediaPlayer(path)
//		// …
//	} else {
//		// 显示占位卡片
//	}
type Cap int

const (
	// CapVideo 视频解码与绘制。
	CapVideo Cap = iota
	// CapAudio 音频播放。
	CapAudio
	// CapWebView 内嵌浏览器（系统 WebView）。
	CapWebView
	// CapTray 系统托盘图标与菜单。
	CapTray
	// CapClipboard 剪贴板读写。
	CapClipboard
	// CapDialogs 文件/文件夹选择对话框。
	CapDialogs
	// CapHotkey 全局热键。
	CapHotkey
	// CapDragDrop 文件拖入窗口。
	CapDragDrop
	// CapCompose 窗口级 GPU 合成（如 Windows DirectComposition）。
	CapCompose
	// CapSystemDark 读取系统深色模式偏好。
	CapSystemDark
	// CapKeepAwake 阻止系统休眠/关屏。
	CapKeepAwake
	// CapRecycleBin 移入回收站。
	CapRecycleBin
	// CapTouch 触摸/手势输入。
	CapTouch
	// CapPrint 系统打印对话框与 GDI 输出。
	CapPrint
	// CapScreenCapture 屏幕截图 / 吸管取色。
	CapScreenCapture
	// CapClipboardWatch 剪贴板变化监听。
	CapClipboardWatch
	// CapMultiMonitor 多显示器枚举与工作区。
	CapMultiMonitor
	// CapSingleInstance 单实例锁 + 二次启动参数。
	CapSingleInstance
	// CapModernNotify 应用内通知中心（按钮/进度）。
	CapModernNotify
	// CapAutoStart 开机自启（当前用户级注册）。
	CapAutoStart
	// CapFileWatch 文件 / 目录变更监听（WatchDir / WatchFile）。
	CapFileWatch
	// CapKnownFolders 系统标准目录（桌面 / 下载 / 文档 / 图片 / 音乐 / 视频 / 临时）。
	CapKnownFolders
	// CapDragOut 把文件拖出到其它应用（资源管理器 / 聊天窗口）。
	CapDragOut
	// CapFileIcon 文件类型图标与缩略图。
	CapFileIcon
	// CapNativeDialog 原生消息框（崩溃兜底 / 无 GL 上下文时仍然可用）。
	CapNativeDialog
	// CapSystemInfo 系统信息（UI 语言 / 用户名 / 主机名 / OS 版本 / 提权）。
	CapSystemInfo
	// CapTaskbar 任务栏集成（进度 / 角标 / 闪烁 / 缩略图工具栏）。
	CapTaskbar
	// CapWindowFX 窗口特效（透明度 / 鼠标穿透 / 置底 / 深色标题栏 / 材质 / 防截屏）。
	CapWindowFX
	// CapClipboardRich 富文本剪贴板（HTML / RTF）。
	CapClipboardRich
	// CapPowerStatus 电源与电池状态。
	CapPowerStatus
	// CapSession 会话控制（锁屏 / 注销 / 关机 / 重启 / 休眠）。
	CapSession
	// CapInputSim 键鼠模拟输入（SendInput）。
	CapInputSim
	// CapShellLink 创建快捷方式与文件关联。
	CapShellLink
)

// CapLast 最后一个能力值（遍历 / 测试用；新增能力时同步更新）。
const CapLast = CapShellLink

// capNames 能力名称（调试与界面展示用）。
var capNames = map[Cap]string{
	CapVideo:          "视频解码",
	CapAudio:          "音频播放",
	CapWebView:        "内嵌浏览器",
	CapTray:           "系统托盘",
	CapClipboard:      "剪贴板",
	CapDialogs:        "文件对话框",
	CapHotkey:         "全局热键",
	CapDragDrop:       "文件拖入",
	CapCompose:        "GPU 合成",
	CapSystemDark:     "深色模式检测",
	CapKeepAwake:      "禁止休眠",
	CapRecycleBin:     "回收站",
	CapTouch:          "触摸输入",
	CapPrint:          "打印",
	CapScreenCapture:  "屏幕捕获",
	CapClipboardWatch: "剪贴板监听",
	CapMultiMonitor:   "多显示器",
	CapSingleInstance: "单实例锁",
	CapModernNotify:   "通知中心",
	CapAutoStart:      "开机自启",
	CapFileWatch:      "文件监听",
	CapKnownFolders:   "标准目录",
	CapDragOut:        "文件拖出",
	CapFileIcon:       "文件图标",
	CapNativeDialog:   "原生对话框",
	CapSystemInfo:     "系统信息",
	CapTaskbar:        "任务栏集成",
	CapWindowFX:       "窗口特效",
	CapClipboardRich:  "富文本剪贴板",
	CapPowerStatus:    "电源状态",
	CapSession:        "会话控制",
	CapInputSim:       "输入模拟",
	CapShellLink:      "快捷方式",
}

// CapName 返回能力的中文名称。
func CapName(c Cap) string {
	if n, ok := capNames[c]; ok {
		return n
	}
	return "未知能力"
}

// Capabilities 当前平台支持的能力列表（由各平台文件提供）。
func Capabilities() []Cap { return platformCaps() }

// HasCap 当前平台是否支持某能力。
func HasCap(c Cap) bool {
	for _, v := range platformCaps() {
		if v == c {
			return true
		}
	}
	return false
}

// UnsupportedCaps 当前平台缺失的能力（便于界面上统一提示）。
func UnsupportedCaps() []Cap {
	var out []Cap
	for c := CapVideo; c <= CapLast; c++ {
		if !HasCap(c) {
			out = append(out, c)
		}
	}
	return out
}
