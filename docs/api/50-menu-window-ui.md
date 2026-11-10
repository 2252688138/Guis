<!-- group: 容器与浮层 -->
# 菜单与自绘标题栏 · MenuBar / MenuLayer / MenuPanel

## 1. 数据结构

```go
type MenuItem struct {
    Label     string
    Hint      string      // 右侧快捷键 / 说明
    Icon      string      // 左侧符号
    OnPick    func()
    Children  []MenuItem  // 非空 = 子菜单（可无限嵌套）
    Separator bool        // 分隔线
    Disabled  bool
    Checked   bool
}

type MenuDef struct {
    Label string
    Items []MenuItem
}
```

## 2. 完整接入（自绘标题栏）

```go
var menu = ui.NewMenuBar()   // 状态跨帧持有

func titleBar() *ui.Node {
    return ui.Row(
        ui.Text("GUIS").St(ui.St().Text(12.5, 600, ui.ColText)),
        ui.MenuBar(menu, menus()),                       // 横向菜单按钮行
        ui.Box().St(ui.St().GrowTo(1).H(40)).           // 空白拖动区
            OnPressEvt(func() { menu.Close(); ui.Current().BeginDragMove() }),
        ui.WindowButtons(),                              // 最小化 / 最大化 / 关闭
    ).St(ui.St().H(40).PadOnly(0, 10, 0, 8).GapOf(14).BgC(ui.ColBgSoft))
}

func view() *ui.Node {
    return ui.Stack(
        ui.Column(titleBar(), body()).St(ui.St().Hp(100)),
        ui.MenuLayer(menu, menus(), anchorX, 40),         // 下拉浮层：anchorX = 菜单栏左边缘
        ui.TooltipLayer(),
        ui.ToastLayer(),
    ).St(ui.St().Hp(100))
}
```

`anchorX` 必须与 `ui.MenuBar` 在标题栏里的实际左边缘一致，否则下拉会错位
（gallery 里是「左内边距 10 + 品牌区宽 + 间距 14」算出来的）。

## 3. MenuBarState

```go
menu.Open        // 打开的顶级菜单索引，-1 = 全部关闭
menu.Sub         // 子菜单展开路径，例如 []int{0, 1} = 第 0 项的第 1 个子菜单
menu.ItemX       // 由 MenuBar 计算的各顶级按钮 x 偏移（只读）
menu.AutoHide = 5 * time.Second   // 鼠标离开后自动收起；<0 关闭该行为
menu.Close(); menu.IsOpen()
```

交互：点击展开、鼠标划过其它顶级菜单自动切换、子菜单悬停展开、点击外部 / ESC 关闭。

## 4. 浮层菜单 MenuPanel（右键菜单 / 任意锚点）

```go
ui.Stack(
    body(),
    ui.If(ctxMenu, func() *ui.Node {
        return ui.MenuPanel(320, 180, []ui.MenuItem{   // x, y = 窗口坐标
            {Label: "复制", Hint: "Ctrl+C", OnPick: copySel},
            {Label: "粘贴", Hint: "Ctrl+V", OnPick: paste},
            {Separator: true},
            {Label: "导出为", Children: []ui.MenuItem{
                {Label: "PNG", OnPick: exportPNG},
                {Label: "PDF", OnPick: exportPDF},
            }},
            {Label: "更多设置…", Disabled: true},
        }, 180)                                        // 面板宽度
    }),
)
```

## 5. 窗口按钮

```go
ui.WindowButtons()   // 最小化 / 最大化 / 关闭，已在内部调 App 的方法
```

移动端与浏览器应隐藏：

```go
ui.If(ui.HasCap(ui.CapDialogs) || runtime.GOOS == "windows", func() *ui.Node { return ui.WindowButtons() })
```

（gallery 用的是 `pages.IsDesktop()`，语义等价。）

## 6. 命令面板（Ctrl+K）

```go
ui.RegisterCommand(ui.Command{
    ID: "save", Title: "保存文件", Sub: "写入当前文档",
    Group: "文件", Shortcut: "Ctrl+S", Icon: "▤", Run: save,
})
ui.OpenCommandPalette(); ui.CloseCommandPalette(); ui.ToggleCommandPalette()
ui.Commands()                  // 已注册命令（注册顺序）
ui.UnregisterCommand("save")
ui.FuzzyScore("sf", "保存文件")  // 子序列评分（连续/词首命中加权，无法匹配 -1）
ui.CommandPaletteLayer()       // 根 Stack 末尾
```

`ToggleCommandPalette()` 返回值表示是否处理（有命令时），供 `App` 的 Ctrl+K 调用。

## 7. 子窗口 / 多窗口入口

```go
child, err := ui.NewWindow(ui.Options{Title: "子窗口", Width: 520, Height: 360}, childView)
```

## 8. 窄屏适配

菜单栏在窄屏放不下时应当隐藏，改用左侧导航或下拉：

```go
ui.If(ui.ViewportW() >= 900, func() *ui.Node { return ui.MenuBar(menu, menus()) })
```

## 9. 菜单状态与页签的联动

`Checked` 用来表达「当前选中」：

```go
{Label: "主题", Children: themeItems()}

// themeItems 把主题列表映射成勾选菜单项（MenuItem 是值类型，普通循环即可）
func themeItems() []ui.MenuItem {
    out := make([]ui.MenuItem, 0, 3)
    for _, t := range []string{"深色", "浅色", "跟随系统"} {
        name := t
        out = append(out, ui.MenuItem{
            Label:   name,
            Checked: theme == name,
            OnPick:  func() { setTheme(name) },
        })
    }
    return out
}
```

`MenuItem` 是值类型，子菜单里可以直接用 `func() ui.MenuItem` 生成（gallery 的
`caseMenuItems()` 就是把分类列表映射成勾选菜单）。
