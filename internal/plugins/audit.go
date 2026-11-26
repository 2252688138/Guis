package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 审计日志文件名与轮转阈值。
const (
	auditFile      = "plugins_audit.jsonl"
	auditRotateMax = 5 << 20
	auditLineMax   = 4 << 10
	auditValueMax  = 200
)

// auditEntry 一次插件工具调用的审计记录。
type auditEntry struct {
	TS          int64          `json:"ts"`
	Plugin      string         `json:"plugin"`
	Tool        string         `json:"tool"`
	Kind        string         `json:"kind,omitempty"`
	ArgsDigest  map[string]any `json:"argsDigest,omitempty"`
	DurationMs  int64          `json:"durationMs"`
	OK          bool           `json:"ok"`
	IsError     bool           `json:"isError,omitempty"`
	ResultBytes int            `json:"resultBytes,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type auditor struct {
	mu   sync.Mutex
	path string
}

func newAuditor(dataDir string) *auditor {
	if strings.TrimSpace(dataDir) == "" {
		return nil
	}
	return &auditor{path: filepath.Join(dataDir, auditFile)}
}

// Path 审计文件路径。
func (a *auditor) Path() string {
	if a == nil {
		return ""
	}
	return a.path
}

func (a *auditor) write(e auditEntry) {
	if a == nil {
		return
	}
	if e.TS == 0 {
		e.TS = time.Now().UnixMilli()
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	if len(raw) > auditLineMax {
		raw = raw[:auditLineMax]
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, err := os.Stat(a.path); err == nil && st.Size() > auditRotateMax {
		_ = os.Rename(a.path, strings.TrimSuffix(a.path, ".jsonl")+".1.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(raw, '\n'))
}

// sensitiveKeys 命中即脱敏的参数键名（小写子串匹配）。
var sensitiveKeys = []string{"token", "key", "secret", "password", "authorization", "cookie"}

// sanitizeArgs 生成参数摘要：敏感值打码，长值截断。
func sanitizeArgs(args map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		lower := strings.ToLower(k)
		masked := false
		for _, s := range sensitiveKeys {
			if strings.Contains(lower, s) {
				out[k] = "***"
				masked = true
				break
			}
		}
		if masked {
			continue
		}
		switch x := v.(type) {
		case string:
			if len(x) > auditValueMax {
				out[k] = x[:auditValueMax] + "…"
			} else {
				out[k] = x
			}
		case nil:
			out[k] = nil
		default:
			raw, err := json.Marshal(x)
			if err != nil {
				out[k] = "***"
				continue
			}
			if len(raw) > auditValueMax {
				out[k] = string(raw[:auditValueMax]) + "…"
			} else {
				out[k] = x
			}
		}
	}
	return out
}
