<!-- group: 布局与样式 -->
# 样式 St() · 尺寸 / 间距 / 描边 / 定位 / 缓存

`ui.St()` 返回 `*Style`，全部方法都返回自身，可链式调用；`Node.St(s)` 把样式挂到节点上。

```go
ui.Text("标题").St(ui.St().Text(18, 700, ui.ColText).PadXY(14, 10))
```

## 1. 尺寸与间距

| 方法 | 说明 |
| --- | --- |
| `W(v)` / `H(v)` | 固定宽 / 高（0 = auto） |
| `Wp(v)` / `Hp(v)` | 占父容器百分比（`Hp(100)` = 满高） |
| `MinWidth/MaxWidth/MinHeight/MaxHeight(v)` | 尺寸约束 |
| `Padding(v)` / `PadXY(h, v)` / `PadOnly(t, r, b, l)` | 内边距 |
| `Mar(v)` / `MarXY(h, v)` / `MarOnly(t, r, b, l)` | 外边距 |
| `GapOf(v)` | 子项间距（flex gap） |
| `GrowTo(v)` | flex-grow |
| `FlexBasis(v)` | 主轴基准尺寸 |
| `Self(a)` | 覆盖父容器的交叉轴对齐 |
| `ColsN(n)` / `ColWidth(v)` | 网格 / 瀑布流列参数 |

## 2. 背景、圆角、描边、阴影

```go
ui.Box().St(ui.St().
    BgC("#151f33").                          // 纯色（也可传 gpu.Paint / uint32）
    R(12).                                   // 统一圆角
    Rx(12, 12, 0, 0).                        // 四角分别指定（左上/右上/右下/左下）
    Stroke(1, ui.ColStroke).                 // 描边（宽 + 色）
    Shadow(24, 8, 0, "rgba(0,0,0,0.35)"))    // 阴影（blur, y, spread, color）
```

渐变：

```go
ui.Box().St(ui.St().BgGrad("#6366f1", "#8b5cf6", 120))   // from, to, 角度(度)
```

渐变端点允许全透明（`rgba(99,102,241,0)`），框架按「显式设置过」判断而非 alpha。

## 3. 文字

```go
ui.Text("正文").St(ui.St().Text(13, 400, ui.ColTextDim))  // 字号, 字重, 颜色
ui.Text("长文本").St(ui.St().Text(13, 400, ui.ColText).WrapOn().Lh(1.7))
ui.Text("居中").St(ui.St().Text(14, 600, ui.ColText).TAlign(ui.AlignCenter))
ui.Btn("按钮", ui.BtnPrimary, fn).St(ui.St().CenterYOn())  // 文字在自身高度内垂直居中（按钮默认开）
ui.Text("Consolas").St(ui.St().Text(12, 400, ui.ColText).Font("Consolas"))
```

* `WrapOn()` 按真实测量宽度折行（测量与绘制共用同一折行算法，不会出现行数不一致）。
* `Lh` 是**倍数**行高；`Font(family)` 需系统字体目录能解析到。

## 4. 透明度、裁剪、毛玻璃

```go
ui.Box(child).St(ui.St().Fade(0.5))                     // 透明度 0..1
ui.Box(child).St(ui.St().Clipped())                     // 裁剪子节点（圆角裁剪也走它）
ui.Box(child).St(ui.St().Backdrop(18, ui.ColGlass))      // 毛玻璃（blur 逻辑像素 + 叠加色）
```

`Backdrop` 走离屏 FBO + Kawase 模糊，代价较高：只在必要的浮层（菜单、弹窗标题区）用。

## 5. 绝对定位

| 方法 | 说明 |
| --- | --- |
| `AbsFill()` | 铺满父容器内边距区 |
| `AbsTL(l, t)` / `AbsTR(r, t)` / `AbsBL(l, b)` / `AbsBR(r, b)` | 四角定位 |
| `AbsCenter()` | 水平 + 垂直居中 |
| `AbsBottom(b)` | 水平居中贴底（Toast 用） |

## 6. 交互与动画标记

| 方法 | 说明 |
| --- | --- |
| `FocusOn()` | 允许获取键盘焦点（输入类控件） |
| `AnimOn()` | 每帧重绘（自绘加载动画、拖拽跟手） |
| `AnimSlowOn()` | 低帧率持续重绘（~30Hz：光标闪烁、骨架屏流光），避免装饰动画吃满 CPU |
| `CacheOn()` | 开启组件栅格缓存（子树快照成纹理） |
| `NoFocusRing()` | 关闭统一焦点环（自绘焦点样式时用） |
| `Translate(x, y)` | 绘制期位移，不参与布局 / 命中（FLIP、抖动） |
| `ScrollY()` / `ScrollbarOn()` | 滚动与滚动条常显 |

## 7. 滚动条

滚动条由框架绘制：默认可见、闲置淡出；`ScrollbarOn()` 常显。
关闭：`Scrollbar: false`（`Style` 字段）或直接不用 `Scroll`。

## 8. 性能开关速查

| 场景 | 做法 |
| --- | --- |
| 大卡片 / 长文档块 / 复杂自绘控件 | `CacheOn()`（要求：不透明底色、无圆角/描边/阴影、无动画/图片/图表/视频/网页） |
| 光标闪烁 / 骨架屏 | `AnimSlowOn()` |
| 拖拽跟手 / 自绘加载环 | `AnimOn()` |
| 滚动历史内容 | 交给自动缓存（`ui.SetCacheLayers(true)`） |
| 排查某页为什么一直重绘 | `GUIS_FPS=1` 看每帧动画节点清单 |

## 9. Style 字段直写

链式方法覆盖不到的字段可以直接写（`Style` 是导出结构体）：

```go
st := ui.St().W(200)
st.Clip = true
st.Opacity = 0.9
st.DrawY = 4        // 绘制期下移 4px
node.St(st)
```

完整字段：`Width/Height/WidthPct/HeightPct/Min*/Max*/Pad/Margin/Gap/Grow/Basis/AlignSelf/Cols/ColW/
Bg/BgTo/BgAngle/Radius[4]/Border/BorderColor/ShadowBlur/ShadowY/ShadowSpread/ShadowColor/
FontSize/FontWeight/FontFamily/Color/LineHeight/TextAlign/CenterY/Wrap/Opacity/Clip/Scrollable/
PosAbs/AbsL/AbsT/AbsR/AbsB/AbsCenterX/AbsCenterY/Focusable/Animated/AnimSlow/BackdropBlur/BackdropTint/
CacheLayer/Scrollbar/NoRing/DrawX/DrawY`。
