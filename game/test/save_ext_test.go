// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guis/game"
)

func TestSaveVersionMigrate(t *testing.T) {
	dir := t.TempDir()
	v1 := game.SaveStore{Dir: dir, Version: 1}
	if err := v1.Put("slot", map[string]any{"score": 10, "name": "甲"}); err != nil {
		t.Fatalf("v1 写入失败: %v", err)
	}

	v2 := game.SaveStore{Dir: dir, Version: 2, Migrate: func(from int, d map[string]any) (map[string]any, error) {
		if s, ok := d["score"].(float64); ok {
			d["score"] = s * 2
		}
		d["migrated_from"] = from
		return d, nil
	}}
	var got map[string]any
	if err := v2.Get("slot", &got); err != nil {
		t.Fatalf("v2 读取失败: %v", err)
	}
	if got["score"] != float64(20) {
		t.Fatalf("迁移后 score = %v，期望 20", got["score"])
	}
	if got["migrated_from"] != float64(1) {
		t.Fatalf("迁移来源版本 = %v，期望 1", got["migrated_from"])
	}

	// 无 Migrate 时读到旧版本应报错，而不是静默丢字段
	v3 := game.SaveStore{Dir: dir, Version: 3}
	if err := v3.Get("slot", &got); err == nil {
		t.Fatal("缺少 Migrate 时读取旧存档应报错")
	}
}

func TestSaveChecksumBackup(t *testing.T) {
	dir := t.TempDir()
	s := game.SaveStore{Dir: dir, Version: 1, Checksum: true}
	if err := s.Put("slot", map[string]int{"hp": 100}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	if err := s.Put("slot", map[string]int{"hp": 50}); err != nil {
		t.Fatalf("二次写入失败: %v", err)
	}
	if !s.HasBackup("slot") {
		t.Fatal("Checksum 开启时应自动生成 .bak")
	}

	// 篡改主存档的 data（不改 crc）→ 读应回退到备份（hp=100）
	path := filepath.Join(dir, "slot.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读存档失败: %v", err)
	}
	tampered := []byte(strings.Replace(string(b), "\"hp\": 50", "\"hp\": 70", 1))
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatalf("篡改存档失败: %v", err)
	}
	var got map[string]int
	if err := s.Get("slot", &got); err != nil {
		t.Fatalf("篡改后读取失败: %v", err)
	}
	if got["hp"] != 100 {
		t.Fatalf("篡改后应回退到备份值 100，实际 %d", got["hp"])
	}

	// 手动恢复备份
	if err := s.RestoreBackup("slot"); err != nil {
		t.Fatalf("恢复备份失败: %v", err)
	}
	var restored map[string]int
	if err := s.Get("slot", &restored); err != nil || restored["hp"] != 100 {
		t.Fatalf("恢复后值 = %v err=%v", restored, err)
	}
}

func TestSaveObfuscate(t *testing.T) {
	dir := t.TempDir()
	s := game.SaveStore{Dir: dir, Obfuscate: true, Key: []byte("secret-key")}
	want := map[string]any{"gold": 320.0, "stage": "2-1"}
	if err := s.Put("slot", want); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "slot.json"))
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if len(b) == 0 || b[0] == '{' {
		t.Fatal("混淆后的存档不应是明文 JSON")
	}
	var got map[string]any
	if err := s.Get("slot", &got); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got["gold"] != float64(320) || got["stage"] != "2-1" {
		t.Fatalf("回读值 = %v", got)
	}
}

func TestSaveListAndLegacyCompat(t *testing.T) {
	dir := t.TempDir()
	s := game.SaveStore{Dir: dir}
	// 旧格式（无版本头）读写仍可用
	if err := s.Put("a", map[string]int{"v": 1}); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := s.Put("b", map[string]int{"v": 2}); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	var got map[string]int
	if err := s.Get("a", &got); err != nil || got["v"] != 1 {
		t.Fatalf("旧格式读取 = %v err=%v", got, err)
	}
	names, err := s.List()
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("List = %v", names)
	}
	if err := s.Delete("a"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if s.Exists("a") {
		t.Fatal("Delete 后不应存在")
	}
}
