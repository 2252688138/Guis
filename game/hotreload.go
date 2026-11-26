// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"os"
	"time"
)

// 资源热重载。
//
// 改完贴图 / 模型 / 关卡 JSON 之后不必重启游戏：把文件登记进来，主循环每帧调
// Tick，文件修改时间一变就执行注册的回调（回调里重新 Load 并替换引用）。
// 纯轮询实现，不依赖 fsnotify，能跑在所有平台。
//
//	hr := game.NewHotReloader()
//	hr.WatchScene("level1.json", resolveMesh, func(sc *game.Scene) {
//	    eng.Scene = sc                 // 直接换掉整张场景
//	})
//	// 每帧：
//	hr.Tick(dt)

// hotEntry 一个被监视的文件。
type hotEntry struct {
	path string
	fn   func(path string) error
	mod  time.Time
	size int64
}

// HotReloader 文件热重载监视器。
type HotReloader struct {
	// Interval 最短轮询间隔（默认 250ms）。
	Interval time.Duration
	// OnError 回调失败时的钩子（默认忽略，只记录 LastError）。
	OnError func(path string, err error)
	// LastError 最近一次失败（成功后清空）。
	LastError error
	// Reloads 累计成功重载次数。
	Reloads int

	entries []hotEntry
	last    time.Time
}

// NewHotReloader 创建监视器。
func NewHotReloader() *HotReloader {
	return &HotReloader{Interval: 250 * time.Millisecond}
}

// Watch 登记一个文件；文件变化时同步执行 fn（在 Tick 的调用线程上）。
// 返回自身以便链式调用。重复登记同一路径会替换回调。
func (h *HotReloader) Watch(path string, fn func(path string) error) *HotReloader {
	if h == nil || path == "" || fn == nil {
		return h
	}
	e := hotEntry{path: path, fn: fn}
	e.mod, e.size = fileStamp(path)
	for i := range h.entries {
		if h.entries[i].path == path {
			h.entries[i] = e
			return h
		}
	}
	h.entries = append(h.entries, e)
	return h
}

// Unwatch 取消监视。
func (h *HotReloader) Unwatch(path string) {
	if h == nil {
		return
	}
	for i := range h.entries {
		if h.entries[i].path == path {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			return
		}
	}
}

// Paths 当前监视的文件列表。
func (h *HotReloader) Paths() []string {
	if h == nil {
		return nil
	}
	out := make([]string, 0, len(h.entries))
	for _, e := range h.entries {
		out = append(out, e.path)
	}
	return out
}

// Tick 用帧间隔推进（未到轮询间隔直接返回 0）。
func (h *HotReloader) Tick(dt float32) int {
	if h == nil {
		return 0
	}
	iv := h.Interval
	if iv <= 0 {
		iv = 250 * time.Millisecond
	}
	now := time.Now()
	if !h.last.IsZero() && now.Sub(h.last) < iv {
		return 0
	}
	h.last = now
	return h.Poll(now)
}

// Poll 立即检查一遍，返回本次成功重载的文件数。
func (h *HotReloader) Poll(now time.Time) int {
	if h == nil || len(h.entries) == 0 {
		return 0
	}
	n := 0
	for i := range h.entries {
		e := &h.entries[i]
		mod, size := fileStamp(e.path)
		if mod.IsZero() || (mod.Equal(e.mod) && size == e.size) {
			continue
		}
		e.mod, e.size = mod, size
		if err := e.fn(e.path); err != nil {
			h.LastError = err
			if h.OnError != nil {
				h.OnError(e.path, err)
			}
			continue
		}
		h.LastError = nil
		h.Reloads++
		n++
	}
	return n
}

// Reload 强制重载指定文件（忽略修改时间）。
func (h *HotReloader) Reload(path string) error {
	if h == nil {
		return nil
	}
	for i := range h.entries {
		if h.entries[i].path != path {
			continue
		}
		e := &h.entries[i]
		err := e.fn(e.path)
		e.mod, e.size = fileStamp(e.path)
		if err != nil {
			h.LastError = err
			return err
		}
		h.LastError = nil
		h.Reloads++
		return nil
	}
	return nil
}

// WatchScene 监视场景 JSON，加载成功后把新场景交给 onLoad（热重载关卡）。
func (h *HotReloader) WatchScene(path string, resolve func(string) *Mesh, onLoad func(sc *Scene)) *HotReloader {
	return h.Watch(path, func(p string) error {
		sc, err := LoadScene(p, resolve)
		if err != nil {
			return err
		}
		if onLoad != nil {
			onLoad(sc)
		}
		return nil
	})
}

// WatchMesh 监视 OBJ 模型，加载成功后把新网格交给 onLoad。
func (h *HotReloader) WatchMesh(path string, onLoad func(m *Mesh)) *HotReloader {
	return h.Watch(path, func(p string) error {
		m, err := LoadOBJ(p)
		if err != nil {
			return err
		}
		if onLoad != nil {
			onLoad(m)
		}
		return nil
	})
}

// WatchTexture 监视图片纹理，加载成功后把新纹理交给 onLoad。
func (h *HotReloader) WatchTexture(path string, onLoad func(t *Texture)) *HotReloader {
	return h.Watch(path, func(p string) error {
		t, err := LoadTexture(p)
		if err != nil {
			return err
		}
		if onLoad != nil {
			onLoad(t)
		}
		return nil
	})
}

// WatchTiled 监视 Tiled 关卡，加载成功后把新地图交给 onLoad。
func (h *HotReloader) WatchTiled(path string, onLoad func(m *TiledMap)) *HotReloader {
	return h.Watch(path, func(p string) error {
		m, err := LoadTiledJSON(p)
		if err != nil {
			return err
		}
		if onLoad != nil {
			onLoad(m)
		}
		return nil
	})
}

// fileStamp 读取文件的修改时间与大小；文件不存在时返回零值。
func fileStamp(path string) (time.Time, int64) {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0
	}
	return st.ModTime(), st.Size()
}
