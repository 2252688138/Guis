package plugins

import (
	"fmt"
	"net/url"
	"strings"
)

// checkHTTPPermission 校验 http 型工具的目标域名是否在白名单内（空白名单 = 不限制）。
func checkHTTPPermission(mf *Manifest, rawURL string) error {
	if len(mf.Permissions.Net) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 无效: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("URL 缺少主机名: %s", rawURL)
	}
	for _, allow := range mf.Permissions.Net {
		a := strings.ToLower(strings.TrimSpace(allow))
		if a == host {
			return nil
		}
		if strings.HasPrefix(a, "*.") {
			suffix := a[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) {
				return nil
			}
		}
	}
	return fmt.Errorf("插件 %s 未被允许访问 %s（permissions.net：%s）", mf.ID, host, strings.Join(mf.Permissions.Net, ", "))
}

// checkCommandPermission 校验是否允许宿主代插件执行命令。
//
// 未声明 permissions 的旧插件保持宽松（兼容 v1 插件）；一旦声明，必须 exec:true。
func checkCommandPermission(mf *Manifest) error {
	if !mf.Permissions.Declared() {
		return nil
	}
	if mf.Permissions.Exec {
		return nil
	}
	return fmt.Errorf("插件 %s 声明了 permissions 但未允许 exec，禁止执行命令类工具", mf.ID)
}
