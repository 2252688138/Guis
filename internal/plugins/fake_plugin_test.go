package plugins

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 测试用假插件：测试二进制自身充当插件进程（GUIS_PLUGIN_TEST_HELPER 触发）。

func TestMain(m *testing.M) {
	if mode := os.Getenv("GUIS_PLUGIN_TEST_HELPER"); mode != "" {
		fakePluginMain(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakePluginMain(mode string) {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
	enc := json.NewEncoder(os.Stdout)
	writeErr := func(id int64, msg string) {
		_ = enc.Encode(map[string]any{"id": id, "error": map[string]any{"code": -32000, "message": msg}})
	}
	writeRes := func(id int64, res any) {
		_ = enc.Encode(map[string]any{"id": id, "result": res})
	}
	if mode == "noisy" {
		for i := 0; i < 300; i++ {
			fmt.Fprintf(os.Stderr, "stderr line %d\n", i)
		}
	}
	if mode == "garbage" {
		fmt.Fprintln(os.Stdout, "this is not json")
		return
	}
	v2 := strings.HasPrefix(mode, "v2")
	silentPing := strings.HasSuffix(mode, "noping")
	cfg := map[string]any{"greeting": "默认问候"}
	caps := []string{"tools"}
	if v2 {
		caps = []string{"tools", "commands", "progress", "log", "config"}
	}
	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintln(os.Stdout, "bad-json")
			return
		}
		switch req.Method {
		case MethodInitialize:
			writeRes(req.ID, InitializeResult{Name: "fake", Version: "0.1.0", Capabilities: caps})
		case MethodToolsList:
			writeRes(req.ID, ToolsListResult{Tools: []ProtoTool{
				{Name: "echo", Description: "回显", Safe: true, Parameters: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)},
				{Name: "fail", Description: "总是失败", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)},
				{Name: "bad name!", Description: "非法名字，应被过滤"},
				{Name: "echo", Description: "重名，应被过滤"},
			}})
		case MethodToolsCall:
			var p ToolsCallParams
			_ = json.Unmarshal(req.Params, &p)
			switch {
			case mode == "crash":
				os.Exit(1)
			case mode == "slow":
				time.Sleep(3 * time.Second)
				writeRes(req.ID, ToolsCallResult{Content: "late"})
			case p.Name == "fail":
				writeRes(req.ID, ToolsCallResult{Content: "boom", IsError: true})
			case p.Name == "echo":
				if v2 {
					_ = enc.Encode(map[string]any{"method": NotifyProgress, "params": ProgressParams{Tool: p.Name, Done: 1, Total: 1, Message: "正在回显"}})
					_ = enc.Encode(map[string]any{"method": NotifyLog, "params": LogParams{Level: "info", Message: "echo 被调用"}})
				}
				writeRes(req.ID, ToolsCallResult{Content: argString(p.Arguments, "text")})
			default:
				writeErr(req.ID, "unknown tool "+p.Name)
			}
		case MethodCommandsList:
			writeRes(req.ID, CommandsListResult{Commands: []CommandInfo{
				{ID: "greet", Title: "问候", Description: "向某人问好"},
				{ID: "", Title: "非法，应被过滤"},
				{ID: "greet", Title: "重名，应被过滤"},
			}})
		case MethodCommandsExec:
			var p CommandsExecParams
			_ = json.Unmarshal(req.Params, &p)
			if p.ID != "greet" {
				writeErr(req.ID, "unknown command "+p.ID)
				continue
			}
			name := argString(p.Args, "name")
			if name == "" {
				name = "世界"
			}
			writeRes(req.ID, ToolsCallResult{Content: argString(cfg, "greeting") + "，" + name})
		case MethodConfigGet:
			writeRes(req.ID, ConfigResult{Config: cfg})
		case MethodConfigSet:
			var p map[string]any
			_ = json.Unmarshal(req.Params, &p)
			if v, ok := p["config"].(map[string]any); ok {
				cfg = v
			}
			writeRes(req.ID, ConfigResult{Config: cfg})
		case MethodPing:
			if silentPing {
				continue
			}
			writeRes(req.ID, map[string]any{"ok": true})
		case MethodShutdown:
			writeRes(req.ID, map[string]any{})
			return
		default:
			writeErr(req.ID, "unknown method "+req.Method)
		}
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// writeFakePlugin 在 dir 下生成一个可运行的进程插件，返回其清单。
func writeFakePlugin(t *testing.T, dir, mode string, tweak func(*RuntimeSpec)) *Manifest {
	t.Helper()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "fake-plugin"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "demo", "SKILL.md"),
		[]byte("---\nname: demo\ndescription: 示例技能\n---\n正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := RuntimeSpec{
		Type:           RuntimeProcess,
		Command:        "bin/" + name,
		Env:            map[string]string{"GUIS_PLUGIN_TEST_HELPER": mode},
		CallTimeoutMs:  2000,
		StartTimeoutMs: 5000,
	}
	if tweak != nil {
		tweak(&rt)
	}
	mf := &Manifest{
		ID:          "fake-proc",
		Name:        "假进程插件",
		Version:     "0.1.0",
		Description: "测试用",
		Prompt:      "测试提示词",
		Skills:      []string{"skills"},
		Runtime:     rt,
		Dir:         dir,
		Tools: []PluginTool{{
			Name:        "declared",
			Description: "声明式工具",
			Parameters:  json.RawMessage(`{"type":"object"}`),
			Type:        "command",
			Command:     "echo hi",
			Safe:        true,
		}},
	}
	if err := Validate(mf); err != nil {
		t.Fatalf("清单校验失败: %v", err)
	}
	raw, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return mf
}
