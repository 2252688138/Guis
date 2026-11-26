package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// 工具表缓存：握手成功后落盘，宿主重启后无需拉起进程即可暴露工具。
const toolsCacheName = ".tools.json"

type toolsCache struct {
	PluginVersion   string      `json:"pluginVersion"`
	ProtocolVersion int         `json:"protocolVersion"`
	SavedAt         int64       `json:"savedAt"`
	Tools           []ProtoTool `json:"tools"`
}

func toolsCachePath(dir string) string { return filepath.Join(dir, toolsCacheName) }

// saveToolsCache 写入工具缓存（版本或协议不匹配时下次读取自动失效）。
func saveToolsCache(dir, pluginVersion string, protocol int, tools []ToolDescriptor) error {
	if dir == "" {
		return nil
	}
	cache := toolsCache{PluginVersion: pluginVersion, ProtocolVersion: protocol, SavedAt: time.Now().UnixMilli()}
	for _, t := range tools {
		cache.Tools = append(cache.Tools, ProtoTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
			Safe:        t.Safe,
		})
	}
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(toolsCachePath(dir), raw)
}

// loadToolsCache 读取工具缓存；插件版本、协议版本不符或文件损坏时返回 nil。
func loadToolsCache(dir, pluginVersion string, protocol int) []ToolDescriptor {
	if dir == "" {
		return nil
	}
	raw, err := os.ReadFile(toolsCachePath(dir))
	if err != nil {
		return nil
	}
	var cache toolsCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil
	}
	if cache.PluginVersion != pluginVersion || cache.ProtocolVersion != protocol {
		return nil
	}
	out := make([]ToolDescriptor, 0, len(cache.Tools))
	for _, t := range cache.Tools {
		params := t.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, ToolDescriptor{
			PluginID:    "",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
			Safe:        t.Safe,
			Kind:        RuntimeProcess,
		})
	}
	return out
}

// clearToolsCache 删除工具缓存。
func clearToolsCache(dir string) {
	if dir == "" {
		return
	}
	_ = os.Remove(toolsCachePath(dir))
}
