// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"sort"
	"strings"
)

// saveEnvelope 带版本头与校验和的存档容器。
type saveEnvelope struct {
	Version int             `json:"version"`
	CRC     uint32          `json:"crc,omitempty"`
	Data    json.RawMessage `json:"data"`
}

// defaultSaveKey 内置混淆密钥（Obfuscate 且 Key 为空时使用）。
var defaultSaveKey = []byte("guis-save-v1")

func (s SaveStore) bakPath(slot string) string { return s.slotPath(slot) + ".bak" }

func (s SaveStore) xorKey() []byte {
	if len(s.Key) > 0 {
		return s.Key
	}
	return defaultSaveKey
}

func (s SaveStore) xorBytes(b []byte) []byte {
	k := s.xorKey()
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ k[i%len(k)]
	}
	return out
}

func (s SaveStore) writeSlot(slot string, v any) error {
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	out := pretty
	if s.Version > 0 || s.Checksum {
		env := saveEnvelope{Version: s.Version, Data: pretty}
		if s.Checksum {
			// CRC 基于紧凑 JSON 计算：容器输出时 data 会被重新缩进，读取端先 compact 再比对
			compact, err := json.Marshal(v)
			if err != nil {
				return err
			}
			env.CRC = crc32.ChecksumIEEE(compact)
		}
		out, err = json.MarshalIndent(env, "", "  ")
		if err != nil {
			return err
		}
	}
	if s.Obfuscate {
		out = s.xorBytes(out)
	}
	path := s.slotPath(slot)
	if s.KeepBackup || s.Checksum {
		if old, err := os.ReadFile(path); err == nil && len(old) > 0 {
			_ = writeFileAtomic(s.bakPath(slot), old)
		}
	}
	if err := writeFileAtomic(path, out); err != nil {
		return err
	}
	return nil
}

func (s SaveStore) readSlot(slot string, v any) error {
	b, err := os.ReadFile(s.slotPath(slot))
	if err != nil {
		return err
	}
	if data, ok := s.decodeSave(b); ok {
		return json.Unmarshal(data, v)
	}
	// 主存档损坏（或版本迁移失败）：回退到备份
	if bak, err := os.ReadFile(s.bakPath(slot)); err == nil {
		if data, ok := s.decodeSave(bak); ok {
			return json.Unmarshal(data, v)
		}
	}
	return fmt.Errorf("game: 存档 %q 校验失败且无可用的备份", slot)
}

// decodeSave 解混淆 + 解容器 + 校验 + 迁移，返回可直接反序列化的 JSON。
func (s SaveStore) decodeSave(b []byte) ([]byte, bool) {
	if s.Obfuscate {
		b = s.xorBytes(b)
	}
	trim := bytes.TrimSpace(b)
	if len(trim) == 0 || trim[0] != '{' {
		return nil, false
	}
	var env saveEnvelope
	if err := json.Unmarshal(trim, &env); err == nil && len(env.Data) > 0 {
		if s.Checksum && env.CRC != 0 {
			var buf bytes.Buffer
			if err := json.Compact(&buf, env.Data); err != nil {
				return nil, false
			}
			if crc32.ChecksumIEEE(buf.Bytes()) != env.CRC {
				return nil, false
			}
		}
		data := []byte(env.Data)
		if s.Version > 0 && env.Version < s.Version {
			migrated, err := s.migrateSave(env.Version, data)
			if err != nil {
				return nil, false
			}
			data = migrated
		}
		return data, true
	}
	// 旧格式（无容器的裸 JSON）
	if !json.Valid(trim) {
		return nil, false
	}
	return trim, true
}

func (s SaveStore) migrateSave(from int, data []byte) ([]byte, error) {
	if s.Migrate == nil {
		return nil, fmt.Errorf("game: 存档版本 %d 落后于 %d 但未提供 Migrate", from, s.Version)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	cur := from
	for cur < s.Version {
		out, err := s.Migrate(cur, m)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("game: 存档迁移 %d→%d 返回空数据", cur, cur+1)
		}
		m = out
		cur++
	}
	return json.Marshal(m)
}

// List 列出全部槽位名（不含 .bak / .tmp，按字典序）。
func (s SaveStore) List() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".json"))
	}
	sort.Strings(out)
	return out, nil
}

// HasBackup 槽位是否有可回退的备份。
func (s SaveStore) HasBackup(slot string) bool {
	_, err := os.Stat(s.bakPath(slot))
	return err == nil
}

// RestoreBackup 把 .bak 拷回主槽位（用于存档被改坏后手动回滚）。
func (s SaveStore) RestoreBackup(slot string) error {
	b, err := os.ReadFile(s.bakPath(slot))
	if err != nil {
		return err
	}
	return writeFileAtomic(s.slotPath(slot), b)
}
