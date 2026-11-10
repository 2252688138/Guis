// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package ui

import "testing"

// TestIconFlattenPath 路径解析：直线命令给出精确顶点，曲线 / 弧线按长度细分。
func TestIconFlattenPath(t *testing.T) {
	subs := iconFlattenPath("M4 4h16v16H4z")
	if len(subs) != 1 {
		t.Fatalf("矩形应为 1 条子路径，实际 %d", len(subs))
	}
	if len(subs[0].Pts) != 4 {
		t.Fatalf("矩形应有 4 个顶点，实际 %d", len(subs[0].Pts))
	}
	if !subs[0].Closed {
		t.Fatal("z 结尾的子路径应标记为闭合")
	}
	if p := subs[0].Pts[1]; p.X != 20 || p.Y != 4 {
		t.Fatalf("h16 后顶点应为 (20,4)，实际 (%v,%v)", p.X, p.Y)
	}

	// 三次贝塞尔：控制多边形足够长，必须被细分成多个点
	curve := iconFlattenPath("M4 12c0-4 4-6 8-6")
	if len(curve) != 1 || len(curve[0].Pts) < 6 {
		t.Fatalf("贝塞尔曲线应被细分，实际 %d 个点", len(curve[0].Pts))
	}

	// 圆弧：整圆由两段 a 命令组成，点数应明显多于直线
	circle := iconFlattenPath("M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z")
	if len(circle) != 1 || len(circle[0].Pts) < 20 {
		t.Fatalf("圆弧采样点过少：%d", len(circle[0].Pts))
	}

	// 脏数据不能 panic / 死循环
	if got := iconFlattenPath("M 12 12 Q q 8"); len(got) != 0 {
		t.Fatalf("非法参数应被跳过，实际 %d 条子路径", len(got))
	}
	if got := iconFlattenPath(""); len(got) != 0 {
		t.Fatalf("空路径应为空，实际 %d 条", len(got))
	}
}

// TestIconRasterCoverage 光栅化：描边在笔画处有覆盖率、空白处为 0；填充走奇偶规则。
func TestIconRasterCoverage(t *testing.T) {
	const n = 48
	alphaAt := func(px []byte, x, y int) byte { return px[(y*n+x)*4+3] }

	plus := iconRaster(iconFlattenPath("M12 5v14M5 12h14"), false, 2, n)
	if a := alphaAt(plus, n/2, n/2); a < 200 {
		t.Fatalf("十字中心应完全覆盖，实际 alpha=%d", a)
	}
	for _, p := range [][2]int{{0, 0}, {n - 1, 0}, {0, n - 1}, {n - 1, n - 1}} {
		if a := alphaAt(plus, p[0], p[1]); a != 0 {
			t.Fatalf("角落 (%d,%d) 不应有覆盖率，实际 alpha=%d", p[0], p[1], a)
		}
	}

	// 描边宽度加倍：覆盖像素数量必须增加
	thin := iconRaster(iconFlattenPath("M12 5v14"), false, 1, n)
	thick := iconRaster(iconFlattenPath("M12 5v14"), false, 4, n)
	if covered(thick) <= covered(thin) {
		t.Fatalf("粗线覆盖像素应更多：thin=%d thick=%d", covered(thin), covered(thick))
	}

	// 填充三角形：内部不透明、外部透明
	tri := iconRaster(iconFlattenPath("M12 2l10 20H2z"), true, 0, n)
	if a := alphaAt(tri, n/2, n*7/10); a < 200 {
		t.Fatalf("三角形内部应完全覆盖，实际 alpha=%d", a)
	}
	if a := alphaAt(tri, 1, 1); a != 0 {
		t.Fatalf("三角形左上角应为空，实际 alpha=%d", a)
	}
}

// covered 统计 alpha 非零像素数（测试辅助）。
func covered(px []byte) int {
	n := 0
	for i := 3; i < len(px); i += 4 {
		if px[i] > 0 {
			n++
		}
	}
	return n
}

// TestIconRegistry 内置图标表完整性：名称唯一、路径可解析、有展示名。
func TestIconRegistry(t *testing.T) {
	names := IconNames()
	if len(names) < 70 {
		t.Fatalf("内置图标过少：%d", len(names))
	}
	if IconCount() != len(names) {
		t.Fatalf("IconCount=%d 与 IconNames=%d 不一致", IconCount(), len(names))
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			t.Fatalf("图标名重复：%s", name)
		}
		seen[name] = true
		def, ok := IconDefOf(name)
		if !ok {
			t.Fatalf("IconDefOf(%s) 失败", name)
		}
		if def.Label == "" {
			t.Fatalf("%s 缺少展示名", name)
		}
		if len(def.Paths) == 0 {
			t.Fatalf("%s 没有路径", name)
		}
		if def.Stroke <= 0 {
			t.Fatalf("%s 线宽未归一化：%v", name, def.Stroke)
		}
		for _, d := range def.Paths {
			if len(iconFlattenPath(d)) == 0 {
				t.Fatalf("%s 的路径无法解析：%q", name, d)
			}
		}
	}
	for _, name := range []string{"search", "settings", "play", "file", "check-circle", "star-fill"} {
		if !IconExists(name) {
			t.Fatalf("缺少常用图标 %s", name)
		}
	}
}

// TestIconNode 图标节点的尺寸 / 颜色 / 线宽约定。
func TestIconNode(t *testing.T) {
	n := Icon("search")
	if n == nil || n.Kind != KIcon {
		t.Fatal("Icon 应返回 KIcon 节点")
	}
	if n.IconName != "search" {
		t.Fatalf("图标名丢失：%q", n.IconName)
	}
	if n.Style.Width != 18 || n.Style.Height != 18 {
		t.Fatalf("默认尺寸应为 18×18，实际 %v×%v", n.Style.Width, n.Style.Height)
	}
	if n.Style.Color.A <= 0 {
		t.Fatal("默认颜色不应透明")
	}

	c := IconColored("heart", 24, "#ff5577")
	if c.Style.Width != 24 || c.Style.Height != 24 {
		t.Fatalf("IconColored 尺寸错误：%v", c.Style.Width)
	}
	if c.Style.Color.R <= c.Style.Color.B {
		t.Fatal("IconColored 未应用自定义颜色")
	}

	s := IconStroked("star", 32, 1.25)
	if s.Style.Width != 32 || s.IconStroke != 1.25 {
		t.Fatalf("IconStroked 未写入线宽：%v / %v", s.Style.Width, s.IconStroke)
	}

	row := IconLabel("folder", "项目文件", 16, ColText)
	if row == nil || len(row.Children) != 2 {
		t.Fatal("IconLabel 应为图标 + 文字两列")
	}
	if row.Children[0].Kind != KIcon {
		t.Fatal("IconLabel 第一列应为图标")
	}
}

// TestRegisterCustomIcon 运行时注册：首次注册 / 同名覆盖 / 注销。
func TestRegisterCustomIcon(t *testing.T) {
	def := IconDef{Name: "unit-test-shape", Label: "测试形状", Paths: []string{"M4 4h16v16H4z"}}
	defer UnregisterIcon(def.Name)

	if RegisterIcon(def) {
		t.Fatal("首次注册不应报告覆盖")
	}
	if !IconExists(def.Name) {
		t.Fatal("注册后 IconExists 应为 true")
	}
	got, _ := IconDefOf(def.Name)
	if got.Stroke != iconDefaultStroke {
		t.Fatalf("默认线宽未归一化：%v", got.Stroke)
	}

	// 同名覆盖：路径替换、顺序不变
	before := len(IconNames())
	def.Paths = []string{"M4 4h10"}
	if !RegisterIcon(def) {
		t.Fatal("同名注册应报告覆盖")
	}
	if after := len(IconNames()); after != before {
		t.Fatalf("覆盖不应改变图标数量：%d → %d", before, after)
	}
	got, _ = IconDefOf(def.Name)
	if len(got.Paths) != 1 || got.Paths[0] != "M4 4h10" {
		t.Fatal("覆盖后路径未更新")
	}

	UnregisterIcon(def.Name)
	if IconExists(def.Name) {
		t.Fatal("注销后 IconExists 应为 false")
	}
	if after := len(IconNames()); after != before-1 {
		t.Fatalf("注销后数量应减一：%d → %d", before, after)
	}
}
