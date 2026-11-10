// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui_test

import (
	"testing"

	"guis/cmd/gallery/pages"
	"guis/ui"
)

// TestGalleryCacheRoots 真实 gallery 页面里有多少节点能成为组件缓存根。
//
// 这个测试不需要窗口 / GL：只建树 + 布局 + 打标记。它回答的是「能启用的全部启用」
// 到底启用了多少——数量为 0 说明门槛太严，数量过多且面积过大则可能有浪费。
func TestGalleryCacheRoots(t *testing.T) {
	const w, h = 1020, 700
	ui.SetViewport(1180, 760)

	total, pagesWith := 0, 0
	for _, c := range pages.Cases {
		pages.SwitchCase(c.ID)
		root := pages.Content()
		if root == nil {
			continue
		}
		ui.LayoutForTest(root, w, h, "Microsoft YaHei UI", 1)
		roots, area := ui.CountCacheRoots(root, w, h)
		if roots > 0 {
			pagesWith++
		}
		total += roots
		t.Logf("%-18s roots=%-3d area=%.0f (%.0f%% of viewport)",
			c.ID, roots, area, float64(area)/float64(w*h)*100)
	}
	if total == 0 {
		t.Fatal("所有页面都没有缓存根：自动组件缓存实际未生效")
	}
	t.Logf("合计缓存根 %d 个，覆盖 %d 个页面", total, pagesWith)
}
