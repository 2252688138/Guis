// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// 通用能力：log / fs / http。
//
//	log.info("启动完成")
//	local s = fs.read("data.txt")
//	fs.write("out.txt", s)
//	local code, body = http.get("https://example.com")
//
// 安全边界：fs.* 只能访问 Options.WorkDir 之下的路径（绝对路径也必须在里面）；
// http.* 只允许 http/https，有超时；NoFS / NoNet 时直接报错。

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// bindSys 注入 log / fs / http 命名空间。
func (e *Engine) bindSys() {
	lg := e.L.NewTable()
	lg.RawSetString("info", e.logFunc("info"))
	lg.RawSetString("warn", e.logFunc("warn"))
	lg.RawSetString("error", e.logFunc("error"))
	e.L.SetGlobal("log", lg)

	f := e.L.NewTable()
	f.RawSetString("read", e.L.NewFunction(e.luaFSReadLimit))
	f.RawSetString("write", e.L.NewFunction(e.luaFSWrite))
	f.RawSetString("append", e.L.NewFunction(e.luaFSAppend))
	f.RawSetString("exists", e.L.NewFunction(e.luaFSExists))
	f.RawSetString("list", e.L.NewFunction(e.luaFSList))
	f.RawSetString("remove", e.L.NewFunction(e.luaFSRemove))
	f.RawSetString("mkdir", e.L.NewFunction(e.luaFSMkdir))
	f.RawSetString("rename", e.L.NewFunction(e.luaFSRename))
	f.RawSetString("copy", e.L.NewFunction(e.luaFSCopy))
	f.RawSetString("stat", e.L.NewFunction(e.luaFSStat))
	e.L.SetGlobal("fs", f)

	h := e.L.NewTable()
	h.RawSetString("get", e.L.NewFunction(e.luaHttpGet))
	h.RawSetString("post", e.L.NewFunction(e.luaHttpPost))
	h.RawSetString("put", e.L.NewFunction(e.luaHttpPut))
	h.RawSetString("delete", e.L.NewFunction(e.luaHttpDelete))
	h.RawSetString("request", e.L.NewFunction(e.luaHttpRequest))
	h.RawSetString("async", e.L.NewFunction(e.luaHttpAsync))
	e.L.SetGlobal("http", h)
}

// logFunc 造 log.info / log.warn / log.error。
func (e *Engine) logFunc(level string) *lua.LFunction {
	return e.L.NewFunction(func(L *lua.LState) int {
		parts := make([]string, 0, L.GetTop())
		for i := 1; i <= L.GetTop(); i++ {
			parts = append(parts, L.Get(i).String())
		}
		e.logf(level, "%s", strings.Join(parts, " "))
		return 0
	})
}

// resolve 把脚本给的路径钳到 WorkDir 下面。
func (e *Engine) resolve(p string) (string, error) {
	if e.opt.NoFS {
		return "", errors.New("fs: 已关闭（Options.NoFS）")
	}
	base := e.opt.WorkDir
	if base == "" {
		base, _ = os.Getwd()
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("fs: 工作目录无效: %w", err)
	}
	full := p
	if !filepath.IsAbs(full) {
		full = filepath.Join(base, p)
	}
	full = filepath.Clean(full)
	rel, err := filepath.Rel(base, full)
	if err != nil {
		return "", fmt.Errorf("fs: 路径 %q 不合法: %w", p, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fs: 路径 %q 超出工作目录 %q", p, base)
	}
	return full, nil
}

// luaFSRead fs.read(path) → 内容。
func (e *Engine) luaFSRead(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	b, err := os.ReadFile(p)
	if err != nil {
		L.RaiseError("fs.read: %v", err)
		return 0
	}
	L.Push(lua.LString(string(b)))
	return 1
}

// luaFSWrite fs.write(path, data)：整文件覆盖。
func (e *Engine) luaFSWrite(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	data := L.CheckString(2)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		L.RaiseError("fs.write: %v", err)
		return 0
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		L.RaiseError("fs.write: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// luaFSAppend fs.append(path, data)。
func (e *Engine) luaFSAppend(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		L.RaiseError("fs.append: %v", err)
		return 0
	}
	defer f.Close()
	if _, err := f.WriteString(L.CheckString(2)); err != nil {
		L.RaiseError("fs.append: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// luaFSExists fs.exists(path) → bool。
func (e *Engine) luaFSExists(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	_, statErr := os.Stat(p)
	L.Push(lua.LBool(statErr == nil))
	return 1
}

// luaFSList fs.list(dir) → 名字数组。
func (e *Engine) luaFSList(L *lua.LState) int {
	p, err := e.resolve(L.OptString(1, "."))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		L.RaiseError("fs.list: %v", err)
		return 0
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		name := ent.Name()
		if ent.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	t := L.NewTable()
	for i, name := range names {
		t.RawSetInt(i+1, lua.LString(name))
	}
	L.Push(t)
	return 1
}

// luaFSRemove fs.remove(path)。
func (e *Engine) luaFSRemove(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	if err := os.Remove(p); err != nil {
		L.RaiseError("fs.remove: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

