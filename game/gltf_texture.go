// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg" // 注册 JPEG 解码器（glTF 允许 JPEG）
	_ "image/png"  // 注册 PNG 解码器
	"os"
	"path/filepath"
	"strings"
)

// glTF 采样器过滤器取值（规范 5.26 / 5.27）。
const (
	gltfFilterNearest       = 9728
	gltfFilterLinear        = 9729
	gltfFilterNearestMipmap = 9984
	gltfFilterLinearMipNear = 9985
	gltfFilterNearestMipLin = 9986
	gltfFilterLinearMipLin  = 9987
)

// texture 按 glTF texture 索引解码贴图。
//
// 同一张图被多个材质引用时只解码一次（结果缓存在 loader 上）。
// 解码失败（缺图 / 坏数据 / 不支持的格式）返回 nil —— 当作没有贴图，
// 不让整个模型导入失败。
func (l *gltfLoader) texture(idx int) *Texture {
	if l == nil || idx < 0 || idx >= len(l.root.Textures) {
		return nil
	}
	if l.texCache == nil {
		l.texCache = make(map[int]*Texture, len(l.root.Textures))
	}
	if t, ok := l.texCache[idx]; ok {
		return t
	}
	l.texCache[idx] = nil // 占位：坏数据或自引用时不会重复解码
	src := l.root.Textures[idx].Source
	if src == nil {
		return nil
	}
	raw := l.imageBytes(*src)
	if len(raw) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	tex := TextureFromImage(img)
	if tex == nil {
		return nil
	}
	if _, mip := l.samplerModes(idx); mip {
		tex.BuildMips() // 只有需要三线性过滤才构建，省内存与启动时间
	}
	l.texCache[idx] = tex
	return tex
}

// samplerModes 由 texture 的 sampler 推出过滤方式。
//
// nearest = 放大用最近邻；mip = 缩小需要 mipmap 链。
func (l *gltfLoader) samplerModes(texIdx int) (nearest, mip bool) {
	if l == nil || texIdx < 0 || texIdx >= len(l.root.Textures) {
		return false, false
	}
	s := l.root.Textures[texIdx].Sampler
	if s == nil || *s < 0 || *s >= len(l.root.Samplers) {
		return false, false
	}
	sp := l.root.Samplers[*s]
	if sp.MagFilter != nil && *sp.MagFilter == gltfFilterNearest {
		nearest = true
	}
	if sp.MinFilter != nil {
		switch *sp.MinFilter {
		case gltfFilterNearestMipmap, gltfFilterLinearMipNear, gltfFilterNearestMipLin, gltfFilterLinearMipLin:
			mip = true
		}
	}
	return nearest, mip
}

// imageBytes 取 image 的原始字节：支持 data URI、外部文件、GLB 内嵌 bufferView。
func (l *gltfLoader) imageBytes(idx int) []byte {
	if l == nil || idx < 0 || idx >= len(l.root.Images) {
		return nil
	}
	im := l.root.Images[idx]
	switch {
	case strings.HasPrefix(im.URI, "data:"):
		i := strings.IndexByte(im.URI, ',')
		if i < 0 {
			return nil
		}
		raw, err := base64.StdEncoding.DecodeString(im.URI[i+1:])
		if err != nil {
			return nil
		}
		return raw
	case im.URI != "":
		if l.dir == "" {
			return nil // 内存字节流导入时不允许读外部文件
		}
		raw, err := os.ReadFile(filepath.Join(l.dir, im.URI))
		if err != nil {
			return nil
		}
		return raw
	case im.BufferView != nil:
		bvIdx := *im.BufferView
		if bvIdx < 0 || bvIdx >= len(l.root.BufferViews) {
			return nil
		}
		bv := l.root.BufferViews[bvIdx]
		if bv.Buffer < 0 || bv.Buffer >= len(l.bufs) {
			return nil
		}
		buf := l.bufs[bv.Buffer]
		start := bv.ByteOffset
		end := start + bv.ByteLength
		if start < 0 || start > end || end > len(buf) {
			return nil
		}
		return buf[start:end]
	}
	return nil
}
