// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 嵌入式脚本虚拟机（树遍历解释器）。
//
//	vm := game.NewScriptVM()
//	if err := vm.Exec(`
//	    let speed = 3.5
//	    fn tick(dt) {
//	        if speed > 0 { speed = speed - dt * 0.1 }
//	    }
//	`); err != nil { ... }
//	vm.Call("tick", game.ScriptNum(dt))
//
// 设计目标是「让策划改数值、写关卡逻辑不用重新编译 Go」：值只有
// nil / bool / number / string / array / object / function，宿主用 Register
// 把 Go 函数暴露给脚本（见 script_bind.go 的引擎绑定）。

// ScriptKind 脚本值类型。
type ScriptKind uint8

const (
	// ScriptNil 空值。
	ScriptNil ScriptKind = iota
	// ScriptBool 布尔。
	ScriptBool
	// ScriptNumber 数值（float64）。
	ScriptNumber
	// ScriptString 字符串。
	ScriptString
	// ScriptArray 数组。
	ScriptArray
	// ScriptObject 对象（字符串键）。
	ScriptObject
	// ScriptFunction 脚本函数。
	ScriptFunction
	// ScriptNative 宿主注册的原生函数。
	ScriptNative
)

// ScriptValue 脚本值。
type ScriptValue struct {
	kind   ScriptKind
	num    float64
	b      bool
	str    string
	arr    []ScriptValue
	obj    map[string]ScriptValue
	fn     *scriptFunc
	native func(args []ScriptValue) (ScriptValue, error)
}

// 构造值的便捷函数。

// ScriptNilValue 空值。
func ScriptNilValue() ScriptValue { return ScriptValue{kind: ScriptNil} }

// ScriptNum 数值。
func ScriptNum(f float64) ScriptValue { return ScriptValue{kind: ScriptNumber, num: f} }

// ScriptBoolValue 布尔。
func ScriptBoolValue(b bool) ScriptValue { return ScriptValue{kind: ScriptBool, b: b} }

// ScriptStr 字符串。
func ScriptStr(s string) ScriptValue { return ScriptValue{kind: ScriptString, str: s} }

// ScriptArrayValue 数组。
func ScriptArrayValue(items []ScriptValue) ScriptValue {
	if items == nil {
		items = []ScriptValue{}
	}
	return ScriptValue{kind: ScriptArray, arr: items}
}

// ScriptObjectValue 对象。
func ScriptObjectValue(m map[string]ScriptValue) ScriptValue {
	if m == nil {
		m = map[string]ScriptValue{}
	}
	return ScriptValue{kind: ScriptObject, obj: m}
}

// Kind 值类型。
func (v ScriptValue) Kind() ScriptKind { return v.kind }

// IsNil 是否空值。
func (v ScriptValue) IsNil() bool { return v.kind == ScriptNil }

// AsFloat 取数值（bool / nil / 数字字符串会做宽松转换）。
func (v ScriptValue) AsFloat() float64 {
	switch v.kind {
	case ScriptNumber:
		return v.num
	case ScriptBool:
		if v.b {
			return 1
		}
		return 0
	case ScriptString:
		if f, err := strconv.ParseFloat(strings.TrimSpace(v.str), 64); err == nil {
			return f
		}
	}
	return 0
}

// AsString 转字符串。
func (v ScriptValue) AsString() string {
	switch v.kind {
	case ScriptNil:
		return "nil"
	case ScriptString:
		return v.str
	case ScriptNumber:
		return formatScriptNum(v.num)
	case ScriptBool:
		if v.b {
			return "true"
		}
		return "false"
	case ScriptArray:
		var sb strings.Builder
		sb.WriteByte('[')
		for i, it := range v.arr {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(it.AsString())
		}
		sb.WriteByte(']')
		return sb.String()
	case ScriptObject:
		var sb strings.Builder
		sb.WriteByte('{')
		first := true
		for k, it := range v.obj {
			if !first {
				sb.WriteString(", ")
			}
			first = false
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(it.AsString())
		}
		sb.WriteByte('}')
		return sb.String()
	case ScriptFunction, ScriptNative:
		return "<函数>"
	}
	return ""
}

// AsBool 真值判定（nil / false / 0 / "" / 空数组为假）。
func (v ScriptValue) AsBool() bool {
	switch v.kind {
	case ScriptNil:
		return false
	case ScriptBool:
		return v.b
	case ScriptNumber:
		return v.num != 0
	case ScriptString:
		return v.str != ""
	case ScriptArray:
		return len(v.arr) > 0
	case ScriptObject:
		return len(v.obj) > 0
	}
	return true
}

// AsArray 取数组内容（非数组返回 nil）。
func (v ScriptValue) AsArray() []ScriptValue { return v.arr }

// AsObject 取对象内容（非对象返回 nil）。
func (v ScriptValue) AsObject() map[string]ScriptValue { return v.obj }

// Interface 转成等价的 Go 值（便于宿主使用）。
func (v ScriptValue) Interface() any {
	switch v.kind {
	case ScriptNil:
		return nil
	case ScriptBool:
		return v.b
	case ScriptNumber:
		return v.num
	case ScriptString:
		return v.str
	case ScriptArray:
		out := make([]any, len(v.arr))
		for i, it := range v.arr {
			out[i] = it.Interface()
		}
		return out
	case ScriptObject:
		out := make(map[string]any, len(v.obj))
		for k, it := range v.obj {
			out[k] = it.Interface()
		}
		return out
	}
	return nil
}

func formatScriptNum(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// ---------------------------------------------------------------- 环境

type scriptEnv struct {
	vars   map[string]ScriptValue
	parent *scriptEnv
}

func newScriptEnv(parent *scriptEnv) *scriptEnv {
	return &scriptEnv{vars: map[string]ScriptValue{}, parent: parent}
}

func (e *scriptEnv) get(name string) (ScriptValue, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return ScriptValue{}, false
}

func (e *scriptEnv) define(name string, v ScriptValue) { e.vars[name] = v }

func (e *scriptEnv) assign(name string, v ScriptValue) bool {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return true
		}
	}
	return false
}

// scriptFunc 脚本函数。
type scriptFunc struct {
	name   string
	params []string
	body   []scriptStmt
	env    *scriptEnv
}

// 控制流哨兵。
type scriptReturnSignal struct{ val ScriptValue }

func (s scriptReturnSignal) Error() string { return "return" }

var (
	errScriptBreak    = errors.New("break")
	errScriptContinue = errors.New("continue")
)

// ScriptVM 脚本虚拟机。
type ScriptVM struct {
	globals *scriptEnv
	// MaxSteps 单次执行的指令上限（防死循环，默认 200000）。
	MaxSteps int
	// OnPrint 处理 print 输出（默认追加到 Output）。
	OnPrint func(s string)
	// Strict 未定义变量访问是否报错（默认 true）。
	Strict bool

	steps  int
	output []string
}

// NewScriptVM 创建虚拟机并注册标准库（print / math / 数组工具）。
func NewScriptVM() *ScriptVM {
	vm := &ScriptVM{
		globals:  newScriptEnv(nil),
		MaxSteps: 200000,
		Strict:   true,
	}
	vm.registerStdlib()
	return vm
}

// Register 注册一个原生函数（参数与返回值皆为 ScriptValue）。
func (vm *ScriptVM) Register(name string, fn func(args []ScriptValue) (ScriptValue, error)) {
	if vm == nil || name == "" || fn == nil {
		return
	}
	vm.globals.define(name, ScriptValue{kind: ScriptNative, native: fn})
}

// Registers 批量注册。
func (vm *ScriptVM) Registers(fns map[string]func(args []ScriptValue) (ScriptValue, error)) {
	for k, f := range fns {
		vm.Register(k, f)
	}
}

// SetGlobal 设置全局变量。
func (vm *ScriptVM) SetGlobal(name string, v ScriptValue) {
	if vm == nil || name == "" {
		return
	}
	if !vm.globals.assign(name, v) {
		vm.globals.define(name, v)
	}
}

// GetGlobal 读取全局变量。
func (vm *ScriptVM) GetGlobal(name string) (ScriptValue, bool) {
	if vm == nil {
		return ScriptValue{}, false
	}
	return vm.globals.get(name)
}

// Output 取并清空 print 缓冲。
func (vm *ScriptVM) Output() []string {
	if vm == nil {
		return nil
	}
	out := vm.output
	vm.output = nil
	return out
}

// Exec 解析并执行一段脚本。
func (vm *ScriptVM) Exec(src string) error {
	if vm == nil {
		return errors.New("game: 脚本虚拟机为空")
	}
	stmts, err := scriptParse(src)
	if err != nil {
		return err
	}
	return vm.ExecStmts(stmts)
}

// ExecStmts 执行已解析的语句列表。
func (vm *ScriptVM) ExecStmts(stmts []scriptStmt) error {
	if vm == nil {
		return errors.New("game: 脚本虚拟机为空")
	}
	vm.steps = 0
	err := vm.execBlock(stmts, vm.globals)
	if err != nil {
		var ret scriptReturnSignal
		if errors.As(err, &ret) {
			return nil // 顶层 return 视为结束
		}
		if errors.Is(err, errScriptBreak) || errors.Is(err, errScriptContinue) {
			return errors.New("脚本：break/continue 不在循环内")
		}
		return err
	}
	return nil
}

// Eval 解析并求值单个表达式。
func (vm *ScriptVM) Eval(expr string) (ScriptValue, error) {
	if vm == nil {
		return ScriptValue{}, errors.New("game: 脚本虚拟机为空")
	}
	stmts, err := scriptParse(expr)
	if err != nil {
		return ScriptValue{}, err
	}
	if len(stmts) != 1 {
		return ScriptValue{}, errors.New("脚本：Eval 需要单个表达式")
	}
	es, ok := stmts[0].(*scriptExprStmt)
	if !ok {
		return ScriptValue{}, errors.New("脚本：Eval 需要单个表达式")
	}
	vm.steps = 0
	return vm.evalExpr(es.x, vm.globals)
}

// Call 调用脚本里定义的函数。
func (vm *ScriptVM) Call(name string, args ...ScriptValue) (ScriptValue, error) {
	fv, ok := vm.GetGlobal(name)
	if !ok {
		return ScriptValue{}, fmt.Errorf("脚本：未定义函数 %q", name)
	}
	vm.steps = 0
	return vm.callValue(fv, args, name)
}

// Reset 清空全局状态（保留已注册的原生函数无法区分，故整体重建并重注册标准库）。
func (vm *ScriptVM) Reset() {
	if vm == nil {
		return
	}
	vm.globals = newScriptEnv(nil)
	vm.output = nil
	vm.steps = 0
	vm.registerStdlib()
}

// ---------------------------------------------------------------- 执行

func (vm *ScriptVM) tick() error {
	vm.steps++
	if vm.MaxSteps > 0 && vm.steps > vm.MaxSteps {
		return fmt.Errorf("脚本：执行步数超过上限 %d（疑似死循环）", vm.MaxSteps)
	}
	return nil
}

func (vm *ScriptVM) execBlock(stmts []scriptStmt, env *scriptEnv) error {
	for _, s := range stmts {
		if err := vm.execStmt(s, env); err != nil {
			return err
		}
	}
	return nil
}

func (vm *ScriptVM) execStmt(s scriptStmt, env *scriptEnv) error {
	if err := vm.tick(); err != nil {
		return err
	}
	switch st := s.(type) {
	case *scriptLet:
		v, err := vm.evalExpr(st.value, env)
		if err != nil {
			return err
		}
		env.define(st.name, v)
		return nil
	case *scriptAssign:
		v, err := vm.evalExpr(st.value, env)
		if err != nil {
			return err
		}
		return vm.assignTo(st.target, st.op, v, env, st.line)
	case *scriptExprStmt:
		_, err := vm.evalExpr(st.x, env)
		return err
	case *scriptBlock:
		return vm.execBlock(st.body, newScriptEnv(env))
	case *scriptIf:
		cond, err := vm.evalExpr(st.cond, env)
		if err != nil {
			return err
		}
		if cond.AsBool() {
			return vm.execBlock(st.then, newScriptEnv(env))
		}
		if len(st.els) > 0 {
			return vm.execBlock(st.els, newScriptEnv(env))
		}
		return nil
	case *scriptWhile:
		for {
			if err := vm.tick(); err != nil {
				return err
			}
			cond, err := vm.evalExpr(st.cond, env)
			if err != nil {
				return err
			}
			if !cond.AsBool() {
				return nil
			}
			if err := vm.execBlock(st.body, newScriptEnv(env)); err != nil {
				if errors.Is(err, errScriptBreak) {
					return nil
				}
				if errors.Is(err, errScriptContinue) {
					continue
				}
				return err
			}
		}
	case *scriptFor:
		loopEnv := newScriptEnv(env)
		if st.init != nil {
			if err := vm.execStmt(st.init, loopEnv); err != nil {
				return err
			}
		}
		for {
			if err := vm.tick(); err != nil {
				return err
			}
			if st.cond != nil {
				cond, err := vm.evalExpr(st.cond, loopEnv)
				if err != nil {
					return err
				}
				if !cond.AsBool() {
					return nil
				}
			}
			if err := vm.execBlock(st.body, newScriptEnv(loopEnv)); err != nil {
				if errors.Is(err, errScriptBreak) {
					return nil
				}
				if !errors.Is(err, errScriptContinue) {
					return err
				}
			}
			if st.post != nil {
				if err := vm.execStmt(st.post, loopEnv); err != nil {
					return err
				}
			}
		}
	case *scriptForIn:
		iter, err := vm.evalExpr(st.iter, env)
		if err != nil {
			return err
		}
		var items []ScriptValue
		switch iter.kind {
		case ScriptArray:
			items = iter.arr
		case ScriptString:
			for _, r := range iter.str {
				items = append(items, ScriptStr(string(r)))
			}
		case ScriptObject:
			for k := range iter.obj {
				items = append(items, ScriptStr(k))
			}
		case ScriptNil:
			return nil
		default:
			return fmt.Errorf("脚本：for-in 不支持 %s", iter.AsString())
		}
		for _, it := range items {
			if err := vm.tick(); err != nil {
				return err
			}
			loopEnv := newScriptEnv(env)
			loopEnv.define(st.name, it)
			if err := vm.execBlock(st.body, loopEnv); err != nil {
				if errors.Is(err, errScriptBreak) {
					return nil
				}
				if !errors.Is(err, errScriptContinue) {
					return err
				}
			}
		}
		return nil
	case *scriptFnDecl:
		fn := &scriptFunc{name: st.name, params: st.params, body: st.body, env: env}
		env.define(st.name, ScriptValue{kind: ScriptFunction, fn: fn})
		return nil
	case *scriptReturn:
		var v ScriptValue
		if st.value != nil {
			var err error
			v, err = vm.evalExpr(st.value, env)
			if err != nil {
				return err
			}
		}
		return scriptReturnSignal{val: v}
	case *scriptBreak:
		return errScriptBreak
	case *scriptContinue:
		return errScriptContinue
	}
	return fmt.Errorf("脚本：未知语句类型 %T", s)
}

// assignTo 处理 x = / obj.k = / arr[i] = 以及复合赋值。
func (vm *ScriptVM) assignTo(target scriptExpr, op string, val ScriptValue, env *scriptEnv, line int) error {
	cur := func() (ScriptValue, error) { return vm.evalExpr(target, env) }
	if op != "=" {
		old, err := cur()
		if err != nil {
			return err
		}
		binOp := strings.TrimSuffix(op, "=")
		val = scriptBinaryOp(binOp, old, val)
	}
	switch t := target.(type) {
	case *scriptIdent:
		if !env.assign(t.name, val) {
			if vm.Strict {
				return fmt.Errorf("脚本第 %d 行：未定义的变量 %q", line, t.name)
			}
			env.define(t.name, val)
		}
		return nil
	case *scriptIndex:
		obj, err := vm.evalExpr(t.obj, env)
		if err != nil {
			return err
		}
		idx, err := vm.evalExpr(t.idx, env)
		if err != nil {
			return err
		}
		switch obj.kind {
		case ScriptArray:
			i := int(idx.AsFloat())
			if i < 0 || i >= len(obj.arr) {
				return fmt.Errorf("脚本第 %d 行：数组下标越界 %d", line, i)
			}
			obj.arr[i] = val
			return nil
		case ScriptObject:
			obj.obj[idx.AsString()] = val
			return nil
		}
		return fmt.Errorf("脚本第 %d 行：该值不可按下标赋值", line)
	case *scriptMember:
		obj, err := vm.evalExpr(t.obj, env)
		if err != nil {
			return err
		}
		if obj.kind != ScriptObject {
			return fmt.Errorf("脚本第 %d 行：该值不可按成员赋值", line)
		}
		obj.obj[t.name] = val
		return nil
	}
	return fmt.Errorf("脚本第 %d 行：不可赋值的目标", line)
}

// evalExpr 求值表达式。
func (vm *ScriptVM) evalExpr(e scriptExpr, env *scriptEnv) (ScriptValue, error) {
	if err := vm.tick(); err != nil {
		return ScriptValue{}, err
	}
	switch x := e.(type) {
	case *scriptNumLit:
		return ScriptNum(x.v), nil
	case *scriptStrLit:
		return ScriptStr(x.v), nil
	case *scriptBoolLit:
		return ScriptBoolValue(x.v), nil
	case *scriptNilLit:
		return ScriptNilValue(), nil
	case *scriptIdent:
		if v, ok := env.get(x.name); ok {
			return v, nil
		}
		if vm.Strict {
			return ScriptValue{}, fmt.Errorf("脚本第 %d 行：未定义的变量 %q", x.line, x.name)
		}
		return ScriptNilValue(), nil
	case *scriptArrayLit:
		items := make([]ScriptValue, 0, len(x.items))
		for _, it := range x.items {
			v, err := vm.evalExpr(it, env)
			if err != nil {
				return ScriptValue{}, err
			}
			items = append(items, v)
		}
		return ScriptArrayValue(items), nil
	case *scriptObjectLit:
		m := make(map[string]ScriptValue, len(x.keys))
		for i, k := range x.keys {
			v, err := vm.evalExpr(x.vals[i], env)
			if err != nil {
				return ScriptValue{}, err
			}
			m[k] = v
		}
		return ScriptObjectValue(m), nil
	case *scriptUnary:
		v, err := vm.evalExpr(x.x, env)
		if err != nil {
			return ScriptValue{}, err
		}
		switch x.op {
		case "-":
			return ScriptNum(-v.AsFloat()), nil
		case "!":
			return ScriptBoolValue(!v.AsBool()), nil
		}
		return ScriptValue{}, fmt.Errorf("脚本：未知一元运算符 %q", x.op)
	case *scriptBinary:
		if x.op == "&&" {
			l, err := vm.evalExpr(x.l, env)
			if err != nil {
				return ScriptValue{}, err
			}
			if !l.AsBool() {
				return ScriptBoolValue(false), nil
			}
			r, err := vm.evalExpr(x.r, env)
			if err != nil {
				return ScriptValue{}, err
			}
			return ScriptBoolValue(r.AsBool()), nil
		}
		if x.op == "||" {
			l, err := vm.evalExpr(x.l, env)
			if err != nil {
				return ScriptValue{}, err
			}
			if l.AsBool() {
				return ScriptBoolValue(true), nil
			}
			r, err := vm.evalExpr(x.r, env)
			if err != nil {
				return ScriptValue{}, err
			}
			return ScriptBoolValue(r.AsBool()), nil
		}
		l, err := vm.evalExpr(x.l, env)
		if err != nil {
			return ScriptValue{}, err
		}
		r, err := vm.evalExpr(x.r, env)
		if err != nil {
			return ScriptValue{}, err
		}
		return scriptBinaryOp(x.op, l, r), nil
	case *scriptCall:
		callee, err := vm.evalExpr(x.callee, env)
		if err != nil {
			return ScriptValue{}, err
		}
		args := make([]ScriptValue, 0, len(x.args))
		for _, a := range x.args {
			v, err := vm.evalExpr(a, env)
			if err != nil {
				return ScriptValue{}, err
			}
			args = append(args, v)
		}
		name := ""
		if id, ok := x.callee.(*scriptIdent); ok {
			name = id.name
		} else if mem, ok := x.callee.(*scriptMember); ok {
			name = mem.name
		}
		return vm.callValue(callee, args, name)
	case *scriptIndex:
		obj, err := vm.evalExpr(x.obj, env)
		if err != nil {
			return ScriptValue{}, err
		}
		idx, err := vm.evalExpr(x.idx, env)
		if err != nil {
			return ScriptValue{}, err
		}
		switch obj.kind {
		case ScriptArray:
			i := int(idx.AsFloat())
			if i < 0 || i >= len(obj.arr) {
				return ScriptNilValue(), nil
			}
			return obj.arr[i], nil
		case ScriptObject:
			if v, ok := obj.obj[idx.AsString()]; ok {
				return v, nil
			}
			return ScriptNilValue(), nil
		case ScriptString:
			i := int(idx.AsFloat())
			rs := []rune(obj.str)
			if i < 0 || i >= len(rs) {
				return ScriptNilValue(), nil
			}
			return ScriptStr(string(rs[i])), nil
		}
		return ScriptValue{}, fmt.Errorf("脚本第 %d 行：该值不可按下标读取", x.line)
	case *scriptMember:
		obj, err := vm.evalExpr(x.obj, env)
		if err != nil {
			return ScriptValue{}, err
		}
		switch obj.kind {
		case ScriptObject:
			if v, ok := obj.obj[x.name]; ok {
				return v, nil
			}
			return ScriptNilValue(), nil
		}
		return ScriptNilValue(), nil
	}
	return ScriptValue{}, fmt.Errorf("脚本：未知表达式 %T", e)
}

// callValue 调用函数值。
func (vm *ScriptVM) callValue(callee ScriptValue, args []ScriptValue, name string) (ScriptValue, error) {
	switch callee.kind {
	case ScriptNative:
		if callee.native == nil {
			return ScriptValue{}, fmt.Errorf("脚本：函数 %q 为空", name)
		}
		v, err := callee.native(args)
		if err != nil {
			return ScriptValue{}, fmt.Errorf("脚本：调用 %q 失败: %w", name, err)
		}
		return v, nil
	case ScriptFunction:
		fn := callee.fn
		if fn == nil {
			return ScriptValue{}, fmt.Errorf("脚本：函数 %q 为空", name)
		}
		callEnv := newScriptEnv(fn.env)
		for i, p := range fn.params {
			if i < len(args) {
				callEnv.define(p, args[i])
			} else {
				callEnv.define(p, ScriptNilValue())
			}
		}
		err := vm.execBlock(fn.body, callEnv)
		if err != nil {
			var ret scriptReturnSignal
			if errors.As(err, &ret) {
				return ret.val, nil
			}
			return ScriptValue{}, err
		}
		return ScriptNilValue(), nil
	}
	if name != "" {
		return ScriptValue{}, fmt.Errorf("脚本：%q 不是可调用函数", name)
	}
	return ScriptValue{}, errors.New("脚本：该值不是可调用函数")
}

// ---------------------------------------------------------------- 运算

// scriptBinaryOp 二元运算（字符串拼接 / 数值比较 / 相等判定）。
func scriptBinaryOp(op string, l, r ScriptValue) ScriptValue {
	switch op {
	case "+":
		if l.kind == ScriptString || r.kind == ScriptString {
			if l.kind == ScriptString || l.kind == ScriptNumber || l.kind == ScriptBool || l.kind == ScriptNil {
				return ScriptStr(l.AsString() + r.AsString())
			}
		}
		if l.kind == ScriptArray && r.kind == ScriptArray {
			out := make([]ScriptValue, 0, len(l.arr)+len(r.arr))
			out = append(out, l.arr...)
			out = append(out, r.arr...)
			return ScriptArrayValue(out)
		}
		return ScriptNum(l.AsFloat() + r.AsFloat())
	case "-":
		return ScriptNum(l.AsFloat() - r.AsFloat())
	case "*":
		return ScriptNum(l.AsFloat() * r.AsFloat())
	case "/":
		d := r.AsFloat()
		if d == 0 {
			return ScriptNum(0)
		}
		return ScriptNum(l.AsFloat() / d)
	case "%":
		d := r.AsFloat()
		if d == 0 {
			return ScriptNum(0)
		}
		return ScriptNum(math.Mod(l.AsFloat(), d))
	case "==":
		return ScriptBoolValue(scriptEqual(l, r))
	case "!=":
		return ScriptBoolValue(!scriptEqual(l, r))
	case "<", "<=", ">", ">=":
		if l.kind == ScriptString && r.kind == ScriptString {
			switch op {
			case "<":
				return ScriptBoolValue(l.str < r.str)
			case "<=":
				return ScriptBoolValue(l.str <= r.str)
			case ">":
				return ScriptBoolValue(l.str > r.str)
			default:
				return ScriptBoolValue(l.str >= r.str)
			}
		}
		a, b := l.AsFloat(), r.AsFloat()
		switch op {
		case "<":
			return ScriptBoolValue(a < b)
		case "<=":
			return ScriptBoolValue(a <= b)
		case ">":
			return ScriptBoolValue(a > b)
		default:
			return ScriptBoolValue(a >= b)
		}
	}
	return ScriptNilValue()
}

// scriptEqual 相等比较（数字与字符串按值，数组/对象按引用内容逐项比较）。
func scriptEqual(l, r ScriptValue) bool {
	if l.kind == ScriptNumber || r.kind == ScriptNumber {
		if l.kind == ScriptNil || r.kind == ScriptNil {
			return l.kind == r.kind
		}
		return l.AsFloat() == r.AsFloat()
	}
	if l.kind != r.kind {
		return false
	}
	switch l.kind {
	case ScriptNil:
		return true
	case ScriptBool:
		return l.b == r.b
	case ScriptString:
		return l.str == r.str
	case ScriptArray:
		if len(l.arr) != len(r.arr) {
			return false
		}
		for i := range l.arr {
			if !scriptEqual(l.arr[i], r.arr[i]) {
				return false
			}
		}
		return true
	case ScriptObject:
		if len(l.obj) != len(r.obj) {
			return false
		}
		for k, v := range l.obj {
			rv, ok := r.obj[k]
			if !ok || !scriptEqual(v, rv) {
				return false
			}
		}
		return true
	}
	return false
}

// ---------------------------------------------------------------- 标准库

func (vm *ScriptVM) registerStdlib() {
	vm.Register("print", func(args []ScriptValue) (ScriptValue, error) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, a.AsString())
		}
		s := strings.Join(parts, " ")
		if vm.OnPrint != nil {
			vm.OnPrint(s)
		} else {
			vm.output = append(vm.output, s)
		}
		return ScriptNilValue(), nil
	})
	vm.Register("str", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 {
			return ScriptStr(""), nil
		}
		return ScriptStr(args[0].AsString()), nil
	})
	vm.Register("num", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 {
			return ScriptNum(0), nil
		}
		return ScriptNum(args[0].AsFloat()), nil
	})
	vm.Register("len", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 {
			return ScriptNum(0), nil
		}
		switch args[0].kind {
		case ScriptArray:
			return ScriptNum(float64(len(args[0].arr))), nil
		case ScriptString:
			return ScriptNum(float64(len([]rune(args[0].str)))), nil
		case ScriptObject:
			return ScriptNum(float64(len(args[0].obj))), nil
		}
		return ScriptNum(0), nil
	})
	vm.Register("append", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) < 2 {
			return ScriptNilValue(), nil
		}
		src := args[0].arr
		out := make([]ScriptValue, len(src), len(src)+1)
		copy(out, src)
		out = append(out, args[1])
		return ScriptArrayValue(out), nil
	})
	vm.Register("keys", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 || args[0].kind != ScriptObject {
			return ScriptArrayValue(nil), nil
		}
		out := make([]ScriptValue, 0, len(args[0].obj))
		for k := range args[0].obj {
			out = append(out, ScriptStr(k))
		}
		return ScriptArrayValue(out), nil
	})
	vm.Register("type", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 {
			return ScriptStr("nil"), nil
		}
		switch args[0].kind {
		case ScriptNil:
			return ScriptStr("nil"), nil
		case ScriptBool:
			return ScriptStr("bool"), nil
		case ScriptNumber:
			return ScriptStr("number"), nil
		case ScriptString:
			return ScriptStr("string"), nil
		case ScriptArray:
			return ScriptStr("array"), nil
		case ScriptObject:
			return ScriptStr("object"), nil
		default:
			return ScriptStr("function"), nil
		}
	})
	vm.Register("range", func(args []ScriptValue) (ScriptValue, error) {
		if len(args) == 0 {
			return ScriptArrayValue(nil), nil
		}
		n := int(args[0].AsFloat())
		if n < 0 {
			n = 0
		}
		if n > 100000 {
			n = 100000
		}
		out := make([]ScriptValue, n)
		for i := 0; i < n; i++ {
			out[i] = ScriptNum(float64(i))
		}
		return ScriptArrayValue(out), nil
	})
	// math 对象
	mathObj := map[string]ScriptValue{}
	native := func(fn func(a []ScriptValue) float64) ScriptValue {
		return ScriptValue{kind: ScriptNative, native: func(args []ScriptValue) (ScriptValue, error) {
			return ScriptNum(fn(args)), nil
		}}
	}
	arg := func(args []ScriptValue, i int) float64 {
		if i < len(args) {
			return args[i].AsFloat()
		}
		return 0
	}
	mathObj["pi"] = ScriptNum(math.Pi)
	mathObj["sin"] = native(func(a []ScriptValue) float64 { return math.Sin(arg(a, 0)) })
	mathObj["cos"] = native(func(a []ScriptValue) float64 { return math.Cos(arg(a, 0)) })
	mathObj["atan2"] = native(func(a []ScriptValue) float64 { return math.Atan2(arg(a, 0), arg(a, 1)) })
	mathObj["sqrt"] = native(func(a []ScriptValue) float64 { return math.Sqrt(math.Max(0, arg(a, 0))) })
	mathObj["abs"] = native(func(a []ScriptValue) float64 { return math.Abs(arg(a, 0)) })
	mathObj["floor"] = native(func(a []ScriptValue) float64 { return math.Floor(arg(a, 0)) })
	mathObj["ceil"] = native(func(a []ScriptValue) float64 { return math.Ceil(arg(a, 0)) })
	mathObj["round"] = native(func(a []ScriptValue) float64 { return math.Round(arg(a, 0)) })
	mathObj["min"] = native(func(a []ScriptValue) float64 {
		if len(a) < 2 {
			return arg(a, 0)
		}
		return math.Min(arg(a, 0), arg(a, 1))
	})
	mathObj["max"] = native(func(a []ScriptValue) float64 {
		if len(a) < 2 {
			return arg(a, 0)
		}
		return math.Max(arg(a, 0), arg(a, 1))
	})
	mathObj["clamp"] = native(func(a []ScriptValue) float64 {
		v, lo, hi := arg(a, 0), arg(a, 1), arg(a, 2)
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	})
	mathObj["lerp"] = native(func(a []ScriptValue) float64 {
		x, y, t := arg(a, 0), arg(a, 1), arg(a, 2)
		return x + (y-x)*t
	})
	vm.globals.define("math", ScriptObjectValue(mathObj))
}
