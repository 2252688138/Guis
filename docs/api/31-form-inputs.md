<!-- group: 基础组件 -->
# 输入与表单 · Input / Textarea / 校验 / 上传 / 属性表

## 1. 单行输入

```go
ui.Input("name", name, "请输入姓名", func(v string) { name = v })
ui.InputSubmit("search", q, "搜索后回车", func(v string) { q = v }, func() { doSearch(q) })
ui.PasswordInput("pwd", pwd, "密码", func(v string) { pwd = v })
ui.NumberInput("count", 1, 1, 99, 1, func(v float64) { count = v })   // 带 +/- 步进
ui.SearchInput("q", q, "搜索", func(v string) { q = v })              // 放大镜 + 清除按钮
ui.Textarea("note", note, "多行备注", 120, func(v string) { note = v })
```

* `Key` 必须唯一，否则多个输入框共享光标 / 选区。
* 输入框内部已注册 `RegisterTextEdit`，Ctrl+A/C/X/V 开箱可用（见《21 文本编辑》）。
* `InputSubmit` 的回车提交适合地址栏、搜索框。

## 2. 表单布局与校验

```go
ui.Column(
    ui.FormItem("用户名", true, errUser, ui.Input("u", user, "", func(v string) { user = v })),
    ui.FormItemInline("邮箱", true, errMail, ui.Input("m", mail, "", setMail)),
    ui.FormItem("备注", false, "", ui.Textarea("n", note, "", 100, setNote)),
    ui.FormFooter(
        ui.Btn("重置", ui.BtnGhost, reset),
        ui.Btn("提交", ui.BtnPrimary, submit),
    ),
).St(ui.St().GapOf(14))
```

常量：`FieldH = 38`（控件标准高）、`LabelW = 92`（横向表单标签宽）。

校验助手（返回错误文案，空串 = 通过）：

```go
err = ui.ValidateRequired("用户名", user)
err = ui.ValidateMinLen("密码", pwd, 6)      // 按 rune 计
err = ui.ValidateEmail("邮箱", mail)
```

```go
func submit() {
    if msg := ui.ValidateRequired("用户名", user); msg != "" {
        errUser = msg; return
    }
    errUser = ""
    save()
}
```

## 3. 标签输入 / 快捷键录制

```go
ui.TagInput("tags", tags, "回车添加标签", func(v []string) { tags = v })
// 回车或逗号添加，退格在空输入时删除末项，自动去重

ui.KeyCapture("hotkey", binding, func(keys string) { binding = keys })
// 点击后按下组合键写入；Esc 取消、Backspace 清空
ui.KeyName(0x53)     // "S"
```

## 4. 上传

```go
type UploadFile struct {
    Path     string
    Name     string
    Size     int64
    Progress float32   // 0..1
    Err      string
}

ui.Upload("files", files, ui.UploadOptions{
    Title:   "点击或拖入文件",
    Filter:  "图片|*.png;*.jpg|所有文件|*.*",
    MaxSize: 20 << 20,
    Height:  150,
}, pickFiles, removeAt, retryAt)
```

组件只负责选文件（走 `ui.PickFiles`）与展示进度 / 错误 / 重试 / 移除，真正的上传由业务实现。

## 5. 取色器

```go
st := ui.NewColor("theme-color")      // 跨帧持有
ui.ColorPicker(st, "#6366f1", func(hex string) { apply(hex) })
ui.ColorLayers()                      // 根 Stack 末尾：浮面板 + 吸管
st.SetColor("#22d3ee"); st.Hex()
```

* 面板：SV 区 + 色相条 + HEX 输入。
* 吸管（`st.Picking = true`）：面板收起，全屏轮询鼠标，点击屏幕任意位置取色
  （依赖 `ScreenPickColor` 等钩子，Windows 已接；其它平台为 nil 时按钮自动降级）。

## 6. 属性表（设置页 / 检查器）

```go
props := []ui.Prop{
    {Key: "name", Label: "名称", Group: "基本", Kind: ui.PropText, Value: name},
    {Key: "size", Label: "字号", Group: "基本", Kind: ui.PropNumber, Value: "13"},
    {Key: "dark", Label: "深色", Group: "外观", Kind: ui.PropBool, Value: "1"},
    {Key: "mode", Label: "模式", Group: "外观", Kind: ui.PropEnum, Options: []string{"紧凑", "宽松"}},
    {Key: "tint", Label: "主色", Group: "外观", Kind: ui.PropColor, Value: "#6366f1"},
    {Key: "path", Label: "文件", Group: "数据", Kind: ui.PropFile},
    {Key: "ver", Label: "版本", Group: "数据", Kind: ui.PropReadonly, Value: "1.2.3"},
    {Key: "custom", Label: "自定义", Kind: ui.PropCustom, Custom: func(v string, set func(string)) *ui.Node {
        return ui.Slider("s", 0.5, func(x float32) { set(fmt.Sprint(x)) })
    }},
}
ui.PropertyGrid("props", props, func(key, val string) { apply(key, val) })
ui.PropResetButton("props", "size", "13", onChange)   // ↺ 重置为默认值
```

`PropKind`：`PropText / PropNumber / PropBool / PropEnum / PropColor / PropFile / PropReadonly / PropCustom`。
`Group` 相同的属性自动归组显示。

## 7. 表单页骨架（可直接抄）

```go
func formPage() *ui.Node {
    return ui.Column(
        pageHead("表单", "校验 / 抽屉 / 弹窗"),
        ui.Card(
            ui.FormItem("姓名", true, errName, ui.Input("f-name", name, "请输入", setName)),
            ui.FormItem("角色", true, "", ui.SelectBox(roleSel,
                []ui.SelectOption{{Value: "dev", Label: "开发"}, {Value: "ops", Label: "运维"}},
                role, "请选择", setRole)),
            ui.FormItem("入职日期", false, "", ui.DateBox(dateSt, hireDate, setHire)),
            ui.FormFooter(ui.Btn("提交", ui.BtnPrimary, submit)),
        ).St(ui.St().Padding(20).R(14).BgC(ui.ColBgSoft)),
        ui.SelectLayers(), ui.DateLayers(),   // 浮层（也可统一挂根 Stack）
    ).St(ui.St().GapOf(16).Padding(24))
}
```

## 8. 坑位清单

| 现象 | 原因 |
| --- | --- |
| 两个输入框光标一起跳 | `Key` 重复 |
| 下拉展开后被压住 | 忘了把 `SelectLayers()` 之类加到根 `Stack` |
| 表单行高不齐 | 控件没给高度，`FormItem` 用 `FieldH` 对齐 |
| 数字输入无法输入负号 | `min` 传了 0；需要负数就把 `min` 设成负值 |
| 校验文案不更新 | 校验结果存在全局变量里，改完要 `ui.RequestRedraw()` |
