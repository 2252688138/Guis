// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"fmt"

	"guis/game"
)

// ExampleEngine 演示最小的离屏渲染闭环（不依赖窗口）。
func ExampleEngine() {
	eng := game.NewEngine(320, 180)
	eng.Scene.SetAmbientIntensity(0.3)
	eng.Scene.AddSun(game.V3(-0.4, -1, -0.6), 1)
	eng.Scene.Add(game.NewMeshNode("cube", game.NewBox(1, 1, 1), game.DefaultMaterial()))

	pix, w, h := eng.RenderViewport(320, 180, 1.0/60.0)
	fmt.Println(w, h, len(pix) == w*h*4)
	// Output:
	// 320 180 true
}
