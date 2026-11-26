// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import (
	"os"
	"path/filepath"
	"strings"
)

// writeFileAtomic 原子写文件（先写 .tmp 再 rename），避免中断留下半文件。
func writeFileAtomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SaveStore 存档槽（每个槽一个文件）。
//
// 默认（Version=0、Checksum/Obfuscate 关闭）写的是纯 JSON，与旧版存档完全兼容；
// 打开 Version > 0 后写入版本头，读取旧档时句链调用 Migrate 升级。
type SaveStore struct {
	// Dir 存档目录（不存在时自动创建）。
	Dir string
	// Version 当前存档版本（0 = 不写版本头，旧格式）。
	Version int
	// Migrate 旧版本迁移函数：从 from 版升到 from+1 版，返回迁移后的字段表。
	// 会一直调用到 Version-1，每次只能用一步。
	Migrate func(from int, data map[string]any) (map[string]any, error)
	// Checksum 写入 CRC32 校验；读取失败时自动回退 .bak。
	Checksum bool
	// Obfuscate 用 Key 异或混淆存档（防手改，不是加密；切换开关不兼容旧明文）。
	Obfuscate bool
	// Key 混淆密钥（Obfuscate 时使用，为空则用内置默认值）。
	Key []byte
	// KeepBackup 写入前把旧存档备份为 .bak（Checksum 为 true 时自动开启）。
	KeepBackup bool
}

// slotPath 拼接槽位文件路径（过滤路径分隔符，防目录穿越）。
func (s SaveStore) slotPath(slot string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(slot)
	return filepath.Join(s.Dir, safe+".json")
}

// Put 写入槽位（原子写；按配置自动加版本头 / 校验和 / 混淆 / 备份）。
func (s SaveStore) Put(slot string, v any) error {
	return s.writeSlot(slot, v)
}

// Get 读取槽位到 v（自动识别旧格式、校验 CRC、备份回退与版本迁移）。
func (s SaveStore) Get(slot string, v any) error {
	return s.readSlot(slot, v)
}

// Delete 删除槽位（不存在不算错）。
func (s SaveStore) Delete(slot string) error {
	err := os.Remove(s.slotPath(slot))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// Exists 槽位是否存在。
func (s SaveStore) Exists(slot string) bool {
	_, err := os.Stat(s.slotPath(slot))
	return err == nil
}
