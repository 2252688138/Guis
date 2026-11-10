<!-- group: 附录 -->
# 附录 C · 文本 / 多媒体 / 办公文档 API 速查

Markdown 渲染、富文本与编辑器、图表、视频、音频、频谱、解码后端的全部导出符号。
办公文档部分同时收录 `internal/office` 包（数据层）的 API。

> 说明：本附录为**自动生成**的完整符号清单，按源文件分组；
> 想按「用途」查找请先读前面的正文章节，本附录保证**没有任何导出 API 被漏掉**。

| 源文件 | 符号数 |
| --- | --- |
| `ui/markdown.go` | 6 |
| `ui/richtext.go` | 4 |
| `ui/richtext_editor.go` | 20 |
| `ui/chart.go` | 20 |
| `ui/video.go` | 52 |
| `ui/video_player.go` | 1 |
| `ui/media.go` | 28 |
| `ui/music.go` | 32 |
| `ui/music_player.go` | 6 |
| `ui/spectrum.go` | 7 |
| `ui/system_decoder.go` | 3 |
| `ui/media_stub.go` | 23 |
| `ui/music_stub.go` | 31 |
| `ui/office.go` | 20 |
| `internal/office/office.go` | 55 |
| `ui/print.go` | 16 |
| `ui/webview.go` | 28 |
| `ui/codeeditor.go` | 59 |
| `ui/codeeditor_find.go` | 21 |
| `ui/codeeditor_lsp.go` | 13 |
| `ui/codeeditor_menu.go` | 4 |
| `ui/codeeditor_multi.go` | 9 |
| `ui/codeeditor_semantic.go` | 7 |
| `ui/codeeditor_view.go` | 6 |
| `ui/lsp.go` | 20 |

合计 **491** 个导出符号。

---

## `ui/markdown.go`

- **类型** `type MarkdownOptions struct { // Width 排版宽度（像素）。0 = min(ViewportW()-96, 860)。 Width float32 // BaseSize 正文字号，默认 13.5。 BaseSize float32 // FontFamily 正文字体（空 = 全局默认微软雅黑）。 FontFamily string // MonoFamily 代码字体，默认 Consolas。 MonoFamily string // LineHeight 正文行高，默认 BaseSize*1.65。 LineHeight float32 // MaxImageHeight 图片最大高度，默认 360。 MaxImageHeight float32 // CodeMaxLines 代码块折叠阈值（0 = 22，<0 不折叠）。 CodeMaxLines int // CodeNoWrap 代码块不自动折行：超宽时横向平移查看（默认折行）。 CodeNoWrap bool // OnLink 链接点击回调；nil 时用 win32.ShellOpen 打开。 OnLink func(url string) // NoTOC 不渲染 [toc] 块。 NoTOC bool }` — Markdown 渲染组件（声明式节点，每帧重建）。
- **函数** `func MarkdownResetState()` — MarkdownResetState 清空页签/折叠/图片预览等交互状态。
- **函数** `func Markdown(key, src string, opts ...MarkdownOptions) *Node` — Markdown 渲染 Markdown 文档。
- **函数** `func MarkdownParse(src string) *mdDoc` — MarkdownParse 解析但不渲染（调试/目录提取用）。
- **函数** `func MarkdownOpenImage(src, alt string)` — MarkdownOpenImage 打开图片预览（供外部调用）。
- **函数** `func MarkdownLightbox() *Node` — MarkdownLightbox 图片预览浮层：放在根 Stack 最后一层，

## `ui/richtext.go`

- **类型** `type TextSpan struct { Text string // Color 文字颜色（空 = 继承基础色）。 Color string // BgColor 片段底色（高亮）。 BgColor string // Size 字号（0 = 继承）。 Size float32 // Weight 字重（0 = 继承）。 Weight int32 // Family 字体家族（空 = 继承）。 Family string // Link 链接标识（非空时该片段可点击，悬停显示下划线）。 Link string // Underline / Strike 下划线 / 删除线。 Underline bool Strike bool // Code 行内代码：等宽字体 + 底色。 Code bool }` — TextSpan 行内片段。零值字段表示继承 RichTextOptions / 节点样式。
- **类型** `type RichTextOptions struct { Size float32 Weight int32 Color string Family string LineHeight float32 // MaxLines > 0 时超出部分截断并追加省略号。 MaxLines int // Align 行内对齐（默认左对齐）。 Align Align // LinkColor 链接默认颜色（空 = #8b93ff）。 LinkColor string // CodeBg 行内代码底色（空 = 主题悬停色）。 CodeBg string // NoWrap 关闭自动折行（需要与 Style.Width 配合裁剪）。 NoWrap bool }` — RichTextOptions 整段富文本的默认样式。
- **函数** `func RichText(spans []TextSpan, opts RichTextOptions) *Node` — RichText 构建富文本节点（默认自动折行）。
- **方法** `func Node.OnSpanClickEvt(fn func(TextSpan)) *Node` — OnSpanClickEvt 行内片段点击回调（链接 / 行内代码均可命中）。

## `ui/richtext_editor.go`

- **类型** `type RichStyle struct { Bold bool Italic bool Underline bool Strike bool Code bool Color string Size float32 Link string }` — RichStyle 行内样式。
- **类型** `type RichRun struct { Text string Style RichStyle }` — RichRun 一段连续同一样式的文字。
- **类型** `type RichEditorState struct { Key string Runs []RichRun Caret int // rune 下标 Anchor int // 选区锚点（与 Caret 不同则有选区） // Style 新输入文字的样式（工具栏修改）。 Style RichStyle Undo *UndoStack }` — RichEditorState 编辑器状态。
- **函数** `func NewRichEditor(key string) *RichEditorState` — NewRichEditor 创建编辑器（默认一段空文本）。
- **函数** `func RichEditor(st *RichEditorState, height float32, onChange func()) *Node` — RichEditor 编辑区（可聚焦，支持输入法 / 方向键 / 选区 / 快捷键）。
- **函数** `func RichToolbar(st *RichEditorState, onChange func()) *Node` — RichToolbar 编辑器工具栏（粗/斜/下划线/删除线/代码/颜色/字号/撤销重做）。
- **函数** `func RichEditorEx(st *RichEditorState, height float32, onChange func()) *Node` — RichEditorEx 工具栏 + 编辑区（常用组合）。
- **函数** `func RichEditorText(st *RichEditorState) string` — RichEditorText 调试用：带样式标记的纯文本导出（"**粗**" 等）。
- **函数** `func RichEditorDebug(st *RichEditorState) string` — RichEditorDebug 打印编辑器结构（调试）。
- **方法** `func RichEditorState.Text() string` — Text 编辑器全文。
- **方法** `func RichEditorState.Selection() (int, int)` — Selection 返回选区 [lo, hi)（无选区时 lo==hi==caret）。
- **方法** `func RichEditorState.Insert(text string)` — Insert 在光标处插入文字。
- **方法** `func RichEditorState.Toggle(apply func(*RichStyle))` — Toggle 切换当前样式（有选区时应用到选区，否则作用于后续输入）。
- **方法** `func RichEditorState.Bold()` — Bold 切换粗体。
- **方法** `func RichEditorState.Italic()` — Italic 切换斜体。
- **方法** `func RichEditorState.Underline()` — Underline 切换下划线。
- **方法** `func RichEditorState.Strike()` — Strike 切换删除线。
- **方法** `func RichEditorState.Code()` — Code 切换行内代码。
- **方法** `func RichEditorState.SetColor(c string)` — SetColor 设置文字颜色。
- **方法** `func RichEditorState.SetSize(v float32)` — SetSize 设置字号。

## `ui/chart.go`

- **类型** `type ChartType uint8` — ChartType 图表类型。
- **类型** `type Series struct { Name string Color string Values []float64 X []float64 }` — Series 一条数据系列。
- **类型** `type Chart struct { Type ChartType Series []Series // Labels X 轴标签 / 饼图扇区名 / 雷达轴名。 Labels []string Title string Unit string ShowLegend bool ShowGrid bool ShowAxis bool // Stacked 柱状 / 面积按系列堆叠。 Stacked bool // Smooth 折线平滑（Catmull-Rom 插值，采样后仍是折线段）。 Smooth bool // Donut 环形图内径比例（0~0.9，仅 ChartDonut 生效）。 Donut float32 // Min / Max 纵轴范围；Max 为 0 表示自动（Min 可为负值）。 Min, Max float64 // YTicks 纵轴刻度数（默认 4）。 YTicks int // Padding 画布内边距（默认 14）。 Padding float32 // OnPick 点击柱 / 数据点 / 扇区回调（series 系列下标，index 数据点下标）。 OnPick func(series, index int) applied bool cache chartCache }` — Chart 图表数据与配置，由业务持有（与 Player 一样不要每帧新建）。
- **函数** `func NewChart(t ChartType, labels []string, series ...Series) *Chart` — NewChart 新建图表（已填默认开关与刻度数）。
- **函数** `func ChartView(key string, c *Chart) *Node` — ChartView 把图表包成节点（默认高度 280、可被 St() 覆盖）。
- **函数** `func LineChart(key string, labels []string, series ...Series) *Node` — LineChart 折线图。
- **函数** `func AreaChart(key string, labels []string, series ...Series) *Node` — AreaChart 面积图。
- **函数** `func BarChart(key string, labels []string, series ...Series) *Node` — BarChart 柱状图（堆叠用 chart.Stacked = true）。
- **函数** `func PieChart(key string, values []float64, labels []string) *Node` — PieChart 饼图：values 每项一个扇区，labels 为扇区名（可少传）。
- **函数** `func DonutChart(key string, values []float64, labels []string) *Node` — DonutChart 环形图（内径 58%）。
- **函数** `func RadarChart(key string, labels []string, series ...Series) *Node` — RadarChart 雷达图。
- **函数** `func ScatterChart(key string, series ...Series) *Node` — ScatterChart 散点图。
- **函数** `func PieSeries(values []float64, labels []string) []Series` — PieSeries 把数值切片摊成扇区系列（名称取 labels[i]，颜色走调色板）。
- **常量** `const ChartLine = iota`
- **常量** `const ChartArea`
- **常量** `const ChartBar`
- **常量** `const ChartPie`
- **常量** `const ChartDonut`
- **常量** `const ChartRadar`
- **常量** `const ChartScatter`

## `ui/video.go`

- **类型** `type VideoFrame struct { Pix []byte W, H int Delay time.Duration }` — VideoFrame 一帧画面：RGBA8 非预乘像素（行优先，长度 = W*H*4），Delay 为本帧停留时长。
- **类型** `type VideoClip struct { Frames []VideoFrame // FPS 在 Delay 全为 0 时使用的帧率（默认 24）。 FPS float64 Loop bool once sync.Once starts []time.Duration total time.Duration w, h int }` — VideoClip 一段可播放的帧序列：GIF、图片序列、逐帧解码结果都归一到这个载体。
- **类型** `type VideoFit uint8` — VideoFit 画面填充方式。
- **类型** `type VideoSource interface { // Size 画面像素尺寸。 Size() (int, int) // Duration 总时长（未知返回 0，此时进度条按未知处理）。 Duration() time.Duration // Frame 返回时间点 pos 的画面；返回 nil, nil 表示该位置暂无画面（保留上一帧）。 Frame(pos time.Duration) (*VideoFrame, error) }` — VideoSource 外部解码器接口（FFmpeg / MediaFoundation / 自研解码器）。
- **类型** `type Player struct { // Clip 内存帧序列；与 Source 二选一（同时存在时优先 Clip）。 Clip *VideoClip // Source 外部帧来源（流式解码）。 Source VideoSource Playing bool Loop bool // Speed 播放速率（<=0 视为 1）。 Speed float32 // Volume 音量 0~1；渲染层不发声，由业务在 OnVolume 中消费。 Volume float32 Muted bool Fit VideoFit // StepMode 逐帧模式：按帧序号 + 毫秒精确定位。 StepMode bool // Rotation 画面旋转（0~3，单位 90° 顺时针，控制条左侧旋转按钮修改）。 Rotation int // Locked 锁定画面：点击画面不切播放、快捷键（空格/方向键）失效，避免误触。 Locked bool // AutoHide 播放中自动隐藏控制条（鼠标移动/暂停时显示）。 AutoHide bool // HideControls 不显示控制条（纯画面）。 HideControls bool // Fullscreen 播放器铺满窗口（VideoPlayer 需放在根 Stack 中才会铺满整窗）。 Fullscreen bool // ShowControls 之外的展示项 // Subtitle 底部字幕（空则不显示；中间的「·」等符号照常渲染） Subtitle string // 回调 OnPlay func() OnPause func() OnSeek func(time.Duration) OnVolume func(v float32, muted bool) OnLoopChange func(loop bool) OnFullscreen func() // OnPrev / OnNext 上一集 / 下一集（非 nil 时控制条显示跳集按钮） OnPrev func() OnNext func() pos time.Duration lastTick float32 started bool scrub bool err error frame *VideoFrame frameIdx int frameSig uint64 rect rectBox // 上一帧画面矩形（控制条自动隐藏判定用） }` — Player 播放状态（业务持有并跨帧复用；由 VideoPlayer 每帧推进）。
- **函数** `func NewVideoClip(frames []VideoFrame, fps float64, loop bool) *VideoClip` — NewVideoClip 用等间隔帧构造片段（fps <= 0 时按 24 帧/秒处理）。
- **函数** `func LoadGIF(path string) (*VideoClip, error)` — LoadGIF 解码 GIF 的全部画面为片段（按 disposal 规则合成为完整 RGBA 帧）。
- **函数** `func GIFToClip(g *gif.GIF) (*VideoClip, error)` — GIFToClip 把已解码的 GIF 转成片段。
- **函数** `func LoadImageSequence(paths []string, fps float64, loop bool) (*VideoClip, error)` — LoadImageSequence 把多张图片按固定帧率组成片段（帧序按传入顺序）。
- **函数** `func NewPlayer(clip *VideoClip) *Player` — NewPlayer 用内存帧序列创建播放器（默认速率 1，循环设置继承片段）。
- **函数** `func NewPlayerOf(src VideoSource) *Player` — NewPlayerOf 用外部帧来源创建播放器。
- **函数** `func NewMediaPlayer(path string) (*Player, error)` — NewMediaPlayer 打开媒体文件/链接（mp4 / mkv / mov / avi / webm / flv / ts / m3u8 等）
- **函数** `func NewMediaPlayerWith(path string, cfg MediaConfig) (*Player, error)` — NewMediaPlayerWith 按配置打开媒体并创建播放器。
- **函数** `func OpenMediaInto(p *Player, path string) error` — OpenMediaInto 在已有播放器上打开新媒体（旧解码源自动关闭）。
- **方法** `func VideoFrame.Valid() bool` — Valid 画面是否可用（尺寸合法且像素足够）。
- **方法** `func VideoClip.FrameDelay(i int) time.Duration` — FrameDelay 返回第 i 帧的停留时长（Delay <= 0 时按 FPS 折算）。
- **方法** `func VideoClip.Duration() time.Duration` — Duration 片段总时长。
- **方法** `func VideoClip.Size() (int, int)` — Size 画面像素尺寸（取首帧，帧尺寸不一致时取最大）。
- **方法** `func VideoClip.FrameCount() int` — FrameCount 帧数。
- **方法** `func VideoClip.FrameStart(i int) time.Duration` — FrameStart 第 i 帧的起始时间。
- **方法** `func VideoClip.FrameIndexAt(pos time.Duration) int` — FrameIndexAt 返回时间点 pos 对应的帧号（空片段返回 -1）。
- **方法** `func Player.Close()` — Close 释放外部解码源（ffmpeg 子进程 / 音频设备），播放器本身仍可继续使用内存片段。
- **方法** `func Player.HasAudio() bool` — HasAudio 当前解码源是否成功开启声音输出。
- **方法** `func Player.Info() MediaInfo` — Info 媒体探测信息（非媒体源返回零值）。
- **方法** `func Player.SetClip(c *VideoClip)` — SetClip 切换片源（重置播放位置，保持播放状态与循环设置）。
- **方法** `func Player.Pos() time.Duration` — Pos 当前播放位置。
- **方法** `func Player.Duration() time.Duration` — Duration 总时长（未知返回 0）。
- **方法** `func Player.Size() (int, int)` — Size 画面像素尺寸。
- **方法** `func Player.Progress() float32` — Progress 播放进度 0~1（时长未知返回 0）。
- **方法** `func Player.SetRotation(q int) int` — SetRotation 设置画面旋转（自动折到 0~3 的 90° 份数），返回生效值。
- **方法** `func Player.RotateBy(delta int) int` — RotateBy 按 90° 份数旋转（可正可负，如 -1 表示逆时针 90°）。
- **方法** `func Player.RotationDeg() int` — RotationDeg 当前旋转角度（度）。
- **方法** `func Player.SetLocked(v bool)` — SetLocked 锁定/解锁画面（锁定后点击与快捷键不再改播放状态）。
- **方法** `func Player.ToggleLock()` — ToggleLock 切换锁定状态。
- **方法** `func Player.Err() error` — Err 最近一次取帧错误。
- **方法** `func Player.PlayingNow() bool` — PlayingNow 是否正在播放。
- **方法** `func Player.SpeedValue() float32` — SpeedValue 生效的播放速率（<=0 视为 1）。
- **方法** `func Player.Play()` — Play 开始播放；已播放到末尾时从头开始。
- **方法** `func Player.Pause()` — Pause 暂停。
- **方法** `func Player.TogglePlay()` — TogglePlay 播放/暂停切换。
- **方法** `func Player.SetPos(d time.Duration)` — SetPos 跳转到指定播放位置（自动夹到 [0, 时长]）。
- **方法** `func Player.SeekBy(d time.Duration)` — SeekBy 相对跳转（负值快退）。
- **方法** `func Player.SeekRatio(v float32)` — SeekRatio 按比例跳转（0~1，进度条点击/拖动使用）。
- **方法** `func Player.SetSpeed(v float32)` — SetSpeed 设置播放速率（夹到 0.25~4）。
- **方法** `func Player.SetVolume(v float32)` — SetVolume 设置音量 0~1（音量 0 自动静音）。
- **方法** `func Player.ToggleMute()` — ToggleMute 静音切换（保留静音前音量）。
- **方法** `func Player.SetLoop(v bool)` — SetLoop 设置循环播放。
- **方法** `func Player.Frame() *VideoFrame` — Frame 当前画面（可能为 nil）。
- **方法** `func Player.FrameIndex() int` — FrameIndex 当前帧号（Clip 有效，否则 -1）。
- **常量** `const FitContain = iota`
- **常量** `const FitCover`
- **常量** `const FitFill`

## `ui/video_player.go`

- **函数** `func VideoPlayer(key string, p *Player) *Node` — VideoPlayer 视频播放器：画面 + 底部渐变控制层。

## `ui/media.go`

- **类型** `type MediaInfo struct { Path string Format string Duration time.Duration Width int Height int FPS float64 Bitrate int64 VideoCodec string AudioCodec string SampleRate int Channels int HasVideo bool HasAudio bool // Backend 实际使用的解码后端："ffmpeg" 或 "system"（Windows Media Foundation）。 Backend string }` — MediaInfo ffprobe（或系统解码器）探测结果。
- **类型** `type MediaConfig struct { // FFmpeg / FFprobe 可执行文件路径（空则用 PATH 里的 ffmpeg / ffprobe）。 FFmpeg string FFprobe string // MaxWidth / MaxHeight 输出上限；都 <= 0 时跟随源尺寸（不缩放）。 MaxWidth int MaxHeight int // FPS 输出帧率（0 = 跟随源帧率）。 FPS float64 // NoAudio 关闭声音解码。 NoAudio bool // Volume 初始音量 0~1（默认 1）。 Volume float32 // HWAccel 尝试硬件解码（D3D11VA，失败由 ffmpeg 自动回退软解）。 HWAccel bool }` — MediaConfig 解码参数（零值即默认）。
- **类型** `type MediaSource struct { path string info MediaInfo cfg MediaConfig outW, outH int fps float64 frameBytes int maxFrames int // pool 帧缓冲复用池（最多 4 块，每块 frameBytes）。 // 1080p 一帧 8MB，旧实现「make 读缓冲 + append 复制」每帧分配两次， // 25fps 下就是 400MB/s 的分配量，Go 堆被迫膨胀、GC 每两秒一轮； // 现在像素缓冲跟帧一起流转：帧被丢弃时归还，解码线程直接复用。 pool [][]byte mu sync.Mutex cond *sync.Cond gen int // 视频窗口代际 agen int // 音频窗口代际 start time.Duration // 当前窗口的媒体起点（音频时钟基准） vstart time.Duration // 视频窗口起点（最小化恢复只重开视频，与音频起点解耦） opened bool // 是否已启动过解码窗口 frames []mediaFrame vcmd *exec.Cmd acmd *exec.Cmd audio audio.Device audioOn bool audioDevOn bool speed float64 paused bool lastPos time.Duration // 拖动进度中：只记录目标位置不重启解码，松手后一次性定位 scrubbing bool pendingSeek time.Duration hasPending bool // 拖动中的单帧预览（last-wins：最新目标位置覆盖旧请求，单进程串行抓帧） previewFrame *VideoFrame previewAt time.Duration previewBusy bool previewWant time.Duration previewHas bool // clockDone 音频流已播完：此时设备时钟不再前进，播放位置改由上层按墙钟推进 clockDone bool lastOpen time.Time closed bool eof bool err error // 系统解码器后端（没装 ffmpeg 时使用；为 nil 表示 ffmpeg 路径） sys sysSource // sysWG 跟踪系统解码协程，关闭前等待退出（避免归还 reader 后仍在调用） sysWG sync.WaitGroup }` — MediaSource ffmpeg 解码的帧来源，实现 VideoSource。
- **函数** `func IsMediaPath(path string) bool` — IsMediaPath 判断路径/链接是否为受支持的媒体（按扩展名判断）。
- **函数** `func MediaFormats() []string` — MediaFormats 支持的容器列表（界面展示用）。
- **函数** `func DecoderPreference() string` — DecoderPreference 解码器偏好（环境变量 GUIS_DECODER）：
- **函数** `func ProbeMedia(path string) (*MediaInfo, error)` — ProbeMedia 用 ffprobe 探测媒体信息。
- **函数** `func ProbeMediaWith(path, ffprobe string) (*MediaInfo, error)` — ProbeMediaWith 指定 ffprobe 路径探测（空则用 PATH）。
- **函数** `func OpenMedia(path string) (*MediaSource, error)` — OpenMedia 打开媒体文件/链接（默认：跟随源尺寸 + 声音）。
- **函数** `func OpenMediaWith(path string, cfg MediaConfig) (*MediaSource, error)` — OpenMediaWith 按配置打开媒体。
- **函数** `func ResumeMediaAll()` — ResumeMediaAll 窗口从最小化恢复时调用：让所有存活的媒体源重建视频解码窗口。
- **方法** `func MediaSource.Info() MediaInfo` — Info 探测到的媒体信息。
- **方法** `func MediaSource.Path() string` — Path 媒体路径。
- **方法** `func MediaSource.Size() (int, int)` — Size 解码输出尺寸。
- **方法** `func MediaSource.Duration() time.Duration` — Duration 总时长（流媒体可能为 0）。
- **方法** `func MediaSource.HasAudio() bool` — HasAudio 是否成功开启音频输出。
- **方法** `func MediaSource.Err() error` — Err 最近一次解码错误。
- **方法** `func MediaSource.Frame(pos time.Duration) (*VideoFrame, error)` — Frame 取时间点 pos 的画面（VideoSource 实现）。
- **方法** `func MediaSource.MediaResume()` — MediaResume 重建视频解码窗口到当前播放位置（音频进程与时钟保持连续）。
- **方法** `func MediaSource.MediaClock() (time.Duration, bool)` — MediaClock 播放时钟：有声音时以音频设备已播样本为准（A/V 同步），否则无效。
- **方法** `func MediaSource.MediaScrub(v bool)` — MediaScrub 拖动进度状态：拖动中不动解码（画面走单帧预览、声音暂停），
- **方法** `func MediaSource.MediaPlay(playing bool)` — MediaPlay 播放 / 暂停（Player 状态变化时调用）。
- **方法** `func MediaSource.MediaVolume(v float32, muted bool)` — MediaVolume 音量 / 静音。
- **方法** `func MediaSource.MediaSpeed(v float32)` — MediaSpeed 播放速率：音频用 atempo 滤镜重采样，需要重启音频进程。
- **方法** `func MediaSource.Close()` — Close 关闭解码进程与音频设备（幂等）。
- **方法** `func errRing.Write(p []byte) (int, error)`
- **方法** `func errRing.String() string`
- **常量** `const MediaFilters = "视频|*.mp4;*.m4v;*.mkv;*.mov;*.avi;*.webm;*.flv;*.ts;*.m2ts;*.mts;*.m3u8|" + "MP4|*.mp4;*.m4v|MKV|*.mkv|MOV|*.mov|AVI|*.avi|WebM|*.webm|FLV|*.flv|" + "TS|*.ts;*.m2ts;*.mts|HLS 播放列表|*.m3u8|所有文件|*.*"` — MediaFilters 文件对话框过滤器（含所有支持的容器）。

## `ui/music.go`

- **类型** `type MusicSource struct { path string info MediaInfo cfg MediaConfig rate, ch int mu sync.Mutex cond *sync.Cond gen int start time.Duration // 当前窗口起点 played time.Duration // 窗口内已播时长 lastTk time.Time opened bool cmd *exec.Cmd audio audio.Device audioOn bool paused bool speed float64 vol float32 muted bool loop bool closed bool eof bool err error lastPos time.Duration // 系统解码器后端（没装 ffmpeg 时使用；为 nil 表示 ffmpeg 路径） sys sysSource // sysWG 跟踪系统解码协程，关闭前等待退出 sysWG sync.WaitGroup an *Spectrum }` — MusicSource 纯音频解码源：ffmpeg → s16le PCM → waveOut，附带频谱分析。
- **函数** `func IsMusicPath(path string) bool` — IsMusicPath 判断路径是否为受支持的音频文件（按扩展名判断）。
- **函数** `func MusicFormats() []string` — MusicFormats 支持的音频容器列表（界面展示用）。
- **函数** `func OpenMusic(path string) (*MusicSource, error)` — OpenMusic 打开音频文件（默认跟随源采样率，声音开启）。
- **函数** `func OpenMusicWith(path string, cfg MediaConfig) (*MusicSource, error)` — OpenMusicWith 按配置打开音频文件。支持 mp3 / flac / wav / m4a / aac / ogg / opus / wma / ape。
- **函数** `func OpenMusicPlayer(path string) (*MusicSource, error)` — OpenMusicPlayer 一步打开音频并自动播放（出错返回 nil + 错误）。
- **方法** `func MusicSource.Path() string` — Path 音频路径。
- **方法** `func MusicSource.Info() MediaInfo` — Info 探测信息（时长 / 编码 / 采样率等）。
- **方法** `func MusicSource.Duration() time.Duration` — Duration 总时长（未知返回 0）。
- **方法** `func MusicSource.HasAudio() bool` — HasAudio 是否成功开启音频输出设备。
- **方法** `func MusicSource.Err() error` — Err 最近一次解码错误。
- **方法** `func MusicSource.Spectrum() *Spectrum` — Spectrum 频谱分析器（可为 nil）。
- **方法** `func MusicSource.Pos() time.Duration` — Pos 当前播放位置。
- **方法** `func MusicSource.Progress() float32` — Progress 播放进度 0~1（时长未知返回 0）。
- **方法** `func MusicSource.Playing() bool` — Playing 是否正在播放。
- **方法** `func MusicSource.Play()` — Play 开始 / 继续播放。
- **方法** `func MusicSource.Pause()` — Pause 暂停。
- **方法** `func MusicSource.TogglePlay()` — TogglePlay 播放 / 暂停切换。
- **方法** `func MusicSource.Seek(pos time.Duration)` — Seek 跳转到指定位置（重开解码窗口，声音与频谱一起定位）。
- **方法** `func MusicSource.SeekRatio(v float32)` — SeekRatio 按比例跳转（0~1）。
- **方法** `func MusicSource.Speed() float32` — Speed 当前倍速（<=0 视为 1）。
- **方法** `func MusicSource.SetSpeed(v float32)` — SetSpeed 设置倍速（0.5~2），用 atempo 重采样后重开音频窗口。
- **方法** `func MusicSource.Volume() float32` — Volume 当前音量（0~1）。
- **方法** `func MusicSource.Muted() bool` — Muted 是否静音。
- **方法** `func MusicSource.SetVolume(v float32)` — SetVolume 设置音量 0~1（0 视为静音）。
- **方法** `func MusicSource.ToggleMute()` — ToggleMute 静音切换（保留静音前音量）。
- **方法** `func MusicSource.Loop() bool` — Loop 是否单曲循环。
- **方法** `func MusicSource.SetLoop(v bool)` — SetLoop 设置单曲循环。
- **方法** `func MusicSource.Tick() bool` — Tick 供每帧调用：播放到结尾时按循环设置重播或停在结尾。
- **方法** `func MusicSource.Close()` — Close 释放解码进程与音频设备（幂等）。
- **方法** `func MusicSource.NowPlaying() MusicTrack` — NowPlaying 便捷：曲目信息（标题 / 歌手 / 封面）。
- **常量** `const MusicFilters = "音乐|*.mp3;*.m4a;*.m4b;*.aac;*.flac;*.wav;*.ogg;*.opus;*.wma;*.ape;*.aiff|" + "MP3|*.mp3|FLAC|*.flac|WAV|*.wav|M4A|*.m4a;*.m4b|AAC|*.aac|OGG / Opus|*.ogg;*.oga;*.opus|" + "WMA|*.wma|APE|*.ape|AIFF|*.aiff;*.aif|所有文件|*.*"` — MusicFilters 音乐文件对话框过滤器。

## `ui/music_player.go`

- **类型** `type MusicTrack struct { Path string Title string Artist string Album string Cover string Dur time.Duration }` — MusicTrack 曲目信息（标题 / 歌手 / 封面均可空，空则回退到文件名与唱片占位）。
- **类型** `type MusicPlayerOptions struct { // Track 曲目信息（标题为空时用文件名）。 Track MusicTrack // Bars 频谱柱数量（默认 40）。 Bars int // Prev / Next 上一首 / 下一首（nil 时按钮置灰不可点）。 Prev func() Next func() // HideSpectrum 不显示频谱（纯音频条模式）。 HideSpectrum bool }` — MusicPlayerOptions 音乐播放器外观与回调。
- **函数** `func MusicPlayer(key string, s *MusicSource, opt MusicPlayerOptions) *Node` — MusicPlayer 音乐播放器组件。
- **函数** `func MusicSpectrum(key string, sp *Spectrum, bars int, from, to string) *Node` — MusicSpectrum 频谱柱状图：对数频段能量驱动柱高（底部对齐），
- **函数** `func MusicMirrorSpectrum(key string, sp *Spectrum, bars int, from, to string) *Node` — MusicMirrorSpectrum 镜像频谱：以中线为轴上下对称（音乐 App 常见的“声浪”效果）。
- **函数** `func MusicPlaylist(key string, tracks []MusicTrack, active int, onPick func(i int)) *Node` — MusicPlaylist 播放列表组件：每行标题 + 副标题，当前曲目高亮并带「播放中」标签。

## `ui/spectrum.go`

- **类型** `type Spectrum struct { mu sync.Mutex ring []float32 // 单声道环形缓冲（最新样本覆盖最旧） pos int total uint64 rate int // 以下字段只在 UI 线程访问 bands []float32 peaks []float32 lastT uint64 hann []float64 re, im []float64 snap []float32 edges []int // 每个频段的 FFT bin 边界 lo, hi int }` — Spectrum FFT 频谱分析器：解码线程写 PCM，UI 线程每帧读频段能量。
- **函数** `func NewSpectrum(bands int) *Spectrum` — NewSpectrum 创建频谱分析器（bands <= 0 时用默认 40 段）。
- **方法** `func Spectrum.PushPCM(pcm []byte, channels, rate int)` — PushPCM 写入一段 s16le PCM（解码线程调用，可多声道：取各声道平均）。
- **方法** `func Spectrum.Reset()` — Reset 清空缓冲与平滑状态（切换曲目 / 跳转时调用）。
- **方法** `func Spectrum.Bands() []float32` — Bands 返回各频段能量（0~1，已做 attack/release 平滑）。UI 线程每帧调用一次。
- **方法** `func Spectrum.Peaks() []float32` — Peaks 返回频段峰值（0~1，缓慢回落），用于绘制顶端峰值点。
- **常量** `const SpectrumBands = 40`

## `ui/system_decoder.go`

- **函数** `func BackendName() string` — BackendName 当前可用的解码后端名称（界面展示用）。
- **函数** `func BackendLabel(name string) string` — BackendLabel 解码后端的中文标签。
- **函数** `func FormatBackend(name string) string` — FormatBackend 后端名短标签（"ffmpeg" / "系统"）。

## `ui/media_stub.go`

- **类型** `type MediaInfo struct { Path string Format string Duration time.Duration Width int Height int FPS float64 Bitrate int64 VideoCodec string AudioCodec string SampleRate int Channels int HasVideo bool HasAudio bool // Backend 实际使用的解码后端；本平台无后端，恒为 ""。 Backend string }` — MediaInfo 媒体探测结果（字段与 Windows 端一致）。
- **类型** `type MediaConfig struct { FFmpeg string FFprobe string MaxWidth int MaxHeight int FPS float64 NoAudio bool Volume float32 HWAccel bool }` — MediaConfig 解码参数（与 Windows 端一致，本平台忽略）。
- **类型** `type MediaSource struct { path string info MediaInfo cfg MediaConfig err error }` — MediaSource 媒体源：占位实现，实现 VideoSource 接口但不产生画面。
- **函数** `func IsMediaPath(path string) bool` — IsMediaPath 按扩展名判断是否媒体文件。
- **函数** `func MediaFormats() []string` — MediaFormats 支持的媒体容器列表（界面展示用）。
- **函数** `func DecoderPreference() string` — DecoderPreference 解码器偏好（本平台无后端）。
- **函数** `func ProbeMedia(path string) (*MediaInfo, error)` — ProbeMedia 探测媒体信息（本平台无后端，返回不支持）。
- **函数** `func ProbeMediaWith(path, ffprobe string) (*MediaInfo, error)` — ProbeMediaWith 指定 ffprobe 路径探测（本平台无后端，返回不支持）。
- **函数** `func OpenMedia(path string) (*MediaSource, error)` — OpenMedia 打开媒体（本平台无后端）。
- **函数** `func OpenMediaWith(path string, cfg MediaConfig) (*MediaSource, error)` — OpenMediaWith 按配置打开媒体（本平台无后端）。
- **方法** `func MediaSource.Info() MediaInfo` — Info 探测信息（零值）。
- **方法** `func MediaSource.Path() string` — Path 媒体路径。
- **方法** `func MediaSource.Size() (int, int)` — Size 画面尺寸（无画面 → 0,0）。
- **方法** `func MediaSource.Duration() time.Duration` — Duration 时长（未知 → 0）。
- **方法** `func MediaSource.HasAudio() bool` — HasAudio 是否有音轨。
- **方法** `func MediaSource.Err() error` — Err 打开/解码错误。
- **方法** `func MediaSource.Frame(pos time.Duration) (*VideoFrame, error)` — Frame 取指定时间点画面（无解码器 → nil, nil，保留上一帧）。
- **方法** `func MediaSource.MediaResume()` — MediaResume 最小化恢复后重建解码器（空操作）。
- **方法** `func MediaSource.MediaClock() (time.Duration, bool)` — MediaClock 解码器时钟（无效）。
- **方法** `func MediaSource.MediaScrub(v bool)` — MediaScrub 拖动定位状态（空操作）。
- **方法** `func MediaSource.MediaPlay(playing bool)` — MediaPlay 播放/暂停（空操作）。
- **方法** `func MediaSource.Close()` — Close 关闭媒体源。
- **常量** `const MediaFilters = "视频|*.mp4;*.m4v;*.mkv;*.mov;*.avi;*.webm;*.flv;*.ts;*.m2ts;*.mts;*.m3u8|" + "音频|*.mp3;*.flac;*.wav;*.m4a;*.aac;*.ogg;*.opus|所有文件|*.*"` — MediaFilters 文件对话框用的媒体过滤器（与 Windows 端一致）。

## `ui/music_stub.go`

- **类型** `type MusicSource struct { path string info MediaInfo cfg MediaConfig an *Spectrum pos time.Duration // 当前播放位置 last time.Time rate float64 vol float32 muted bool loop bool play bool ended bool }` — MusicSource 纯音频解码源（本平台为占位实现：无声音、按墙钟推进）。
- **函数** `func IsMusicPath(path string) bool` — IsMusicPath 按扩展名判断是否音频文件。
- **函数** `func OpenMusic(path string) (*MusicSource, error)` — OpenMusic 打开音频文件（本平台无解码后端，返回占位源）。
- **函数** `func OpenMusicWith(path string, cfg MediaConfig) (*MusicSource, error)` — OpenMusicWith 按配置打开音频文件（本平台无解码后端，返回占位源）。
- **函数** `func OpenMusicPlayer(path string) (*MusicSource, error)` — OpenMusicPlayer 打开音频并立即播放（本平台无音频输出，仅推进时间轴）。
- **方法** `func MusicSource.Path() string` — Path 音频路径。
- **方法** `func MusicSource.Info() MediaInfo` — Info 探测信息。
- **方法** `func MusicSource.Duration() time.Duration` — Duration 时长（无解码后端 → 0，界面显示 --:--）。
- **方法** `func MusicSource.HasAudio() bool` — HasAudio 是否有音轨。
- **方法** `func MusicSource.Err() error` — Err 错误（占位源无错误）。
- **方法** `func MusicSource.Spectrum() *Spectrum` — Spectrum 频谱分析器（无声源，能量恒为 0）。
- **方法** `func MusicSource.Pos() time.Duration` — Pos 当前播放位置。
- **方法** `func MusicSource.Progress() float32` — Progress 播放进度 0~1。
- **方法** `func MusicSource.Playing() bool` — Playing 是否正在播放。
- **方法** `func MusicSource.Play()` — Play 开始播放。
- **方法** `func MusicSource.Pause()` — Pause 暂停。
- **方法** `func MusicSource.TogglePlay()` — TogglePlay 切换播放/暂停。
- **方法** `func MusicSource.Seek(pos time.Duration)` — Seek 跳转到指定位置。
- **方法** `func MusicSource.SeekRatio(v float32)` — SeekRatio 按比例跳转 0~1。
- **方法** `func MusicSource.Speed() float32` — Speed 播放速率。
- **方法** `func MusicSource.SetSpeed(v float32)` — SetSpeed 设置播放速率（<=0 视为 1）。
- **方法** `func MusicSource.Volume() float32` — Volume 音量 0~1。
- **方法** `func MusicSource.Muted() bool` — Muted 是否静音。
- **方法** `func MusicSource.SetVolume(v float32)` — SetVolume 设置音量。
- **方法** `func MusicSource.ToggleMute()` — ToggleMute 切换静音。
- **方法** `func MusicSource.Loop() bool` — Loop 是否循环。
- **方法** `func MusicSource.SetLoop(v bool)` — SetLoop 设置循环。
- **方法** `func MusicSource.Tick() bool` — Tick 每帧推进；返回 true 表示当前曲目已播完（组件据此自动下一首）。
- **方法** `func MusicSource.Close()` — Close 释放音源。
- **方法** `func MusicSource.NowPlaying() MusicTrack` — NowPlaying 当前曲目信息（标题回退文件名）。
- **常量** `const MusicFilters = "音频|*.mp3;*.flac;*.wav;*.m4a;*.aac;*.ogg;*.opus;*.wma|所有文件|*.*"` — MusicFilters 文件对话框用的音频过滤器。

## `ui/office.go`

- **类型** `type SheetViewOptions struct { // MaxRows / MaxCols 最多渲染多少行 / 列（默认 300 / 24，防止超大表拖慢布局）。 MaxRows, MaxCols int // RowH 行高（默认 30）。 RowH float32 // FontSize 字号（默认 12）。 FontSize float32 // ShowColName 没有表头时是否用 A/B/C 作为列名（默认 true）。 ShowColName bool // OnCell 单元格点击回调（行、列从 0 开始，-1 表示表头行）。 OnCell func(row, col int) }` — SheetViewOptions 工作表渲染选项（零值即默认）。
- **函数** `func SheetTable(key string, sh *office.Sheet, opt SheetViewOptions) *Node` — SheetTable 把一张工作表渲染成表格组件。
- **函数** `func SpreadsheetView(key string, wb *office.Spreadsheet, active string, onPick func(name string)) *Node` — SpreadsheetView 工作簿：顶部工作表页签 + 当前表。
- **函数** `func WordView(key string, doc *office.WordDoc) *Node` — WordView 把 Word 文档渲染成文档流。
- **函数** `func PDFView(key string, doc *office.PDFDoc, zoom float32) *Node` — PDFView 把 PDF 渲染成页面卡片流（zoom 为 pt → 逻辑像素的缩放，默认 1）。
- **函数** `func OfficeView(key string, f *office.File) *Node` — OfficeView 按文件类型渲染（Excel / Word / PDF 自动分派）。
- **函数** `func PDFPrintable(doc *office.PDFDoc) Printable` — PDFPrintable 把 PDF 文档适配成 Printable（打印 / 打印预览用）。
- **函数** `func SheetPrintable(sh *office.Sheet, title string) Printable` — SheetPrintable 把工作表适配成 Printable（自动分页）。
- **函数** `func WordPrintable(doc *office.WordDoc) Printable` — WordPrintable 把 Word 文档适配成 Printable（按行流式排版 + 自动分页）。
- **函数** `func OfficePrintable(f *office.File) Printable` — OfficePrintable 任意办公文档 → Printable。
- **函数** `func ExportPDF(path, title string, doc Printable) error` — ExportPDF 把任意 Printable 导出成 PDF 文件（自动嵌入中文字体子集）。
- **方法** `func pdfPrintable.Title() string` — office.File 无标题时兜底的任务名。
- **方法** `func pdfPrintable.Pages() int`
- **方法** `func pdfPrintable.DrawPage(c *PrintCanvas, page int)`
- **方法** `func sheetPrintable.Title() string` — Title 打印任务名（工作表名）。
- **方法** `func sheetPrintable.Pages() int`
- **方法** `func sheetPrintable.DrawPage(c *PrintCanvas, page int)`
- **方法** `func wordPrintable.Title() string` — Title 打印任务名（文档标题）。
- **方法** `func wordPrintable.Pages() int`
- **方法** `func wordPrintable.DrawPage(c *PrintCanvas, page int)`

## `internal/office/office.go`

- **类型** `type Kind int` — Kind 文档类型。
- **类型** `type File struct { Path string Kind Kind Sheet *Spreadsheet Word *WordDoc PDF *PDFDoc }` — File 一份已打开的文档（Kind 决定哪个字段有效，其余为 nil）。
- **类型** `type Cell struct { Text string Value float64 Num bool Bold bool // Fill 背景色（"#rrggbb"，空 = 无）。 Fill string }` — Cell 一个单元格。Text 是展示文本，Value/Num 保留原始数值（排序 / 合计用）。
- **类型** `type Sheet struct { Name string Rows [][]Cell // Cols 最大列数（供渲染铺表头 A/B/C…）。 Cols int // Header 首行是否为表头（读入时按「首行加粗」推断）。 Header bool }` — Sheet 一张工作表：Rows[r][c]，行列从 0 开始。
- **类型** `type Spreadsheet struct { Sheets []Sheet }` — Spreadsheet 工作簿。
- **类型** `type BlockKind int` — BlockKind Word 内容块类型。
- **类型** `type Block struct { Kind BlockKind // Level 标题级别（1-6）或列表缩进级别。 Level int // Text 段落 / 标题 / 列表项文本。 Text string // Bold 整段加粗。 Bold bool // Rows 表格数据（BlockTable）。 Rows [][]string // HeaderRows 表格表头行数（BlockTable）。 HeaderRows int // Widths 表格各列相对宽度（空 = 等分）。 Widths []float32 }` — Block 一个内容块。
- **类型** `type WordDoc struct { // Title 文档标题（取自 docProps/core.xml 的 dc:title，可空）。 Title string Blocks []Block }` — WordDoc Word 文档。
- **类型** `type PDFItemKind int` — PDFItemKind PDF 绘制指令类型。
- **类型** `type PDFItem struct { Kind PDFItemKind Text string X, Y float32 // Size 字号（PDFText）。 Size float32 // W/H 宽高（PDFRect / PDFImage）。 W, H float32 // X2/Y2 终点（PDFLine）。 X2, Y2 float32 // Color 文字 / 线条颜色 "#rrggbb"（空 = 黑色）。 Color string // Fill 填充色（PDFRect，空 = 不填充）。 Fill string // Stroke 描边宽度（PDFRect / PDFLine），<=0 不描边。 Stroke float32 // Bold 文本是否加粗（渲染用）。 Bold bool }` — PDFItem 一条绘制指令。坐标系与 ui.PrintCanvas 一致：原点左上、Y 向下、单位 pt。
- **类型** `type PDFPage struct { // W/H 页面尺寸（pt）。 W, H float32 Items []PDFItem }` — PDFPage 一页。
- **类型** `type PDFDoc struct { Pages []PDFPage // Title 文档标题（取自 /Info /Title，可空）。 Title string }` — PDFDoc PDF 文档。
- **函数** `func Paragraph(text string) Block` — Paragraph 构造段落块。
- **函数** `func Heading(level int, text string) Block` — Heading 构造标题块（level 1-6）。
- **函数** `func Bullet(text string) Block` — Bullet 构造列表项。
- **函数** `func Table(rows [][]string) Block` — Table 构造表格块（首行为表头）。
- **函数** `func NewPDFPage(w, h float32) *PDFPage` — NewPDFPage 新建一页（w/h <= 0 时用 A4）。
- **函数** `func Detect(path string) Kind` — Detect 按扩展名 + 文件头判断类型（扩展名优先，文件头兜底）。
- **函数** `func Sniff(data []byte) Kind` — Sniff 按内容判断类型（字节流，不落盘）。
- **函数** `func Open(path string) (*File, error)` — Open 打开并解析文档。类型不支持时返回 ErrUnsupported。
- **函数** `func OpenBytes(path string, data []byte) (*File, error)` — OpenBytes 用给定字节解析（data 为 nil 时读文件）。path 只用于定位 / 判断类型。
- **函数** `func OpenBytesKind(path string, kind Kind, data []byte) (*File, error)` — OpenBytesKind 已知类型时直接按类型解析（避免重复探测）。
- **函数** `func FormatNumber(v float64) string` — FormatNumber 数值 → 展示文本：整数不带小数点，小数最多保留 6 位且去掉尾随 0。
- **方法** `func Kind.String() string` — String 类型名（界面展示用）。
- **方法** `func Kind.Ext() string` — Ext 推荐扩展名（导出对话框用）。
- **方法** `func Sheet.Cell(r, c int) Cell` — Cell 安全取单元格（越界返回零值）。
- **方法** `func Sheet.SetCell(r, c int, v Cell)` — SetCell 写入单元格（自动扩展行 / 列）。
- **方法** `func Sheet.Set(r, c int, text string)` — Set 写入文本单元格。
- **方法** `func Sheet.SetNum(r, c int, v float64)` — SetNum 写入数值单元格。
- **方法** `func Spreadsheet.First() *Sheet` — First 第一张工作表（没有则返回 nil）。
- **方法** `func Spreadsheet.SheetAt(i int) *Sheet` — SheetAt 按索引取工作表（越界返回 nil）。
- **方法** `func Spreadsheet.SheetByName(name string) *Sheet` — SheetByName 按名字取工作表（没有则返回第一张）。
- **方法** `func WordDoc.PlainText() string` — PlainText 纯文本化（导出 / 预览摘要用）。
- **方法** `func PDFPage.Text(x, y, size float32, s string) *PDFPage` — Text 追加文本（x/y 为左上角，Y 向下）。
- **方法** `func PDFPage.TextColor(x, y, size float32, s, color string) *PDFPage` — TextColor 追加带颜色的文本。
- **方法** `func PDFPage.TextBold(x, y, size float32, s string) *PDFPage` — TextBold 追加加粗文本。
- **方法** `func PDFPage.Line(x1, y1, x2, y2, width float32, color string) *PDFPage` — Line 追加直线。
- **方法** `func PDFPage.Rect(x, y, w, h, stroke float32, strokeColor, fill string) *PDFPage` — Rect 追加矩形（fill 为空只描边，stroke<=0 只填充）。
- **方法** `func File.Summary() string` — Summary 一句话摘要（列表 / 状态栏展示用）。
- **常量** `const KindUnknown = iota`
- **常量** `const KindExcel`
- **常量** `const KindWord`
- **常量** `const KindPDF`
- **变量** `var ErrUnsupported = errors.New("office: 不支持的文档类型（仅支持 .xlsx / .docx / .pdf）")` — ErrUnsupported 不支持的文档类型。
- **常量** `const BlockParagraph = iota`
- **常量** `const BlockHeading`
- **常量** `const BlockListItem`
- **常量** `const BlockTable`
- **常量** `const BlockQuote`
- **常量** `const PDFText = iota`
- **常量** `const PDFLine`
- **常量** `const PDFRect`
- **常量** `const PDFImage`
- **变量** `var A4 = [2]float32{595.28, 841.89}` — PDFPageSize 常用页面尺寸（pt）：A4 / Letter。
- **变量** `var Letter = [2]float32{612, 792}` — PDFPageSize 常用页面尺寸（pt）：A4 / Letter。

## `ui/print.go`

- **类型** `type PrintCanvas struct { W, H float32 cmds []printCmd // scale 打印时的设备像素 / 逻辑像素 scale float32 dc uintptr live bool }` — PrintCanvas 页面绘制画布（打印与预览共用同一套坐标）。
- **类型** `type Printable interface { Pages() int DrawPage(c *PrintCanvas, page int) }` — Printable 可打印文档。
- **函数** `func Print(title string, doc Printable) (bool, error)` — Print 弹出系统打印对话框并打印文档；用户取消返回 (false, nil)。
- **函数** `func PrintPreview(key string, doc Printable, zoom float32) *Node` — PrintPreview 打印预览（按 A4 比例渲染页面卡片，可缩放）。
- **函数** `func Printers() []string` — Printers 可用打印机名（默认打印机排第一位；非 Windows / Web 端返回 nil）。
- **函数** `func DefaultPrinter() string` — DefaultPrinter 默认打印机名（没有则空串）。
- **函数** `func PrintTo(printer, title string, doc Printable) (bool, error)` — PrintTo 不弹系统对话框，直接打印到指定打印机。
- **函数** `func PrintDialogOpen() bool` — PrintDialogOpen 当前是否有内置打印对话框在显示。
- **函数** `func OpenPrintDialog(title string, doc Printable, reason string)` — OpenPrintDialog 打开内置打印对话框（打印机列表 + 打印 / 取消）。
- **函数** `func ClosePrintDialog()` — ClosePrintDialog 关闭内置打印对话框。
- **函数** `func PrintDialogLayer() *Node` — PrintDialogLayer 内置打印对话框浮层（挂在根 Stack 上；未打开时返回 nil）。
- **方法** `func PrintCanvas.Text(x, y, size float32, s, color string)` — Text 绘制文本（x/y 为左上角）。
- **方法** `func PrintCanvas.Rect(x, y, w, h, stroke float32, strokeColor, fill string)` — Rect 绘制矩形（fill 为空只描边；stroke<=0 只填充）。
- **方法** `func PrintCanvas.Line(x1, y1, x2, y2, width float32, color string)` — Line 绘制直线。
- **常量** `const PageW96 = 794` — A4 页面逻辑尺寸（96 DPI）。
- **常量** `const PageH96 = 1123` — A4 页面逻辑尺寸（96 DPI）。

## `ui/webview.go`

- **类型** `type WebView struct { core *win32.WebView2 app *App // 上一帧的逻辑矩形与可见状态（调试 / 命中测试用） rect geom.Rect visible bool // radius 网页区域圆角（逻辑像素；>0 时用窗口区域真裁剪，而非画圆角） radius float32 // 合成模式（B 阶段）：网页是 DComp 图层，位置/裁剪走视觉属性 compose bool compVisual uintptr }` — WebView 内嵌浏览器节点（系统 WebView2 / Edge 内核）。
- **类型** `type WebViewOptions struct { // URL 初始地址（http/https/file 均可）。 URL string // HTML 直接加载 HTML（优先于 URL，适合离线页面 / 自绘模板）。 HTML string // UserDataDir 用户数据目录（cookie / 缓存），空 = %LOCALAPPDATA%\GUIS\WebView2。 UserDataDir string // Zoom 初始缩放（1 = 100%）。 Zoom float64 // Transparent 背景透明（网页自身透明处透出下层自绘内容）。 Transparent bool // OnChange 页面状态变化（加载开始/完成、地址、标题、历史）时回调。 OnChange func() }` — WebViewOptions 创建内嵌浏览器参数。
- **类型** `type WebViewBarOptions struct { // Address 地址栏当前文本（由调用方持有）。 Address string // Key 地址输入框的 key（同一界面内唯一）。 Key string // OnAddress 地址文本变化。 OnAddress func(string) // OnGo 回车或点击「打开」时触发（通常调 wv.Navigate(addr)）。 OnGo func() // ShowTitle 在地址栏右侧显示页面标题。 ShowTitle bool }` — WebViewBarOptions 工具条参数。
- **函数** `func NewWebView(opts WebViewOptions) (*WebView, error)` — NewWebView 在当前窗口创建内嵌浏览器（须在 UI 线程调用，如按钮回调里）。
- **函数** `func NewWebViewIn(a *App, opts WebViewOptions) (*WebView, error)` — NewWebViewIn 在指定窗口创建内嵌浏览器。
- **函数** `func WebViewAvailable() bool` — Available WebView2 运行时是否可用（未安装时组件降级为占位卡片）。
- **函数** `func Browser(wv *WebView) *Node` — Browser WebView 节点（网页区域的占位节点；真实像素由浏览器子窗口绘制）。
- **函数** `func WebViewNode(wv *WebView) *Node` — WebViewNode WebView 节点，用法同 ui.Video(p)：尺寸由样式决定，
- **函数** `func WebViewBar(wv *WebView, o WebViewBarOptions) *Node` — WebViewBar 自绘浏览器工具条：← → ↻ + 地址栏 + 打开 + 加载指示。
- **函数** `func WebViewStatus(wv *WebView) *Node` — WebViewStatus 页面状态行：地址 / 标题 / 加载中提示。
- **方法** `func WebView.Ready() bool` — Ready 浏览器内核是否已就绪。
- **方法** `func WebView.Err() error` — Err 创建错误（无则 nil）。
- **方法** `func WebView.Navigate(url string)` — Navigate 打开地址。
- **方法** `func WebView.LoadHTML(html string)` — LoadHTML 加载一段 HTML。
- **方法** `func WebView.Back()` — Back / Forward / Reload / Stop 导航控制。
- **方法** `func WebView.Forward()`
- **方法** `func WebView.Reload()`
- **方法** `func WebView.Stop()`
- **方法** `func WebView.Focus()` — Focus 把焦点交回网页。
- **方法** `func WebView.SetRadius(r float32)` — SetRadius 网页区域圆角（逻辑像素，>0 生效）。
- **方法** `func WebView.SetZoom(z float64)` — SetZoom 缩放（1 = 100%）。
- **方法** `func WebView.ExecuteScript(js string)` — ExecuteScript 在页面执行 JS。
- **方法** `func WebView.URL() string` — URL 当前地址。
- **方法** `func WebView.Title() string` — Title 当前页面标题。
- **方法** `func WebView.Loading() bool` — Loading 是否正在加载。
- **方法** `func WebView.CanGoBack() bool` — CanGoBack / CanGoForward 历史状态。
- **方法** `func WebView.CanGoForward() bool`
- **方法** `func WebView.Close()` — Close 关闭浏览器（窗口销毁时会自动调用，也可手动提前释放）。

## `ui/codeeditor.go`

- **常量** `const CodeDefaultLineHeight = 20` — 编辑器默认度量（逻辑像素）。
- **常量** `const CodeDefaultFontSize = 13` — 编辑器默认度量（逻辑像素）。
- **类型** `type CodePos struct{ Line, Col int }` — CodePos 文档位置：行号 + 行内列号（均从 0 开始，列按 rune 计）。
- **方法** `func CodePos.Before(q CodePos) bool` — Before 位置是否严格在前。
- **方法** `func CodePos.After(q CodePos) bool` — After 位置是否严格在后。
- **方法** `func CodePos.Equal(q CodePos) bool` — Equal 位置是否相同。
- **类型** `type CodeRange struct{ Start, End CodePos }` — CodeRange 文档区间（[Start, End)）。
- **类型** `type CodeDiagnostic struct { Line, Col int // 起点（0 基） EndLine, EndCol int // 终点 Severity int // 1=error 2=warning 3=info 4=hint Message string Source string }` — CodeDiagnostic 一条诊断（LSP publishDiagnostics 与内置解析器共用）。
- **类型** `type CodeTheme struct { Bg, GutterBg, GutterText, GutterActive gpu.Paint CurrentLine, Selection, Caret gpu.Paint IndentGuide, Scrollbar, MinimapBg gpu.Paint MinimapView, MinimapLine gpu.Paint Text gpu.Paint Keyword, Type, String, Comment gpu.Paint Number, Function, Operator gpu.Paint Punct, Constant, Attribute gpu.Paint Tag, Invalid gpu.Paint }` — CodeTheme 编辑器配色（独立于 UI 主题，切主题时由页面自行换一套）。
- **函数** `func DarkCodeTheme() CodeTheme` — DarkCodeTheme 深色配色（默认）。
- **函数** `func LightCodeTheme() CodeTheme` — LightCodeTheme 浅色配色。
- **类型** `type CodeMenuItem struct { Label string Hint string Sep bool Checked bool Run func() }` — CodeMenuItem 内建右键菜单项（空格分隔的 Hint 显示在右侧）。
- **类型** `type CodeCompletion struct { Label string Detail string Insert string Kind int DocPath string Line int Col int }` — CodeCompletion 一条补全项（内置关键字 / LSP CompletionItem 统一结构）。
- **类型** `type CodeEditorState struct { // ---- 文档（UI 线程独占）---- Lines []string Pos CodePos // Anchor 选区锚点：与 Pos 相同表示无选区。 Anchor CodePos // ---- 视图 ---- ScrollY, ScrollX float32 LineH float32 FontSize float32 FontFamily string TabSize int Theme CodeTheme MinW, MinH float32 ShowGutter bool ShowMinimap bool ShowCurrentLine bool ShowIndentGuide bool MinimapW float32 ReadOnly bool Dirty bool // ---- 身份 / 回调 ---- Key string Path string // 文件路径（LSP 文档 URI 用） Language string // go / javascript / typescript / json / python / … Title string OnChange func(text string) OnCaret func(line, col int) OnSave func(text string) OnStatus func(msg string) // ---- 内部 ---- undo, redo []codeDelta cache *codeCache viewW float32 viewH float32 gutterW float32 menuOpen bool menuX, menuY float32 menuSel int findOpen bool findMode int // 0 = 查找，1 = 转到行，2 = 重命名 findText string findCase bool findAt int lastSync time.Time compOpen bool compItems []CodeCompletion compSel int compX, compY float32 // renameWord 重命名输入条打开时的原符号名 renameWord string // textW 本帧文本区宽度（由绘制写入，命中测试 / 补全面板复用） textW float32 // ---- 视图扩展（软换行 / 折叠 / 视觉行前缀缓存，见 codeeditor_view.go）---- Wrap bool folds []codeFold folded map[int]bool hidden []int32 foldsReady bool rowCount []int32 rowBefore []int32 rowsValid int // ---- 交互扩展 ---- // dragKind 本次按下的落点分区（文本 / 滚动条 / 小地图），OnDrag 期间不变。 dragKind codeHitKind // dragGrab 按下时鼠标在滑块内的抓取偏移，保证拖动不跳。 dragGrab float32 // dragActive 本次拖动是否真的在「非文本区」（滚动条 / 小地图）。 dragActive bool // dragMoved 本次按下发生过拖动（松手时不再当成单击）。 dragMoved bool // dragStartX/Y 按下帧的位置（判定是否真的拖动过）。 dragStartX, dragStartY float32 // dragPress 按下帧落点对应的文档位置（真正拖动时选区从这里开始）。 dragPress CodePos // dragSelecting 文本区是否已经越过阈值、开始按拖动扩展选区。 dragSelecting bool // skipClick 拖动结束后的第一次松手点击忽略（否则会折叠掉刚拖出的选区）。 skipClick bool // blinkAnchor 光标闪烁的相位起点（AnimTime 秒）；输入后短暂常亮。 blinkAnchor float32 // cursors 额外光标（主光标仍是 Pos / Anchor）；Ctrl+Alt+↑↓ / Alt+点击添加。 cursors []codeCursor // diagOpen 诊断列表面板是否展开；diagSel 选中项。 diagOpen bool diagSel int // replaceText 替换输入框内容；replaceFocus 0=查找框 1=替换框。 replaceText string replaceFocus int // findCaret / replaceCaret 两个输入框的插入位置（按 rune 计）；findSelAll Ctrl+A 全选。 findCaret int replaceCaret int findSelAll bool // semantic LSP 语义高亮片段（行 → 片段）；semanticOn 是否启用。 semantic map[int][]codeSpan semanticOn bool semanticStamp uint64 // semanticSeq 语义 token 请求序号：每次新请求 +1，后台结果回来时序号对不上就丢弃。 semanticSeq uint64 // semanticBusy 已有在飞的语义 token 请求（含退避重试），避免每次按键都开一串协程。 semanticBusy bool // semanticEmpty 轮询重试都拿不到 token（服务端没返回，页面不要一直说“等待服务端”）。 semanticEmpty bool // quiet 批量编辑期间跳过逐次 OnChange / LSP 同步（多光标只通知一次）。 quiet bool hoverText string hoverX, hoverY float32 hoverOpen bool // ---- 鼠标悬停语法提示 ---- // hoverKeyNow 鼠标当前所在符号（行:列:词），用于去重与判断是否仍停在同一个词上。 hoverKeyNow string // hoverSeq 悬停去抖序号（计时回调投递时比对，过期丢弃）。 hoverSeq int // hoverPinned 提示由鼠标弹出：移到别的词 / 离开编辑器 / 输入 / 点击时收起。 hoverPinned bool // hoverMouseX/Y 鼠标在节点内的坐标（提示面板定位用）。 hoverMouseX, hoverMouseY float32 lspMu sync.Mutex lspDiags []CodeDiagnostic builtinDiags []CodeDiagnostic lspStatus string // lspLog 服务端日志（单行化，最多保留 codeLSPLogMax 条；不写状态栏，状态栏只放一行摘要）。 lspLog []string srv *LSPClient hoverLines []string // hoverKinds 与 hoverLines 等长：每行的排版类型（代码 / 标题 / 列表 / 虚线）。 hoverKinds []codeHoverKind lastClick time.Time lastClickLine int autoPairs bool changeSeq uint64 searchSeq int }` — CodeEditorState 代码编辑器状态（跨帧保持；由 CodeEditor 构建成节点）。
- **函数** `func NewCodeEditorState(key, text string) *CodeEditorState` — NewCodeEditorState 新建编辑器状态（key 必须全局唯一：同一时间只有一个编辑器持有焦点）。
- **方法** `func CodeEditorState.SetText(s string)` — SetText 整体替换文档（重置撤销栈、光标与高亮缓存）。
- **方法** `func CodeEditorState.Text() string` — Text 文档全文。
- **方法** `func CodeEditorState.LineCount() int` — LineCount 行数。
- **方法** `func CodeEditorState.SetLanguage(name string)` — SetLanguage 指定语法着色语言名（go / json / …；未知名称按 plain 处理）。
- **方法** `func CodeEditorState.SetLanguageByPath(path string)` — SetLanguageByPath 按文件名推断语言（.go / .json / .ts …）。
- **方法** `func CodeEditorState.SetTheme(t CodeTheme)` — SetTheme 换配色。
- **方法** `func CodeEditorState.MarkSaved()` — MarkSaved 标记已保存（清脏标记）。
- **方法** `func CodeEditorState.Selection() (CodeRange, bool)` — Selection 返回规范化后的选区（无选区时 ok=false）。
- **方法** `func CodeEditorState.SelectedText() string` — SelectedText 选中的文本（无选区返回空串）。
- **方法** `func CodeEditorState.SetCaret(p CodePos, keep bool)` — SetCaret 定位光标（keep=false 折叠选区）。
- **方法** `func CodeEditorState.SetCaretLine(line int)` — SetCaretLine 跳到指定行首（1 基，供「转到行」使用）。
- **方法** `func CodeEditorState.SelectAll()` — SelectAll 全选。
- **方法** `func CodeEditorState.InsertText(s string)` — InsertText 在光标处插入文本（有选区先替换选区）。
- **方法** `func CodeEditorState.Undo() bool` — Undo 撤销。
- **方法** `func CodeEditorState.Redo() bool` — Redo 重做。
- **方法** `func CodeEditorState.DeleteBack() bool` — DeleteBack 退格（有选区先删选区；多光标时所有光标同时删）。
- **方法** `func CodeEditorState.DeleteForward() bool` — DeleteForward 删除光标后一个字符（多光标时所有光标同时删）。
- **方法** `func CodeEditorState.InsertNewline()` — InsertNewline 回车：自动保持当前缩进（花括号后再多缩进一档）。
- **方法** `func CodeEditorState.IndentSelection(outdent bool)` — IndentSelection 选中多行时整体增减缩进；无选区时插入 / 删除一档空格。
- **方法** `func CodeEditorState.MoveCaret(dLine, dCol int, extend bool)` — MoveCaret 移动光标（extend=true 时保持锚点即扩展选区）。
- **方法** `func CodeEditorState.MoveLineStart(extend bool)` — MoveLineStart 行首 / 首个非空白（Home 键两段式）。
- **方法** `func CodeEditorState.MoveLineEnd(extend bool)` — MoveLineEnd 行尾。
- **方法** `func CodeEditorState.MoveWord(dir int, extend bool)` — MoveWord 按词移动（Ctrl+←/→）。
- **方法** `func CodeEditorState.PageMove(dir int, extend bool)` — PageMove 翻页（PageUp / PageDown）。
- **方法** `func CodeEditorState.Copy() bool` — Copy 复制选区到系统剪贴板。
- **方法** `func CodeEditorState.Cut() bool` — Cut 剪切选区。
- **方法** `func CodeEditorState.Paste() bool` — Paste 粘贴剪贴板文本。
- **方法** `func CodeEditorState.MoveLines(dir int)` — MoveLines 上下移动当前行 / 选中行。
- **方法** `func CodeEditorState.DuplicateLine()` — DuplicateLine 复制当前行（Ctrl+D）。
- **方法** `func CodeEditorState.ToggleComment()` — ToggleComment 行注释切换（// 、# 、-- 按语言）。
- **方法** `func CodeEditorState.Find(text string, forward bool) bool` — Find 查找 next / prev（区分大小写受 findCase 控制），返回是否命中。
- **方法** `func CodeEditorState.FindText() string` — FindText 当前查找串。
- **方法** `func CodeEditorState.FindOpen() bool` — FindOpen 查找栏是否展开。
- **方法** `func CodeEditorState.HandleKey(vk uint32) bool` — HandleKey 处理按键（框架 RegisterKeyHandler 调用；返回 true 表示已消费）。
- **方法** `func CodeEditorState.HandleChar(r rune)` — HandleChar 处理字符输入（OnChar）。
- **方法** `func CodeEditorState.ScrollByLines(n float32)` — ScrollByLines 键盘 / API 滚动。
- **方法** `func CodeEditorState.ScrollToLine(line int)` — ScrollToLine 把指定行滚动到视口顶部（1 基）。
- **方法** `func CodeEditorState.PosAt(x, y float32) CodePos` — PosAt 把节点内坐标换算成文档位置（不修改状态；软换行 / 折叠已经纳入）。
- **方法** `func CodeEditorState.ClickAt(x, y float32)` — ClickAt 左键点击：定位光标 / 双击选词。
- **方法** `func CodeEditorState.DragTo(x, y float32)` — DragTo 拖动选择（文本区）。
- **方法** `func CodeEditorState.TextClick(x, y float32, alt bool)` — TextClick 文本区左键单击：Alt+点击新增光标，普通点击收起多光标后定位。
- **方法** `func CodeEditorState.MouseDrag(x, y float32)` — MouseDrag 鼠标拖动（含按下帧）：按下帧只记录落点，越过阈值后才开始选。
- **方法** `func CodeEditorState.MouseDragEnd()` — MouseDragEnd 结束拖动（拖动后的第一次松手点击不再折叠刚选出来的选区）。
- **函数** `func CodeEditor(st *CodeEditorState) *Node` — CodeEditor 把状态构建成节点（每帧调用；回调与快捷键在此重新登记）。


## `ui/codeeditor_find.go`

- **方法** `func CodeEditorState.OpenFind(mode int)` — OpenFind 打开查找栏（mode：0 查找 / 1 转到行 / 2 重命名 / 3 查找替换）。
- **方法** `func CodeEditorState.OpenReplace()` — OpenReplace 打开查找替换面板（Ctrl+H）。
- **方法** `func CodeEditorState.CloseFind()` — CloseFind 关闭查找栏。
- **方法** `func CodeEditorState.FindBackspace()` — FindBackspace 面板退格（删插入点前一个字符；全选时清空）。
- **方法** `func CodeEditorState.FindDelete()` — FindDelete 面板 Delete（删插入点处字符）。
- **方法** `func CodeEditorState.FindCaretMove(delta int)` — FindCaretMove 左右移动插入点。
- **方法** `func CodeEditorState.FindCaretHome()` — FindCaretHome 插入点移到行首。
- **方法** `func CodeEditorState.FindCaretEnd()` — FindCaretEnd 插入点移到行尾。
- **方法** `func CodeEditorState.FindSelectAll()` — FindSelectAll Ctrl+A：全选（下一次输入 / 退格直接替换全部）。
- **方法** `func CodeEditorState.FindPaste()` — FindPaste 粘贴剪贴板（换行压成空格，面板是单行输入框）。
- **方法** `func CodeEditorState.FindPanelClick(x, y float32) bool` — FindPanelClick 面板点击；返回 true 表示事件已被面板消费。
- **方法** `func CodeEditorState.MatchCount() int` — MatchCount 查找串的命中总数（最多统计 9999 处，十万行也不卡输入）。
- **方法** `func CodeEditorState.ReplaceCurrent() bool` — ReplaceCurrent 替换当前命中（无命中先定位下一个）。
- **方法** `func CodeEditorState.ReplaceAll() int` — ReplaceAll 全文替换，返回替换处数（一条撤销记录）。
- **方法** `func CodeEditorState.ToggleDiagPanel()` — ToggleDiagPanel 展开 / 收起诊断列表（F8 / Ctrl+Shift+M）。
- **方法** `func CodeEditorState.DiagPanelOpen() bool` — DiagPanelOpen 问题面板是否展开。
- **方法** `func CodeEditorState.DiagnosticsList() []CodeDiagnostic` — DiagnosticsList 诊断列表（按行、列排序）。
- **方法** `func CodeEditorState.FocusDiagCaret()` — FocusDiagCaret 选中光标处 / 之后的诊断。
- **方法** `func CodeEditorState.DiagPanelClick(x, y float32) bool` — DiagPanelClick 诊断面板点击；返回 true 表示已消费。
- **方法** `func CodeEditorState.PickDiagnostic(idx int)` — PickDiagnostic 跳到第 idx 条诊断。
- **方法** `func CodeEditorState.DiagStep(dir int)` — DiagStep 在诊断列表里上下移动（面板打开时 ↑↓ / F8 Shift+F8）。


## `ui/codeeditor_lsp.go`

- **方法** `func CodeEditorState.AttachLSP(c *LSPClient)` — AttachLSP 绑定语言服务器（传 nil 解绑并回退内置解析器）。
- **方法** `func CodeEditorState.SetStatus(msg string)` — SetStatus 设置状态栏文案（可在后台协程调用：投递到 UI 线程，下一帧生效）。
- **方法** `func CodeEditorState.LSP() *LSPClient` — LSP 当前绑定的服务（未接为 nil）。
- **方法** `func CodeEditorState.LSPStatus() string` — LSPStatus 一行连接状态（页面顶部展示）。
- **方法** `func CodeEditorState.LSPLog() []string` — LSPLog 最近的服务端日志（旧 → 新；页面可做成日志面板）。
- **方法** `func CodeEditorState.SaveDocument()` — SaveDocument 保存（通知 LSP + 触发 OnSave 回调）。
- **方法** `func CodeEditorState.RequestCompletion()` — RequestCompletion 触发补全（LSP 异步 / 内置同步）。
- **方法** `func CodeEditorState.RequestHover()` — RequestHover 悬停信息（F1 / 右键菜单）：作用在光标所在符号上。
- **方法** `func CodeEditorState.RequestDefinition()` — RequestDefinition 转到定义（F12）。
- **方法** `func CodeEditorState.RequestRename()` — RequestRename 打开重命名输入条（F2）。
- **方法** `func CodeEditorState.SetDiagnostics(ds []CodeDiagnostic)` — SetDiagnostics 外部注入诊断（业务侧把编译错误塞进编辑器）。
- **方法** `func CodeEditorState.Diagnostics() []CodeDiagnostic` — Diagnostics 当前全部诊断（内置 + LSP）。
- **方法** `func CodeEditorState.GoToLine(line int)` — GoToLine 跳到指定行（1 基，滚动到视口中间）。


## `ui/codeeditor_menu.go`

- **方法** `func CodeEditorState.AcceptCompletion()` — AcceptCompletion 接受当前补全项（替换光标前的单词前缀）。
- **方法** `func CodeEditorState.FormatDocument()` — FormatDocument 格式化整个文档（有 LSP 用 LSP，否则做括号缩进整理）。
- **方法** `func CodeEditorState.NextDiagnostic()` — NextDiagnostic 跳到下一条诊断（循环）。
- **函数** `func CodeEditorShortcuts() [][2]string` — CodeEditorShortcuts 编辑器内建快捷键（帮助面板 / 文档用）。


## `ui/codeeditor_multi.go`

- **方法** `func CodeEditorState.Carets() [][2]CodePos` — Carets 全部光标（主光标在前），按文档顺序升序返回。
- **方法** `func CodeEditorState.CursorCount() int` — CursorCount 光标数量（含主光标）。
- **方法** `func CodeEditorState.HasMultiCursor() bool` — HasMultiCursor 是否有额外光标。
- **方法** `func CodeEditorState.ClearCursors()` — ClearCursors 收起所有额外光标（Esc / 单击普通位置时调用）。
- **方法** `func CodeEditorState.AddCursor(dLine int) bool` — AddCursor 在相邻行新增光标（Ctrl+Alt+↑ / ↓）。最多 16 个，保持不重复。
- **方法** `func CodeEditorState.AddCursorAt(p CodePos)` — AddCursorAt Alt+点击：在指定位置添加光标。
- **方法** `func CodeEditorState.InsertAtCarets(s string) bool` — InsertAtCarets 在所有光标处插入文本（有选区先替换选区）。
- **方法** `func CodeEditorState.SelectLine(line int)` — SelectLine 选中整行（行号栏点击）。
- **方法** `func CodeEditorState.MoveCarets(dLine int)` — MoveCarets 上下移动所有光标（无选区时的 Alt+↑↓ 多光标版本）。


## `ui/codeeditor_semantic.go`

- **方法** `func LSPClient.SupportsSemantic() bool` — SupportsSemantic 服务端是否支持语义 token。
- **方法** `func LSPClient.SemanticTokens(uri string) ([]codeSemToken, bool)` — SemanticTokens 请求全文语义 token（同步，调用方在后台协程里跑）。
- **方法** `func CodeEditorState.SetSemantic(on bool)` — SetSemantic 开关语义高亮（UI 线程调用；开启且已接 LSP 时立即请求一次）。
- **方法** `func CodeEditorState.SetSemanticAsync(on bool)` — SetSemanticAsync 开关语义高亮（可在后台协程调用，下一帧在 UI 线程生效）。
- **方法** `func CodeEditorState.SemanticOn() bool` — SemanticOn 语义高亮是否开启。
- **方法** `func CodeEditorState.RequestSemantic()` — RequestSemantic 异步请求语义 token（服务端不支持时静默返回）。
- **方法** `func CodeEditorState.SemanticSummary() string` — semanticSummary 语义高亮来源摘要（页面状态栏展示）。


## `ui/codeeditor_view.go`

- **方法** `func CodeEditorState.SetWrap(on bool)` — SetWrap 开 / 关软换行（关闭时清掉换行缓存并恢复水平滚动）。
- **方法** `func CodeEditorState.ToggleFold(line int) bool` — ToggleFold 折叠 / 展开以 line 起始的块。
- **方法** `func CodeEditorState.FoldAll(collapse bool)` — FoldAll 全部折叠 / 展开。
- **方法** `func CodeEditorState.ToggleFoldAtCaret() bool` — ToggleFoldAtCaret 折叠光标所在的块（块内时折叠该块，块首时折叠自身）。
- **方法** `func CodeEditorState.FoldedCount() int` — FoldedCount 已折叠块数量。
- **方法** `func CodeEditorState.StatusLine() string` — StatusLine 一行状态摘要：语言 · 软换行 · 折叠 · 多光标 · 诊断数。


## `ui/lsp.go`

- **类型** `type CodeLocation struct { Path string Line int Col int }` — CodeLocation 定义 / 引用的位置（0 基行列，与编辑器一致）。
- **类型** `type LSPClient struct { // Name 显示名（gopls / tsserver …）。 Name string // Command 可执行文件（PATH 内或绝对路径）。 Command string Args []string // Root 工作区根目录（go.mod / package.json 所在目录）。 Root string // Env 追加环境变量（KEY=VALUE）。 Env []string // Timeout 单次请求超时（默认 6s）。 Timeout time.Duration // InitOptions initialize 请求里的 initializationOptions。 // gopls 的 semanticTokens 选项默认关闭，必须在这里（或 workspace/configuration） // 显式打开，否则 semanticTokens/full 永远返回空数组。 InitOptions map[string]any // Settings workspace/configuration 的应答内容（对请求里的每个 section 各回一份）。 Settings map[string]any // OnDiagnostics 诊断推送回调（在读取协程里调用，需要自己切回 UI 线程）。 OnDiagnostics func(uri string, diags []CodeDiagnostic) // OnLog 服务端日志（window/logMessage / 进程 stderr）。 OnLog func(string) // OnError 连接层错误（进程退出 / 读取失败），区别于服务端的普通日志。 OnError func(string) mu sync.Mutex stdin io.WriteCloser nextID int pending map[int]chan lspMessage opened map[string]int diags map[string][]CodeDiagnostic started bool stopped bool stopping bool stopFn func() error capabs map[string]any }` — LSPClient 一个语言服务器会话。
- **函数** `func NewLSPClient(name, command string, args ...string) *LSPClient` — NewLSPClient 新建客户端（尚未启动）。
- **函数** `func FileURI(path string) string` — FileURI 把本地路径转成 file:// URI（盘符、空格、中文都按 RFC 3986 转义）。
- **函数** `func URIPath(uri string) string` — URIPath 把 file:// URI 还原成本地路径
- **方法** `func LSPClient.Start() error` — Start 启动服务进程并完成 initialize 握手。
- **方法** `func LSPClient.Started() bool` — Started 是否已完成握手。
- **方法** `func LSPClient.Capability(name string) bool` — Capability 服务端能力（initialize 结果里的 capabilities 子项）。
- **方法** `func LSPClient.Stop()` — Stop 关闭会话（先 shutdown 请求，再结束进程）。
- **方法** `func LSPClient.DidOpen(uri, languageID, text string)` — DidOpen 打开文档（通知服务端文档内容）。
- **方法** `func LSPClient.DidChange(uri, text string)` — DidChange 全文同步（version 自增；服务端按版本应用）。
- **方法** `func LSPClient.DidSave(uri, text string)` — DidSave 保存通知。
- **方法** `func LSPClient.DidClose(uri string)` — DidClose 关闭文档。
- **方法** `func LSPClient.Diagnostics(uri string) []CodeDiagnostic` — Diagnostics 最近一次收到的诊断。
- **方法** `func LSPClient.Completion(uri, langID string, line, col int, text string) []CodeCompletion` — Completion 请求补全（同步，调用方应在后台协程里跑）。
- **方法** `func LSPClient.Hover(uri, langID string, line, col int, text string) string` — Hover 悬停信息（返回去标记的纯文本多行）。
- **方法** `func LSPClient.HoverMarkdown(uri, langID string, line, col int, text string) string` — HoverMarkdown 悬停信息（返回服务端原始 Markdown，编辑器自行排版高亮）。
- **方法** `func LSPClient.Definition(uri, langID string, line, col int, text string) []CodeLocation` — Definition 转到定义（可能有多个位置）。
- **方法** `func LSPClient.Format(uri, text string, tabSize int) (string, bool)` — Format 请求格式化，返回新全文（ok=false 表示服务端不支持或失败）。
- **方法** `func LSPClient.Rename(uri, langID string, line, col int, text, newName string) (map[string]string, bool)` — Rename 重命名符号，返回「文件 → 新全文」的修改集合。
