// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"strconv"
	"sync"
)

// Iconfont 图标库：内置 80+ 矢量图标 + 运行时注册能力。
//
// 用法：
//
//	ui.Icon("search")                       // 18×18，主题次要文字色
//	ui.IconSized("star", 28)                // 指定尺寸
//	ui.IconColored("heart", 22, "#f472b6")  // 指定颜色
//	ui.IconLabel("folder", "项目文件", 16, ui.ColTextDim)
//	ui.IconAction("trash", 18, func() { ... })
//
// 自定义图标（SVG path 数据，24×24 设计网格，支持多段路径）：
//
//	ui.RegisterIcon(ui.IconDef{
//	    Name: "logo", Label: "品牌",
//	    Paths: []string{"M4 4h16v16H4z", "M8 12h8"},
//	})
//
// 图标由矢量路径光栅化成纹理（见 iconfont_raster.go），与系统字体无关：
// 任意尺寸都清晰、可随时换色，缺图标时也不会画出「方块」。
// 同一（名称、像素尺寸、线宽）只光栅化一次，后续帧直接贴图。

// IconDef 一个图标的矢量定义（24×24 设计网格，与 SVG path 语法一致）。
type IconDef struct {
	// Name 唯一名称（Icon 的参数），大小写敏感。
	Name string
	// Label 展示名（图标库示例页 / 无障碍朗读用）。
	Label string
	// Paths SVG path 数据，可含多条（同一份设计网格内叠加）。
	Paths []string
	// Fill 为 true 时按奇偶规则填充（实心图标），否则描边。
	Fill bool
	// Stroke 描边宽度（设计网格单位，0 = 默认 2）。
	Stroke float32
}

// iconDefaultStroke 默认描边宽度（24 网格里 2 单位 ≈ Lucide / Feather 的观感）。
const iconDefaultStroke = 2.0

// 注册表用独立变量而不是匿名 struct：零值即空表，pin 住读写锁。
var (
	iconMu     sync.RWMutex
	iconByName = map[string]IconDef{}
	iconOrder  []string
)

func init() { registerBuiltinIcons() }

// RegisterIcon 注册（或同名覆盖）一个图标，返回是否覆盖了已有同名图标。
//
// 同名时后注册的生效且保留原有顺序，业务可以用自有品牌图标替换内置图标。
func RegisterIcon(def IconDef) bool {
	if def.Name == "" {
		return false
	}
	if def.Stroke <= 0 {
		def.Stroke = iconDefaultStroke
	}
	def.Paths = append([]string(nil), def.Paths...)
	iconMu.Lock()
	defer iconMu.Unlock()
	_, exists := iconByName[def.Name]
	iconByName[def.Name] = def
	if !exists {
		iconOrder = append(iconOrder, def.Name)
	}
	return exists
}

// UnregisterIcon 移除一个图标（示例页演示注册自定义图标后还原用）。
func UnregisterIcon(name string) {
	iconMu.Lock()
	defer iconMu.Unlock()
	if _, ok := iconByName[name]; !ok {
		return
	}
	delete(iconByName, name)
	for i, n := range iconOrder {
		if n == name {
			iconOrder = append(iconOrder[:i], iconOrder[i+1:]...)
			break
		}
	}
}

// IconDefOf 按名称取图标定义。
func IconDefOf(name string) (IconDef, bool) {
	iconMu.RLock()
	defer iconMu.RUnlock()
	def, ok := iconByName[name]
	return def, ok
}

// IconExists 图标是否已注册。
func IconExists(name string) bool {
	iconMu.RLock()
	defer iconMu.RUnlock()
	_, ok := iconByName[name]
	return ok
}

// IconNames 返回全部图标名（注册顺序，示例页按此铺网格）。
func IconNames() []string {
	iconMu.RLock()
	defer iconMu.RUnlock()
	return append([]string(nil), iconOrder...)
}

// IconCount 已注册图标数量。
func IconCount() int {
	iconMu.RLock()
	defer iconMu.RUnlock()
	return len(iconOrder)
}

// Icon 图标节点：默认 18×18，颜色为次要文字色（`.St(St().Text(0, 0, "#fff"))` 可改色）。
func Icon(name string) *Node { return IconSized(name, 18) }

// IconSized 指定边长（正方形）的图标节点。
func IconSized(name string, size float32) *Node {
	if size <= 0 {
		size = 18
	}
	return &Node{
		Kind:     KIcon,
		IconName: name,
		Style:    Style{Opacity: 1, Width: size, Height: size, Color: C(ColTextDim)},
	}
}

// IconColored 指定尺寸与颜色的图标节点。
func IconColored(name string, size float32, color any) *Node {
	n := IconSized(name, size)
	n.Style.Color = C(color)
	return n
}

// IconStroked 指定线宽（设计网格单位，0 = 图标自带线宽）的图标节点。
// 细线适合 12px 小图标，粗线适合 48px 以上的大图标。
func IconStroked(name string, size, stroke float32) *Node {
	n := IconSized(name, size)
	n.IconStroke = stroke
	return n
}

// IconLabel 图标 + 文字的一行（侧栏条目、按钮内图文、列表项通用）。
func IconLabel(name, label string, size float32, color any) *Node {
	return Row(
		IconColored(name, size, color),
		Text(label).St(St().Text(13, 500, color)),
	).St(St().GapOf(8)).AlignTo(AlignCenter)
}

// IconAction 方形图标按钮（图标居中、悬停提亮），工具栏 / 卡片右上角操作区用。
func IconAction(name string, size float32, onClick func()) *Node {
	if size <= 0 {
		size = 18
	}
	side := size + 18
	color := ColTextDim
	key := "iconact:" + name + ":" + strconv.Itoa(int(size))
	if IsHovered(key) {
		color = ColText
	}
	n := IconColored(name, size, color).St(St().W(side).H(side).Padding(9).
		R(9).BgC(ColBgSoft).Stroke(1, ColStroke))
	n.Key = key
	n.OnClick = func() {
		if onClick != nil {
			onClick()
		}
		RequestRedraw()
	}
	n.OnHover = func(bool) { RequestRedraw() }
	return n
}
