package game_test

import (
	"strings"
	"testing"

	"guis/game"
)

// TestScriptBasics 变量、运算、字符串拼接、条件与循环。
func TestScriptBasics(t *testing.T) {
	vm := game.NewScriptVM()
	if err := vm.Exec(`
		let a = 1 + 2 * 3
		let b = a - 1
		let s = "hp=" + b
		let flag = false
		if b > 5 && a == 7 { flag = true }
		let sum = 0
		for i in range(5) { sum = sum + i }
		let n = 0
		while n < 3 { n = n + 1 }
		print(s, sum, n, flag)
	`); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	out := vm.Output()
	if len(out) != 1 {
		t.Fatalf("输出行数 %d", len(out))
	}
	if out[0] != "hp=6 10 3 true" {
		t.Fatalf("输出内容: %q", out[0])
	}
	if v, ok := vm.GetGlobal("a"); !ok || v.AsFloat() != 7 {
		t.Fatalf("a = %v", v.AsFloat())
	}
}

// TestScriptFunctions 函数定义、递归、局部作用域。
func TestScriptFunctions(t *testing.T) {
	vm := game.NewScriptVM()
	src := `
		fn fib(n) {
			if n < 2 { return n }
			return fib(n - 1) + fib(n - 2)
		}
		fn add(a, b) { return a + b }
		let r = 0
		fn outer() {
			let r = 100
			return r
		}
		r = fib(10)
	`
	if err := vm.Exec(src); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if v, _ := vm.GetGlobal("r"); v.AsFloat() != 55 {
		t.Fatalf("fib(10) 应为 55，实际 %v", v.AsFloat())
	}
	got, err := vm.Call("add", game.ScriptNum(2), game.ScriptNum(40))
	if err != nil || got.AsFloat() != 42 {
		t.Fatalf("add(2,40) = %v err=%v", got.AsFloat(), err)
	}
	got, err = vm.Call("outer")
	if err != nil || got.AsFloat() != 100 {
		t.Fatalf("局部变量应遮蔽全局: %v err=%v", got.AsFloat(), err)
	}
}

// TestScriptArraysObjects 数组、对象、for-in 与成员访问。
func TestScriptArraysObjects(t *testing.T) {
	vm := game.NewScriptVM()
	if err := vm.Exec(`
		let arr = [1, 2, 3]
		arr[1] = 20
		let total = 0
		for v in arr { total = total + v }
		let cfg = { speed: 2.5, name: "hero" }
		cfg.speed = cfg.speed * 2
		let keys = len(cfg)
		let k = 0
		for key in cfg { k = k + 1 }
		let s = cfg.name
	`); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	checks := map[string]float64{"total": 24, "keys": 2, "k": 2}
	for name, want := range checks {
		if v, _ := vm.GetGlobal(name); v.AsFloat() != want {
			t.Fatalf("%s = %v，期望 %v", name, v.AsFloat(), want)
		}
	}
	if v, _ := vm.GetGlobal("cfg"); v.Kind() != game.ScriptObject {
		t.Fatalf("cfg 应是对象")
	} else if sp := v.AsObject()["speed"]; sp.AsFloat() != 5 {
		t.Fatalf("cfg.speed = %v", sp.AsFloat())
	}
}

// TestScriptStdlib 内置 math 与字符串函数。
func TestScriptStdlib(t *testing.T) {
	vm := game.NewScriptVM()
	if err := vm.Exec(`
		let a = math.floor(2.7)
		let b = math.max(3, 8)
		let c = math.clamp(12, 0, 10)
		let d = math.sqrt(16)
		let s = str(42) + "!"
		let l = len("hello")
	`); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	wants := map[string]float64{"a": 2, "b": 8, "c": 10, "d": 4, "l": 5}
	for name, want := range wants {
		if v, _ := vm.GetGlobal(name); v.AsFloat() != want {
			t.Fatalf("%s = %v，期望 %v", name, v.AsFloat(), want)
		}
	}
	if v, _ := vm.GetGlobal("s"); v.AsString() != "42!" {
		t.Fatalf("s = %q", v.AsString())
	}
}

// TestScriptErrors 语法错误与死循环保护。
func TestScriptErrors(t *testing.T) {
	vm := game.NewScriptVM()
	err := vm.Exec("let = 3")
	if err == nil || !strings.Contains(err.Error(), "第 1 行") {
		t.Fatalf("语法错误应带行号: %v", err)
	}
	err = vm.Exec("let a = \n\n  1 +")
	if err == nil {
		t.Fatalf("不完整表达式应报错")
	}
	// 死循环应被步数上限截断
	vm2 := game.NewScriptVM()
	vm2.MaxSteps = 5000
	err = vm2.Exec("let i = 0\nwhile true { i = i + 1 }")
	if err == nil || !strings.Contains(err.Error(), "步数") {
		t.Fatalf("死循环应被拦截: %v", err)
	}
	// 未定义变量
	vm3 := game.NewScriptVM()
	if err := vm3.Exec("let a = nope + 1"); err == nil {
		t.Fatalf("未定义变量应报错")
	}
	vm3.Strict = false
	if err := vm3.Exec("let a = nope + 1"); err != nil {
		t.Fatalf("宽松模式不应报错: %v", err)
	}
}

// TestScriptEval 单表达式求值与分支/continue。
func TestScriptEval(t *testing.T) {
	vm := game.NewScriptVM()
	v, err := vm.Eval("1 + 2 * 3")
	if err != nil || v.AsFloat() != 7 {
		t.Fatalf("Eval = %v err=%v", v.AsFloat(), err)
	}
	if err := vm.Exec(`
		let sum = 0
		for i in range(10) {
			if i % 2 == 0 { continue }
			if i > 7 { break }
			sum = sum + i
		}
	`); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if v, _ := vm.GetGlobal("sum"); v.AsFloat() != 1+3+5+7 {
		t.Fatalf("sum = %v", v.AsFloat())
	}
}

// TestScriptBindEngine 引擎绑定：增删/移动/查询实体、相机与输入。
func TestScriptBindEngine(t *testing.T) {
	eng := game.NewEngine(160, 120)
	vm := game.NewScriptVM()
	vm.BindEngine(eng)
	if err := vm.Exec(`
		let id = entity_add("hero", 1, 0, 2)
		entity_move(id, 0, 1, 0)
		let p = entity_pos(id)
		let n = entity_count()
		let found = entity_find("hero")
		let name = entity_name(id)
		camera_set_pos(0, 5, 10)
		camera_look_at(0, 0, 0)
	`); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if v, _ := vm.GetGlobal("p"); v.Kind() != game.ScriptObject {
		t.Fatalf("entity_pos 应返回 {x,y,z} 对象")
	} else {
		pos := v.AsObject()
		if pos["y"].AsFloat() != 1 || pos["x"].AsFloat() != 1 || pos["z"].AsFloat() != 2 {
			t.Fatalf("entity 位置错误: %v", pos)
		}
	}
	if v, _ := vm.GetGlobal("n"); v.AsFloat() < 2 {
		t.Fatalf("实体数应至少 2（root + hero），实际 %v", v.AsFloat())
	}
	if v, _ := vm.GetGlobal("found"); v.AsFloat() < 0 {
		t.Fatalf("应能找到 hero")
	}
	if v, _ := vm.GetGlobal("name"); v.AsString() != "hero" {
		t.Fatalf("实体名 %q", v.AsString())
	}
	if eng.Scene.Camera.Pos.Y != 5 || eng.Scene.Camera.Target.Len() != 0 {
		t.Fatalf("相机未被脚本修改: pos=%v target=%v", eng.Scene.Camera.Pos, eng.Scene.Camera.Target)
	}

	// 删除实体
	if err := vm.Exec(`
		let id = entity_find("hero")
		let ok = entity_remove(id)
		let after = entity_find("hero")
	`); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if v, _ := vm.GetGlobal("ok"); !v.AsBool() {
		t.Fatalf("entity_remove 应成功")
	}
	if v, _ := vm.GetGlobal("after"); v.AsFloat() != -1 {
		t.Fatalf("删除后应找不到 hero，实际 %v", v.AsFloat())
	}

	// 输入查询
	eng.Input.Keys["w"] = true
	if err := vm.Exec(`let w = key_down("w") let q = key_down("q")`); err != nil {
		t.Fatalf("输入查询失败: %v", err)
	}
	if v, _ := vm.GetGlobal("w"); !v.AsBool() {
		t.Fatalf("w 应为按下")
	}
	if v, _ := vm.GetGlobal("q"); v.AsBool() {
		t.Fatalf("q 不应按下")
	}
}
