// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package script

// fs 扩展：mkdir / rename / copy / stat / 带体积上限的 read。
// 所有路径照旧经过 resolve() 钳制在 Options.WorkDir 之下。

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// luaFSReadLimit fs.read(path[, maxBytes]) → 内容（超过上限报错）。
func (e *Engine) luaFSReadLimit(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	limit := e.opt.FSMaxReadBytes
	if limit <= 0 {
		limit = defaultFSMaxReadBytes
	}
	if n := luaToInt(L.Get(2)); n > 0 {
		limit = n
	}
	st, err := os.Stat(p)
	if err != nil {
		L.RaiseError("fs.read: %v", err)
		return 0
	}
	if st.Size() > limit {
		L.RaiseError("fs.read: %s 有 %d 字节，超过上限 %d（第二个参数可调）", p, st.Size(), limit)
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

// luaFSMkdir fs.mkdir(path[, recursive]) → true。
func (e *Engine) luaFSMkdir(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	recursive := true
	if L.GetTop() >= 2 {
		recursive = lua.LVAsBool(L.Get(2))
	}
	if recursive {
		err = os.MkdirAll(p, 0o755)
	} else {
		err = os.Mkdir(p, 0o755)
	}
	if err != nil {
		L.RaiseError("fs.mkdir: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// luaFSRename fs.rename(old, new) → true。
func (e *Engine) luaFSRename(L *lua.LState) int {
	from, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	to, err := e.resolve(L.CheckString(2))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	if err := os.Rename(from, to); err != nil {
		L.RaiseError("fs.rename: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// luaFSCopy fs.copy(src, dst) → true。
func (e *Engine) luaFSCopy(L *lua.LState) int {
	from, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	to, err := e.resolve(L.CheckString(2))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	if err := copyFile(from, to); err != nil {
		L.RaiseError("fs.copy: %v", err)
		return 0
	}
	L.Push(lua.LTrue)
	return 1
}

// luaFSStat fs.stat(path) → { size, dir, mtime }。
func (e *Engine) luaFSStat(L *lua.LState) int {
	p, err := e.resolve(L.CheckString(1))
	if err != nil {
		L.RaiseError("%v", err)
		return 0
	}
	st, err := os.Stat(p)
	if err != nil {
		L.RaiseError("fs.stat: %v", err)
		return 0
	}
	t := L.NewTable()
	t.RawSetString("size", lua.LNumber(st.Size()))
	t.RawSetString("mtime", lua.LNumber(st.ModTime().Unix()))
	t.RawSetString("dir", lua.LBool(st.IsDir()))
	t.RawSetString("name", lua.LString(st.Name()))
	L.Push(t)
	return 1
}

// copyFile 文件复制（流式，不整体进内存）。
func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("写入 %s: %w", out.Name(), err)
	}
	return out.Close()
}
