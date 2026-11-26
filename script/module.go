// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 模块化：受限 require + package.loaded 语义。
//
//	local util = require("util")          -- WorkDir 下 util.lua
//	local sib  = require("./sibling")     -- 相对当前模块所在目录
//	local lib  = require("mylib.util")    -- Options.ModulePath 里的目录
//	local pkg  = require("pkg")           -- 目录包 pkg/init.lua
//
// 安全边界：模块必须落在 WorkDir 或显式配置的 ModulePath 之内；
// 同一模块只执行一次，结果同时写进 package.loaded；
// 热重载脚本时缓存清空，改了子模块也能生效。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// bindModule 注入 require 与只读 package 表（沙箱里没有 package 库）。
func (e *Engine) bindModule() {
	e.loadedT = e.L.NewTable()
	e.L.SetGlobal("require", e.L.NewFunction(e.luaRequire))

	pkg := e.L.NewTable()
	pkg.RawSetString("loaded", e.loadedT)
	pkg.RawSetString("path", lua.LString(e.modulePathString()))
	e.L.SetGlobal("package", pkg)
}

// modulePathString 给 package.path 用的可读描述。
func (e *Engine) modulePathString() string {
	parts := make([]string, 0, 4)
	base := e.opt.WorkDir
	if base == "" {
		base, _ = os.Getwd()
	}
	parts = append(parts, filepath.Join(base, "?.lua"))
	for _, p := range e.opt.ModulePath {
		parts = append(parts, filepath.Join(p, "?.lua"))
	}
	return strings.Join(parts, ";")
}

// luaRequire require(name) → 模块返回值（无返回值时为 true）。
func (e *Engine) luaRequire(L *lua.LState) int {
	name := L.CheckString(1)
	if v, ok := e.modules[name]; ok {
		L.Push(v)
		return 1
	}
	path, err := e.modulePath(name)
	if err != nil {
		L.RaiseError("require: %v", err)
		return 0
	}
	src, err := os.ReadFile(path)
	if err != nil {
		L.RaiseError("require %q: %v", name, err)
		return 0
	}
	fn, err := L.LoadString(string(src))
	if err != nil {
		L.RaiseError("require %q: %v", name, err)
		return 0
	}
	// 先登记占位：循环 require 时对方拿到 true，而不是无限递归。
	e.modules[name] = lua.LTrue
	if e.loadedT != nil {
		e.loadedT.RawSetString(name, lua.LTrue)
	}
	e.modStack = append(e.modStack, path)
	// 已经持有 Lua 锁且在 e.run 的 ctx 内，这里直接调用，不再套一层 run。
	cerr := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, lua.LString(name))
	e.modStack = e.modStack[:len(e.modStack)-1]
	if cerr != nil {
		delete(e.modules, name)
		if e.loadedT != nil {
			e.loadedT.RawSetString(name, lua.LNil)
		}
		L.RaiseError("require %q: %v", name, cerr)
		return 0
	}
	ret := L.Get(-1)
	L.Pop(1)
	if ret == lua.LNil {
		ret = lua.LTrue
	}
	e.modules[name] = ret
	if e.loadedT != nil {
		e.loadedT.RawSetString(name, ret)
	}
	L.Push(ret)
	return 1
}

// modulePath 解析模块名：相对当前模块（./ ../）、WorkDir、ModulePath。
func (e *Engine) modulePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("模块名为空")
	}
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
		base := e.opt.WorkDir
		if base == "" {
			base, _ = os.Getwd()
		}
		abs, err := filepath.Abs(base)
		if err != nil {
			return "", err
		}
		if n := len(e.modStack); n > 0 {
			abs = filepath.Dir(e.modStack[n-1])
		}
		full := filepath.Clean(filepath.Join(abs, name))
		if p, ok := firstExisting(moduleCandidates(full)); ok {
			return p, nil
		}
		return "", fmt.Errorf("找不到模块 %q（相对 %s）", name, abs)
	}

	bases := make([]string, 0, 1+len(e.opt.ModulePath))
	if wd := e.opt.WorkDir; wd != "" {
		bases = append(bases, wd)
	} else if cwd, err := os.Getwd(); err == nil {
		bases = append(bases, cwd)
	}
	bases = append(bases, e.opt.ModulePath...)
	for _, b := range bases {
		abs, err := filepath.Abs(b)
		if err != nil {
			continue
		}
		for _, cand := range moduleCandidates(filepath.Join(abs, name)) {
			if !within(abs, cand) {
				continue // 越界（如 "../../etc/passwd"）直接跳过
			}
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand, nil
			}
		}
	}
	return "", fmt.Errorf("找不到模块 %q（已搜索 %s）", name, strings.Join(bases, ", "))
}

// moduleCandidates 一个模块名的候选文件名。
func moduleCandidates(p string) []string {
	out := []string{p}
	if !strings.HasSuffix(p, ".lua") {
		out = append(out, p+".lua", filepath.Join(p, "init.lua"))
	}
	return out
}

// firstExisting 第一个存在的文件。
func firstExisting(cands []string) (string, bool) {
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, true
		}
	}
	return "", false
}

// within p 是否在 base 之内。
func within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
