// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// gallery 的唯一入口：桌面 / 浏览器 / 安卓三端共用本文件。
//
// 没有 main_android.go 之类的平台入口：
// 平台差异（Android 的 NativeActivity 回调、浏览器的字体自举）已下沉到 ui 包的
// ui.Main（见 ui/main_entry_desktop.go / main_entry_android.go / boot_wasm.go），
// 业务侧永远只有「一份 opts + 一份 View」，且两者都在 init() 里注册——
// Android 的 NativeActivity 在 dlopen 之后立刻回调 ANativeActivity_onCreate，
// 早于 main()，只在 main() 里准备参数会错过回调（表现为全黑屏）。
//
//	go run ./cmd/gallery                       → 桌面（Win32 / WGL）
//	GOOS=js GOARCH=wasm go build ./cmd/gallery → Web（canvas + WebGL 2）
//	./build-android-apk.ps1                    → 安卓 APK（NativeActivity + EGL/GLES3）
//
// 界面本体（自绘标题栏 + 左侧导航 + 内容区 + 浮层）在 pages/shell.go 的 View()，
// 按平台能力与窗口宽度自适应：宽屏保留完整标题栏与 232px 侧栏，
// 窄屏（手机竖屏 / 拖窄的窗口）侧栏收窄、隐藏菜单栏，其余布局与条目完全一致。
//
// 各界面案例已拆分到 ./pages 子包（package pages），并按组件类型聚合为 9 个分类页：
//
//	pages/state.go          共享状态、页面路由（Content）、全局动作（SwitchCase / SetTheme）
//	pages/shell.go          统一壳层（View）、标题栏、菜单栏、左侧导航
//	pages/case_groups.go    分类页组合（基础 / 表单 / 列表 / 媒体 / 布局 / 图表 / 文本 / 系统）
//	pages/case_controls.go  基础控件          pages/case_settings.go  开关 / 单选 / 滑块
//	pages/case_form.go      表单与弹窗        pages/case_suite.go     高级组件卡片（按类型组合）
//	pages/case_table.go     数据表格          pages/case_chat.go      聊天列表
//	pages/case_lists.go     列表与表格页（页内页签：文件树 / 图文卡片 / 列表 / 社区帖子 / 分页）
//	pages/case_video.go     视频播放          pages/case_music.go     音频播放
//	pages/case_layouts.go   布局实验室        pages/case_dashboard.go 仪表盘
//	pages/case_charts.go    图表              pages/case_markdown.go  Markdown 渲染
//	pages/case_system.go    系统能力          pages/case_webview.go   WebView 浏览器
//	pages/case_office.go    办公文档（Excel / Word / PDF 读写与渲染）
//
// 登录 / 注册等业务页面已移除：本程序只做组件演示，不再模拟登录流程。
//
// 运行：go run ./cmd/gallery
// 布局调试：go run ./cmd/gallery --dump dump.txt --case table
package main

import (
	"os"
	"strconv"
	"strings"

	"guis/cmd/gallery/pages"
	"guis/internal/version"
	"guis/ui"
)

// init 注册入口：必须放在 init 而不是 main。
//
// Android 的 NativeActivity 在 dlopen 期间就回调 ANativeActivity_onCreate，
// 那时 main() 可能还没被调度；把「窗口选项 + 整页构建函数」提前注册好，
// 三端才能共用同一个 main()（桌面 / 浏览器直接调 Main，安卓由 ui 包接回调）。
func init() {
	// 页面内的轻提示统一走组件库 Toast（pages 包通过钩子回调，不直接依赖 ui.ToastShow）。
	pages.Toast = ui.ToastShow
	// 多语言：载入 gallery 自己的 zh-CN / en-US 文案（顶部菜单「语言」用它）。
	// 必须在 ui.Run 之前，ui 随后的 AutoLocale 才能按「偏好 > 系统 > en-US」选语言。
	pages.InitLocales()

	ui.SetEntry(func() (ui.Options, func() *ui.Node) {
		return options(), pages.View
	})
}

func main() {
	// 组件栅格缓存「能缓则缓」：纯静态、不透明底色、无描边/阴影的最外层容器
	// 首次绘制后快照成纹理，后续帧一张图贴回。子项含交互控件 / 动画 / 图片 /
	// 图表 / 视频 / 网页的会被自动排除，不会画错；GUIS_CACHE=0 可关闭。
	ui.SetCacheLayers(os.Getenv("GUIS_CACHE") != "0")
	// 三端同一个入口：桌面直接跑，浏览器先自举字体，安卓等 NativeActivity 回调。
	ui.Main()
}

// options 组装窗口选项（参数 / URL 查询串在首次启动时解析一次）。
func options() ui.Options {
	opts := ui.Options{
		Title:      "GUIS 组件演示 " + version.String(),
		Width:      1180,
		Height:     760,
		NoTitleBar: true,
		// 窗口圆角：Win11 走 DWM 原生圆角（平滑），低版本回退 SetWindowRgn 区域圆角
		CornerRadius: 5,
		// FontFamily 预设值（按子串匹配、区分大小写；未命中的任意名称一律回落微软雅黑）：
		//   空 / "Microsoft YaHei" → msyh.ttc（微软雅黑，粗体 msyhbd.ttc）  默认
		//   "SimSun" / 宋体 → simsun.ttc          "SimHei" / 黑体 → simhei.ttf
		//   "Consolas" / Mono → consola.ttf       "Segoe UI" / Segoe → segoeui.ttf
		//   "Symbol" → seguisym.ttf               "Emoji" → seguiemj.ttf
		FontFamily: "Mtrueicrosoft YaHei UI", // 备注：含 "YaHei" 即命中雅黑（多了 true 也不影响匹配）
		Background: "#070a1215",
	}

	// 多实例：不再做单实例互斥，连点运行 / 多次 go run 都会开出独立窗口，
	// 各自拥有独立的页面状态与滚动位置。
	applyArgs(os.Args[1:], &opts)

	// Web 端用 URL 参数指定初始页面：?case=video / ?case=webview
	// （桌面端没有 URL，StartupQuery 恒返回空串）
	if c := ui.StartupQuery("case"); c != "" {
		pages.Current = c
	}
	// Web 端可直接指定要播放的媒体直链：?media=https://…/demo.mp4
	if u := ui.StartupQuery("media"); u != "" {
		pages.SetVideoURL(u)
		pages.Current = "video"
	}
	// 浏览器不支持 HLS（.m3u8 需要 hls.js），Web 端默认换成 mp4 示例
	if ui.BackendName() == "browser" && strings.HasSuffix(strings.ToLower(pages.VideoURL()), ".m3u8") {
		pages.SetVideoURL("https://www.w3schools.com/html/mov_bbb.mp4")
	}
	return opts
}

// applyArgs 解析命令行参数（桌面启动 / 调试用；移动端没有命令行，解析不到就跳过）。
func applyArgs(args []string, opts *ui.Options) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dump":
			if opts != nil && i+1 < len(args) {
				i++
				opts.DumpLayout = args[i]
			}
		case "--md-tab":
			// 调试用：Markdown 示例页默认页签（gfm / ext / code / long）
			if i+1 < len(args) {
				i++
				pages.MdTab = args[i]
			}
		case "--case":
			if i+1 < len(args) {
				i++
				pages.Current = args[i]
			}
		case "--api-doc":
			// 调试用：API 文档页的初始文档，例如 --api-doc 20-video.md
			if i+1 < len(args) {
				i++
				pages.APIInitialDoc = args[i]
			}
		case "--fullscreen":
			// 启动即全屏（调试 / 工控机 / 展台场景，也用于验证全屏渲染）
			pages.RequestFullscreen()
		case "--menu":
			// 调试用：启动即展开指定顶级菜单（索引从 0 开始）
			if i+1 < len(args) {
				i++
				if n, err := strconv.Atoi(args[i]); err == nil {
					pages.TitleMenu.Open = n
				}
			}
		case "--sub":
			// 调试用：展开子菜单路径，例如 --sub 0,1
			if i+1 < len(args) {
				i++
				for _, p := range strings.Split(args[i], ",") {
					if n, err := strconv.Atoi(p); err == nil {
						pages.TitleMenu.Sub = append(pages.TitleMenu.Sub, n)
					}
				}
			}
		}
	}
}
