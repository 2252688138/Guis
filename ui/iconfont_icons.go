// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

// 内置图标集：24×24 设计网格 + SVG path 数据（与 Lucide / Feather 同一套视觉语言：
// 2 单位线宽、圆端点圆转角）。全部自带路径，不依赖系统字体，也不带 .ttf 文件。
//
// 命名约定：名词用单数（file / folder），动作用动词（plus / search / trash），
// 方向用 -left/-right/-up/-down，实心变体加 -fill 后缀。
// 示例页按注册顺序铺网格，所以这里按「操作 → 导航 → 用户 → 文件 → 媒体 → 状态 → 商务」分组。

// registerBuiltinIcons 注册全部内置图标（init 时调用）。
func registerBuiltinIcons() {
	for _, def := range builtinIcons {
		RegisterIcon(def)
	}
}

// builtinIcons 内置图标定义表。
var builtinIcons = []IconDef{
	// ---- 常用操作 ----
	{Name: "plus", Label: "新增", Paths: []string{"M12 5v14M5 12h14"}},
	{Name: "minus", Label: "减少", Paths: []string{"M5 12h14"}},
	{Name: "close", Label: "关闭", Paths: []string{"M6 6l12 12M18 6L6 18"}},
	{Name: "check", Label: "完成", Paths: []string{"M4 12.5l5.5 5.5L20 6.5"}},
	{Name: "search", Label: "搜索", Paths: []string{"M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM16.2 16.2L21 21"}},
	{Name: "filter", Label: "筛选", Paths: []string{"M3 5h18l-7 8v6l-4 2v-8z"}},
	{Name: "settings", Label: "设置", Paths: []string{
		"M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z",
		"M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3",
		"M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M18.7 5.3l-2.1 2.1M7.4 16.6l-2.1 2.1",
	}},
	{Name: "more-h", Label: "更多（横）", Paths: []string{"M5 12h.01M12 12h.01M19 12h.01"}},
	{Name: "more-v", Label: "更多（竖）", Paths: []string{"M12 5v.01M12 12v.01M12 19v.01"}},
	{Name: "trash", Label: "删除", Paths: []string{
		"M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v6M14 11v6",
	}},
	{Name: "edit", Label: "编辑", Paths: []string{"M4 20h4L20 8l-4-4L4 16z", "M14 6l4 4"}},
	{Name: "copy", Label: "复制", Paths: []string{"M9 9h11v11H9z", "M15 9V4H4v11h5"}},
	{Name: "download", Label: "下载", Paths: []string{"M12 4v11M8 11l4 4 4-4M4 19h16"}},
	{Name: "upload", Label: "上传", Paths: []string{"M12 20V9M8 13l4-4 4 4M4 5h16"}},
	{Name: "refresh", Label: "刷新", Paths: []string{
		"M20.5 12a8.5 8.5 0 1 1-2.5-6", "M20 3.5v5h-5",
	}},
	{Name: "share", Label: "分享", Paths: []string{
		"M18 8a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
		"M6 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
		"M18 22a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
		"M8.6 10.6l6.8-4M8.6 13.4l6.8 4",
	}},
	{Name: "save", Label: "保存", Paths: []string{"M5 3h11l3 3v15H5z", "M8 3v6h8V3M8 21v-7h8v7"}},
	{Name: "print", Label: "打印", Paths: []string{"M6 9V3h12v6M6 18H4v-7h16v7h-2M8 14h8v7H8z"}},
	{Name: "link", Label: "链接", Paths: []string{
		"M10 13a4 4 0 0 0 6 .5l2-2a4 4 0 0 0-5.7-5.7L11 7",
		"M14 11a4 4 0 0 0-6-.5l-2 2a4 4 0 0 0 5.7 5.7L13 17",
	}},
	{Name: "lock", Label: "锁定", Paths: []string{"M5 11h14v10H5z", "M8 11V7a4 4 0 0 1 8 0v4", "M12 16v2"}},
	{Name: "unlock", Label: "解锁", Paths: []string{"M5 11h14v10H5z", "M9 11V7a3 3 0 0 1 6 0", "M12 16v2"}},

	// ---- 导航 ----
	{Name: "chevron-left", Label: "左折角", Paths: []string{"M15 5l-7 7 7 7"}},
	{Name: "chevron-right", Label: "右折角", Paths: []string{"M9 5l7 7-7 7"}},
	{Name: "chevron-up", Label: "上折角", Paths: []string{"M5 15l7-7 7 7"}},
	{Name: "chevron-down", Label: "下折角", Paths: []string{"M5 9l7 7 7-7"}},
	{Name: "arrow-left", Label: "左箭头", Paths: []string{"M20 12H4M10 6l-6 6 6 6"}},
	{Name: "arrow-right", Label: "右箭头", Paths: []string{"M4 12h16M14 6l6 6-6 6"}},
	{Name: "arrow-up", Label: "上箭头", Paths: []string{"M12 20V4M6 10l6-6 6 6"}},
	{Name: "arrow-down", Label: "下箭头", Paths: []string{"M12 4v16M6 14l6 6 6-6"}},
	{Name: "menu", Label: "菜单", Paths: []string{"M4 7h16M4 12h16M4 17h16"}},
	{Name: "grid", Label: "网格", Paths: []string{"M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z"}},
	{Name: "list", Label: "列表", Paths: []string{"M4 7h16M4 12h16M4 17h10"}},
	{Name: "external", Label: "外部打开", Paths: []string{"M14 4h6v6M20 4l-8 8M18 14v6H4V6h6"}},
	{Name: "home", Label: "主页", Paths: []string{"M3 11l9-7 9 7M6 9.5V21h12V9.5M10 21v-6h4v6"}},
	{Name: "compass", Label: "罗盘", Paths: []string{
		"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z", "M15.5 8.5l-2 5-5 2 2-5z",
	}},

	// ---- 用户与消息 ----
	{Name: "user", Label: "用户", Paths: []string{"M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0"}},
	{Name: "users", Label: "用户组", Paths: []string{
		"M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM2 21a7 7 0 0 1 14 0",
		"M17 3.5a4 4 0 0 1 0 7.5M18 15a6 6 0 0 1 4 6",
	}},
	{Name: "mail", Label: "邮件", Paths: []string{"M3 5h18v14H3zM3 6l9 7 9-7"}},
	{Name: "bell", Label: "通知", Paths: []string{"M6 10a6 6 0 0 1 12 0c0 5 2 6 2 6H4s2-1 2-6zM10 20a2 2 0 0 0 4 0"}},
	{Name: "eye", Label: "显示", Paths: []string{
		"M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6z",
		"M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
	}},
	{Name: "eye-off", Label: "隐藏", Paths: []string{
		"M3 3l18 18",
		"M10.6 6.2A9.7 9.7 0 0 1 12 6c6.5 0 10 6 10 6a17 17 0 0 1-3.3 3.7",
		"M6.6 7.6A17 17 0 0 0 2 12s3.5 6 10 6a9.6 9.6 0 0 0 4.3-1",
		"M9.9 9.9a3 3 0 0 0 4.2 4.2",
	}},
	{Name: "heart", Label: "喜欢", Paths: []string{"M12 20s-8-4.6-8-10a4.6 4.6 0 0 1 8-3 4.6 4.6 0 0 1 8 3c0 5.4-8 10-8 10z"}},
	{Name: "heart-fill", Label: "喜欢（实心）", Fill: true, Paths: []string{"M12 20s-8-4.6-8-10a4.6 4.6 0 0 1 8-3 4.6 4.6 0 0 1 8 3c0 5.4-8 10-8 10z"}},
	{Name: "star", Label: "收藏", Paths: []string{"M12 3l2.8 5.7 6.2.9-4.5 4.4 1.1 6.2L12 17.3 6.4 20.2l1.1-6.2L3 9.6l6.2-.9z"}},
	{Name: "star-fill", Label: "收藏（实心）", Fill: true, Paths: []string{"M12 3l2.8 5.7 6.2.9-4.5 4.4 1.1 6.2L12 17.3 6.4 20.2l1.1-6.2L3 9.6l6.2-.9z"}},
	{Name: "bookmark", Label: "书签", Paths: []string{"M6 3h12v18l-6-4.5L6 21z"}},
	{Name: "calendar", Label: "日历", Paths: []string{"M4 6h16v15H4zM4 10h16M8 3v4M16 3v4"}},
	{Name: "clock", Label: "时间", Paths: []string{"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 7v5l3.5 2"}},
	{Name: "message", Label: "消息", Paths: []string{"M21 12a8 8 0 0 1-11.6 7.1L4 21l1.9-5.4A8 8 0 1 1 21 12z"}},
	{Name: "message-fill", Label: "消息（实心）", Fill: true, Paths: []string{"M21 12a8 8 0 0 1-11.6 7.1L4 21l1.9-5.4A8 8 0 1 1 21 12z"}},
	{Name: "send", Label: "发送", Paths: []string{"M21 3L10 14M21 3l-7 18-4-7-7-4z"}},
	{Name: "tag", Label: "标签", Paths: []string{"M3 12V4h8l10 10-8 8z", "M7.5 7.5h.01"}},
	{Name: "shield", Label: "安全", Paths: []string{"M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z"}},
	{Name: "award", Label: "奖章", Paths: []string{
		"M12 15a6 6 0 1 0 0-12 6 6 0 0 0 0 12z",
		"M8.5 14l-1.5 7 5-2.5 5 2.5-1.5-7",
	}},

	// ---- 文件与数据 ----
	{Name: "file", Label: "文件", Paths: []string{"M6 3h8l4 4v14H6zM14 3v4h4M9 13h6M9 17h6"}},
	{Name: "file-plus", Label: "新建文件", Paths: []string{"M6 3h8l4 4v14H6zM14 3v4h4M12 11v6M9 14h6"}},
	{Name: "folder", Label: "文件夹", Paths: []string{"M3 6h6l2 2h10v11H3z"}},
	{Name: "folder-open", Label: "打开文件夹", Paths: []string{"M3 6h6l2 2h10v3H3zM3 11h18l-2 8H5z"}},
	{Name: "image", Label: "图片", Paths: []string{
		"M3 4h18v16H3zM3 16l5-5 4 4 3-3 6 6",
		"M16 9a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3z",
	}},
	{Name: "code", Label: "代码", Paths: []string{"M8 7l-5 5 5 5M16 7l5 5-5 5M13.5 4l-3 16"}},
	{Name: "terminal", Label: "终端", Paths: []string{"M3 4h18v16H3zM7 9l3 3-3 3M13 15h4"}},
	{Name: "database", Label: "数据库", Paths: []string{
		"M12 3c4.4 0 8 1.3 8 3s-3.6 3-8 3-8-1.3-8-3 3.6-3 8-3z",
		"M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6",
		"M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3",
	}},
	{Name: "server", Label: "服务器", Paths: []string{"M3 4h18v6H3zM3 14h18v6H3zM7 7h.01M7 17h.01"}},
	{Name: "cloud", Label: "云", Paths: []string{"M7 18a4 4 0 0 1 0-8 5.5 5.5 0 0 1 10.6-1A3.5 3.5 0 0 1 18 18z"}},
	{Name: "table", Label: "表格", Paths: []string{"M3 4h18v16H3zM3 9h18M3 14h18M9 9v11"}},
	{Name: "chart-bar", Label: "柱状图", Paths: []string{"M4 20V10M10 20V4M16 20v-7M22 20H2"}},
	{Name: "chart-pie", Label: "饼图", Paths: []string{"M12 3a9 9 0 1 0 9 9h-9z", "M14 3.5A9 9 0 0 1 20.5 10H14z"}},
	{Name: "book", Label: "文档", Paths: []string{
		"M4 5a3 3 0 0 1 3-2h5v18H7a3 3 0 0 1-3-2z",
		"M20 5a3 3 0 0 0-3-2h-5v18h5a3 3 0 0 0 3-2z",
	}},

	// ---- 媒体 ----
	{Name: "play", Label: "播放", Paths: []string{"M8 5l11 7-11 7z"}},
	{Name: "play-fill", Label: "播放（实心）", Fill: true, Paths: []string{"M8 5l11 7-11 7z"}},
	{Name: "pause", Label: "暂停", Paths: []string{"M8 5v14M16 5v14"}},
	{Name: "stop", Label: "停止", Paths: []string{"M6 6h12v12H6z"}},
	{Name: "volume", Label: "音量", Paths: []string{"M4 9h4l5-4v14l-5-4H4z", "M16 9a4 4 0 0 1 0 6"}},
	{Name: "volume-x", Label: "静音", Paths: []string{"M4 9h4l5-4v14l-5-4H4z", "M17 9l5 6M22 9l-5 6"}},
	{Name: "music", Label: "音乐", Paths: []string{
		"M9 18a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM12 15V4l9-2v11",
		"M18 16a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
	}},
	{Name: "video", Label: "视频", Paths: []string{"M3 6h13v12H3zM16 10l5-3v10l-5-3z"}},
	{Name: "camera", Label: "相机", Paths: []string{"M3 7h4l2-2h6l2 2h4v13H3z", "M12 17a4 4 0 1 0 0-8 4 4 0 0 0 0 8z"}},
	{Name: "mic", Label: "麦克风", Paths: []string{
		"M9 5a3 3 0 0 1 6 0v6a3 3 0 0 1-6 0z",
		"M5 11a7 7 0 0 0 14 0M12 18v3M8 21h8",
	}},
	{Name: "headphones", Label: "耳机", Paths: []string{
		"M4 15v-3a8 8 0 0 1 16 0v3",
		"M4 15h3v5H5a1 1 0 0 1-1-1zM20 15h-3v5h2a1 1 0 0 0 1-1z",
	}},
	{Name: "sun", Label: "浅色", Paths: []string{
		"M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10z",
		"M12 1v3M12 20v3M1 12h3M20 12h3M4 4l2 2M18 18l2 2M20 4l-2 2M6 18l-2 2",
	}},
	{Name: "moon", Label: "深色", Paths: []string{"M20 15.5A9 9 0 1 1 8.5 4 7 7 0 0 0 20 15.5z"}},

	// ---- 状态与提示 ----
	{Name: "info", Label: "信息", Paths: []string{"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 11v5M12 7.5h.01"}},
	{Name: "help", Label: "帮助", Paths: []string{
		"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z",
		"M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.7.4-1 1-1 1.7M12 17h.01",
	}},
	{Name: "alert-triangle", Label: "警告", Paths: []string{"M12 3l9.5 17H2.5z", "M12 9v5M12 17.5h.01"}},
	{Name: "alert-circle", Label: "错误", Paths: []string{"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 7v6M12 16.5h.01"}},
	{Name: "check-circle", Label: "成功", Paths: []string{"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM8 12l3 3 5-6"}},
	{Name: "x-circle", Label: "失败", Paths: []string{"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM9 9l6 6M15 9l-6 6"}},
	{Name: "zap", Label: "闪电", Paths: []string{"M13 2L4 14h7l-1 8 9-12h-7z"}},
	{Name: "wifi", Label: "无线", Paths: []string{"M2 8.5a15 15 0 0 1 20 0M5.5 12a10 10 0 0 1 13 0M9 15.5a5 5 0 0 1 6 0M12 19h.01"}},
	{Name: "battery", Label: "电量", Paths: []string{"M2 7h16v10H2zM20 10v4", "M5 10h6v4H5z"}},
	{Name: "globe", Label: "全球", Paths: []string{
		"M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18",
		"M12 3c3 3.5 3 14 0 18M12 3c-3 3.5-3 14 0 18",
	}},
	{Name: "map-pin", Label: "位置", Paths: []string{
		"M12 21s7-6 7-11a7 7 0 1 0-14 0c0 5 7 11 7 11z",
		"M12 13a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z",
	}},

	// ---- 商务与场景 ----
	{Name: "shopping-cart", Label: "购物车", Paths: []string{
		"M2 3h3l2.5 12h11L21 7H6",
		"M9 21a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zM18 21a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3z",
	}},
	{Name: "credit-card", Label: "银行卡", Paths: []string{"M2 5h20v14H2zM2 10h20M6 15h4"}},
	{Name: "package", Label: "包裹", Paths: []string{"M12 2l9 5v10l-9 5-9-5V7z", "M3 7l9 5 9-5M12 12v10"}},
	{Name: "truck", Label: "配送", Paths: []string{
		"M1 5h13v11H1zM14 9h4l3 3v4h-7z",
		"M6 19a2 2 0 1 0 0-4 2 2 0 0 0 0 4zM18 19a2 2 0 1 0 0-4 2 2 0 0 0 0 4z",
	}},
	{Name: "gift", Label: "礼物", Paths: []string{
		"M3 9h18v12H3zM3 13h18M12 9v12",
		"M8 9a2.5 2.5 0 1 1 0-5c2 0 4 5 4 5M16 9a2.5 2.5 0 1 0 0-5c-2 0-4 5-4 5",
	}},
	{Name: "flag", Label: "旗帜", Paths: []string{"M5 21V3M5 4h12l-2 4 2 4H5"}},
	{Name: "trophy", Label: "奖杯", Paths: []string{
		"M7 4h10v5a5 5 0 0 1-10 0zM7 6H4v2a3 3 0 0 0 3 3M17 6h3v2a3 3 0 0 1-3 3",
		"M9 20h6M12 14v6",
	}},
	{Name: "palette", Label: "调色板", Paths: []string{
		"M12 3a9 9 0 1 0 0 18c1.7 0 2-1 1.3-2-.8-1.2.2-2.5 1.7-2.5H18a3 3 0 0 0 3-3A9 9 0 0 0 12 3z",
		"M7.5 9.5h.01M12 7h.01M16.5 9.5h.01",
	}},
	{Name: "rocket", Label: "发布", Paths: []string{
		"M5 15l-2 6 6-2",
		"M9 19c-1-3 0-7 3-10 3-3 7-4 9-4 0 2-1 6-4 9-3 3-7 4-10 3z",
		"M14 7h.01",
	}},
}
