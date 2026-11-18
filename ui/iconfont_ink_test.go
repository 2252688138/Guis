// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import (
	"math"
	"testing"
)

// TestIconInkCentered 内置图标的墨迹在 24 网格里必须基本居中。
//
// 图标节点按「盒子中心」绘制（paintIcon 等比居中），所以只要矢量内容在设计
// 网格里不整体偏，图标就不会有人为的上下漂移。这条测试挡住新增 / 替换图标时
// 把内容整体画偏的情况：折算到 24 网格后偏离不超过 2.5 单位（music 这类天然
// 偏上的音符符号约 2.1，属于设计本身）。
func TestIconInkCentered(t *testing.T) {
	const px = 96
	const limit = 2.5
	var worst float64
	worstName := ""
	for _, name := range IconNames() {
		def, ok := IconDefOf(name)
		if !ok || len(def.Paths) == 0 {
			continue
		}
		subs := make([]iconSubPath, 0, len(def.Paths))
		for _, d := range def.Paths {
			subs = append(subs, iconFlattenPath(d)...)
		}
		buf := iconRaster(subs, def.Fill, float64(def.Stroke), px)
		top, bot := -1, -1
		for y := 0; y < px; y++ {
			hit := false
			for x := 0; x < px; x++ {
				if buf[(y*px+x)*4+3] > 32 {
					hit = true
					break
				}
			}
			if hit {
				if top < 0 {
					top = y
				}
				bot = y
			}
		}
		if top < 0 {
			continue
		}
		center := float64(top+bot) / 2
		d := (center - float64(px)/2) / float64(px) * 24 // 折算到 24 网格单位
		if math.Abs(d) > worst {
			worst = math.Abs(d)
			worstName = name
		}
	}
	if worst > limit {
		t.Fatalf("图标 %q 的墨迹在设计网格里偏离 %.3f 单位（上限 %.1f）", worstName, worst, limit)
	}
}
