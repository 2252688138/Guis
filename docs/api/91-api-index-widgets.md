<!-- group: 附录 -->
# 附录 B · 组件 API 速查（基础 / 输入 / 展示 / 容器 / 浮层）

全部可视化组件的导出符号一览：基础控件、开关、表单、输入扩展、展示组件、
菜单、列表、表格、树、图表外的容器类组件与浮层。

> 说明：本附录为**自动生成**的完整符号清单，按源文件分组；
> 想按「用途」查找请先读前面的正文章节，本附录保证**没有任何导出 API 被漏掉**。

| 源文件 | 符号数 |
| --- | --- |
| `ui/widgets.go` | 42 |
| `ui/switch.go` | 23 |
| `ui/form.go` | 25 |
| `ui/inputs_extra.go` | 39 |
| `ui/display_extra.go` | 29 |
| `ui/cards.go` | 10 |
| `ui/tree.go` | 11 |
| `ui/pager.go` | 2 |
| `ui/table_extra.go` | 8 |
| `ui/vlist.go` | 5 |
| `ui/propgrid.go` | 12 |
| `ui/menu.go` | 10 |
| `ui/arrow.go` | 1 |
| `ui/dialog.go` | 12 |
| `ui/toast.go` | 6 |
| `ui/notify.go` | 14 |
| `ui/states.go` | 7 |
| `ui/ellipsis.go` | 2 |
| `ui/split.go` | 4 |
| `ui/dock.go` | 7 |
| `ui/transitions.go` | 14 |
| `ui/anim.go` | 19 |
| `ui/flip.go` | 5 |
| `ui/image.go` | 2 |
| `ui/icon.go` | 2 |
| `ui/iconfont.go` | 13 |
| `ui/i18n.go` | 32 |
| `ui/i18n_format.go` | 15 |
| `ui/i18n_platform.go` | 4 |
| `ui/i18n_builtin.go` | 1 |
| `ui/i18n_rtl.go` | 1 |
| `ui/i18n_widget.go` | 5 |
| `ui/command.go` | 11 |
| `ui/undo.go` | 17 |
| `ui/crash.go` | 7 |

合计 **417** 个导出符号。

---

## `ui/widgets.go`

- **类型** `type ButtonKind int` — ButtonKind 按钮变体。
- **函数** `func Divider() *Node` — Divider 分隔线。
- **函数** `func Title(text, sub string) *Node` — Title 区块标题（带副标题可选）。
- **函数** `func Card(children ...*Node) *Node` — Card 卡片容器。
- **函数** `func Tag(label, color string) *Node` — Tag 标签 / 徽章文本。
- **函数** `func Dot(color string, size float32) *Node` — Dot 状态圆点。
- **函数** `func Avatar(initial, color string) *Node` — Avatar 圆形头像（文字首字）。
- **函数** `func Badge(count int) *Node` — Badge 角标数字（附着在父节点右上角，需父节点为 Stack）。
- **函数** `func Link(label string, onClick func()) *Node` — Link 文字链接。
- **函数** `func KeyValue(k, v string) *Node` — KeyValue 左标题右值的一行。
- **函数** `func Empty(title, sub string) *Node` — Empty 空状态占位。
- **函数** `func Btn(label string, kind ButtonKind, onClick func()) *Node` — Btn 带变体与禁用态的按钮。
- **函数** `func IconButton(glyph string, onClick func()) *Node` — IconButton 方形图标按钮（glyph 建议用 + - × → ← ↑ ↓ 等有字形符号）。
- **函数** `func Checkbox(key, label string, checked bool, onChange func(bool)) *Node` — Checkbox 复选框。
- **函数** `func Radio(key, label string, selected bool, onPick func()) *Node` — Radio 单选按钮。
- **函数** `func Switch(key string, on bool, onChange func(bool)) *Node` — Switch 开关（轨道 + 旋钮，切换时 140ms 过渡）。
- **函数** `func Slider(key string, v float32, onChange func(float32)) *Node` — Slider 滑块（0~1，支持点击定位与拖动，拖动过程中实时跟手）。
- **函数** `func Progress(v float32, color string) *Node` — Progress 进度条（0~1）。宽度由父容器决定（Column 拉伸 / Row 内 Grow），
- **函数** `func Ring(v float32, size float32, color string) *Node` — Ring 环形进度（分段圆点近似，gio 风格的小尺寸指示器）。
- **函数** `func InputSubmit(key, value, placeholder string, onChange func(string), onSubmit func()) *Node` — InputSubmit 带回车提交的单行输入框：回车触发 onSubmit（地址栏、搜索框用）。
- **函数** `func Input(key, value, placeholder string, onChange func(string)) *Node` — Input 单行输入框：点击获焦、光标闪烁、左右/Home/End/退格/删除。
- **函数** `func PasswordInput(key, value, placeholder string, onChange func(string)) *Node` — PasswordInput 密码输入框（掩码显示）。
- **函数** `func Spinner(size float32, color string) *Node` — Spinner 旋转加载指示器（逐帧动画）。
- **函数** `func Skeleton(w, h float32) *Node` — Skeleton 骨架屏占位（呼吸 + 流光动画）。
- **函数** `func Toast(msg string) *Node` — Toast 轻提示（悬浮于容器底部中央，需放在 Stack 中）。
- **函数** `func Banner(kind, title, body string, onClose func()) *Node` — Banner 信息条（左侧色条 + 标题/正文 + 关闭）。
- **函数** `func Modal(open bool, title, body string, onClose func()) *Node` — Modal 模态框（遮罩 + 卡片，放在 Stack 中并已 AbsFill）。
- **函数** `func ModalEx(open bool, title, body, cancelLabel, okLabel string, onCancel, onConfirm, onClose func(), width float32) *Node` — ModalEx 可定制按钮与宽度的模态框（带进出场过渡与焦点陷阱）。
- **函数** `func ModalKey(key string, open bool, title, body, cancelLabel, okLabel string, onCancel, onConfirm, onClose func(), width float32) *Node` — ModalKey 带唯一 Key 的模态框：多个同标题弹窗共存时用它避免焦点/动画状态互撞。
- **函数** `func Tabs(keys, labels []string, active string, onPick func(string)) *Node` — Tabs 分段页签。
- **函数** `func TabsBar(keys, labels []string, active string, onPick func(string)) *Node` — TabsBar 下划线风格页签（gio 常见导航样式）。
- **函数** `func Breadcrumb(items []string, onPick func(int)) *Node` — Breadcrumb 面包屑（点击项回调，最后一项为当前页）。
- **函数** `func Accordion(key, title, body string, open bool, onToggle func()) *Node` — Accordion 折叠面板。
- **函数** `func ListItem(key, title, sub, color string, onClick func()) *Node` — ListItem 可点击列表项（左侧色点 + 标题 + 右侧说明）。
- **函数** `func Table(headers []string, widths []float32, rows [][]*Node, onRow func(int)) *Node` — Table 数据表格：固定/弹性列宽、表头、斑马纹、整行点击。
- **函数** `func TooltipOf(n *Node, text string) *Node` — TooltipOf 给节点绑定悬停提示。
- **函数** `func TooltipLayer() *Node` — TooltipLayer 悬停提示浮层（放在根 Stack 最后一层）。
- **常量** `const BtnPrimary = iota`
- **常量** `const BtnSecondary`
- **常量** `const BtnGhost`
- **常量** `const BtnDanger`
- **常量** `const BtnSuccess`

## `ui/switch.go`

- **类型** `type SwitchSize int` — SwitchSize 开关尺寸预设。
- **类型** `type SwitchStyle struct { Size SwitchSize Icon string // 左侧图标字形（Boxed 时显示在轨道左侧） IconC any // 图标颜色，默认琥珀色 OnLabel string // 打开态轨道内文字 OffLabel string // 关闭态轨道内文字 OnFrom any // 打开态轨道渐变起点，默认 ColPrimary OnTo any // 打开态轨道渐变终点，默认 ColPrimary2 OffBg any // 关闭态轨道底色，默认 ColTrack KnobC any // 旋钮颜色，默认白色 Border any // 外框描边色（Boxed 用） Glow bool // 打开态外发光 Boxed bool // 外框胶囊：图标 + 轨道 Disabled bool // 禁用态：半透明且不响应点击 // ---- 自定义装饰（均可选，为空时回落到上面的字形/文字字段）---- IconNode *Node // 自定义图标节点（替代 Icon 字形，渲染在外框左侧 th×th 区域内） OnNode *Node // 自定义打开态轨道内容（任意节点，替代 OnLabel） OffNode *Node // 自定义关闭态轨道内容（任意节点，替代 OffLabel） LabelW float32 // 轨道内容区宽度（0 = 按 OnLabel/OffLabel 文字宽度自动） LabelSize float32 // 轨道文字字号（0 = 按尺寸预设） OnTextC any // 打开态文字颜色，默认 #ffffff OffTextC any // 关闭态文字颜色，默认 #e8ecff }` — SwitchStyle 开关外观配置。零值 = 默认样式（等于 widgets.go 的 Switch）。
- **函数** `func SwitchStyleDefault() SwitchStyle` — SwitchStyleDefault 默认样式（与 Switch 视觉一致）。
- **函数** `func SwitchStyleSparkle() SwitchStyle` — SwitchStyleSparkle 「✦ 图标 + 紫色渐变 + 发光 + 外框」样式。
- **函数** `func SwitchStyleLabeled(onLabel, offLabel string) SwitchStyle` — SwitchStyleLabeled 轨道内 ON/OFF 文字样式。
- **函数** `func SwitchStyleGradient(from, to any) SwitchStyle` — SwitchStyleGradient 纯渐变 + 发光样式。
- **函数** `func SwitchEx(key string, on bool, st SwitchStyle, onChange func(bool)) *Node` — SwitchEx 通用开关：按 SwitchStyle 渲染，切换时 160ms 缓动过渡。
- **函数** `func SwitchDefault(key string, on bool, onChange func(bool)) *Node` — SwitchDefault 默认样式的开关（与 Switch 一致，便于统一走 SwitchEx 体系）。
- **函数** `func SwitchSized(key string, size SwitchSize, on bool, onChange func(bool)) *Node` — SwitchSized 按尺寸预设渲染的默认样式开关。
- **函数** `func SwitchIcon(key, icon string, on bool, onChange func(bool)) *Node` — SwitchIcon 左侧带图标的胶囊开关（图标 + 轨道）。
- **函数** `func SwitchSparkle(key string, on bool, onChange func(bool)) *Node` — SwitchSparkle 「✦ 紫色发光」胶囊开关（SwitchStyleSparkle 的一行封装）。
- **函数** `func SwitchLabeled(key, onLabel, offLabel string, on bool, onChange func(bool)) *Node` — SwitchLabeled 轨道内带 ON / OFF 文字的开关。
- **函数** `func SwitchGradient(key string, from, to any, on bool, onChange func(bool)) *Node` — SwitchGradient 渐变发光开关（自定义起止色）。
- **函数** `func SwitchDisabled(key string, on bool) *Node` — SwitchDisabled 禁用态开关（半透明、不可点击）。
- **方法** `func SwitchStyle.WithIcon(glyph string, color any) SwitchStyle` — WithIcon 自定义图标字形与颜色。
- **方法** `func SwitchStyle.WithIconNode(n *Node) SwitchStyle` — WithIconNode 用任意节点作为图标（自动开启外框胶囊），节点在 th×th 区域内居中。
- **方法** `func SwitchStyle.WithLabels(onLabel, offLabel string) SwitchStyle` — WithLabels 自定义开 / 关文案（轨道内文字，自动按文字宽度撑开轨道）。
- **方法** `func SwitchStyle.WithLabelNodes(on, off *Node) SwitchStyle` — WithLabelNodes 用任意节点作为开 / 关轨道内容（宽度由 WithLabelWidth 指定）。
- **方法** `func SwitchStyle.WithLabelStyle(onColor, offColor any, size float32) SwitchStyle` — WithLabelStyle 设置轨道文字颜色与字号（size <= 0 保持尺寸预设字号）。
- **方法** `func SwitchStyle.WithLabelWidth(w float32) SwitchStyle` — WithLabelWidth 指定轨道内容区宽度（自定义节点建议显式指定，文字默认自动测量）。
- **常量** `const SwitchNormal = iota`
- **常量** `const SwitchSmall`
- **常量** `const SwitchLarge`

## `ui/form.go`

- **类型** `type SelectOption struct { Value string Label string Hint string // 右侧附加说明（可选） Disabled bool }` — SelectOption 下拉选项。
- **类型** `type SelectState struct { Key string Open bool Hover int // 键盘/鼠标高亮项索引，-1 表示无 Value string opts []SelectOption onChange func(string) }` — SelectState 下拉框状态（调用方长期持有，用 NewSelect 创建）。
- **类型** `type DateState struct { Key string Open bool Y, M int // 面板当前显示的年月 Value string onChange func(string) }` — DateState 日历浮层状态。
- **函数** `func FormItem(label string, required bool, errMsg string, child *Node) *Node` — FormItem 纵向表单行：标签（可带必填星号）+ 控件 + 错误提示。
- **函数** `func FormItemInline(label string, required bool, errMsg string, child *Node) *Node` — FormItemInline 横向表单行：左侧固定宽度标签，右侧控件占满剩余宽度。
- **函数** `func FormFooter(buttons ...*Node) *Node` — FormFooter 表单操作行（右对齐按钮组）。
- **函数** `func ValidateRequired(label, v string) string` — ValidateRequired 必填校验：空串返回错误文案。
- **函数** `func ValidateMinLen(label, v string, n int) string` — ValidateMinLen 最小长度校验（len 按 rune 计）。
- **函数** `func ValidateEmail(label, v string) string` — ValidateEmail 邮箱格式校验（够用的宽松规则）。
- **函数** `func NewSelect(key string) *SelectState` — NewSelect 创建下拉框状态（同一个 key 重复创建会覆盖注册项）。
- **函数** `func SelectBox(s *SelectState, opts []SelectOption, value, placeholder string, onChange func(string)) *Node` — SelectBox 下拉框控件本体。
- **函数** `func SelectLayers() *Node` — SelectLayers 渲染所有展开的下拉浮层（放在根 Stack 末尾）。
- **函数** `func Textarea(key, value, placeholder string, height float32, onChange func(string)) *Node` — Textarea 多行文本输入：换行 / 上下左右 / Home-End / 退格删除 / 光标闪烁，
- **函数** `func NumberInput(key string, v float64, min, max, step float64, onChange func(float64)) *Node` — NumberInput 数字输入：左右 +/- 步进按钮 + 中间可编辑输入框。
- **函数** `func SearchInput(key, value, placeholder string, onChange func(string)) *Node` — SearchInput 搜索框：左侧放大镜（矢量方块拼）+ 有内容时右侧清除按钮。
- **函数** `func Chip(label string, active bool, onToggle func()) *Node` — Chip 可选中的标签胶囊。
- **函数** `func ChipRemovable(label string, onRemove func()) *Node` — ChipRemovable 带删除按钮的标签（用于多选结果）。
- **函数** `func DateField(label, value string) *Node` — DateField 只读日期展示框（带日历图标与小锁提示）。
- **函数** `func NewDate(key string) *DateState` — NewDate 创建日历状态。
- **函数** `func DateBox(s *DateState, value string, onChange func(string)) *Node` — DateBox 日期选择控件（点击展开日历浮层，选择后回调 YYYY-MM-DD）。
- **函数** `func DateLayers() *Node` — DateLayers 渲染所有展开的日历浮层（放在根 Stack 末尾）。
- **方法** `func SelectState.Close()` — Close 收起下拉浮层。
- **方法** `func SelectState.Toggle()` — Toggle 展开 / 收起。
- **常量** `const FieldH = 38`
- **常量** `const LabelW = 92`

## `ui/inputs_extra.go`

- **类型** `type AutoCompleteState struct { Key string Open bool Hover int Value string onChange func(string) opts []string items []string }` — AutoCompleteState 输入联想状态。
- **类型** `type TimeState struct { Key string Open bool // Second 是否显示秒列。 Second bool H, M, S int onChange func(string) }` — TimeState 时间选择状态。
- **类型** `type CascadeOption struct { Value string Label string Disabled bool Children []CascadeOption // Load 懒加载子级（Children 为空时点击调用一次）。 Load func() []CascadeOption }` — CascadeOption 级联选项。
- **类型** `type CascadeState struct { Key string Open bool Path []int Value []string opts []CascadeOption onChange func([]string) }` — CascadeState 级联选择状态。
- **类型** `type UploadFile struct { Path string Name string Size int64 Progress float32 // 0..1 Err string }` — UploadFile 上传项。
- **类型** `type UploadOptions struct { Title string Filter string // 形如 "图片|*.png;*.jpg|所有文件|*.*" MaxSize int64 // 字节，0 表示不限制 Height float32 }` — UploadOptions 上传组件参数。
- **类型** `type ColorState struct { Key string Open bool H float32 // 0..360 S float32 // 0..1 V float32 // 0..1 Value string // Picking 吸管拾色中：面板收起，全屏轮询鼠标位置，点击取色。 // pickWaitRelease 记下起始时左键是否还按着（那次按下不能算取色）。 Picking bool pickWaitRelease bool pickPreview string onChange func(string) }` — ColorState HSV 取色器状态。
- **函数** `func NewAutoComplete(key string) *AutoCompleteState` — NewAutoComplete 创建（或复用）联想状态。
- **函数** `func AutoComplete(st *AutoCompleteState, opts []string, value, placeholder string, onChange func(string)) *Node` — AutoComplete 输入联想框（↑↓ 选择，Enter 确认，Esc 关闭）。
- **函数** `func AutoCompleteLayers() *Node` — AutoCompleteLayers 渲染当前展开的联想浮层（根 Stack 末尾）。
- **函数** `func TagInput(key string, tags []string, placeholder string, onChange func([]string)) *Node` — TagInput 标签输入：回车 / 逗号添加，退格在空输入时删除末项，自动去重。
- **函数** `func Rate(key string, v float32, max int, allowHalf bool, onChange func(float32)) *Node` — Rate 星级评分（默认整星，allowHalf 支持半星）。
- **函数** `func RangeSlider(key string, lo, hi float32, onChange func(lo, hi float32)) *Node` — RangeSlider 双滑块区间选择（lo/hi 均为 0..1）。
- **函数** `func KeyCapture(key, binding string, onChange func(string)) *Node` — KeyCapture 快捷键录制框：点击后按下组合键写入；Esc 取消，Backspace 清空。
- **函数** `func CtrlDown() bool` — CtrlDown / ShiftDown / AltDown 当前修饰键状态（快捷键组件用）。
- **函数** `func ShiftDown() bool`
- **函数** `func AltDown() bool`
- **函数** `func KeyName(vk uint32) string` — KeyName 虚拟键码转显示名（快捷键展示 / 序列化）。
- **函数** `func NewTime(key string) *TimeState` — NewTime 创建时间选择状态。
- **函数** `func TimePicker(st *TimeState, value string, onChange func(string)) *Node` — TimePicker 时间选择框（点击展开时/分/秒列）。
- **函数** `func TimeLayers() *Node` — TimeLayers 时间浮层（根 Stack 末尾）。
- **函数** `func NewCascade(key string) *CascadeState` — NewCascade 创建级联状态。
- **函数** `func Cascader(st *CascadeState, opts []CascadeOption, path []string, placeholder string, onChange func([]string)) *Node` — Cascader 级联选择框（省市区 / 分类等）。
- **函数** `func CascaderLayers() *Node` — CascaderLayers 级联浮层（根 Stack 末尾）。
- **函数** `func Upload(key string, files []UploadFile, opts UploadOptions, onPick func(), onRemove func(int), onRetry func(int)) *Node` — Upload 上传列表：选择文件、进度、重试、移除、拖入。
- **函数** `func NewColor(key string) *ColorState` — NewColor 创建取色器状态。
- **函数** `func ColorPicker(st *ColorState, value string, onChange func(string)) *Node` — ColorPicker 取色器（点击展开 SV 面板 + 色相条 + HEX 输入）。
- **函数** `func ColorLayers() *Node` — ColorLayers 取色浮层（根 Stack 末尾）。
- **方法** `func AutoCompleteState.Close()` — Close 收起浮层。
- **方法** `func TimeState.TimeValue() string` — TimeValue 当前时间 HH:MM 或 HH:MM:SS。
- **方法** `func CascadeState.CascaderPath() string` — CascaderPath 当前选中的标签路径。
- **方法** `func ColorState.SetColor(hex string)` — SetColor 设置颜色（HEX / #RGB），同步 HSV。
- **方法** `func ColorState.Hex() string` — Hex 当前颜色 HEX。
- **变量** `var ScreenPickColor` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，
- **变量** `var ScreenPixelAt` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，
- **变量** `var CursorAt` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，
- **变量** `var LeftButtonDown` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，
- **变量** `var RightButtonDown` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，
- **变量** `var KeyDown` — 屏幕取色钩子（Windows 由 system_extra_windows.go 的 init 赋值；其他平台为 nil，

## `ui/display_extra.go`

- **类型** `type CarouselOptions struct { Height float32 Interval float32 // 自动播放间隔（秒，默认 4） Autoplay bool Loop bool Arrows bool Dots bool }` — CarouselOptions 轮播参数。
- **类型** `type TimelineItem struct { Time string Title string Sub string Color string Body *Node }` — TimelineItem 时间线节点。
- **类型** `type StepItem struct { Title string Sub string }` — StepItem 步骤。
- **类型** `type KV struct{ K, V string }` — KV 键值对。
- **类型** `type StatOptions struct { Prefix string Suffix string Decimals int Trend float32 // 正数上升 / 负数下降 / 0 无 Animate bool }` — StatOptions 指标卡参数。
- **类型** `type CalendarEvent struct { Date string // YYYY-MM-DD Label string Color string }` — CalendarEvent 日历事件点。
- **类型** `type CalendarState struct { Key string Y int M int }` — CalendarState 日历状态。
- **类型** `type PreviewImage struct { Src string Title string }` — PreviewImage 预览图片。
- **类型** `type AvatarItem struct { Initial string Color string Src string }` — AvatarItem 头像项。
- **类型** `type TourStep struct { Title string Body string Target string // 目标节点 Key（为空时居中显示） Place string // bottom / top / right / left }` — TourStep 引导步骤。
- **类型** `type TourState struct { Key string Open bool Step int Seen map[string]bool }` — TourState 引导状态。
- **函数** `func Carousel(key string, slides []*Node, opts CarouselOptions) *Node` — Carousel 轮播图：指示器、左右箭头、自动播放、循环。
- **函数** `func Timeline(items []TimelineItem) *Node` — Timeline 垂直时间线。
- **函数** `func Steps(items []StepItem, active int) *Node` — Steps 步骤条（横向；active 为当前步，0 起）。
- **函数** `func Segmented(keys, labels []string, active string, onPick func(string)) *Node` — Segmented 分段控制器（滑动指示条的 Tabs 单选变体）。
- **函数** `func Descriptions(items []KV, columns int) *Node` — Descriptions 键值详情列表（columns 列，自适应）。
- **函数** `func Statistic(key, label string, value float64, opts StatOptions) *Node` — Statistic 指标数值（可选滚动动效 + 趋势）。
- **函数** `func NewCalendar(key string) *CalendarState` — NewCalendar 创建日历状态。
- **函数** `func Calendar(st *CalendarState, events []CalendarEvent, value string, onChange func(string)) *Node` — Calendar 独立日历组件（月视图 + 事件点）。
- **函数** `func Watermark(content string, children ...*Node) *Node` — Watermark 区域水印（平铺文字，低透明度，不拦截交互）。
- **函数** `func ImagePreviewOpen(images []PreviewImage, idx int)` — ImagePreviewOpen 打开通用图片灯箱。
- **函数** `func ImagePreviewOpenFiles(paths []string, idx int)` — ImagePreviewOpenFiles 用图片路径打开灯箱。
- **函数** `func ImagePreviewClose()` — ImagePreviewClose 关闭灯箱。
- **函数** `func ImagePreviewLayer() *Node` — ImagePreviewLayer 灯箱浮层（根 Stack 末尾）。
- **函数** `func AvatarGroup(items []AvatarItem, max int) *Node` — AvatarGroup 头像叠放组（超出 max 显示 +N）。
- **函数** `func NewTour(key string) *TourState` — NewTour 创建引导状态（Seen 持久化到偏好）。
- **函数** `func TourLayer(st *TourState, steps []TourStep) *Node` — TourLayer 引导浮层（根 Stack 末尾）。targets 用于显示目标高亮。
- **方法** `func TourState.Start()` — StartTour 开始引导。
- **方法** `func TourState.Finish()` — Finish 结束并记录已看。

## `ui/cards.go`

- **类型** `type MediaItem struct { Key string Cover string // 封面图路径；为空时用渐变占位 Title string Sub string // 副标题（作者 / 来源） Badge string // 左上角角标（视频 / 电影…） BadgeBg string Score string // 右上角评分（如 "8.5"） Tags []string Action string // 底部按钮文字（为空不显示） Accent string // 主题色（渐变占位 / 标签底色） ActionBg string // 按钮底色（默认主色渐变） }` — MediaItem 一张图文卡片的数据。
- **类型** `type MediaCardOptions struct { Width float32 // 卡片宽度，默认 190 CoverH float32 // 封面高度，默认 118 OnPick func(it MediaItem) OnAction func(it MediaItem) }` — MediaCardOptions 图文卡片参数。
- **类型** `type ListRowItem struct { Key string Title string Sub string Meta string // 右侧说明（时间 / 大小） Tag string TagColor string Icon string // 头像文字（与 Color 配合） Glyph string // 图标字形（优先于 Icon；如 ✓ ★ ⚑） Color string // 图标 / 头部色块颜色 Checked bool // 多选态（选项框显示） }` — ListRowItem 列表行数据。
- **类型** `type ListRowsOptions struct { Key string Selected string Dividers bool // 行间分隔线 CheckBox bool // 显示多选框 Compact bool // 紧凑行（高 36） OnPick func(it ListRowItem, i int) OnCheck func(it ListRowItem, i int, on bool) }` — ListRowsOptions 列表行参数。
- **类型** `type PostItem struct { Key string Author string Avatar string // 头像文字（空则取昵称首字） AvatarBg string Time string Role string // 身份标签（如「楼主」「开发者」） Body string Images []string // 配图路径 Tags []string Likes int Comments int Shares int Liked bool }` — PostItem 一条社区帖子。
- **类型** `type PostCardOptions struct { Width float32 // 默认 0 = 填满父容器 OnLike func(it PostItem) OnOpen func(it PostItem) }` — PostCardOptions 帖子卡片参数。
- **函数** `func MediaCard(it MediaItem, opts MediaCardOptions) *Node` — MediaCard 图文卡片：封面（含角标 / 评分）→ 标题 → 标签 → 操作按钮。
- **函数** `func MediaCards(items []MediaItem, opts MediaCardOptions, gap float32) *Node` — MediaCards 批量构建图文卡片（Wrap 流式排列，放不下自动换行）。
- **函数** `func ListRows(items []ListRowItem, opts ListRowsOptions) *Node` — ListRows 列表组件：左侧图标 / 头像 + 标题与副标题 + 右侧标签 / 说明。
- **函数** `func PostCard(it PostItem, opts PostCardOptions) *Node` — PostCard 社区帖子卡片：头像 / 昵称 / 时间 → 正文 → 图片 → 点赞评论转发。

## `ui/tree.go`

- **类型** `type TreeKind uint8` — TreeKind 树节点类型（决定图标）。
- **类型** `type TreeNode struct { Key string // 唯一键（展开 / 选中按它保存）；为空时按父路径自动生成 Label string Kind TreeKind Tag string // 右侧标签（如「12」「已改」） TagColor string Disabled bool Children []TreeNode }` — TreeNode 文件树节点。Children 非空即为可展开分支。
- **类型** `type TreeOptions struct { Key string Expanded []string // 默认展开的节点 Key（仅首次构建时生效，之后由点击记忆） Selected string Indent float32 // 每级缩进，默认 16 RowH float32 // 行高，默认 30 OnPick func(n TreeNode) OnToggle func(n TreeNode, open bool) }` — TreeOptions 文件树参数。
- **函数** `func FileTree(items []TreeNode, opts TreeOptions) *Node` — FileTree 文件树：行内「箭头 + 图标 + 名称 + 右侧标签」，选中行高亮。
- **函数** `func TreeSetOpen(key, nodeKey string, open bool)` — TreeSetOpen 展开 / 收起某个节点（外部按钮「全部展开 / 折叠」用）。
- **函数** `func TreeSetAllOpen(key string, items []TreeNode, open bool)` — TreeSetAllOpen 展开 / 收起整棵树。
- **函数** `func TreeSelected(key string) string` — TreeSelected 当前选中节点的 Key（未选中返回空串）。
- **函数** `func TreeSetSelected(key, nodeKey string)` — TreeSetSelected 设置选中节点。
- **常量** `const TreeFolder = iota`
- **常量** `const TreeFile`
- **常量** `const TreeLink`

## `ui/pager.go`

- **类型** `type PagerOptions struct { Key string Page int // 当前页（1 起） Total int // 总页数 Window int // 当前页左右各显示几页，默认 2 Locked int // 前 Locked 页之后上锁（0 = 不上锁） OnPick func(page int) }` — PagerOptions 分页器参数。
- **函数** `func Pager(opts PagerOptions) *Node` — Pager 数字分页器。

## `ui/table_extra.go`

- **类型** `type TableGroup struct { Label string Start int Count int }` — TableGroup 分组区间（Start 起 Count 行）。
- **类型** `type TableVConfig struct { Key string Headers []string // Widths 列宽；0 表示弹性列（均分剩余宽度）。 Widths []float32 RowH float32 Height float32 Count int // Row 构建第 i 行单元格（数量与 Headers 对齐）。 Row func(i int) []*Node // Groups 非空时按分组展示（可折叠）。 Groups []TableGroup // Footer 合计行内容。 Footer []*Node // Resizable 允许拖拽表头分隔条调整列宽（持久化到偏好）。 Resizable bool // SortCol / SortAsc 当前排序列（-1 表示无）；OnSort 点击表头时回调。 SortCol int SortAsc bool OnSort func(col int) OnRow func(i int) }` — TableVConfig 虚拟表格参数。
- **类型** `type ReorderListOptions struct { Key string Count int RowH float32 Height float32 Row func(i int) *Node OnMove func(from, to int) Disabled bool }` — ReorderListOptions 拖拽重排列表参数。
- **函数** `func TableV(cfg TableVConfig) *Node` — TableV 虚拟表格（只渲染可见行）。
- **函数** `func TableColWidth(key string, col int) float32` — TableColWidth 读取某列当前宽度（拖拽后）。
- **函数** `func SetTableColWidth(key string, col int, w float32)` — SetTableColWidth 设置某列宽度（并持久化）。
- **函数** `func ReorderList(opts ReorderListOptions) *Node` — ReorderList 固定行高、整行拖动重排的列表（插入位置高亮 + 松手回调）。
- **函数** `func MoveSlice[T any](items []T, from, to int) []T` — MoveSlice 就地移动元素（from → to），供 OnMove 回调使用。

## `ui/vlist.go`

- **类型** `type VListOptions struct { // RowHeight 行高（逻辑像素）。 RowHeight float32 // Height 视口高度。 Height float32 // Overscan 视口外额外渲染的行数（默认 4，减少快速滚动白边）。 Overscan int // Gap 行间距。 Gap float32 // Empty 列表为空时显示的内容（默认 EmptyState）。 Empty *Node }` — VListOptions 虚拟列表参数（VListOpt 使用）。
- **函数** `func VList(key string, count int, rowH, height float32, build func(i int) *Node) *Node` — VList 固定行高虚拟列表。
- **函数** `func VListOpt(key string, count int, build func(i int) *Node, opts VListOptions) *Node` — VListOpt 带完整参数的虚拟列表。
- **函数** `func VListVar(key string, heights []float32, height float32, build func(i int) *Node) *Node` — VListVar 变行高虚拟列表（heights 为每行高度，内部做前缀和）。
- **函数** `func VListVisible(count int, off, viewH, rowH float32, overscan int) (int, int)` — VListVisible 返回给定偏移下应渲染的行区间 [first, last)（供自定义虚拟化复用 / 测试）。

## `ui/propgrid.go`

- **类型** `type PropKind int` — PropKind 编辑器类型。
- **类型** `type Prop struct { Key string Label string Group string Kind PropKind Value string Options []string Hint string // Custom 自定义编辑器（Kind == PropCustom）。 Custom func(value string, onChange func(string)) *Node }` — Prop 一条属性。
- **函数** `func PropertyGrid(key string, props []Prop, onChange func(key, val string)) *Node` — PropertyGrid 渲染属性表；onChange 在每次修改时回调。
- **函数** `func PropResetButton(grid, propKey string, def string, onChange func(key, val string)) *Node` — PropResetButton 给属性行绑定「重置为默认值」动作（默认渲染的 ↺ 只做展示，业务可自行接管）。
- **常量** `const PropText = iota`
- **常量** `const PropNumber`
- **常量** `const PropBool`
- **常量** `const PropEnum`
- **常量** `const PropColor`
- **常量** `const PropFile`
- **常量** `const PropReadonly`
- **常量** `const PropCustom`

## `ui/menu.go`

- **类型** `type MenuItem struct { Label string Hint string // 右侧快捷键/说明 Icon string // 左侧符号（可选） OnPick func() Children []MenuItem // 非空 = 子菜单 Separator bool // 分隔线 Disabled bool Checked bool }` — MenuItem 菜单项。
- **类型** `type MenuDef struct { Label string Items []MenuItem }` — MenuDef 顶级菜单（标题 + 下拉项）。
- **类型** `type MenuBarState struct { Open int // 打开的顶级菜单索引，-1 = 关闭 Sub []int // 子菜单展开路径（逐级的项索引） // ItemX 由 MenuBar 计算：各顶级菜单按钮相对菜单栏左边缘的 x 偏移。 ItemX []float32 // AutoHide 鼠标离开菜单后自动隐藏的等待时间（0 = 默认 3s，<0 = 不自动隐藏）。 AutoHide time.Duration // lastHover 最近一次悬停在菜单（菜单栏或浮层）内的时间； // hideTimer 到点请求重绘，让帧内检查能关掉菜单（期间不持续刷新，省 CPU）。 lastHover time.Time hideTimer *time.Timer // inside 指针当前是否停在菜单栏/浮层内（停住不动时不应超时隐藏） inside bool }` — MenuBarState 菜单栏状态（跨帧保持；由调用方持有）。
- **函数** `func NewMenuBar() *MenuBarState` — NewMenuBar 创建菜单栏状态。
- **函数** `func MenuBar(st *MenuBarState, menus []MenuDef) *Node` — MenuBar 渲染标题栏里的横向菜单按钮行。
- **函数** `func MenuLayer(st *MenuBarState, menus []MenuDef, anchorX, anchorY float32) *Node` — MenuLayer 下拉菜单浮层（含多级子菜单）。
- **函数** `func MenuPanel(x, y float32, items []MenuItem, width float32) *Node` — MenuPanel 浮层菜单（放在 Stack 中，用绝对坐标定位）。支持子菜单、分隔线、勾选与禁用。
- **函数** `func WindowButtons() *Node` — WindowButtons 自绘标题栏右侧的最小化 / 最大化 / 关闭按钮。
- **方法** `func MenuBarState.Close()` — Close 关闭菜单（含所有子菜单）。
- **方法** `func MenuBarState.IsOpen() bool` — IsOpen 当前是否有菜单展开。

## `ui/arrow.go`

- **函数** `func Arrow(color any) *Node` — Arrow 子菜单指示三角节点（右向）。

## `ui/dialog.go`

- **类型** `type DialogKind int` — DialogKind 弹窗语义（决定图标色与默认按钮色）。
- **类型** `type DialogOpts struct { Key string // 动画 Key（同一弹窗保持固定） Open bool Kind DialogKind Title string Body string // 纯文本正文（BodyNode 非空时忽略） BodyNode *Node // 自定义正文 Width float32 // Closable 右上角显示关闭按钮，并允许点遮罩关闭。 Closable bool OnClose func() // Footer 自定义底部（为 nil 时用 OKLabel/CancelLabel 生成）。 Footer *Node OKLabel string CancelLabel string OnOK func() OnCancel func() // Blur 遮罩是否叠加毛玻璃（真模糊窗口背景）。 Blur float32 }` — DialogOpts 弹窗内容配置。
- **类型** `type DrawerOpts struct { Key string Open bool Title string Body *Node Width float32 OnClose func() }` — DrawerOpts 侧边抽屉配置。
- **函数** `func ModalPanel(o DialogOpts) *Node` — ModalPanel 通用弹窗：遮罩 + 卡片（标题 / 图标 / 正文 / 底部按钮），带淡入动画。
- **函数** `func AlertDlg(key string, open bool, kind DialogKind, title, body string, onClose func()) *Node` — AlertDlg 提示弹窗（单个「知道了」按钮）。
- **函数** `func ConfirmDlg(key string, open bool, kind DialogKind, title, body, okLabel string, onCancel, onOK func()) *Node` — ConfirmDlg 确认弹窗（取消 + 确定）。
- **函数** `func PromptDlg(key string, open bool, kind DialogKind, title, label, value, placeholder, okLabel string, onOK func(string), onCancel func()) *Node` — PromptDlg 输入弹窗（回车提交 / Esc 取消）。
- **函数** `func Drawer(o DrawerOpts) *Node` — Drawer 右侧滑出抽屉（AbsFill 遮罩 + 右侧面板，淡入）。
- **常量** `const DlgInfo = iota`
- **常量** `const DlgSuccess`
- **常量** `const DlgWarn`
- **常量** `const DlgError`

## `ui/toast.go`

- **函数** `func ToastShow(msg string)` — ToastShow 显示一条轻提示，默认 ToastDefaultSeconds 秒后自动隐藏；msg 为空等同 ToastHide。
- **函数** `func ToastShowFor(msg string, seconds float32)` — ToastShowFor 按指定秒数显示轻提示（seconds <= 0 时用默认时长）。
- **函数** `func ToastHide()` — ToastHide 立即隐藏轻提示。
- **函数** `func ToastVisible() bool` — ToastVisible 当前是否有轻提示在显示（含尚未被下一帧绘制成空的过期态）。
- **函数** `func ToastLayer() *Node` — ToastLayer 渲染当前轻提示（带淡入淡出）；到点后播完退出动画再消失。
- **常量** `const ToastDefaultSeconds = 2` — ToastDefaultSeconds 轻提示默认显示时长（秒）。

## `ui/notify.go`

- **类型** `type NoticeButton struct { Label string Run func() }` — NoticeButton 通知中的按钮。
- **类型** `type Notice struct { ID string Kind string Title string Body string Icon string Buttons []NoticeButton Progress float32 // >=0 且 <=1 时显示进度条 Duration float32 // 秒，<=0 用 4 秒；<0 表示常驻 }` — Notice 一条通知。
- **函数** `func NoticeMax(n int)` — NoticeMax 设置同时显示的最大条数（默认 5，超出淘汰最旧）。
- **函数** `func Notify(n Notice)` — Notify 显示一条通知（同 ID 就地覆盖并重置计时）。
- **函数** `func NotifyProgress(id string, progress float32, body string)` — NotifyProgress 更新带进度通知（不存在时创建）。
- **函数** `func NotifyClose(id string)` — NotifyClose 关闭指定通知。
- **函数** `func NotifyClear()` — NotifyClear 关闭全部通知。
- **函数** `func NotifyCount() int` — NotifyCount 当前通知数量。
- **函数** `func NotifyLayer() *Node` — NotifyLayer 通知浮层（根 Stack 末尾）。右上角堆叠，悬停暂停过期。
- **函数** `func NoticeText(n Notice) string` — NoticeText 通知的纯文本摘要（调试 / 无障碍朗读）。
- **常量** `const NoticeInfo = "info"` — NoticeKind 通知类型。
- **常量** `const NoticeSuccess = "success"` — NoticeKind 通知类型。
- **常量** `const NoticeWarn = "warn"` — NoticeKind 通知类型。
- **常量** `const NoticeError = "error"` — NoticeKind 通知类型。

## `ui/states.go`

- **函数** `func SkeletonBlock(w, h float32) *Node` — SkeletonBlock 骨架块（shimmer 流光）。
- **函数** `func SkeletonText(lines int, width float32) *Node` — SkeletonText 多行文本骨架（最后一行 60% 宽）。
- **函数** `func SkeletonCard(width float32) *Node` — SkeletonCard 卡片骨架（头像 + 两行文字）。
- **函数** `func LoadingOverlay(show bool, label string) *Node` — LoadingOverlay 区域级加载遮罩：覆盖父容器、拦截点击、居中 spinner + 文案。
- **函数** `func EmptyState(icon, title, sub, actionLabel string, onAction func()) *Node` — EmptyState 空状态：插画位（图标字形）+ 标题 + 说明 + 可选操作按钮。
- **函数** `func ResultPage(kind, title, sub string, actions ...*Node) *Node` — ResultPage 整页结果反馈（成功 / 失败 / 警示 / 404 / 500）。
- **函数** `func AsyncSection(loading bool, errMsg string, empty bool, onRetry func(), body func() *Node) *Node` — AsyncSection 四态容器：加载中 / 错误 / 空 / 正常内容。

## `ui/ellipsis.go`

- **类型** `type EllipsisOptions struct { Size float32 Weight int32 Color string LineH float32 // Expandable 是否显示「展开 / 收起」。 Expandable bool }` — EllipsisOptions 省略文本参数。
- **函数** `func EllipsisText(key, text string, lines int, width float32, opts EllipsisOptions) *Node` — EllipsisText 多行截断文本（默认 2 行 + 展开按钮）。

## `ui/split.go`

- **类型** `type SplitOptions struct { // Dir Horizontal = 左右分栏（默认）；Vertical = 上下分栏。 Dir Direction // Pos 初始比例 0..1（0 = 0.28）。 Pos float32 // MinA / MinB 两侧最小尺寸（逻辑像素）。 MinA, MinB float32 // HandleW 分隔条厚度（默认 8）。 HandleW float32 // PrefsKey 非空时把比例写入偏好（重启恢复）。 PrefsKey string // OnChange 比例变化回调（拖动过程中持续触发）。 OnChange func(pos float32) }` — SplitOptions 分栏参数。
- **函数** `func SplitPos(key string) float32` — SplitsPos 读取某分栏当前比例（0..1）。
- **函数** `func SetSplitPos(key string, pos float32, prefsKey string)` — SetSplitPos 设置分栏比例并持久化（代码里做「收起侧栏」等操作）。
- **函数** `func Split(key string, a, b *Node, opts SplitOptions) *Node` — Split 构建分栏。a 占 Pos 比例，b 占剩余。

## `ui/dock.go`

- **类型** `type DockPanel struct { Title string Icon string Content func() *Node // Badge 非空时在页签上显示小标记（如未读数）。 Badge string }` — DockPanel 一个可停靠面板。
- **类型** `type DockState struct { Key string // Left / Right / Bottom 各侧面板 ID 顺序。 Left, Right, Bottom []string // Sizes 各侧尺寸（像素）与折叠态。 LeftW, RightW, BottomH float32 Collapsed map[string]bool // Active 每个区域当前页签（区域键：left/right/bottom）。 Active map[string]string }` — DockState 停靠布局状态。
- **函数** `func NewDock(key string) *DockState` — NewDock 创建（或复用）停靠状态。
- **函数** `func DockLayout(st *DockState, center *Node, panels map[string]DockPanel) *Node` — DockLayout 渲染停靠布局。
- **方法** `func DockState.Save()` — Save 持久化布局。
- **方法** `func DockState.TogglePanel(id string)` — TogglePanel 显示/折叠某面板（侧栏首个面板折叠时整侧收起）。
- **方法** `func DockState.Activate(side, id string)` — Activate 切换某区域的活动页签。

## `ui/transitions.go`

- **函数** `func KeepAlive(key string, open bool, dur float32) (bool, float32)` — KeepAlive 浮层进出场状态：
- **函数** `func ResetKeepAlive(key string)` — ResetKeepAlive 清除某个浮层的过渡状态（强制下次重新入场）。
- **函数** `func PopOffset(t float32) (opacity, dy float32)` — PopOffset 浮层统一造型：返回不透明度与纵向位移（0.96→1 缩放用 ScaleOf）。
- **函数** `func ScaleOf(t float32) float32` — ScaleOf 浮层缩放（配合 Translate 做「弹入」观感，绘制层不直接支持缩放时用于尺寸计算）。
- **函数** `func Scrim(t float32, key string) *Node` — Scrim 遮罩层（opacity 0..1），放在 Stack 中铺满父容器。
- **函数** `func Duration(d float32) float32` — Duration 按 reduced-motion 设置调整过渡时长（开启时归零）。
- **函数** `func ReducedMotion() bool` — ReducedMotion 系统是否要求「减弱动态效果」。
- **函数** `func ResetTimeline(key string)` — ResetTimeline 重置某个编排时间轴。
- **函数** `func StaggerAt(key string, i int, step, dur float32) float32` — StaggerAt 错峰动画进度：第 i 项在 start 之后延迟 i*step 开始，持续 dur。
- **函数** `func SequenceAt(key string, i int, dur float32) float32` — SequenceAt 串行动画：第 i 段从 start + i*dur 开始，返回该段自身进度。
- **函数** `func ParallelAt(key string, dur float32) float32` — ParallelAt 并行动画：所有段同时开始，返回当前进度。
- **函数** `func AnimatedNumber(key string, target float64, dur float32, decimals int) float64` — AnimatedNumber 数值缓动到目标值（statistic / KPI 用）。
- **函数** `func FormatNumber(v float64, decimals int) string` — FormatNumber 千分位格式化（decimals 为小数位）。
- **函数** `func ShimmerPhase(speed float32) float32` — ShimmerPhase 骨架屏流光相位 0..1（循环）。

## `ui/anim.go`

- **类型** `type Ease func(t float32) float32` — Ease 缓动函数：输入 0..1 输出 0..1（可越界，如回弹）。
- **类型** `type Key struct { At float32 // 0..1 Value float32 Ease Ease }` — Key 时间轴关键帧。
- **函数** `func Tween(key string, target float32, dur float32, ease Ease) float32` — Tween 按 key 做补间动画，返回当前值。
- **函数** `func TweenFrom(key string, initial, target, dur float32, ease Ease) float32` — TweenFrom 首次调用时以 initial 作为起点（用于入场动画）。
- **函数** `func Spring(key string, target float32, stiffness, damping float32) float32` — Spring 弹簧动画：按刚度/阻尼做数值积分，产生自然的过冲与回弹。
- **函数** `func Keyframes(key string, keys []Key, dur float32, loop bool) float32` — Keyframes 按关键帧插值（自动循环或单次）。
- **函数** `func ResetAnim(key string)` — ResetAnim 重置指定 key 的动画（重建入场动画时用）。
- **函数** `func AnimActive() bool` — AnimActive 是否仍有动画在播放（业务一般不需要调用）。
- **变量** `var EaseLinear = func(t float32) float32 { return t }`
- **变量** `var EaseInQuad = func(t float32) float32 { return t * t }`
- **变量** `var EaseOutQuad = func(t float32) float32 { return 1 - (1-t)*(1-t) }`
- **变量** `var EaseInOutQuad = func(t float32) float32 { if t < 0.5 { return 2 * t * t } return 1 - 2*(1-t)*(1-t) }`
- **变量** `var EaseInCubic = func(t float32) float32 { return t * t * t }`
- **变量** `var EaseOutCubic = func(t float32) float32 { u := 1 - t return 1 - u*u*u }`
- **变量** `var EaseInOutCubic = func(t float32) float32 { if t < 0.5 { return 4 * t * t * t } u := -2*t + 2 return 1 - u*u*u/2 }`
- **变量** `var EaseOutQuart = func(t float32) float32 { u := 1 - t return 1 - u*u*u*u }`
- **变量** `var EaseOutBack = func(t float32) float32 { const c1 = 1.70158 const c3 = c1 + 1 u := t - 1 return 1 + c3*u*u*u + c1*u*u }`
- **变量** `var EaseOutElastic = func(t float32) float32 { if t <= 0 { return 0 } if t >= 1 { return 1 } const c4 = float32(2 * math.Pi / 3) return float32(math.Pow(2, -10*float64(t)))*float32(math.Sin((float64(t)*10-0.75)*float64(c4))) + 1 }`
- **变量** `var EaseOutBounce = func(t float32) float32 { const n1 = 7.5625 const d1 = 2.75 switch { case t < 1/d1: return n1 * t * t case t < 2/d1: t -= 1.5 / d1 return n1*t*t + 0.75 case t < 2.5/d1: t -= 2.25 / d1 return n1*t*t + 0.9375 default: t -= 2.625 / d1 return n1*t*t + 0.984375 } }`

## `ui/flip.go`

- **函数** `func FLIP(key string, n *Node, dur float32) *Node` — FLIP 布局动画：元素位置/尺寸变化时从旧位置平滑过渡到新位置。
- **函数** `func FLIPFrom(key string, n *Node, fromX, fromY float32, dur float32) *Node` — FLIPFrom 用显式旧矩形驱动一次 FLIP（适合虚拟列表 / 增删行）。
- **函数** `func ResetFLIP(key string)` — ResetFLIP 清除某个 FLIP 记录。
- **函数** `func ListEnter(key string, index int, n *Node) *Node` — ListEnter 列表项入场：前 n 项错峰淡入上移。
- **函数** `func TabSlide(key string, dir int) float32` — TabSlide 页签切换的滑动过渡：返回新页面相对位移（0 = 到位）。

## `ui/image.go`

- **函数** `func ImageSize(path string) (int, int)` — ImageSize 返回图片像素尺寸（带缓存，失败返回 0,0）。
- **函数** `func DecodeImage(path string) (*image.NRGBA, error)` — DecodeImage 解码图片为 NRGBA（非预乘，可直接上传 GL）。

## `ui/icon.go`

- **函数** `func DefaultIcon() []byte` — DefaultIcon 返回内置图标的 ICO 字节（只读，勿修改）。
- **函数** `func SetWindowIcon(ico []byte) bool` — SetWindowIcon 用 .ico 字节设置当前窗口图标（大小图标 + 窗口类图标）。

## `ui/i18n.go`

- **类型** `type TextDirection uint8` — TextDirection 文字方向。
- **常量** `DirLTR`
- **常量** `DirRTL`
- **类型** `type LocaleInfo struct` — LocaleInfo 一种语言的元数据：名称、方向、日期格式、月份 / 星期名、货币等。
- **函数** `func LoadCatalog(locale, jsonBody string) error` — LoadCatalog 载入一个语言的字符串表（JSON：{"key":"文本"}）。
- **函数** `func LoadCatalogFile(locale, path string) error` — LoadCatalogFile 从文件载入字符串表（桌面端；Web / 移动端请用 LoadCatalog + go:embed）。
- **函数** `func RegisterLocale(info LocaleInfo)` — RegisterLocale 注册 / 覆盖一种语言的元数据（名称、方向、日期与月份名等）。
- **函数** `func Locales() []LocaleInfo` — Locales 已注册语言（按注册顺序；内置 zh-CN / en-US 在最前）。
- **函数** `func HasLocale(locale string) bool` — HasLocale 是否已载入该语言的字符串表或元数据。
- **函数** `func LocaleInfoOf(locale string) (LocaleInfo, bool)` — LocaleInfoOf 取某种语言的元数据。
- **函数** `func SetLocale(locale string)` — SetLocale 切换语言（未知语言也接受：只影响格式化与方向，翻译走回退链）。
- **函数** `func UseLocale(locale string)` — UseLocale 切换语言并记住选择（写偏好 ui.locale），下次启动 AutoLocale 会恢复它。
- **函数** `func Locale() string` — Locale 当前语言（canonical，如 "zh-CN"）。
- **函数** `func LocaleGeneration() uint64` — LocaleGeneration 语言代数：每次切换 +1，业务可据此判断缓存是否失效。
- **函数** `func SetFallbackLocale(locale string)` — SetFallbackLocale 设置兜底语言（翻译缺失时最后查它，默认 en-US）。
- **函数** `func FallbackLocale() string` — FallbackLocale 兜底语言。
- **函数** `func PrefLocale() string` — PrefLocale 偏好里保存的语言（未设置返回空串）。
- **函数** `func AutoLocale() string` — AutoLocale 自动选择语言：偏好 > 系统语言 > 兜底语言，并立即生效。
- **函数** `func MatchLocale(prefs ...string) string` — MatchLocale 在已注册语言里挑出与候选列表最匹配的一个（BCP-47 协商）。
- **函数** `func OnLocaleChange(fn func(code string)) func()` — OnLocaleChange 注册语言切换回调（返回取消函数）。
- **函数** `func T(key string) string` — T 取当前语言的翻译。
- **函数** `func TOr(key, fallback string) string` — TOr 取翻译，缺失时返回 fallback（只在当前语言链里查，渐进接入 i18n 用）。
- **函数** `func Tf(key string, args map[string]string) string` — Tf 带命名参数的翻译（占位 {name}），例：
- **函数** `func Tn(key string, n int) string` — Tn 复数翻译：按当前语言的 CLDR 规则在 key.one / key.other / key.few… 中取值，
- **函数** `func Tnf(key string, n int, args map[string]string) string` — Tnf 复数 + 命名参数。
- **函数** `func Ts(key string, args ...string) string` — Ts 位置参数版本（把 {0} {1}… 依次替换）。
- **函数** `func SetRTL(on bool)` — SetRTL 手动开关从右到左布局（会关闭语言方向的自动切换）。
- **函数** `func SetRTLAuto(on bool)` — SetRTLAuto 是否随语言自动切换 RTL（默认开启）。
- **函数** `func IsRTL() bool` — IsRTL 当前是否 RTL 布局。
- **函数** `func DirAlign() Align` — DirAlign 文本默认对齐方向：RTL 时返回 AlignEnd。
- **函数** `func MirrorRow(children ...*Node) *Node` — MirrorRow 行布局镜像：RTL 时反转子节点顺序。
- **函数** `func MirrorIcon(glyph string) string` — MirrorIcon 需要镜像的方向图标（← →、‹ ›）。

## `ui/i18n_format.go`

- **函数** `func LocaleNumber(v float64, decimals int) string` — LocaleNumber 按当前语言格式化数字（decimals 为小数位；-1 表示最多 3 位且去掉尾零）。
- **函数** `func LocaleInt(n int64) string` — LocaleInt 整数（带千分位）。
- **函数** `func LocalePercent(v float64, decimals int) string` — LocalePercent 按当前语言格式化百分比（v 已经是百分比数值，如 12.5 → "12.5%"）。
- **函数** `func LocaleCurrency(v float64, code string) string` — LocaleCurrency 按当前语言格式化金额。
- **函数** `func LocaleDate(t time.Time, style string) string` — LocaleDate 按当前语言格式化日期：style = "short" / "medium" / "long" / "full"。
- **函数** `func LocaleTime(t time.Time, style string) string` — LocaleTime 按当前语言格式化时间：style = "short" / "long"。
- **函数** `func LocaleDateTime(t time.Time, style string) string` — LocaleDateTime 日期 + 时间（用空格连接，RTL 语言用 "，" 之外的普通空格）。
- **函数** `func LocaleDatePattern(t time.Time, pattern string) string` — LocaleDatePattern 按自定义图案格式化日期。
- **函数** `func LocaleWeekday(wd time.Weekday, full bool) string` — LocaleWeekday 星期名（full = 完整写法，否则缩写）。
- **函数** `func LocaleMonth(m time.Month, full bool) string` — LocaleMonth 月份名（full = 完整写法，否则缩写）。
- **函数** `func LocaleWeekStart() int` — LocaleWeekStart 一周第一天（0 = 周日，1 = 周一）。
- **函数** `func LocaleRelative(t, now time.Time) string` — LocaleRelative 相对时间（"3 分钟前" / "in 2 hours"）。
- **函数** `func LocaleDuration(d time.Duration) string` — LocaleDuration 时长（"1 小时 5 分钟" / "1 h 5 min"）。
- **函数** `func LocaleList(items []string) string` — LocaleList 用当前语言的连接符拼接列表（"1、2 和 3" / "1, 2 and 3"）。
- **函数** `func LocaleBytes(n int64) string` — LocaleBytes 字节数（"1.5 MB" / "1,5 MB"）。

## `ui/i18n_platform.go`

- **函数** `func SetSystemLanguages(codes ...string)` — SetSystemLanguages 注入系统语言列表（按优先级）。
- **函数** `func RefreshSystemLanguages()` — RefreshSystemLanguages 清空缓存，下次 SystemLanguages 重新探测（用户在系统里改了语言）。
- **函数** `func SystemLanguages() []string` — SystemLanguages 系统语言列表（按优先级；最差返回 ["en-US"]）。
- **函数** `func SystemLocaleTag() string` — SystemLocaleTag 系统首选语言（等价 SystemLanguages()[0]）。

## `ui/i18n_builtin.go`

- **函数** `func AvailableLocales() []LocaleInfo` — AvailableLocales 随包提供翻译的语言（语言切换器只列这些）。

## `ui/i18n_rtl.go`

- **函数** `func MirrorX(originX, boxW, x, w float32) float32` — mirrorX 单点/矩形水平镜像（绘制期给自绘控件用）。

## `ui/i18n_widget.go`

- **函数** `func LocaleOptions() (codes, labels []string)` — LocaleOptions 语言切换器的选项（代码 + 本地名称）。
- **函数** `func LocaleLabel(locale string) string` — LocaleLabel 某语言的展示名（本地写法，缺失时退回英文名 / 代码）。
- **函数** `func LocaleSwitcher(key string) *Node` — LocaleSwitcher 现成的语言切换器（横向胶囊按钮，选中项高亮）。
- **函数** `func LocaleSegmented(key string) *Node` — LocaleSegmented 语言切换器（分段控件，适合 2~4 种语言放一行）。
- **函数** `func LocaleSettingsRow(key string) *Node` — LocaleSettingsRow 设置页里的一行：「语言 + 切换器」。


## `ui/command.go`

- **类型** `type Command struct { ID string Title string Sub string Group string Shortcut string Icon string Disabled bool Run func() }` — Command 一条可执行命令。
- **函数** `func RegisterCommand(c Command)` — RegisterCommand 注册（或覆盖）命令。
- **函数** `func UnregisterCommand(id string)` — UnregisterCommand 移除命令。
- **函数** `func Commands() []Command` — Commands 返回已注册命令（注册顺序）。
- **函数** `func CommandPaletteOpen() bool` — CommandPaletteOpen 面板是否打开。
- **函数** `func OpenCommandPalette()` — OpenCommandPalette 打开命令面板并聚焦搜索框。
- **函数** `func CloseCommandPalette()` — CloseCommandPalette 关闭面板。
- **函数** `func ToggleCommandPalette() bool` — ToggleCommandPalette 切换面板；返回值表示是否处理（有命令时）。供 App 的 Ctrl+K 调用。
- **函数** `func FuzzyScore(query, target string) int` — FuzzyScore 子序列匹配评分：连续命中 / 词首命中加权，无法匹配返回 -1。
- **函数** `func CommandPaletteLayer() *Node` — CommandPaletteLayer 渲染面板（放在根 Stack 末尾）。
- **函数** `func ColTextFaint() string` — ColTextFaint 弱化文字色（token 别名）。

## `ui/undo.go`

- **类型** `type UndoEntry struct { Label string Undo func() Redo func() }` — UndoEntry 一条可撤销操作。
- **类型** `type UndoStack struct { entries []UndoEntry index int Limit int txLabel string txUndo []func() txRedo []func() busy bool }` — UndoStack 命令栈（非线程安全，只在 UI 线程操作）。
- **函数** `func NewUndo() *UndoStack` — NewUndo 创建命令栈（默认上限 100 步）。
- **函数** `func DefaultUndo() *UndoStack` — DefaultUndo 默认命令栈（已绑定 Ctrl+Z / Ctrl+Y）。
- **函数** `func UndoHistoryPanel(u *UndoStack) *Node` — UndoHistoryPanel 历史面板。
- **方法** `func UndoStack.Do(label string, redo func(), undo func())` — Do 执行并记录一步操作。
- **方法** `func UndoStack.Push(label string, undo, redo func())` — Push 只记录（操作已由业务自己完成）。
- **方法** `func UndoStack.CanUndo() bool` — CanUndo / CanRedo 是否可撤销 / 重做。
- **方法** `func UndoStack.CanRedo() bool`
- **方法** `func UndoStack.Undo()` — Undo 撤销一步。
- **方法** `func UndoStack.Redo()` — Redo 重做一步。
- **方法** `func UndoStack.Clear()` — Clear 清空历史。
- **方法** `func UndoStack.Label() string` — Label 当前可撤销操作的名称（菜单显示「撤销：xxx」）。
- **方法** `func UndoStack.BeginTx(label string)` — BeginTx 开始事务（多次修改合并为一步）。
- **方法** `func UndoStack.Tx(undo, redo func())` — Tx 向当前事务追加一段反向 / 正向操作。
- **方法** `func UndoStack.CommitTx()` — CommitTx 提交事务（逆序执行反向操作）。
- **方法** `func UndoStack.History() []string` — UndoHistory 历史列表（调试 / 面板展示）。

## `ui/crash.go`

- **类型** `type CrashReport struct { Time time.Time Message string Stack string File string }` — CrashReport 一次崩溃的信息。
- **函数** `func SetCrashHandler(fn func(CrashReport))` — SetCrashHandler 注册崩溃回调（在 panic 恢复后、写盘后调用）。
- **函数** `func SetCrashDir(dir string)` — SetCrashDir 指定崩溃日志目录（默认用户配置目录/guis/crashes）。
- **函数** `func CrashLast() (CrashReport, bool)` — CrashLast 最近一次崩溃（无则 ok=false）。
- **函数** `func ReportCrash(v any, stack []byte) CrashReport` — ReportCrash 记录一次崩溃（Run 的 recover 与业务自捕获都可调用）。
- **函数** `func RecoverToReport(tag string)` — RecoverToReport 供业务在 goroutine 里 defer 使用：
- **函数** `func CrashPanel() *Node` — CrashPanel 崩溃日志面板（最近一次崩溃摘要 + 文件路径）。

## `ui/iconfont.go`

- **类型** `type IconDef struct { // Name 唯一名称（Icon 的参数），大小写敏感。 Name string // Label 展示名（图标库示例页 / 无障碍朗读用）。 Label string // Paths SVG path 数据，可含多条（同一份设计网格内叠加）。 Paths []string // Fill 为 true 时按奇偶规则填充（实心图标），否则描边。 Fill bool // Stroke 描边宽度（设计网格单位，0 = 默认 2）。 Stroke float32 }` — IconDef 一个图标的矢量定义（24×24 设计网格，与 SVG path 语法一致）。
- **函数** `func RegisterIcon(def IconDef) bool` — RegisterIcon 注册（或同名覆盖）一个图标，返回是否覆盖了已有同名图标。
- **函数** `func UnregisterIcon(name string)` — UnregisterIcon 移除一个图标（示例页演示注册自定义图标后还原用）。
- **函数** `func IconDefOf(name string) (IconDef, bool)` — IconDefOf 按名称取图标定义。
- **函数** `func IconExists(name string) bool` — IconExists 图标是否已注册。
- **函数** `func IconNames() []string` — IconNames 返回全部图标名（注册顺序，示例页按此铺网格）。
- **函数** `func IconCount() int` — IconCount 已注册图标数量。
- **函数** `func Icon(name string) *Node` — Icon 图标节点：默认 18×18，颜色为次要文字色（`.St(St().Text(0, 0, "#fff"))` 可改色）。
- **函数** `func IconSized(name string, size float32) *Node` — IconSized 指定边长（正方形）的图标节点。
- **函数** `func IconColored(name string, size float32, color any) *Node` — IconColored 指定尺寸与颜色的图标节点。
- **函数** `func IconStroked(name string, size, stroke float32) *Node` — IconStroked 指定线宽（设计网格单位，0 = 图标自带线宽）的图标节点。
- **函数** `func IconLabel(name, label string, size float32, color any) *Node` — IconLabel 图标 + 文字的一行（侧栏条目、按钮内图文、列表项通用）。
- **函数** `func IconAction(name string, size float32, onClick func()) *Node` — IconAction 方形图标按钮（图标居中、悬停提亮），工具栏 / 卡片右上角操作区用。
