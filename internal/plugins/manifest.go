package plugins

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	idRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	nameRe   = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	tplRe    = regexp.MustCompile(`\{\{\s*[a-zA-Z0-9_.-]+\s*\}\}`)
	toolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// ValidID 插件 id 必须是 kebab-case（小写字母 / 数字 / 连字符）。
func ValidID(s string) bool { return idRe.MatchString(s) }

// SanitizeToolName 把任意名称规范成模型可见的工具名。
func SanitizeToolName(s string) string {
	s = nameRe.ReplaceAllString(strings.TrimSpace(s), "_")
	if s == "" {
		return "tool"
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// ReadManifest 读取并校验一个插件目录。
func ReadManifest(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("缺少 plugin.json: %w", err)
	}
	var mf Manifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return nil, fmt.Errorf("plugin.json 解析失败: %w", err)
	}
	// 记录是否显式声明了 permissions（未声明的旧插件保持宽松）
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) == nil {
		_, mf.Permissions.declared = probe["permissions"]
	}
	mf.Dir = dir
	if err := Validate(&mf); err != nil {
		return nil, err
	}
	return &mf, nil
}

// Validate 规范化并校验清单（就地修改）。
func Validate(mf *Manifest) error {
	mf.ID = strings.ToLower(strings.TrimSpace(mf.ID))
	if !ValidID(mf.ID) {
		return fmt.Errorf("插件 id 必须是 kebab-case（小写字母 / 数字 / 连字符）：%q", mf.ID)
	}
	if strings.TrimSpace(mf.Name) == "" {
		mf.Name = mf.ID
	}
	if strings.TrimSpace(mf.Version) == "" {
		mf.Version = "0.0.0"
	}
	seen := map[string]bool{}
	for i := range mf.Tools {
		t := &mf.Tools[i]
		t.Name = strings.TrimSpace(t.Name)
		if t.Name == "" {
			return fmt.Errorf("工具 #%d 缺少 name", i+1)
		}
		if seen[t.Name] {
			return fmt.Errorf("工具名重复：%s", t.Name)
		}
		seen[t.Name] = true
		if strings.TrimSpace(t.Description) == "" {
			return fmt.Errorf("工具 %s 缺少 description", t.Name)
		}
		if len(t.Parameters) == 0 {
			t.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var probe map[string]any
		if err := json.Unmarshal(t.Parameters, &probe); err != nil {
			return fmt.Errorf("工具 %s 的 parameters 不是合法 JSON 对象", t.Name)
		}
		switch strings.ToLower(strings.TrimSpace(t.Type)) {
		case "http":
			t.Type = "http"
			if t.Request == nil || strings.TrimSpace(t.Request.URL) == "" {
				return fmt.Errorf("HTTP 工具 %s 缺少 request.url", t.Name)
			}
			if strings.TrimSpace(t.Request.Method) == "" {
				t.Request.Method = http.MethodGet
			}
			t.Request.Method = strings.ToUpper(strings.TrimSpace(t.Request.Method))
		case "command", "":
			t.Type = "command"
			if strings.TrimSpace(t.Command) == "" {
				return fmt.Errorf("命令工具 %s 缺少 command", t.Name)
			}
		default:
			return fmt.Errorf("工具 %s 的 type 只能是 http 或 command", t.Name)
		}
	}
	seenSrv := map[string]bool{}
	for i := range mf.MCPServers {
		s := &mf.MCPServers[i]
		s.ID = SanitizeToolName(s.ID)
		if s.ID == "" || s.ID == "tool" {
			s.ID = fmt.Sprintf("srv%d", i+1)
		}
		if seenSrv[s.ID] {
			return fmt.Errorf("MCP 服务器 id 重复：%s", s.ID)
		}
		seenSrv[s.ID] = true
		if strings.TrimSpace(s.Command) == "" {
			return fmt.Errorf("MCP 服务器 %s 缺少 command", s.ID)
		}
	}
		if err := validateMeta(mf); err != nil {
		return err
	}
	return validateRuntime(mf)
}

// validateMeta 校验权限、配置、版本约束与信任等级。
func validateMeta(mf *Manifest) error {
	if len(mf.Configuration) > 0 {
		var probe map[string]any
		if err := json.Unmarshal(mf.Configuration, &probe); err != nil {
			return fmt.Errorf("configuration 必须是 JSON 对象: %w", err)
		}
	}
	mf.MinHostVersion = strings.TrimSpace(mf.MinHostVersion)
	if mf.MinHostVersion != "" && CompareVersion(HostVersion, mf.MinHostVersion) < 0 {
		return fmt.Errorf("插件要求宿主版本 >= %s，当前 %s", mf.MinHostVersion, HostVersion)
	}
	trust := strings.ToLower(strings.TrimSpace(mf.Trust))
	switch trust {
	case "":
		mf.Trust = TrustUnverified
	case TrustUnverified, TrustVerified:
		mf.Trust = trust
	default:
		return fmt.Errorf("trust 只能是 %s 或 %s（当前：%s）", TrustUnverified, TrustVerified, mf.Trust)
	}
	seenNet := map[string]bool{}
	for i, host := range mf.Permissions.Net {
		h := strings.ToLower(strings.TrimSpace(host))
		h = strings.TrimPrefix(strings.TrimPrefix(h, "http://"), "https://")
		if h == "" {
			return errors.New("permissions.net 不能包含空域名")
		}
		if strings.ContainsAny(h, "/?#") {
			return fmt.Errorf("permissions.net 只能写域名（不含协议 / 路径）：%s", host)
		}
		if seenNet[h] {
			continue
		}
		seenNet[h] = true
		mf.Permissions.Net[i] = h
	}
	return nil
}

func validateRuntime(mf *Manifest) error {
	rt := &mf.Runtime
	rt.Type = strings.ToLower(strings.TrimSpace(rt.Type))
	if rt.Type == "" {
		return nil
	}
	if rt.Type != RuntimeProcess {
		return fmt.Errorf("runtime.type 只能是 process（当前：%s）", rt.Type)
	}
	proto := strings.ToLower(strings.TrimSpace(rt.Protocol))
	if proto == "" {
		proto = ProtocolJSONRPCStdio
	}
	if proto != ProtocolJSONRPCStdio {
		return fmt.Errorf("runtime.protocol 只支持 %s（当前：%s）", ProtocolJSONRPCStdio, rt.Protocol)
	}
	rt.Protocol = proto
		if rt.ProtocolVersion == 0 {
		rt.ProtocolVersion = ProtocolCurrent
	}
	if rt.ProtocolVersion != 1 && rt.ProtocolVersion != ProtocolCurrent {
		return fmt.Errorf("runtime.protocolVersion 只支持 1 或 %d（当前：%d）", ProtocolCurrent, rt.ProtocolVersion)
	}
	cmd := strings.TrimSpace(rt.Command)
	if cmd == "" {
		return errors.New("进程插件缺少 runtime.command")
	}
	if filepath.IsAbs(cmd) {
		return fmt.Errorf("runtime.command 必须是插件目录内的相对路径：%s", cmd)
	}
	abs, err := resolvePluginPath(mf.Dir, cmd)
	if err != nil {
		return fmt.Errorf("runtime.command 非法: %w", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("runtime.command 指向的文件不存在：%s", cmd)
	}
	if st.IsDir() {
		return fmt.Errorf("runtime.command 指向的是目录：%s", cmd)
	}
	return nil
}

// resolvePluginPath 把插件内相对路径解析成绝对路径，并拒绝越界。
func resolvePluginPath(dir, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("路径不能为空")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("不允许绝对路径：%s", rel)
	}
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if !insideDir(dir, abs) {
		return "", fmt.Errorf("路径越出插件目录：%s", rel)
	}
	return abs, nil
}

// expandTemplate 把 {{name}} 替换为参数值；{{workspace}} / {{cwd}} 替换为工作区。
func expandTemplate(tpl string, args map[string]any, workspace string, urlEscape bool) string {
	if tpl == "" {
		return ""
	}
	return tplRe.ReplaceAllStringFunc(tpl, func(m string) string {
		key := strings.TrimSpace(m[2 : len(m)-2])
		val := ""
		switch key {
		case "workspace", "cwd":
			val = workspace
		default:
			if args != nil {
				val = argString(args, key)
			}
		}
		if urlEscape {
			return url.QueryEscape(val)
		}
		return val
	})
}

// argString 取参数标量文本。
func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	case json.Number:
		return x.String()
	default:
		raw, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(raw)
	}
}

func nowMilli() int64 { return time.Now().UnixMilli() }

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

// insideDir 判断 child 是否在 parent 内（或相等）。
func insideDir(parent, child string) bool {
	p, err1 := filepath.Abs(parent)
	c, err2 := filepath.Abs(child)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(p, c)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

// copyTree 复制目录树（跳过 node_modules / .git）。
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, d os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, d.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// installZip 解压 zip 到插件根目录，返回清单。
func installZip(src, pluginsDir string) (*Manifest, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer zr.Close()

	// 找到 plugin.json 的相对位置，确定包根
	root := ""
	found := false
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if name == "plugin.json" {
			found, root = true, ""
			break
		}
		if strings.HasSuffix(name, "/plugin.json") {
			found, root = true, strings.TrimSuffix(name, "plugin.json")
			break
		}
	}
	if !found {
		return nil, errors.New("zip 里找不到 plugin.json")
	}

	tmp, err := os.MkdirTemp("", "plugins-install-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if root != "" {
			if !strings.HasPrefix(name, root) {
				continue
			}
			name = strings.TrimPrefix(name, root)
		}
		name = strings.TrimPrefix(name, "/")
		if name == "" {
			continue
		}
		// 防目录穿越（zip-slip）
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || filepath.IsAbs(clean) {
			continue
		}
		target := filepath.Join(tmp, clean)
		if !insideDir(tmp, target) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode().Perm()|0o600)
		if err != nil {
			rc.Close()
			return nil, err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return nil, err
		}
		out.Close()
		rc.Close()
	}

	mf, err := ReadManifest(tmp)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(pluginsDir, mf.ID)
	if err := os.RemoveAll(target); err != nil {
		return nil, err
	}
	if err := copyTree(tmp, target); err != nil {
		return nil, fmt.Errorf("安装插件失败: %w", err)
	}
	return ReadManifest(target)
}

// validToolName 进程插件上报的工具名是否合法。
func validToolName(s string) bool { return toolName.MatchString(s) }
