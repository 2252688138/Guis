// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// AssetLoader 资源字节加载器（通常是 go:embed 或目录读取）。
type AssetLoader func(name string) ([]byte, error)

// Assets 资源缓存（纹理 / 网格 / 音效，按名命中不重复加载）。
type Assets struct {
	load   AssetLoader
	mu     sync.Mutex
	tex    map[string]*Texture
	meshes map[string]*Mesh
	sounds map[string]*Sound
}

// NewAssets 创建资源表（loader 为 nil 时所有加载失败）。
func NewAssets(load AssetLoader) *Assets {
	return &Assets{
		load:   load,
		tex:    map[string]*Texture{},
		meshes: map[string]*Mesh{},
		sounds: map[string]*Sound{},
	}
}

// NewDirAssets 从目录加载资源（name 为相对目录的文件名）。
func NewDirAssets(dir string) *Assets {
	return NewAssets(func(name string) ([]byte, error) {
		return osReadFile(filepath.Join(dir, name))
	})
}

func (a *Assets) bytes(name string) ([]byte, error) {
	if a == nil || a.load == nil {
		return nil, fmt.Errorf("game: 资源未配置 loader: %s", name)
	}
	return a.load(name)
}

// Texture 取纹理（首次加载后缓存）。
func (a *Assets) Texture(name string) (*Texture, error) {
	if a == nil {
		return nil, fmt.Errorf("game: Assets 为空")
	}
	a.mu.Lock()
	if t, ok := a.tex[name]; ok {
		a.mu.Unlock()
		return t, nil
	}
	a.mu.Unlock()
	b, err := a.bytes(name)
	if err != nil {
		return nil, err
	}
	t, err := LoadTextureBytes(b)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.tex[name] = t
	a.mu.Unlock()
	return t, nil
}

// Mesh 取网格（.obj）。
func (a *Assets) Mesh(name string) (*Mesh, error) {
	if a == nil {
		return nil, fmt.Errorf("game: Assets 为空")
	}
	a.mu.Lock()
	if m, ok := a.meshes[name]; ok {
		a.mu.Unlock()
		return m, nil
	}
	a.mu.Unlock()
	b, err := a.bytes(name)
	if err != nil {
		return nil, err
	}
	m, err := LoadOBJBytes(b)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.meshes[name] = m
	a.mu.Unlock()
	return m, nil
}

// Sound 取音效（.wav）。
func (a *Assets) Sound(name string) (*Sound, error) {
	if a == nil {
		return nil, fmt.Errorf("game: Assets 为空")
	}
	a.mu.Lock()
	if s, ok := a.sounds[name]; ok {
		a.mu.Unlock()
		return s, nil
	}
	a.mu.Unlock()
	b, err := a.bytes(name)
	if err != nil {
		return nil, err
	}
	s, err := LoadWAVBytes(b)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.sounds[name] = s
	a.mu.Unlock()
	return s, nil
}

// loadAny 按扩展名分派加载。
func (a *Assets) loadAny(name string) error {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".obj":
		_, err := a.Mesh(name)
		return err
	case ".wav":
		_, err := a.Sound(name)
		return err
	default:
		_, err := a.Texture(name)
		return err
	}
}

// Preload 同步预加载一组资源（遇错立即返回）。
func (a *Assets) Preload(names ...string) error {
	for _, n := range names {
		if err := a.loadAny(n); err != nil {
			return fmt.Errorf("game: 预加载 %s 失败: %w", n, err)
		}
	}
	return nil
}

// LoadAsync 后台加载，边加载边通过 progress 回调（在后台 goroutine 中调用），
// 全部结束后向返回的 channel 发一条结果（nil 表示成功）。
func (a *Assets) LoadAsync(names []string, progress func(done, total int)) <-chan error {
	ch := make(chan error, 1)
	go func() {
		total := len(names)
		if progress != nil {
			progress(0, total)
		}
		for i, n := range names {
			if err := a.loadAny(n); err != nil {
				ch <- err
				return
			}
			if progress != nil {
				progress(i+1, total)
			}
		}
		ch <- nil
	}()
	return ch
}

// Clear 清空缓存。
func (a *Assets) Clear() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.tex = map[string]*Texture{}
	a.meshes = map[string]*Mesh{}
	a.sounds = map[string]*Sound{}
	a.mu.Unlock()
}

// Cached 已缓存的资源数量。
func (a *Assets) Cached() (textures, meshes, sounds int) {
	if a == nil {
		return 0, 0, 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.tex), len(a.meshes), len(a.sounds)
}
