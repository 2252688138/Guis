// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"testing"

	"guis/game"
)

func TestEngineHandleTouch(t *testing.T) {
	e := game.NewEngine(320, 180)
	e.HandleTouch(10, 20, 0, 0)
	if len(e.Input.Touches) == 0 {
		t.Fatal("触摸按下后 Touches 应有数据")
	}
	if e.Input.Touches[0].Phase != game.TouchBegin {
		t.Fatalf("阶段 = %v，期望 TouchBegin", e.Input.Touches[0].Phase)
	}
	e.HandleTouch(30, 40, 2, 0)
	last := e.Input.Touches[len(e.Input.Touches)-1]
	if last.Phase != game.TouchEnd {
		t.Fatalf("阶段 = %v，期望 TouchEnd", last.Phase)
	}
	// nil 安全
	var nilEngine *game.Engine
	nilEngine.HandleTouch(0, 0, 0, 0)
}
