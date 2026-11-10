<!-- group: 容器与浮层 -->
# 浮层体系与弹窗 · Stack 层级 / Dialog / Drawer / 灯箱 / 引导

## 1. 浮层的唯一规则：放进根 Stack，越靠后越上层

```go
func view() *ui.Node {
    return ui.Stack(
        pageBody(),                 // 0 内容
        videoLayer(),               // 播放器全屏
        menuLayer,                  // 下拉菜单
        ui.TooltipLayer(),          // 悬停提示
        ui.SelectLayers(),          // 下拉 / 日历 / 时间 / 级联 / 联想 / 取色
        formLayers(),               // 弹窗 / 抽屉
        ui.MarkdownLightbox(),      // 图片灯箱
        ui.ImagePreviewLayer(),     // 通用图片灯箱
        ui.CommandPaletteLayer(),   // 命令面板
        ui.TourLayer(tour, steps),  // 引导
        ui.PrintDialogLayer(),      // 打印对话框
        ui.NotifyLayer(),           // 通知中心
        ui.ToastLayer(),            // 轻提示（永远最上层）
    ).St(ui.St().Hp(100))
}
```

后声明的画在上层，也**先**接收命中。顺序错乱 = 弹窗被内容压住 / 提示被弹窗盖住。

## 2. 通用弹窗 ModalPanel

```go
type DialogOpts struct {
    Key      string        // 动画 / 焦点状态 Key（同一弹窗保持固定）
    Open     bool
    Kind     DialogKind    // DlgInfo / DlgSuccess / DlgWarn / DlgError（决定图标与按钮色）
    Title    string
    Body     string        // 纯文本正文（BodyNode 非空时忽略）
    BodyNode *Node         // 自定义正文
    Width    float32
    Closable bool          // 右上角关闭按钮 + 允许点遮罩关闭
    OnClose  func()
    Footer   *Node         // 自定义底部（为 nil 时用 OKLabel / CancelLabel 生成）
    OKLabel, CancelLabel string
    OnOK, OnCancel func()
    Blur     float32       // 遮罩毛玻璃强度
}

ui.ModalPanel(ui.DialogOpts{
    Key: "del", Open: confirmDel, Kind: ui.DlgWarn,
    Title: "删除文件", Body: "删除后不可恢复，确定继续吗？",
    OKLabel: "删除", CancelLabel: "取消",
    OnOK: func() { doDelete(); confirmDel = false },
    OnCancel: func() { confirmDel = false },
    OnClose:  func() { confirmDel = false },
    Closable: true,
    Blur:     12,
})
```

## 3. 四个常用封装

```go
ui.AlertDlg("info-1", show, ui.DlgInfo, "提示", "操作已完成", func() { show = false })
ui.ConfirmDlg("del-1", show, ui.DlgWarn, "确认删除", "不可恢复", "删除", onCancel, onOK)
ui.PromptDlg("name-1", show, ui.DlgInfo, "重命名", "新名称", cur, "请输入", "确定",
    func(v string) { renameTo(v); show = false }, func() { show = false })
ui.Drawer(ui.DrawerOpts{
    Key: "detail", Open: openDetail, Title: "详情", Body: detailBody(), Width: 380,
    OnClose: func() { openDetail = false },
})
```

* `PromptDlg` 支持回车提交 / Esc 取消。
* `Drawer` 从右侧滑出（遮罩 + 面板），适合详情 / 设置。
* 所有弹窗都自带淡入动画、遮罩点击关闭（`Closable` 时）、焦点陷阱与 ESC 关闭。

## 4. 简单模态（widgets 里的老接口）

```go
ui.Modal(open, "标题", "正文", onClose)
ui.ModalEx(open, "标题", "正文", "取消", "确定", onCancel, onOK, onClose, 420)
ui.ModalKey("k1", open, "标题", "正文", "取消", "确定", onCancel, onOK, onClose, 400)
```

## 5. 图片灯箱

```go
ui.ImagePreviewOpen([]ui.PreviewImage{{Src: "a.png", Title: "封面"}}, 0)
ui.ImagePreviewOpenFiles([]string{"a.png", "b.png"}, 1)
ui.ImagePreviewClose()
ui.ImagePreviewLayer()      // 根 Stack 末尾
```

Markdown 内部图片点击会自动走灯箱，无需额外代码。

## 6. 引导 Tour

```go
var tour = ui.NewTour("onboarding")     // Seen 持久化到偏好

steps := []ui.TourStep{
    {Title: "这里是导航", Body: "点击切换分类", Target: "nav", Place: "right"},
    {Title: "这里是内容区", Body: "滚动查看全部示例", Target: "content", Place: "bottom"},
    {Title: "欢迎", Body: "随时可以在帮助里重看引导"},   // Target 为空 = 居中显示
}
tour.Start()          // 开始
tour.Finish()         // 结束并记录已看
ui.TourLayer(tour, steps)
```

`Target` 是节点的 `Key`，浮层会自动算出高亮区域；`Place` 取 `bottom / top / right / left`。

## 7. 遮罩与过渡

```go
ui.Scrim(opacity, "scrim-key")     // 遮罩层（放在 Stack 里铺满父容器）
ui.KeepAlive("panel", open, 0.18)  // (是否仍需绘制, 过渡进度 0..1) —— 浮层进出场
ui.ResetKeepAlive("panel")
opacity, dy := ui.PopOffset(t)     // 弹入造型：不透明度 + 纵向位移
ui.ResetTimeline("panel")
```

`KeepAlive` 是「关闭动画」的关键：`open=false` 时它还会返回 true 一段时间，
让浮层淡出完再真正移除。

## 8. 弹窗常见坑

| 现象 | 原因 |
| --- | --- |
| 弹窗被页面压住 | 没放进根 `Stack`，或放在 `Scroll` 里 |
| 弹窗内容跟着页面滚动 | 弹窗挂在滚动容器内部，应挂根 Stack |
| 关掉后再开没有动画 | `Key` 每次都变（用固定 key） |
| 多个弹窗互相抢焦点 | 同 key；改用 `ModalKey("del-1", …)` 区分 |
| 点遮罩关不掉 | `Closable` 未开 |
| 背景不模糊 | `Blur` 为 0，或该平台不支持离屏模糊（wasm 端降级为纯色遮罩） |
