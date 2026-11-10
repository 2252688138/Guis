// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"guis/internal/gpu"
)

// errBadDataURL data URL 格式不合法。
var errBadDataURL = errors.New("ui: 非法的 data URL")

// imageReader 打开图片数据源：支持本地路径与 data: URL（Web 端选文件后
// 图片以 data URL 形式返回，桌面端永远是路径）。blob: 需要异步 fetch，不支持。
func imageReader(path string) (io.ReadCloser, error) {
	if !strings.HasPrefix(path, "data:") {
		return os.Open(path)
	}
	i := strings.Index(path, ",")
	if i < 0 {
		return nil, errBadDataURL
	}
	meta, body := path[:i], path[i+1:]
	if strings.Contains(meta, ";base64") {
		data, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	s, err := url.PathUnescape(body)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(s)), nil
}

// imageMeta 图片尺寸缓存（布局阶段只需尺寸，不加载像素）。
type imageMeta struct {
	w, h int
	err  error
}

var (
	imageMetaMu sync.RWMutex
	imageMetas  = map[string]imageMeta{}
)

// imageMetaMax 尺寸缓存条数上限：长列表/相册滚动时会遇到成千上万个路径，
// 无上限的 map 会把每个路径字符串带进常驻内存（缓存本身比数据还大）。
const imageMetaMax = 4096

// ImageSize 返回图片像素尺寸（带缓存，失败返回 0,0）。
//
// 布局阶段每个图片节点每帧都会调一次：命中路径走读锁，
// 且解码不再在全局锁内进行（旧实现在持锁时 DecodeConfig 会阻塞其它窗口）。
func ImageSize(path string) (int, int) {
	imageMetaMu.RLock()
	m, ok := imageMetas[path]
	imageMetaMu.RUnlock()
	if ok {
		return m.w, m.h
	}
	m = imageMeta{}
	f, err := imageReader(path)
	if err != nil {
		m.err = err
	} else {
		cfg, _, derr := image.DecodeConfig(f)
		f.Close()
		if derr != nil {
			m.err = derr
		} else {
			m.w, m.h = cfg.Width, cfg.Height
		}
	}
	imageMetaMu.Lock()
	if len(imageMetas) >= imageMetaMax {
		imageMetas = map[string]imageMeta{}
	}
	imageMetas[path] = m
	imageMetaMu.Unlock()
	return m.w, m.h
}

// DecodeImage 解码图片为 NRGBA（非预乘，可直接上传 GL）。
func DecodeImage(path string) (*image.NRGBA, error) {
	f, err := imageReader(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	if n, ok := src.(*image.NRGBA); ok {
		return n, nil
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst, nil
}

// texUpload 后台解码结果（由 UI 线程完成 GL 上传，纹理只能在该线程创建）。
type texUpload struct {
	img *image.NRGBA
	err error
}

// texEntry 一张图片的纹理状态。
type texEntry struct {
	tex     *gpu.Texture
	bytes   int64
	stamp   int64 // 最近一次被绘制的帧号（LRU 回收用）
	err     error
	failed  bool
	failSz  int64 // 失败时的文件大小 / 修改时间：文件变更后允许重试
	failMod int64
	started atomic.Bool
	ready   atomic.Pointer[texUpload]
}

// imageTextures 每个窗口一份纹理缓存（GL 资源随窗口销毁）。
type imageTextures struct {
	mu sync.Mutex
	m  map[string]*texEntry
	// bytes 当前缓存纹理的总字节数（用于按预算回收，避免长列表图片页显存只增不减）
	bytes int64
	// waiting 限流而被推迟的解码请求数（>0 时需继续刷帧重试）
	waiting atomic.Int32
}

// imageTexBudget 旧固定预算（已改为按窗口面积自适应的 renderer.ImageTexBudget，
// 这里保留常量供文档与旧调用点对照）。
// 单窗口图片纹理预算：超出后按「最近未使用」回收纹理。
// 旧实现只增不减，翻完几百张图的相册/长列表后显存与堆都不会回落。
// 96MB 的预算对「一屏几张缩略图」的常规界面而言过于宽松，
// 现在收紧到 32MB：缩略图约可同时驻留 20~40 张，超过后按 LRU 回收、
// 再次进入视口时自动重新解码（数据层有序，观感无差异）。
const imageTexBudget int64 = 32 << 20

func newImageTextures() *imageTextures { return &imageTextures{m: map[string]*texEntry{}} }

// count 已缓存的图片条目数。
func (t *imageTextures) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.m)
}

// maxImageLoads 同时解码的图片数量上限（一下子滚到很多图片时避免 CPU 峰值）。
const maxImageLoads = 4

var imageLoads atomic.Int32

// imagePending 仍在后台解码/待上传的图片数量（>0 时需继续刷帧推进）。
func imagePending() int { return int(imageLoads.Load()) }

// get 取纹理：首次访问返回 nil（本帧不画），解码完成后请求重绘再上传。
// 以前是在绘制路径里同步解码 + 上传，遇到大图或一次出现多张图时会让 UI 线程
// 停顿几十上百毫秒，表现出来就是滚动/翻页时的卡顿。
func (t *imageTextures) get(r *gpu.Renderer, a *App, path string) *gpu.Texture {
	e := t.entry(path)
	e.stamp = int64(a.frameCount)
	if e.tex != nil {
		return e.tex
	}
	if up := e.ready.Swap(nil); up != nil {
		if up.err != nil || up.img == nil {
			e.err, e.failed = up.err, true
			e.failSz, e.failMod = fileStamp(path)
			return nil
		}
		b := up.img.Bounds()
		e.tex = r.NewTextureRGBA(b.Dx(), b.Dy(), up.img.Pix)
		if e.tex != nil {
			e.bytes = int64(b.Dx()) * int64(b.Dy()) * 4
			t.mu.Lock()
			t.bytes += e.bytes
			t.mu.Unlock()
			t.evict(r, e.stamp)
		}
		return e.tex
	}
	if e.failed {
		// 失败可能只是「当时文件还没写完」（例如皮肤背景图正在后台生成、
		// 图片刚下载到磁盘）。文件大小 / 修改时间变了就允许重试一次，
		// 否则该路径会永久失败，界面只剩纯色底，必须重启才恢复。
		if sz, mod := fileStamp(path); sz != e.failSz || mod != e.failMod {
			e.failed, e.err = false, nil
			e.failSz, e.failMod = 0, 0
		} else {
			return nil
		}
	}
	t.start(e, a, path)
	return nil
}

// fileStamp 返回文件大小与修改时间（失败纹理的重试判断用；读不到返回 0,0）。
func fileStamp(path string) (int64, int64) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, 0
	}
	return st.Size(), st.ModTime().UnixNano()
}

// evict 显存超预算时按最近未使用回收纹理。
// keep 为本帧帧号：本帧正在绘制的纹理即使很旧也不能删，
// 否则已经进批次但尚未提交的顶点会引用已删除的纹理。
func (t *imageTextures) evict(r *gpu.Renderer, keep int64) {
	t.mu.Lock()
	if t.bytes <= r.ImageTexBudget() {
		t.mu.Unlock()
		return
	}
	var victimKey string
	var victim *texEntry
	for k, e := range t.m {
		if e.tex == nil || e.stamp == keep {
			continue
		}
		if victim == nil || e.stamp < victim.stamp {
			victimKey, victim = k, e
		}
	}
	if victim == nil {
		t.mu.Unlock()
		return
	}
	t.bytes -= victim.bytes
	delete(t.m, victimKey)
	t.mu.Unlock()

	r.DeleteTexture(victim.tex)
	victim.tex = nil
	victim.bytes = 0
	victim.started.Store(false) // 再次进入视口时重新解码
}

// beginFrame 每帧开始前重置限流标记。
// 旧实现里 waiting 一旦被置 1 就再也没人清 0（图片滚出视口后不会再有 start 调用），
// frame() 看到 waiting>0 会持续请求重绘 —— 表现为滚过一批图片后窗口永久 100% CPU。
func (t *imageTextures) beginFrame() { t.waiting.Store(0) }

// releaseAll 释放全部图片纹理（窗口不可见时调用），保留条目以便重新解码。
// 条目里的 started 要复位，否则下次回到该页面时不会重新拉起解码。
func (t *imageTextures) releaseAll(r *gpu.Renderer) {
	t.releaseWhere(r, func(*texEntry) bool { return true })
}

// releaseIdle 浅空闲回收：只释放「最后一帧没画到」的图片纹理。
//
// 与 releaseAll 的差别很关键：上一帧确实画在屏幕上的图（皮肤壁纸、头像、
// 缩略图）必须留下纹理。否则空闲 3 秒后用户刚一动鼠标，这些图就被当成
// 「首次访问」重新丢进后台解码，解码完成前的那几帧没有纹理可画——整窗
// 缺了壁纸，看起来就是闪一下。视口外（翻走的长列表、换页后的封面）的
// 纹理没有保留价值，照常释放。
//
// keepStamp 传「最后一帧的帧号」（App.frameCount-1）：stamp 比它小的条目
// 说明最后一帧没有画到。
func (t *imageTextures) releaseIdle(r *gpu.Renderer, keepStamp int64) {
	t.releaseWhere(r, func(e *texEntry) bool { return e.stamp < keepStamp })
}

// releaseWhere 释放满足条件的纹理（drop 返回 true 的条目）。
func (t *imageTextures) releaseWhere(r *gpu.Renderer, drop func(*texEntry) bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	victims := make([]*texEntry, 0, len(t.m))
	for _, e := range t.m {
		if !drop(e) {
			continue
		}
		// 记账无论有没有纹理都要清（旧实现 releaseAll 直接把总字节置 0），
		// 否则“有记账没纹理”的条目会让预算永久偏低。
		t.bytes -= e.bytes
		e.bytes = 0
		if e.tex != nil {
			victims = append(victims, e)
		}
	}
	if t.bytes < 0 {
		t.bytes = 0
	}
	t.mu.Unlock()
	for _, e := range victims {
		if r != nil {
			r.DeleteTexture(e.tex)
		}
		e.tex, e.bytes = nil, 0
		e.started.Store(false)
	}
}

func (t *imageTextures) entry(path string) *texEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[path]
	if !ok {
		e = &texEntry{}
		t.m[path] = e
	}
	return e
}

// start 启动一次后台解码（已有结果、失败或进行中则跳过）。
func (t *imageTextures) start(e *texEntry, a *App, path string) {
	if !e.started.CompareAndSwap(false, true) {
		return
	}
	if imageLoads.Add(1) > maxImageLoads {
		imageLoads.Add(-1)
		e.started.Store(false) // 稍后的帧再试
		t.waiting.Store(1)
		return
	}
	t.waiting.Store(0)
	go func() {
		defer recoverReport("image-decode")
		defer imageLoads.Add(-1)
		img, err := DecodeImage(path)
		e.ready.Store(&texUpload{img: img, err: err})
		RequestRedrawIn(a)
	}()
}
