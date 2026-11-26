// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"encoding/json"
	"os"
)

// sceneVersion 场景文档版本号。
const sceneVersion = 1

type nodeDoc struct {
	Name          string
	Mesh          string
	Color         RGBA
	Shade         int
	Specular      float32
	Shininess     float32
	Pos, Rot, Scl [3]float32
	Visible       bool
	Children      []nodeDoc
}

type lightDoc struct {
	Kind      int
	Dir, Pos  [3]float32
	Color     RGBA
	Intensity float32
	Range     float32
}

type camDoc struct {
	Pos, Target, Up [3]float32
	FOVY, Near, Far float32
	Ortho           bool
	OrthoSize       float32
}

type sceneDoc struct {
	Version    int
	Ambient    RGBA
	Background RGBA
	Fog        Fog
	Camera     *camDoc
	Lights     []lightDoc
	Nodes      []nodeDoc
}

func v3doc(v Vec3) [3]float32 { return [3]float32{v.X, v.Y, v.Z} }

func docV3(a [3]float32) Vec3 { return Vec3{X: a[0], Y: a[1], Z: a[2]} }

func nodeToDoc(n *Node, meshName func(*Mesh) string) nodeDoc {
	d := nodeDoc{Name: n.Name, Pos: v3doc(n.Pos), Rot: v3doc(n.Rot), Scl: v3doc(n.Scl), Visible: n.Visible}
	if n.Material != nil {
		d.Color = n.Material.BaseColor
		d.Shade = int(n.Material.Shade)
		d.Specular = n.Material.Specular
		d.Shininess = n.Material.Shininess
	}
	if n.Mesh != nil && meshName != nil {
		d.Mesh = meshName(n.Mesh)
	}
	for _, c := range n.Children {
		if c != nil {
			d.Children = append(d.Children, nodeToDoc(c, meshName))
		}
	}
	return d
}

func docToNode(d nodeDoc, resolve func(string) *Mesh) *Node {
	n := NewNode(d.Name)
	n.Pos = docV3(d.Pos)
	n.Rot = docV3(d.Rot)
	n.Scl = docV3(d.Scl)
	n.Visible = d.Visible
	if d.Scl == [3]float32{} {
		n.Scl = V3(1, 1, 1)
	}
	if resolve != nil && d.Mesh != "" {
		n.Mesh = resolve(d.Mesh)
	}
	if d.Color.A != 0 {
		m := DefaultMaterial().WithColor(d.Color).WithShade(ShadeModel(d.Shade))
		m.Specular = d.Specular
		m.Shininess = d.Shininess
		n.Material = m
	}
	for _, c := range d.Children {
		n.Children = append(n.Children, docToNode(c, resolve))
	}
	return n
}

// SceneJSON 把场景序列化为 JSON（几何体不写入，靠 meshName/resolve 对应）。
func SceneJSON(sc *Scene, meshName func(*Mesh) string) ([]byte, error) {
	if sc == nil {
		return nil, os.ErrInvalid
	}
	doc := sceneDoc{Version: sceneVersion, Ambient: sc.Ambient, Background: sc.Background, Fog: sc.Fog}
	if sc.Camera != nil {
		doc.Camera = &camDoc{
			Pos: v3doc(sc.Camera.Pos), Target: v3doc(sc.Camera.Target), Up: v3doc(sc.Camera.Up),
			FOVY: sc.Camera.FOVY, Near: sc.Camera.Near, Far: sc.Camera.Far,
			Ortho: sc.Camera.Ortho, OrthoSize: sc.Camera.OrthoSize,
		}
	}
	for _, l := range sc.Lights {
		doc.Lights = append(doc.Lights, lightDoc{
			Kind: int(l.Kind), Dir: v3doc(l.Dir), Pos: v3doc(l.Pos), Color: l.Color,
			Intensity: l.Intensity, Range: l.Range,
		})
	}
	if sc.Root != nil {
		for _, c := range sc.Root.Children {
			if c != nil {
				doc.Nodes = append(doc.Nodes, nodeToDoc(c, meshName))
			}
		}
	}
	return json.MarshalIndent(doc, "", "  ")
}

// SaveScene 保存场景到文件。
func SaveScene(sc *Scene, path string, meshName func(*Mesh) string) error {
	b, err := SceneJSON(sc, meshName)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

// LoadSceneJSON 从 JSON 重建场景（resolve 负责把网格名换回 *Mesh）。
func LoadSceneJSON(b []byte, resolve func(string) *Mesh) (*Scene, error) {
	var doc sceneDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	sc := NewScene()
	sc.Ambient = doc.Ambient
	sc.Background = doc.Background
	sc.Fog = doc.Fog
	if doc.Camera != nil {
		c := NewCamera()
		c.Pos = docV3(doc.Camera.Pos)
		c.Target = docV3(doc.Camera.Target)
		c.Up = docV3(doc.Camera.Up)
		c.FOVY = doc.Camera.FOVY
		c.Near = doc.Camera.Near
		c.Far = doc.Camera.Far
		c.Ortho = doc.Camera.Ortho
		c.OrthoSize = doc.Camera.OrthoSize
		sc.Camera = c
	}
	sc.Lights = nil
	for _, l := range doc.Lights {
		sc.Lights = append(sc.Lights, Light{
			Kind: LightKind(l.Kind), Dir: docV3(l.Dir), Pos: docV3(l.Pos), Color: l.Color,
			Intensity: l.Intensity, Range: l.Range,
		})
	}
	for _, d := range doc.Nodes {
		sc.Add(docToNode(d, resolve))
	}
	return sc, nil
}

// LoadScene 从文件读取场景。
func LoadScene(path string, resolve func(string) *Mesh) (*Scene, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadSceneJSON(b, resolve)
}
