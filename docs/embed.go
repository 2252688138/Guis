// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package docs 提供随二进制一起分发的内置文档。
//
// 文档本体是 Markdown（见 api/ 目录），通过 go:embed 打进可执行文件：
// 桌面 / 浏览器 / 安卓三端都能直接读，不依赖外部文件、不怕安装包漏拷。
// gallery 的「帮助 → API 文档」页面就是逐篇渲染这里的文件。
package docs

import "embed"

// API 内嵌的 API 手册（api/*.md，按文件名前缀排序）。
//
// 新增一篇文档只需往 api/ 目录丢一个 NN-xxx.md（首行 `# 标题`，
// 可选的 HTML 注释前置元数据 `<!-- group: 分组名 -->` 决定左侧列表分组）。
//
//go:embed api/*.md
var API embed.FS
