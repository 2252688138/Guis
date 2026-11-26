// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"fmt"
	"time"
)

// 引擎 API 的脚本绑定：把场景/相机/输入暴露成脚本函数。
//
//	vm := game.NewScriptVM()
//	vm.BindEngine(eng)
//	vm.Exec(`
//	    fn tick(dt) {
//	        let p = camera_pos()
//	        camera_look_at(0, 0, 0)
//	        for i in range(entity_count()) {
//	            if entity_name(i) == "enemy" {
//	                entity_move(i, 0, 0, -dt)
//	            }
//	        }
//	    }
//	`)
//	// 每帧：
//	vm.Call("tick", game.ScriptNum(dt))
//
// 实体 id 是「按场景遍历顺序的序号」，每次调用都会重建一次快照，因此同一帧内
// 稳定、跨帧可能变化（增删节点后尤其要注意重新 entity_find）。

// scriptBinder 保存绑定状态。
type scriptBinder struct {
	eng   *Engine
	start time.Time
	table []*Node
}

// BindEngine 注册引擎 API（可对同一个 VM 重复调用以切换引擎）。
func (vm *ScriptVM) BindEngine(eng *Engine) *ScriptVM {
	if vm == nil || eng == nil {
		return vm
	}
	b := &scriptBinder{eng: eng, start: time.Now()}

	vm.Register("time", func(args []ScriptValue) (ScriptValue, error) {
		return ScriptNum(time.Since(b.start).Seconds()), nil
	})
	vm.Register("vec3", func(args []ScriptValue) (ScriptValue, error) {
		get := func(i int) float64 {
			if i < len(args) {
				return args[i].AsFloat()
			}
			return 0
		}
		return scriptVec3(get(0), get(1), get(2)), nil
	})
	vm.Register("entity_count", func(args []ScriptValue) (ScriptValue, error) {
		return ScriptNum(float64(len(b.refresh()))), nil
	})
	vm.Register("entity_name", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return ScriptStr(""), nil
		}
		return ScriptStr(n.Name), nil
	})
	vm.Register("entity_find", func(args []ScriptValue) (ScriptValue, error) {
		name := argStr(args, 0)
		for i, n := range b.refresh() {
			if n.Name == name {
				return ScriptNum(float64(i)), nil
			}
		}
		return ScriptNum(-1), nil
	})
	vm.Register("entity_pos", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return scriptVec3(0, 0, 0), nil
		}
		p := n.WorldPos()
		return scriptVec3(float64(p.X), float64(p.Y), float64(p.Z)), nil
	})
	vm.Register("entity_set_pos", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return ScriptNilValue(), nil
		}
		n.Pos = V3(argF(args, 1), argF(args, 2), argF(args, 3))
		return ScriptNilValue(), nil
	})
	vm.Register("entity_move", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return ScriptNilValue(), nil
		}
		n.Pos = n.Pos.Add(V3(argF(args, 1), argF(args, 2), argF(args, 3)))
		return ScriptNilValue(), nil
	})
	vm.Register("entity_scale", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return ScriptNilValue(), nil
		}
		s := argF(args, 1)
		if s <= 0 {
			s = 1
		}
		n.Scl = V3(s, s, s)
		return ScriptNilValue(), nil
	})
	vm.Register("entity_add", func(args []ScriptValue) (ScriptValue, error) {
		if b.eng.Scene == nil {
			return ScriptNum(-1), nil
		}
		name := argStr(args, 0)
		if name == "" {
			name = "entity"
		}
		node := NewMeshNode(name, NewBox(1, 1, 1), DefaultMaterial())
		node.Pos = V3(argF(args, 1), argF(args, 2), argF(args, 3))
		if b.eng.Scene.Root == nil {
			b.eng.Scene.Root = &Node{Name: "root", Visible: true}
		}
		b.eng.Scene.Root.Add(node)
		table := b.refresh()
		for i, n := range table {
			if n == node {
				return ScriptNum(float64(i)), nil
			}
		}
		return ScriptNum(float64(len(table) - 1)), nil
	})
	vm.Register("entity_remove", func(args []ScriptValue) (ScriptValue, error) {
		n := b.byID(argInt(args, 0))
		if n == nil {
			return ScriptBoolValue(false), nil
		}
		if p := b.parentOf(n); p != nil {
			for i, c := range p.Children {
				if c == n {
					p.Children = append(p.Children[:i], p.Children[i+1:]...)
					break
				}
			}
			b.table = nil
			return ScriptBoolValue(true), nil
		}
		return ScriptBoolValue(false), nil
	})
	vm.Register("entity_distance", func(args []ScriptValue) (ScriptValue, error) {
		a, c := b.byID(argInt(args, 0)), b.byID(argInt(args, 1))
		if a == nil || c == nil {
			return ScriptNum(0), nil
		}
		return ScriptNum(float64(a.WorldPos().Dist(c.WorldPos()))), nil
	})
	vm.Register("camera_pos", func(args []ScriptValue) (ScriptValue, error) {
		cam := b.eng.Scene.Camera
		if cam == nil {
			return scriptVec3(0, 0, 0), nil
		}
		return scriptVec3(float64(cam.Pos.X), float64(cam.Pos.Y), float64(cam.Pos.Z)), nil
	})
	vm.Register("camera_look_at", func(args []ScriptValue) (ScriptValue, error) {
		cam := b.eng.Scene.Camera
		if cam == nil {
			return ScriptNilValue(), nil
		}
		cam.Target = V3(argF(args, 0), argF(args, 1), argF(args, 2))
		return ScriptNilValue(), nil
	})
	vm.Register("camera_set_pos", func(args []ScriptValue) (ScriptValue, error) {
		cam := b.eng.Scene.Camera
		if cam == nil {
			return ScriptNilValue(), nil
		}
		cam.Pos = V3(argF(args, 0), argF(args, 1), argF(args, 2))
		return ScriptNilValue(), nil
	})
	vm.Register("camera_move", func(args []ScriptValue) (ScriptValue, error) {
		cam := b.eng.Scene.Camera
		if cam == nil {
			return ScriptNilValue(), nil
		}
		d := V3(argF(args, 0), argF(args, 1), argF(args, 2))
		cam.Pos = cam.Pos.Add(d)
		cam.Target = cam.Target.Add(d)
		return ScriptNilValue(), nil
	})
	vm.Register("key_down", func(args []ScriptValue) (ScriptValue, error) {
		in := b.eng.Input
		if in == nil || in.Keys == nil {
			return ScriptBoolValue(false), nil
		}
		return ScriptBoolValue(in.Keys[argStr(args, 0)]), nil
	})
	vm.Register("log", func(args []ScriptValue) (ScriptValue, error) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, a.AsString())
		}
		msg := ""
		for i, p := range parts {
			if i > 0 {
				msg += " "
			}
			msg += p
		}
		if vm.OnPrint != nil {
			vm.OnPrint(msg)
		}
		return ScriptNilValue(), nil
	})
	return vm
}

// refresh 重建节点快照表。
func (b *scriptBinder) refresh() []*Node {
	b.table = b.table[:0]
	if b.eng == nil || b.eng.Scene == nil || b.eng.Scene.Root == nil {
		return b.table
	}
	b.eng.Scene.BuildWorld()
	b.eng.Scene.Each(func(n *Node) {
		b.table = append(b.table, n)
	})
	return b.table
}

// byID 按序号取节点。
func (b *scriptBinder) byID(id int) *Node {
	t := b.refresh()
	if id < 0 || id >= len(t) {
		return nil
	}
	return t[id]
}

// parentOf 查找父节点。
func (b *scriptBinder) parentOf(n *Node) *Node {
	var parent *Node
	for _, p := range b.refresh() {
		for _, c := range p.Children {
			if c == n {
				parent = p
				break
			}
		}
		if parent != nil {
			break
		}
	}
	return parent
}

// scriptVec3 生成 {x, y, z} 对象。
func scriptVec3(x, y, z float64) ScriptValue {
	return ScriptObjectValue(map[string]ScriptValue{
		"x": ScriptNum(x),
		"y": ScriptNum(y),
		"z": ScriptNum(z),
	})
}

func argF(args []ScriptValue, i int) float32 {
	if i < len(args) {
		return float32(args[i].AsFloat())
	}
	return 0
}

func argInt(args []ScriptValue, i int) int {
	return int(argF(args, i))
}

func argStr(args []ScriptValue, i int) string {
	if i < len(args) {
		return args[i].AsString()
	}
	return ""
}

// 保证 fmt 在后续扩展绑定报错信息时可用。
var _ = fmt.Sprintf
