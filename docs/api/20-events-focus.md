<!-- group: 事件与交互 -->
# 事件与焦点 · 点击 / 拖动 / 拖入 / 键盘 / 浮层栈

## 1. 事件对象

平台差异（Win32 消息 / DOM 事件 / 触摸）统一收敛成 `ui.Event`：

```go
type Event struct {
    Type    EventType
    X, Y    float32   // 逻辑像素
    DX, DY  float32
    Button  int
    Key     uint32
    Char    rune
    Runes   []rune
    Width, Height, Scale float32
    Text    string
    Caret   int
    Pointer int       // 0 鼠标 / 1 触摸或手写笔
    Paths   []string  // 拖入的文件路径
    HotkeyID int
}
```

`EventType`：`EvResize / EvClose / EvFocusGained / EvFocusLost / EvMouseMove / EvMouseDown /
EvMouseUp / EvMouseWheel / EvMouseLeave / EvKeyDown / EvKeyUp / EvChar / EvComposition* /
EvSizeMoveStart / EvSizeMoveEnd / EvDropFiles / EvHotkey / EvTouchDown / EvTouchMove / EvTouchUp /
EvTextInput / EvBack`。

业务一般不直接碰 `Event`，用节点回调即可。

## 2. 节点回调（全部）

| 方法 | 触发时机 | 备注 |
| --- | --- | --- |
| `Tap(fn)` | 点击（按下 + 松开在同一节点） | nil 安全 |
| `TapAt(fn(x, y))` | 带相对坐标的点击 | 滑块、进度跳转 |
| `OnPressEvt(fn)` | **按下即触发**，早于拖动 / 点击 | 需设 `Key`；自绘标题栏拖动 |
| `DragEvt(fn(x, y))` | 按住拖动 | 需 `Key` |
| `DragEndEvt(fn)` | 松手 | 拖动类控件据此提交值 |
| `OnHoverEvt(fn(bool))` | 悬停进入 / 离开 | |
| `ScrollEvt(fn(dy))` | 在节点上滚动 | |
| `OnCharEvt(fn(rune))` | 字符输入 | 需 `FocusOn()` |
| `OnKeyEvt(fn(vk))` | 按键 | 需 `FocusOn()` |
| `ActivateEvt(fn)` | Enter / Space 激活 | 无障碍键盘操作 |
| `DropEvt(fn(paths))` | 文件拖入落点命中 | 沿祖先冒泡 |
| `DisabledOn(v)` | 禁用点击 / 拖动 | 外观自行处理 |
| `OnSpanClickEvt(fn(TextSpan))` | 富文本片段点击 | 链接 / 行内代码 |

```go
ui.Box(ui.Text("按住拖我")).K("draggable").
    DragEvt(func(x, y float32) { offset = x }).
    DragEndEvt(func() { save(offset) }).
    St(ui.St().W(200).H(36))
```

> 只有设了 `DragEvt` 的节点才会建立拖动会话；`OnPressEvt` 需要 `Key` 才去重
> （曾因用节点指针去重导致按住期间每帧重放，选区的锚点被反复重置）。

## 3. 键盘焦点

```go
ui.SetFocus("input-name")     // 设置焦点（空串取消）
ui.FocusedKey()               // 当前焦点 Key
ui.IsFocused("input-name")
ui.FocusNext() / ui.FocusPrev()   // Tab / Shift+Tab 顺序
ui.FocusVisible()             // 是否应绘制焦点环（键盘导航时为真）
ui.SetFocusVisible(false)     // 触摸端可关闭焦点环
```

焦点作用域与浮层栈：

```go
ui.PushFocusScope("modal-1", true)   // trap=true：Tab 只在作用域内循环
defer ui.PopFocusScope("modal-1")    // 退出并恢复进入前的焦点
ui.OpenOverlay("menu", func() bool { /* 处理 ESC，返回 true 表示已消费 */ return true })
ui.CloseOverlay("menu")
ui.OverlayDepth()
```

`OpenOverlay` 让 ESC 按「最后打开的浮层优先」逐层关闭，是所有浮层（菜单 / 下拉 / 弹窗）的统一约定。

## 4. 全局按键

```go
ui.OnKeyGlobal(func(vk uint32) {
    switch vk {
    case ui.VK_F11: toggleFullscreen()
    case ui.VK_ESCAPE: closeSomething()
    }
})
ui.Current().SetOnKey(fn)   // 只对某窗口生效
```

常用虚拟键码常量：`VK_BACK / VK_TAB / VK_RETURN / VK_SHIFT / VK_CONTROL / VK_MENU /
VK_ESCAPE / VK_SPACE / VK_LEFT / VK_UP / VK_RIGHT / VK_DOWN / VK_DELETE / VK_F11`。
修饰键状态：`ui.CtrlDown() / ShiftDown() / AltDown()`；键码转显示名：`ui.KeyName(vk)`。

## 5. 快捷键系统（作用域 + 冲突检测）

```go
ui.RegisterShortcut(ui.ScopeGlobal, ui.Shortcut{
    ID: "save", Keys: "Ctrl+S", Label: "保存", Group: "文件", Run: save,
})

ui.RegisterShortcut(ui.ScopePanel, ui.Shortcut{ID: "find", Keys: "Ctrl+F", Run: openFind})
ui.PushShortcutScope(ui.ScopeModal)   // 弹窗里覆盖下层
defer ui.PopShortcutScope()

ui.ShortcutConflict(ui.ScopeGlobal, "Ctrl+S")  // 返回占用者 ID（空串 = 无冲突）
ui.ShortcutRebind(ui.ScopeGlobal, "save", "Ctrl+Shift+S")  // 冲突则返回 false
ui.DispatchShortcuts(vk)                       // App 事件循环内部调用
ui.ShortcutList(ui.ScopeGlobal)                // 帮助面板列列表
ui.ShortcutsHelp(ui.ScopeGlobal)               // 现成的快捷键帮助节点
ui.ShortcutSerialize(ui.ScopeGlobal)           // "id=keys" 行，存偏好用
```

作用域常量：`ScopeGlobal` / `ScopePanel` / `ScopeModal`。

## 6. 文件拖入

```go
// 节点级：把文件拖进这个框里松手
ui.DropZone("drop", "把文件拖到这里", 160, func(paths []string) { openFiles(paths) })

// 任意节点：命中该区域时触发（沿祖先冒泡）
ui.Box(...).DropEvt(func(paths []string) { ... })

// 应用级兜底（没有节点命中时）
ui.SetOnDropFiles(func(paths []string, x, y float32) { ... })
ui.SetAcceptDrops(false)   // 关闭接收

ui.PickFiles("选择文件", "图片|*.png;*.jpg|所有文件|*.*")   // 多选（同步，桌面）
ui.FileRow(path, func(){})   // 拖入结果行：名称 + 大小 + 目录
ui.FormatBytes(1536)         // "1.5 KB"
```

限制：`WM_DROPFILES` 只在**松手**时到达，没有 dragenter/dragover，因此没有悬停高亮
（需要高亮得改用 OLE `IDropTarget`，框架未提供）。回调里的节点指针只在当前帧有效，
状态改完记得 `ui.RequestRedraw()`。

## 7. 触摸 / 移动端

`Event.Pointer == 1` 表示触摸；`Anchor` 语义与鼠标一致（按下 → 拖动 → 抬起）。
移动端没有 hover，`IsHovered` 恒 false，不要依赖悬停才显示的交互。

## 8. 无障碍元数据

```go
ui.Btn("保存", ui.BtnPrimary, save).A11yOn(ui.A11yButton, "保存", "保存当前文件")
ui.A11ySnapshot()      // 先序导出界面中的无障碍节点（调试 / 朗读）
ui.A11yRoleName(ui.A11yButton)   // "button"
```

角色：`A11yButton / Text / Link / CheckBox / Radio / Switch / Slider / TextField / List /
ListItem / Tab / Dialog / Menu / MenuItem / Progress / Image / Table / Tree`。
