// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 示例脚本冒烟：examples/lua-demo/ui.lua 必须能装载并构树成功。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLuaDemoSmoke(t *testing.T) {
	path := filepath.Join("..", "examples", "lua-demo", "ui.lua")
	if _, err := os.Stat(path); err != nil {
		t.Skip("示例脚本不存在")
	}
	e, err := Attach(path, Options{Interval: -1})
	if err != nil {
		t.Fatalf("装载示例失败: %v", err)
	}
	defer e.Close()
	if root := e.Root(); root == nil {
		t.Fatal("示例构树返回 nil")
	}
	if err := e.LastError(); err != nil {
		t.Fatalf("示例构树失败: %v", err)
	}
}
