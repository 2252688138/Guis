<!-- group: 基础组件 -->
# 选择器 · Select / Date / Time / Cascader / AutoComplete

所有「点击展开浮层」的控件都遵循同一套约定：

1. 控件本体只画按钮 + 文本，状态结构体（`*SelectState` / `*DateState` / …）由业务**长期持有**；
2. 展开的浮层**不在原地画**，而是由 `XxxLayers()` 统一渲染，必须挂到根 `Stack` 的末尾；
3. 浮层自带点击外部关闭与 ESC 关闭（走 `ui.OpenOverlay` 栈）。

```go
return ui.Stack(
    pageBody(),            // 里面放各种控件本体
    ui.SelectLayers(),
    ui.DateLayers(),
    ui.TimeLayers(),
    ui.CascaderLayers(),
    ui.AutoCompleteLayers(),
    ui.ColorLayers(),
)
```

## 1. 下拉框 Select

```go
var roleSel = ui.NewSelect("role")      // 同一个 key 重复创建会覆盖注册项

ui.SelectBox(roleSel,
    []ui.SelectOption{
        {Value: "dev", Label: "研发", Hint: "写代码"},
        {Value: "ops", Label: "运维", Hint: "守夜"},
        {Value: "test", Label: "测试", Disabled: true},
    },
    role,                 // 当前值
    "请选择角色",          // 占位文案
    func(v string) { role = v },   // 选择回调
)
ui.SelectLayers()         // 根 Stack 末尾

roleSel.Toggle(); roleSel.Close()
```

`SelectState`：`Key / Open / Hover / Value`；`SelectOption`：`Value / Label / Hint / Disabled`。

## 2. 日期

```go
var dateSt = ui.NewDate("hire")

ui.DateBox(dateSt, hire, func(v string) { hire = v })   // 回调格式 YYYY-MM-DD
ui.DateLayers()
ui.DateField("只读日期", "2026-11-04")   // 只读展示框（带日历图标）
```

`DateState`：`Key / Open / Y / M / Value`（面板当前年月 + 选中值）。

独立日历（月视图 + 事件点）：

```go
var cal = ui.NewCalendar("cal")   // CalendarState{Key, Y, M}
ui.Calendar(cal,
    []ui.CalendarEvent{{Date: "2026-11-04", Label: "评审", Color: ui.ColWarn}},
    selectedDate,
    func(d string) { selectedDate = d },
)
```

## 3. 时间

```go
var timeSt = ui.NewTime("meet")     // Second: true 时显示秒列

ui.TimePicker(timeSt, "09:30", func(v string) { meet = v })   // HH:MM 或 HH:MM:SS
ui.TimeLayers()
timeSt.TimeValue()      // 当前值
```

## 4. 级联选择 Cascader

```go
var citySt = ui.NewCascade("city")

ui.Cascader(citySt,
    []ui.CascadeOption{
        {Value: "zj", Label: "浙江", Children: []ui.CascadeOption{
            {Value: "hz", Label: "杭州", Children: []ui.CascadeOption{{Value: "xh", Label: "西湖"}}},
        }},
        {Value: "sh", Label: "上海", Load: func() []ui.CascadeOption { return fetchDistricts() }},
    },
    []string{"zj", "hz"},      // 当前路径（值）
    "请选择地区",
    func(path []string) { region = path },
)
ui.CascaderLayers()
citySt.CascaderPath()   // 当前路径的可读文本
```

`Children` 为空且 `Load` 非空时，点击会调用一次懒加载。

## 5. 输入联想 AutoComplete

```go
var acSt = ui.NewAutoComplete("user")

ui.AutoComplete(acSt,
    []string{"alice", "bob", "carol"},   // 候选（静态）
    keyword,
    "输入用户名",
    func(v string) { keyword = v },
)
ui.AutoCompleteLayers()
acSt.Close()
```

键盘：↑↓ 选择、Enter 确认、Esc 关闭。

## 6. 分段 / 步进（无需浮层）

```go
ui.Segmented([]string{"d", "w", "m"}, []string{"日", "周", "月"}, span, func(k string) { span = k })
ui.Steps([]ui.StepItem{{Title: "填写", Sub: "基本信息"}, {Title: "支付"}, {Title: "完成"}}, 1)
```

## 7. 状态结构的生命周期

* 在包的 `var` 里创建一次即可（`ui.NewSelect("role")` 内部按 key 注册）。
* 不要在 `view()` 里 `ui.NewSelect(...)` —— 每帧新建会不断覆盖注册项，
  虽然不崩但会丢掉 `Hover` / `Open` 状态（实现上是复用，仍建议放 `var`）。
* 控件与浮层必须用**同一个**状态指针，否则浮层找不到锚点。

## 8. 平台差异

浏览器 / 移动端同样可用：浮层是自绘节点，不依赖系统对话框。
只有文件 / 文件夹选择（`ui.OpenFileDialog` 等）在浏览器端走 `<input type=file>` 异步实现，
必须用 `ui.PickFile` / `ui.PickFilesAsync` 回调式 API。
