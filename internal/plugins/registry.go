package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// registryFileName 本地插件仓库清单文件名（位于 DataDir 下）。
const registryFileName = "plugins_registry.json"

// registryMaxBytes 单个安装包下载上限。
const registryMaxBytes = 200 << 20

// RegistryEntry registry 里的一条插件记录。
type RegistryEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	// URL 安装包地址（http/https 或本地文件绝对路径）。
	URL string `json:"url"`
	// SHA256 安装包摘要（强烈建议填写，校验通过后插件标记为 verified）。
	SHA256 string `json:"sha256,omitempty"`
}

type registryFile struct {
	Plugins []RegistryEntry `json:"plugins"`
}

// RegistryPath registry 清单路径。
func (m *Manager) RegistryPath() string { return filepath.Join(m.DataDir, registryFileName) }

// Registry 读取本地 registry 清单。
func (m *Manager) Registry() ([]RegistryEntry, error) {
	raw, err := os.ReadFile(m.RegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rf registryFile
	if err := json.Unmarshal(raw, &rf); err != nil {
		return nil, fmt.Errorf("plugins_registry.json 解析失败: %w", err)
	}
	out := make([]RegistryEntry, 0, len(rf.Plugins))
	for _, e := range rf.Plugins {
		e.ID = strings.TrimSpace(e.ID)
		e.URL = strings.TrimSpace(e.URL)
		if e.ID == "" || e.URL == "" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// InstallFromRegistry 按 registry 条目下载并安装（sha256 校验通过后标记 verified）。
func (m *Manager) InstallFromRegistry(ctx context.Context, id string) (*Manifest, error) {
	entries, err := m.Registry()
	if err != nil {
		return nil, err
	}
	var entry *RegistryEntry
	for i := range entries {
		if strings.EqualFold(entries[i].ID, id) {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("registry 里没有插件 %s（清单：%s）", id, m.RegistryPath())
	}

	src := entry.URL
	if !isRemoteURL(src) {
		if _, err := os.Stat(src); err != nil {
			return nil, fmt.Errorf("registry 里的本地路径不可用: %w", err)
		}
		return m.InstallWithOptions(ctx, src, InstallOptions{ExpectedSHA256: entry.SHA256})
	}

	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("registry 里的下载地址非法：%s", entry.URL)
	}
	tmpDir, err := os.MkdirTemp("", "plugins-download-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	target := filepath.Join(tmpDir, "plugin.zip")
	if err := downloadFile(ctx, src, target); err != nil {
		return nil, err
	}
	return m.InstallWithOptions(ctx, target, InstallOptions{ExpectedSHA256: entry.SHA256})
}

func isRemoteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func downloadFile(ctx context.Context, src, dst string) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, registryMaxBytes))
	if err != nil {
		return err
	}
	if n >= registryMaxBytes {
		return fmt.Errorf("安装包超过 %d MB 上限", registryMaxBytes>>20)
	}
	return nil
}
