// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// 第三阶段：动画映射、菜单栏接线、降级与 3D 视口注册。

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"guis/ui"
)

// 动画声明 → AnimSpec 的映射与兜底。
func TestAnimSpecMapping(t *testing.T) {
	specs := toAnimSpecs([]AnimDef{
		{Kind: "pop", Dur: 0.2, Ease: "out_back", Delay: 0.1},
		{Kind: "slide", Side: "right", Dist: 40, Out: true},
		{Kind: "breathe", Loop: true, Amp: 0.06},
		{Kind: "nope", Side: "nope", Ease: "nope"},
	})
	if len(specs) != 4 {
		t.Fatalf("期望 4 条 spec，得到 %d", len(specs))
	}
	if specs[0].Kind != ui.AnimScale {
		t.Fatalf("pop 应映射为 AnimScale，得到 %v", specs[0].Kind)
	}
	if specs[0].Ease == nil || specs[0].Delay != 0.1 {
		t.Fatal("ease / delay 未透传")
	}
	if specs[1].Kind != ui.AnimSlide || specs[1].Side != ui.SideRight || !specs[1].Out || specs[1].Dist != 40 {
		t.Fatalf("slide 参数未透传: %+v", specs[1])
	}
	if specs[2].Kind != ui.AnimPulse || !specs[2].Loop || specs[2].Amp != 0.06 {
		t.Fatalf("breathe 参数未透传: %+v", specs[2])
	}
	if specs[3].Kind != ui.AnimFade || specs[3].Side != ui.SideLeft || specs[3].Ease != nil {
		t.Fatalf("未知值应退化为 fade/left/nil，得到 %+v", specs[3])
	}
}

// Canvas 的 points 支持 {x,y} 与 [x,y] 两种写法。
func TestCanvasPointForms(t *testing.T) {
	raw := `{"type":"canvas","w":64,"h":32,"shapes":[` +
		`{"kind":"polygon","points":[[0,0],[10,0],[5,10]],"fill":"#fff"},` +
		`{"kind":"polyline","points":[{"x":1,"y":2},{"x":3,"y":4}],"stroke":"#fff"}]}`
	var n Node
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		t.Fatalf("canvas 描述解码失败: %v", err)
	}
	if len(n.Shapes) != 2 {
		t.Fatalf("shapes 条数 %d", len(n.Shapes))
	}
	if got := n.Shapes[0].Points; len(got) != 3 || got[1].X != 10 || got[2].Y != 10 {
		t.Fatalf("[x,y] 写法解析不对: %+v", got)
	}
	if got := n.Shapes[1].Points; len(got) != 2 || got[1].X != 3 || got[1].Y != 4 {
		t.Fatalf("{x,y} 写法解析不对: %+v", got)
	}
}

// 节点表里的 anim 字段能被解码并挂到构建出的节点上。
func TestNodeAnimApplied(t *testing.T) {
	raw := `{"type":"card","anim":[{"kind":"fade"},{"kind":"float","loop":true}]}`
	var n Node
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		t.Fatalf("anim 字段解码失败: %v", err)
	}
	if len(n.Anim) != 2 {
		t.Fatalf("anim 条数 %d", len(n.Anim))
	}
	out := BuildWith(n, Handlers{})
	if out == nil {
		t.Fatal("构建结果为空")
	}
	if len(out.Children) > 0 {
		t.Fatal("card 不应凭空多出子节点")
	}
}

// 菜单项的 onClick 能接到回调并触发。
func TestMenuDefsWiring(t *testing.T) {
	fired := ""
	h := Handlers{Click: map[string]func(){
		"new":  func() { fired = "new" },
		"quit": func() { fired = "quit" },
	}}
	defs := toMenuDefs([]MenuDefDef{
		{Label: "文件", Items: []MenuItemDef{
			{Label: "新建", OnClick: "new", Hint: "Ctrl+N"},
			{Separator: true},
			{Label: "退出", OnClick: "quit", Checked: true},
			{Label: "更多", Children: []MenuItemDef{{Label: "子项", OnClick: "quit"}}},
		}},
	}, h)
	if len(defs) != 1 || len(defs[0].Items) != 4 {
		t.Fatalf("菜单结构不对: %+v", defs)
	}
	it := defs[0].Items[0]
	if it.OnPick == nil || it.Hint != "Ctrl+N" {
		t.Fatalf("菜单项未接线: %+v", it)
	}
	it.OnPick()
	if fired != "new" {
		t.Fatalf("点击回调未触发，fired=%q", fired)
	}
	if !defs[0].Items[1].Separator || !defs[0].Items[2].Checked {
		t.Fatal("separator / checked 未透传")
	}
	if len(defs[0].Items[3].Children) != 1 || defs[0].Items[3].Children[0].OnPick == nil {
		t.Fatal("子菜单未接线")
	}
}

// 终端：行内容进 TermState，同 key 复用。
func TestTermNode(t *testing.T) {
	ResetExtraStates()
	n := BuildWith(Node{Type: "term", Key: "t1", Lines: []string{"a", "b"}}, Handlers{})
	if n == nil || n.Term == nil {
		t.Fatal("term 节点未创建")
	}
	first := n.Term
	if !strings.Contains(first.Text(), "a") {
		t.Fatalf("终端内容未写入: %q", first.Text())
	}
	n2 := BuildWith(Node{Type: "term", Key: "t1", Lines: []string{"a", "b", "c"}}, Handlers{})
	if n2.Term != first {
		t.Fatal("同 key 应复用同一个 TermState")
	}
	if !strings.Contains(n2.Term.Text(), "c") {
		t.Fatalf("增量内容未写入: %q", n2.Term.Text())
	}
	ResetExtraStates()
	n3 := BuildWith(Node{Type: "term", Key: "t1", Lines: []string{"z"}}, Handlers{})
	if n3.Term == first {
		t.Fatal("ResetExtraStates 后应重建 TermState")
	}
}

// 视频打不开时降级成文本并记一条降级信息。
func TestVideoDegrades(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("视频降级矩阵按平台后端解码能力而异（macOS runner 上走不同分支）")
	}
	ResetExtraStates()
	DrainDegrades()
	n := videoNode(Node{Type: "video", Key: "v1", Src: "不存在的文件.mp4"}, "p")
	if n == nil {
		t.Fatal("video 降级返回 nil")
	}
	if n.Kind != ui.KText || !strings.Contains(n.Text, "video") {
		t.Fatalf("未降级为提示文本: kind=%v text=%q", n.Kind, n.Text)
	}
	if msgs := DrainDegrades(); len(msgs) == 0 {
		t.Fatal("降级信息未记录")
	}
}

// 3D 视口：没注册渲染源降级；注册后按 key 调用提供者。
func TestViewportProvider(t *testing.T) {
	ResetExtraStates()
	RegisterViewport3DProvider(nil)
	n := viewportNode(Node{Type: "viewport", Key: "main"}, "p")
	if n == nil || n.Kind != ui.KText || !strings.Contains(n.Text, "3D 视口") {
		t.Fatalf("未注册时应降级，得到 kind=%v text=%q", n.Kind, n.Text)
	}

	got := ""
	RegisterViewport3DProvider(func(key string) *ui.Viewport3DOpts {
		got = key
		return nil // 返回 nil 也要安全降级
	})
	defer RegisterViewport3DProvider(nil)
	n2 := viewportNode(Node{Type: "viewport", Key: "main"}, "p")
	if got != "main" {
		t.Fatalf("提供者未按 key 调用，got=%q", got)
	}
	if n2 == nil || n2.Kind != ui.KText {
		t.Fatal("提供者返回 nil 时应降级")
	}
}
