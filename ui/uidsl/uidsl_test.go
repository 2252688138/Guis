// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"guis/ui"
)

func TestParseDSLNesting(t *testing.T) {
	src := `
col gap=12
  text "标题"
  row
    text "左"
    text "右"
  spacer
`
	root, err := parseDSL(src)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if root.Type != "col" || len(root.Children) != 3 {
		t.Fatalf("根节点不对: %+v", root)
	}
	if root.Children[0].Type != "text" || root.Children[0].Text != "标题" {
		t.Fatalf("第一个子节点不对: %+v", root.Children[0])
	}
	row := root.Children[1]
	if row.Type != "row" || len(row.Children) != 2 {
		t.Fatalf("row 子节点不对: %+v", row)
	}
	if row.Children[1].Text != "右" {
		t.Fatalf("row 第二个子节点不对: %+v", row.Children[1])
	}
	if root.Children[2].Type != "spacer" {
		t.Fatalf("第三个子节点应为 spacer: %+v", root.Children[2])
	}
}

func TestParseDSLQuotedText(t *testing.T) {
	root, err := parseDSL(`text "带 空格 的 文案" size=18`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if root.Text != "带 空格 的 文案" {
		t.Fatalf("引号文本解析错: %q", root.Text)
	}
	if root.Size != 18 {
		t.Fatalf("size 应为 18，得到 %v", root.Size)
	}
}

func TestParseDSLAttrs(t *testing.T) {
	root, err := parseDSL(`col gap=12 pad=16 bg=#0f1626 r=8 stroke=1 clip=true opacity=0.5 align=center`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if root.Gap != 12 || root.Pad != 16 || root.R != 8 || root.Stroke != 1 {
		t.Fatalf("数值属性解析错: %+v", root)
	}
	if root.BG != "#0f1626" {
		t.Fatalf("颜色属性解析错: %q", root.BG)
	}
	if !root.Clip || root.Opacity != 0.5 || root.Align != "center" {
		t.Fatalf("其它属性解析错: %+v", root)
	}
}

func TestParseDSLKeyAliasAndTextAttr(t *testing.T) {
	root, err := parseDSL(`button id=ok text="确定" onclick=confirm`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if root.Key != "ok" || root.Text != "确定" || root.OnClick != "confirm" {
		t.Fatalf("id/text/onclick 解析错: %+v", root)
	}
}

func TestParseDSLUnknownAttrError(t *testing.T) {
	_, err := parseDSL("col\n  text \"x\" sizee=12")
	if err == nil {
		t.Fatal("未知属性应该报错")
	}
	if !strings.Contains(err.Error(), "第 2 行") {
		t.Fatalf("错误信息应带行号: %v", err)
	}
}

func TestParseDSLComments(t *testing.T) {
	root, err := parseDSL("# 注释\n// 也是注释\ncol\n  # 缩进注释\n  text \"x\"\n")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(root.Children) != 1 {
		t.Fatalf("注释不该产生节点: %+v", root)
	}
}

func TestParseDSLMultipleRoots(t *testing.T) {
	root, err := parseDSL("text \"a\"\ntext \"b\"")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if root.Type != "col" || len(root.Children) != 2 {
		t.Fatalf("多个根应自动包一层 col: %+v", root)
	}
}

func TestParseDSLEmpty(t *testing.T) {
	if _, err := parseDSL("# 只有注释\n\n"); err == nil {
		t.Fatal("空文档应报错")
	}
}

func TestParseJSONFlat(t *testing.T) {
	src := `{"type":"col","gap":10,"children":[{"type":"text","text":"hi","size":20}]}`
	n, err := Parse([]byte(src), FormatJSON)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if n.Type != "col" || n.Gap != 10 || len(n.Children) != 1 || n.Children[0].Text != "hi" {
		t.Fatalf("JSON 解析错: %+v", n)
	}
}

func TestParseJSONRootWrapper(t *testing.T) {
	src := `{"root":{"type":"row","children":[{"type":"icon","icon":"star"}]}}`
	n, err := Parse([]byte(src), FormatJSON)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if n.Type != "row" || len(n.Children) != 1 || n.Children[0].Icon != "star" {
		t.Fatalf("root 包裹解析错: %+v", n)
	}
}

func TestParseJSONEmpty(t *testing.T) {
	if _, err := Parse([]byte(`{}`), FormatJSON); err == nil {
		t.Fatal("空 JSON 应报错")
	}
}

func TestBuildKindsAndStyle(t *testing.T) {
	decl, err := parseDSL(`
col gap=12 pad=16 bg=#0f1626 r=8
  text "标题" size=22 weight=700 color=#e6edf7
  row
    button "点我" id=go onClick=hello
    icon star size=20
    image logo.png
  scroll grow=1
`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	n := Build(decl, Actions{"hello": func() {}})
	if n.Dir != ui.Vertical || len(n.Children) != 3 {
		t.Fatalf("根节点构造错: kind=%v dir=%v kids=%d", n.Kind, n.Dir, len(n.Children))
	}
	if n.Style.Gap != 12 || n.Style.FontSize != 0 {
		t.Fatalf("容器样式错: gap=%v size=%v", n.Style.Gap, n.Style.FontSize)
	}
	txt := n.Children[0]
	if txt.Kind != ui.KText || txt.Text != "标题" || txt.Style.FontSize != 22 || txt.Style.FontWeight != 700 {
		t.Fatalf("文本节点错: kind=%v text=%q size=%v weight=%v", txt.Kind, txt.Text, txt.Style.FontSize, txt.Style.FontWeight)
	}
	row := n.Children[1]
	if row.Dir != ui.Horizontal || len(row.Children) != 3 {
		t.Fatalf("row 构造错: dir=%v kids=%d", row.Dir, len(row.Children))
	}
	btn := row.Children[0]
	if btn.Kind != ui.KButton || btn.Key != "go" || btn.OnClick == nil {
		t.Fatalf("按钮构造错: kind=%v key=%q click=%v", btn.Kind, btn.Key, btn.OnClick != nil)
	}
	if ic := row.Children[1]; ic.Kind != ui.KIcon || ic.IconName != "star" {
		t.Fatalf("图标构造错: kind=%v name=%q", ic.Kind, ic.IconName)
	}
	if im := row.Children[2]; im.Kind != ui.KImage || im.Src != "logo.png" {
		t.Fatalf("图片构造错: kind=%v src=%q", im.Kind, im.Src)
	}
	sc := n.Children[2]
	if sc.Kind != ui.KScroll || sc.Style.Grow != 1 {
		t.Fatalf("滚动容器构造错: kind=%v grow=%v", sc.Kind, sc.Style.Grow)
	}
	if !strings.HasPrefix(sc.Key, "uidsl-") {
		t.Fatalf("滚动容器应自动补稳定 Key，得到 %q", sc.Key)
	}
}

func TestBuildUnknownType(t *testing.T) {
	n := Build(Node{Type: "whatever"}, nil)
	if !strings.Contains(n.Text, "未知节点类型") {
		t.Fatalf("未知类型应渲染成提示文本: %q", n.Text)
	}
}

func TestBuildMissingAction(t *testing.T) {
	n := Build(Node{Type: "button", Text: "x", OnClick: "nope"}, nil)
	if n.OnClick != nil {
		t.Fatal("未注册的事件不该绑定回调")
	}
}

func TestBuildButtonDefaults(t *testing.T) {
	// 框架的 ui.Button 没设字号与文字色（不设画不出字），uidsl 必须补上默认样式。
	n := Build(Node{Type: "button", Text: "确定"}, nil)
	if n.Style.FontSize <= 0 {
		t.Fatalf("按钮默认字号缺失: %v", n.Style.FontSize)
	}
	if n.Style.Color.A == 0 {
		t.Fatal("按钮默认文字色缺失（会被画成透明）")
	}
	if !n.Style.BgSet {
		t.Fatal("按钮默认底色缺失")
	}
	if n.Style.Height != 34 || n.Style.Radius[0] != 8 {
		t.Fatalf("按钮默认高度 / 圆角不对: h=%v r=%v", n.Style.Height, n.Style.Radius[0])
	}
	// 描述文件里显式写了就尊重用户的写法。
	n2 := Build(Node{Type: "button", Text: "确定", Size: 18, Color: "#ff0000", BG: "#123456", H: 48}, nil)
	if n2.Style.FontSize != 18 || n2.Style.Height != 48 {
		t.Fatalf("显式样式被默认值覆盖了: size=%v h=%v", n2.Style.FontSize, n2.Style.Height)
	}
}

func TestLiveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.json")
	if err := os.WriteFile(path, []byte(`{"type":"text","text":"v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	defer l.Close()
	l.SetInterval(20 * time.Millisecond)
	if got := l.Root().Text; got != "v1" {
		t.Fatalf("首次加载错: %q", got)
	}
	// 改文件：等轮询把新内容换上去。
	time.Sleep(30 * time.Millisecond) // 让 mtime 明显变化
	if err := os.WriteFile(path, []byte(`{"type":"text","text":"v2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.Root().Text == "v2" {
			if l.LastError() != nil {
				t.Fatalf("成功后不该有错误: %v", l.LastError())
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("热加载没有生效，仍是 %q", l.Root().Text)
}

func TestLiveParseErrorKeepsOldTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.dsl")
	if err := os.WriteFile(path, []byte("text \"ok\""), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	defer l.Close()
	l.SetInterval(20 * time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if err := os.WriteFile(path, []byte("text \"没有收尾的引号\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.LastError() != nil {
			if got := l.Root().Text; got != "ok" {
				t.Fatalf("解析失败时应保留旧树，得到 %q", got)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("没有记录解析错误")
}
