// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"fmt"
	"os"
)

// skinFile 自研 .skin 文件格式（JSON）。
//
// 设计目标：一个文件同时装下网格（顶点 / 索引）、骨架、蒙皮权重与一段动画，
// 方便导出器与引擎直接对接；不追求紧凑，需要时可在外层压缩。
type skinFile struct {
	Version   int          `json:"version"`
	Bones     []Bone       `json:"bones"`
	Positions []Vec3       `json:"positions"`
	Indices   []uint32     `json:"indices"`
	Skin      []SkinVertex `json:"skin"`
	Clip      *SkinClip    `json:"clip,omitempty"`
}

// LoadSkin 从 .skin 文件加载蒙皮网格与动画（无 clip 时返回 nil）。
func LoadSkin(path string) (*SkinnedMesh, *SkinClip, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return LoadSkinBytes(b)
}

// LoadSkinBytes 从字节加载（便于内嵌资源）。
func LoadSkinBytes(b []byte) (*SkinnedMesh, *SkinClip, error) {
	var f skinFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, nil, fmt.Errorf("解析 .skin 失败: %w", err)
	}
	if f.Version <= 0 {
		f.Version = 1
	}
	if len(f.Bones) == 0 {
		return nil, nil, fmt.Errorf(".skin 缺少 bones")
	}
	mesh := NewMesh(f.Positions, f.Indices)
	sk := &Skeleton{Bones: f.Bones}
	for i := range sk.Bones {
		if sk.Bones[i].Scl.X == 0 && sk.Bones[i].Scl.Y == 0 && sk.Bones[i].Scl.Z == 0 {
			sk.Bones[i].Scl = Vec3{X: 1, Y: 1, Z: 1}
		}
	}
	if bad := sk.Validate(); bad >= 0 {
		return nil, nil, fmt.Errorf(".skin 骨骼 %d 的父索引非法或拓扑无序", bad)
	}
	sm := NewSkinnedMesh(mesh, sk, f.Skin)
	return sm, f.Clip, nil
}

// SaveSkin 把蒙皮网格（可选动画）写成 .skin 文件。
func SaveSkin(path string, sm *SkinnedMesh, clip *SkinClip) error {
	b, err := MarshalSkin(sm, clip)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// MarshalSkin 序列化为 .skin 字节。
func MarshalSkin(sm *SkinnedMesh, clip *SkinClip) ([]byte, error) {
	if sm == nil || sm.Mesh == nil || sm.Skeleton == nil {
		return nil, fmt.Errorf("蒙皮网格为空")
	}
	f := skinFile{
		Version:   1,
		Bones:     sm.Skeleton.Bones,
		Positions: sm.Mesh.Positions,
		Indices:   sm.Mesh.Indices,
		Skin:      sm.Skin,
		Clip:      clip,
	}
	return json.MarshalIndent(&f, "", "  ")
}
