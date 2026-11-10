// guis — 办公文档页（case_office.go）的冒烟测试
//
// 无需窗口：用 ui.LayoutForTest 走一遍真实布局，确认三个页签都能生成示例文档、
// 页面能排开、打印预览不 panic。

package pages

import (
	"testing"

	"guis/ui"
)

// TestOfficePageRenders 三个页签逐个构建 + 布局，校验示例文档与内容区尺寸。
func TestOfficePageRenders(t *testing.T) {
	oldCurrent, oldTab := Current, officeTab
	oldPanes := officePanes
	officePanes = map[string]*officePane{}
	defer func() {
		Current, officeTab, officePanes = oldCurrent, oldTab, oldPanes
	}()

	Current = "office"
	for _, tab := range []string{"excel", "word", "pdf"} {
		officeTab = tab
		// 先排一帧让视口宽度同步
		ui.LayoutForTest(View(), 1180, 900, "", 1)
		root := View()
		ui.LayoutForTest(root, 1180, 900, "", 1)

		p := officePanes[tab]
		if p == nil || p.file == nil {
			t.Fatalf("%s：示例文档未生成（%+v）", tab, p)
		}
		if p.size <= 0 {
			t.Fatalf("%s：示例文档字节数为 0", tab)
		}
		if p.msg == "" {
			t.Fatalf("%s：状态文案为空", tab)
		}
		switch tab {
		case "excel":
			if p.file.Sheet == nil || len(p.file.Sheet.Sheets) < 2 {
				t.Fatalf("excel：工作表数量异常 %+v", p.file.Sheet)
			}
			if rows := len(p.file.Sheet.First().Rows); rows < 5 {
				t.Fatalf("excel：明细行数 = %d", rows)
			}
			// 页面里应能找到表格页签容器
			if n := findKeyNode(root, "office-sheet-"+p.file.Sheet.First().Name); n == nil {
				t.Fatalf("excel：缺少表格节点 office-sheet-%s", p.file.Sheet.First().Name)
			}
		case "word":
			if p.file.Word == nil || len(p.file.Word.Blocks) < 5 {
				t.Fatalf("word：内容块数量异常 %+v", p.file.Word)
			}
			if n := findKeyNode(root, "office-word"); n == nil || n.Frame.H <= 0 {
				t.Fatalf("word：文档流未撑开")
			}
		case "pdf":
			if p.file.PDF == nil || len(p.file.PDF.Pages) == 0 {
				t.Fatalf("pdf：没有页面 %+v", p.file.PDF)
			}
			if n := findKeyNode(root, "office-pdf"); n == nil || n.Frame.H <= 0 {
				t.Fatalf("pdf：页面节点未撑开")
			}
		}

		// 打印预览（页面主体整体替换）
		p.preview = true
		ui.LayoutForTest(View(), 1180, 900, "", 1)
		prev := View()
		ui.LayoutForTest(prev, 1180, 900, "", 1)
		if n := findKeyNode(prev, "office-preview-preview-scroll"); n == nil {
			t.Fatalf("%s：打印预览未渲染", tab)
		}
		p.preview = false
	}
}

// TestOfficePDFPageGeometry PDF 页面节点：白纸尺寸 = 页面 pt × 缩放，且文本项就位。
func TestOfficePDFPageGeometry(t *testing.T) {
	oldCurrent, oldTab := Current, officeTab
	oldPanes := officePanes
	officePanes = map[string]*officePane{}
	defer func() {
		Current, officeTab, officePanes = oldCurrent, oldTab, oldPanes
	}()

	Current, officeTab = "office", "pdf"
	ui.LayoutForTest(View(), 1400, 900, "", 1)
	root := View()
	ui.LayoutForTest(root, 1400, 900, "", 1)

	n := findKeyNode(root, "office-pdf")
	if n == nil || len(n.Children) == 0 {
		t.Fatal("缺少 PDF 页面节点")
	}
	page := n.Children[0]
	p := officePanes["pdf"]
	wantW := p.file.PDF.Pages[0].W * p.zoom
	if d := page.Frame.W - wantW; d > 1 || d < -1 {
		t.Fatalf("页面宽度 %.1f，期望 %.1f（pt × zoom）", page.Frame.W, wantW)
	}

	// 页面上必须有标题文本与几条表格线（矩形 / 线条）
	var texts, shapes int
	var walk func(*ui.Node)
	walk = func(x *ui.Node) {
		if x == nil {
			return
		}
		if x.Text == "季度经营简报" {
			texts++
		}
		if x.Kind == ui.KBox {
			shapes++
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(page)
	if texts == 0 {
		t.Fatal("页面上没有找到标题文本（PDF 文本项未渲染）")
	}
	if shapes < 3 {
		t.Fatalf("页面上矩形/线条数量 = %d，偏少（底纹/表头/页脚线）", shapes)
	}
}

// TestOfficeNarrowLayout 窄屏（手机竖屏）下办公页仍然可排开。
func TestOfficeNarrowLayout(t *testing.T) {
	oldCurrent, oldTab := Current, officeTab
	defer func() { Current, officeTab = oldCurrent, oldTab }()

	Current, officeTab = "office", "excel"
	ui.LayoutForTest(View(), 480, 853, "", 1)
	root := View()
	ui.LayoutForTest(root, 480, 853, "", 1)
	if n := findKeyNode(root, "scroll-office-excel"); n == nil || n.Frame.H <= 0 {
		t.Fatal("窄屏：办公页内容区未撑开")
	}
}
