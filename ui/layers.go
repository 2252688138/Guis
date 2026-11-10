// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"sync/atomic"

	"guis/internal/geom"
	"guis/internal/gpu"
)

// 组件栅格缓存（Style.CacheLayer）。
//
// 思路：某个子树的内容稳定时，第一次照常绘制，随后把这块屏幕区域「快照」进一张纹理；
// 下一页起只要内容指纹没变，就直接把这张纹理贴回去——整棵子树不再产生顶点、不再查字形、
// 不再走 SDF 着色，一次 DrawImage 搞定。典型的适用对象是：大卡片、图表、长文档块、
// 自绘的复杂控件（自带不透明背景、内容不随帧变化）。
//
// 为什么用「快照」而不是「画进离屏 FBO 再贴回」：
//   - 不需要改着色器、不用切换 FBO、不用给顶点加平移，绘制路径完全不变，风险最小；
//   - 与 DComp 合成目标天然兼容（glCopyTexSubImage2D 直接从当前绘制目标拷贝）；
//   - 代价是首次绘制后要等下一帧才可能命中，这对「滚动/切页后的重复绘制」毫无影响。
//
// 限制（也是必须遵守的语义）：缓存的是「屏幕像素」，因此
//   - 节点自带背景必须不透明或其后景稳定，否则半透明区域会把底层内容一并烘进缓存；
//   - 节点不能包含视频 / 网页（独立子窗口，无法拷进纹理）或持续动画；
//   - 只在节点完整落在窗口与当前裁剪区内时启用，避免把被裁掉的部分烘成空白。
//
// 内存：每张缓存 = 区域像素 × 4B（不含超采样，快照来自已合成的结果），
// 总预算由 layerBudget 控制并按最近使用淘汰；窗口最小化 / 长时间空闲时整块释放。

// layerEntry 一张组件缓存纹理。
type layerEntry struct {
	tex    *gpu.Texture
	fp     uint64
	w, h   int32
	radius [4]float32
	stamp  int64
}

func (e *layerEntry) bytes() int64 {
	if e == nil || e.tex == nil {
		return 0
	}
	return int64(e.tex.W) * int64(e.tex.H) * 4
}

// layerBudget 组件缓存的总字节预算（基准 16MB，按窗口面积自适应）。
func (a *App) layerBudget() int64 {
	if a == nil || a.r == nil {
		return 16 << 20
	}
	return a.r.LayerBudget()
}

// cachedLayer 命中缓存时返回条目，否则返回 nil（由调用方照常绘制子树）。
func (a *App) cachedLayer(n *Node, r geom.Rect) *layerEntry {
	if a.layers == nil || n == nil || n.layerKey == 0 {
		return nil
	}
	e := a.layers[n.layerKey]
	if e == nil || e.tex == nil {
		return nil
	}
	// 尺寸变了（窗口缩放 / 布局变化）→ 缓存作废，重新绘制并重拍
	if e.w != int32(r.W) || e.h != int32(r.H) {
		a.dropLayer(n.layerKey)
		return nil
	}
	if a.layerFingerprint(n) != e.fp {
		return nil
	}
	a.layerStamp++
	e.stamp = a.layerStamp
	return e
}

// layerShot 本帧需要快照的区域（绘制阶段收集，帧末统一捕获）。
// 指纹在绘制阶段算好：帧末再走一遍子树没有意义，而且那时节点树已可能被下一帧重建。
type layerShot struct {
	key    uint64
	rect   geom.Rect
	fp     uint64
	radius [4]float32
}

// captureLayers 把本帧收集到的 CacheLayer 区域拷进缓存纹理。
//
// 必须在渲染器 End() 之前调用：End 会 flush 并清空批次，之后再拷就拷不到本帧像素了。
func (a *App) captureLayers() {
	if len(a.layerPending) == 0 {
		return
	}
	if a.layers == nil {
		a.layers = map[uint64]*layerEntry{}
	}
	for _, s := range a.layerPending {
		a.storeLayer(s.key, s.rect, s.fp, s.radius)
	}
	a.layerPending = a.layerPending[:0]
}

// currentClip 当前生效的裁剪区（无裁剪时返回整个窗口）与其是否已处于裁剪中。
func (a *App) currentClip() (geom.Rect, bool) {
	if a.clipN > 0 {
		return a.clipStack[a.clipN-1], true
	}
	return geom.Rect{X: 0, Y: 0, W: a.winW, H: a.winH}, false
}

// storeLayer 把刚画完的区域快照进缓存（下一帧起可命中）。
func (a *App) storeLayer(key uint64, r geom.Rect, fp uint64, radius [4]float32) {
	if a.layers == nil || key == 0 || r.W < 4 || r.H < 4 {
		return
	}
	e := a.layers[key]
	if e == nil {
		// 单张缓存超过总预算一半就不缓存：一张几乎满屏的卡片会把其它卡片全挤掉，
		// 命中率反而下降，而且显存被单张占满。
		s := float64(1)
		if a.r != nil && a.r.Scale() > 0 {
			s = float64(a.r.Scale())
		}
		need := int64(float64(r.W)*s) * int64(float64(r.H)*s) * 4
		if need*2 > a.layerBudget() {
			return
		}
		e = &layerEntry{}
		a.layers[key] = e
	}
	if e.tex != nil && (e.w != int32(r.W) || e.h != int32(r.H)) {
		a.r.DeleteTexture(e.tex)
		e.tex = nil
	}
	if e.tex == nil {
		a.layerEvict(int64(r.W) * int64(r.H) * 4)
	}
	if !a.r.CaptureRGBA(&e.tex, r) {
		return
	}
	e.fp, e.w, e.h = fp, int32(r.W), int32(r.H)
	e.radius = radius
	a.layerStamp++
	e.stamp = a.layerStamp
}

// dropLayer 丢弃一条缓存。
func (a *App) dropLayer(key uint64) {
	if a.layers == nil {
		return
	}
	if e := a.layers[key]; e != nil {
		if e.tex != nil {
			a.r.DeleteTexture(e.tex)
		}
		delete(a.layers, key)
	}
}

// dropAllLayers 丢弃全部组件缓存（含纹理）。
//
// 整屏级变化（打开/关闭浮层、切换主题）后必须调用：缓存里存的是「当时那块屏幕像素」，
// 浮层曾经盖在上面时快照会被污染，贴回时就会看到浮层内容的残影。
func (a *App) dropAllLayers() {
	if a == nil || a.layers == nil {
		return
	}
	for k, e := range a.layers {
		if e.tex != nil {
			a.r.DeleteTexture(e.tex)
		}
		delete(a.layers, k)
	}
	a.layerOpen = false
	a.layerPending = a.layerPending[:0]
}

// DropCachedLayers 丢弃所有窗口的组件栅格缓存。
//
// 业务侧在「打开/关闭弹窗、切换主题、整页路由切换」这类会大范围改变画面的事件里调用，
// 可确保不会把旧快照贴回造成残影。必须在 UI 线程调用（会删 GL 纹理）。
func DropCachedLayers() {
	for _, a := range snapshotApps() {
		a.dropAllLayers()
	}
}

// layerEvict 腾出至少 need 字节（按最近使用淘汰）。
func (a *App) layerEvict(need int64) {
	if a.layers == nil {
		return
	}
	budget := a.layerBudget()
	total := int64(0)
	for _, e := range a.layers {
		total += e.bytes()
	}
	for total+need > budget && len(a.layers) > 0 {
		var victimKey uint64
		var victim *layerEntry
		for k, e := range a.layers {
			if victim == nil || e.stamp < victim.stamp {
				victimKey, victim = k, e
			}
		}
		if victim == nil {
			return
		}
		total -= victim.bytes()
		if victim.tex != nil {
			a.r.DeleteTexture(victim.tex)
		}
		delete(a.layers, victimKey)
	}
}

// layerReleaseAll 释放全部组件缓存（空闲 / 最小化 / 缩放变化时调用）。
func (a *App) layerReleaseAll() {
	if a == nil || a.layers == nil {
		return
	}
	for k, e := range a.layers {
		if e.tex != nil && a.r != nil {
			a.r.DeleteTexture(e.tex)
		}
		delete(a.layers, k)
	}
	a.layers = nil
}

// layerBytes 当前组件缓存占用（诊断用）。
func (a *App) layerBytes() int64 {
	n := int64(0)
	for _, e := range a.layers {
		n += e.bytes()
	}
	return n
}

// CachedLayer 把静态内容包一层「栅格缓存容器」。
//
// 首次绘制后整块区域被快照成一张纹理；内容（几何 / 文本 / 颜色 / 滚动偏移）不变时，
// 后续帧只用一次 DrawImage 贴回，整棵子树不再产生顶点、不再查字形、不再跑 SDF。
//
// 用法：
//
//	ui.CachedLayer("doc-body", ui.ColBg, ui.Column(...)) // 长文档块
//
// 不生效（自动回退普通绘制，不会画错）的情形：内容本身带描边/阴影/半透明底色；
// 子树内含交互节点（按钮/输入框）、动画、视频、网页、图表、图片；节点超出窗口或被裁剪。
// 开启 GUIS_FPS=1 可在日志里看到 layer=命中/未命中 计数，用来确认是否真的生效。
func CachedLayer(key string, bg any, child *Node) *Node {
	n := Box(child).K(key).St(St().CacheOn())
	if bg != nil {
		n.Style.Bg = C(bg)
		n.Style.BgSet = true
	}
	return n
}

// ---- 全局开关与逐帧候选标记 ----

// autoCacheLayers 进程级默认值：SetCacheLayers(true) 后，新建 / 现有窗口都按
// 「能缓则缓」处理（无需逐节点写 CacheOn）。
var autoCacheLayers atomic.Bool

// cacheLayers 当前窗口是否开启「能缓则缓」模式（SetCacheLayers(true)）。

// SetCacheLayers 开启/关闭自动组件缓存（进程级默认值，新建窗口生效）。
//
// 开启后无需逐个节点写 CacheOn()：每帧在布局完成后自底向上标记一次，
// 所有「纯静态 + 不透明底色 + 无描边/阴影 + 尺寸足够」的最外层容器都会成为缓存根。
// 子树内有交互控件（按钮/输入框）、动画、图片、图表、视频、网页的会被自动排除，
// 所以开启后不会画错，只是真能命中的节点多少因页面而异。
func SetCacheLayers(on bool) { autoCacheLayers.Store(on) }

// CacheLayersEnabled 当前是否开启了自动组件缓存。
func CacheLayersEnabled() bool { return autoCacheLayers.Load() }

// startLayerFrame 每帧（布局之后、绘制之前）打一次静态与缓存根标记。
//
// 只做一次后序遍历 + 一次前序遍历，无任何堆分配（标记直接写在 Node 上）。
// 不论是否开启自动缓存都会跑：显式 Style.CacheLayer 的节点也依赖这轮标记拿到
// 结构路径键（layerKey），否则没有稳定索引可用。
func (a *App) startLayerFrame(root *Node) {
	a.layerOpen = false
	a.cacheLayers = autoCacheLayers.Load()
	a.layerRoots = 0
	markStaticTree(root)
	a.markCacheRoots(root, 0xCBF29CE484222325, false)
}

// markStaticTree 自底向上标记「子树是否纯静态」，返回值供父节点使用。
//
// 「静态」只看会不会**每帧变化**：动画 / 毛玻璃 / 视频 / 网页 / 图表 / 图片（异步流式加载）
// 以及获得焦点的节点（焦点环画在内容之上）。
// 按钮、输入框这类交互控件**不再**排除：它们的外观变化（悬停 / 按下）会通过
// 内容指纹反映出来，命中时稳定、变化时只是多一次重拍，不会画错。
func markStaticTree(n *Node) bool {
	if n == nil {
		return true
	}
	ok := true
	switch n.Kind {
	case KVideo, KWebView, KChart, KImage, KRichText, KTerm:
		ok = false
	}
	if ok && (n.Style.Animated || n.Style.BackdropBlur > 0 || n.Focus) {
		ok = false
	}
	for _, c := range n.Children {
		// 不短路：所有子节点都必须被标记
		if !markStaticTree(c) {
			ok = false
		}
	}
	if ok {
		n.staticOK = 1
	} else {
		n.staticOK = 2
	}
	return ok
}

// markCacheRoots 前序遍历：给每个节点算结构路径键，并选出最外层的缓存根。
//
// 键 = 父键 + 子序号 的哈希：节点树每帧重建，但结构稳定时路径也稳定，
// 因此不需要业务写 Key（旧实现依赖 Key，导致 gallery 里绝大多数容器都启用不了）。
func (a *App) markCacheRoots(n *Node, parentKey uint64, inside bool) {
	if n == nil {
		return
	}
	n.layerKey = parentKey
	n.cacheRoot = false
	if !inside && a.layerRootEligible(n) {
		n.cacheRoot = true
		a.layerRoots++
	}
	for i, c := range n.Children {
		if c == nil {
			continue
		}
		a.markCacheRoots(c, fnvU64(parentKey, uint64(i)+1), inside || n.cacheRoot)
	}
}

// layerRootEligibleWin 自动模式下的额外条件：整窗大小的容器不做缓存。
//
// 内容一变就要拷整屏（1400×900×4B ≈ 5MB/次），得不偿失；
// 显式 Style.CacheLayer 不受此限（由开发者自己判断）。
func (a *App) layerRootEligible(n *Node) bool {
	if !layerRootEligible(n) {
		return false
	}
	if a.winW > 0 && a.winH > 0 && n.Frame.W*n.Frame.H > a.winW*a.winH*0.5 {
		return false
	}
	return true
}

// layerRootEligible 静态条件（不看窗口/裁剪，那些在绘制时才知道）。
//
// 最小 64×64：小块（图标/标签/扇页）缓存收益抵不过一次快照复制。
func layerRootEligible(n *Node) bool {
	st := &n.Style
	// 滚动容器不当缓存根：内容随偏移变化，滚动期间会每帧「失效→重拍」，
	// 反而比直绘更贵（它内部的子项仍可各自成为缓存根）。
	if n.Kind == KScroll {
		return false
	}
	return n.layerKey != 0 &&
		n.staticOK == 1 &&
		st.Bg.A >= 0.99 &&
		st.ShadowColor.A <= 0 &&
		n.Frame.W >= 64 && n.Frame.H >= 64
}

// CountCacheRoots 诊断用：对一棵已布局的节点树重新标记，返回「缓存根」的数量
// 与总面积（逻辑像素）。不依赖 GL，可在测试 / 无窗口环境下验证自动缓存到底能
// 命中多少节点；业务侧一般只需看 GUIS_FPS=1 日志里的 roots= 字段。
func CountCacheRoots(root *Node, w, h float32) (roots int, area float32) {
	if root == nil {
		return 0, 0
	}
	a := &App{winW: w, winH: h}
	markStaticTree(root)
	a.markCacheRoots(root, 0xCBF29CE484222325, false)
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.cacheRoot {
			roots++
			area += n.Frame.W * n.Frame.H
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return roots, area
}

// layerFingerprint 计算子树的内容指纹：几何 + 文本 + 外观 + 滚动偏移。
//
// 节点树每帧重建，因此「内容有没有变」只能靠指纹判断。指纹只覆盖影响像素的字段，
// 不覆盖回调函数（按钮的 OnClick 变化不影响画面，也不应让缓存失效）。
func (a *App) layerFingerprint(n *Node) uint64 {
	return a.fpNode(n, 0xF1E2D3C4B5A69788, 0)
}

// fpNode 递归累加指纹。深度上限用于防御异常深的树（正常 UI 远低于此）。
func (a *App) fpNode(n *Node, h uint64, depth int) uint64 {
	if n == nil || depth > 64 {
		return h
	}
	h = fnvU64(h, uint64(n.Kind))
	h = fnvStr(h, n.Key)
	h = fnvStr(h, n.Text)
	h = fnvStr(h, n.Src)
	// 几何：量化到 1/16 逻辑像素，避免浮点抖动导致永不命中
	h = fnvU64(h, fpQ(n.Frame.X))
	h = fnvU64(h, fpQ(n.Frame.Y))
	h = fnvU64(h, fpQ(n.Frame.W))
	h = fnvU64(h, fpQ(n.Frame.H))
	st := &n.Style
	h = fnvU64(h, fpQ(st.Opacity))
	h = fnvU64(h, fpQ(st.DrawX))
	h = fnvU64(h, fpQ(st.DrawY))
	h = fnvPaint(h, st.Bg)
	h = fnvPaint(h, st.BgTo)
	h = fnvU64(h, fpQ(st.BgAngle))
	h = fnvPaint(h, st.Color)
	h = fnvPaint(h, st.BorderColor)
	h = fnvPaint(h, st.ShadowColor)
	h = fnvPaint(h, st.BackdropTint)
	h = fnvU64(h, fpQ(st.Border))
	h = fnvU64(h, fpQ(st.ShadowBlur))
	h = fnvU64(h, fpQ(st.ShadowSpread))
	h = fnvU64(h, fpQ(st.ShadowY))
	h = fnvU64(h, fpQ(st.FontSize))
	h = fnvU64(h, uint64(uint32(st.FontWeight)))
	h = fnvU64(h, fpQ(st.LineHeight))
	h = fnvStr(h, st.FontFamily)
	h = fnvU64(h, fpQ(st.BackdropBlur))
	for i := 0; i < 4; i++ {
		h = fnvU64(h, fpQ(st.Radius[i]))
	}
	// 渲染开关（True 值本身就影响像素）
	flags := uint64(0)
	if st.Wrap {
		flags |= 1 << 0
	}
	if st.CenterY {
		flags |= 1 << 1
	}
	if st.TextAlign == AlignCenter {
		flags |= 1 << 2
	}
	if st.TextAlign == AlignEnd {
		flags |= 1 << 3
	}
	if n.Pressed {
		flags |= 1 << 4
	}
	h = fnvU64(h, flags)
	// 滚动容器的偏移：内容随偏移变化，必须进指纹
	if n.Kind == KScroll && a != nil {
		h = fnvU64(h, fpQ(a.scrolls[n.Key]))
	}
	for _, c := range n.Children {
		h = a.fpNode(c, h, depth+1)
	}
	return h
}

// fpQ 把浮点量化成整数参与哈希（1/16 像素精度）。
func fpQ(v float32) uint64 {
	if v != v { // NaN 防御
		return 0
	}
	return uint64(int64(math.Round(float64(v) * 16)))
}

func fnvPaint(h uint64, p gpu.Paint) uint64 {
	h = fnvU64(h, fpQ(p.R))
	h = fnvU64(h, fpQ(p.G))
	h = fnvU64(h, fpQ(p.B))
	h = fnvU64(h, fpQ(p.A))
	return h
}

// fnvU64 64 位 FNV-1a 混合。
func fnvU64(h, v uint64) uint64 {
	h ^= v
	h *= 1099511628211
	return h
}

// fnvStr 把字符串混进指纹（逐字节，避免额外分配）。
func fnvStr(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h ^ 0x9E3779B97F4A7C15
}

// layerCacheable 判断节点是否适合做栅格缓存。
//
// 硬性要求（缺一不可）：
//   - **自带不透明背景**：缓存的是屏幕像素，贴回时是整块覆盖；
//     半透明区域会被重复混合，所以底色必须不透明；
//   - 无描边 / 阴影（会画出矩形之外，快照盖不住）；圆角可以，因为贴回时
//     用 img 着色器的 SDF 圆角遮罩，圆角外的旧像素会被直接丢弃；
//   - 不含动画 / 毛玻璃 / 视频 / 网页 / 图表 / 图片 / 焦点节点；
//   - 完整落在窗口与当前裁剪区内（否则被裁掉的部分会烘成空白）。
func (a *App) layerCacheable(n *Node, r geom.Rect, clipped bool, cr geom.Rect) bool {
	if n == nil || n.Key == "" || r.W < 8 || r.H < 8 {
		return false
	}
	st := &n.Style
	if st.Animated || st.BackdropBlur > 0 {
		return false
	}
	if st.Bg.A < 0.99 {
		return false // 必须有铺满的不透明底色，否则会重复混合
	}
	if st.ShadowColor.A > 0 {
		// 阴影会画出矩形边界之外，快照区域盖不住（描边是向内画的，可以缓存）
		return false
	}
	if !nodeCacheableTree(n) {
		return false
	}
	if a.winW > 0 && (r.X < 0 || r.Y < 0 || r.Right() > a.winW+0.5 || r.Bottom() > a.winH+0.5) {
		return false
	}
	if clipped && !(r.X >= cr.X-0.5 && r.Y >= cr.Y-0.5 &&
		r.Right() <= cr.Right()+0.5 && r.Bottom() <= cr.Bottom()+0.5) {
		return false
	}
	return true
}

// nodeCacheableTree 子树里不能有每帧变化的节点，也不能有获得焦点的节点。
//
// 交互控件（按钮 / 输入框）允许存在：它们的外观变化会改变指纹，
// 后果只是「变化的那一帧多一次重拍」，不会出现陈旧画面。
func nodeCacheableTree(n *Node) bool {
	if n == nil {
		return true
	}
	switch n.Kind {
	case KVideo, KWebView, KChart, KImage, KRichText, KTerm:
		return false
	}
	if n.Style.Animated || n.Style.BackdropBlur > 0 {
		return false
	}
	if n.Focus {
		return false
	}
	for _, c := range n.Children {
		if !nodeCacheableTree(c) {
			return false
		}
	}
	return true
}
