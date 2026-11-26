package geom

import "testing"

func near(a, b float32) bool {
	d := a - b
	return d < 0.0001 && d > -0.0001
}

func TestPointOps(t *testing.T) {
	p := Pt(1.5, 2).Add(Pt(0.5, 3))
	if !near(p.X, 2) || !near(p.Y, 5) {
		t.Fatalf("Add 错误：%+v", p)
	}
	q := p.Sub(Pt(1, 1))
	if !near(q.X, 1) || !near(q.Y, 4) {
		t.Fatalf("Sub 错误：%+v", q)
	}
	r := Pt(1.2, 1.8).Round()
	if !near(r.X, 1) || !near(r.Y, 2) {
		t.Fatalf("Round 错误：%+v", r)
	}
	n := Pt(-1.8, -1.2).Round()
	if !near(n.X, -2) || !near(n.Y, -1) {
		t.Fatalf("负值 Round 错误：%+v", n)
	}
}

func TestSizeEmpty(t *testing.T) {
	if !Sz(0, 10).IsEmpty() || !Sz(10, -1).IsEmpty() {
		t.Fatal("宽或高 <= 0 应判空")
	}
	if Sz(1, 1).IsEmpty() {
		t.Fatal("正尺寸不应判空")
	}
}

func TestRectDerived(t *testing.T) {
	r := R(10, 20, 30, 40)
	if !near(r.Right(), 40) || !near(r.Bottom(), 60) {
		t.Fatalf("Right/Bottom 错误：%v %v", r.Right(), r.Bottom())
	}
	if s := r.Size(); !near(s.W, 30) || !near(s.H, 40) {
		t.Fatalf("Size 错误：%+v", s)
	}
	if m := r.Min(); !near(m.X, 10) || !near(m.Y, 20) {
		t.Fatalf("Min 错误：%+v", m)
	}
	if m := r.Max(); !near(m.X, 40) || !near(m.Y, 60) {
		t.Fatalf("Max 错误：%+v", m)
	}
	if c := r.Center(); !near(c.X, 25) || !near(c.Y, 40) {
		t.Fatalf("Center 错误：%+v", c)
	}
	if !R(0, 0, 0, 5).IsEmpty() {
		t.Fatal("宽为 0 应判空")
	}
	if R(0, 0, 5, 5).IsEmpty() {
		t.Fatal("正尺寸不应判空")
	}
}

// TestRectContainsHalfOpen 命中判定用左上闭、右下开区间，
// 否则相邻两个矩形在边界上会同时命中（列表行间隙误点）。
func TestRectContainsHalfOpen(t *testing.T) {
	r := R(0, 0, 10, 10)
	if !r.Contains(Pt(0, 0)) || !r.Contains(Pt(9.9, 9.9)) {
		t.Fatal("左上闭区间内应命中")
	}
	if r.Contains(Pt(10, 5)) || r.Contains(Pt(5, 10)) {
		t.Fatal("右下边界不应命中")
	}
	if r.Contains(Pt(-0.1, 5)) || r.Contains(Pt(5, -0.1)) {
		t.Fatal("外部点不应命中")
	}
}

func TestRectInsetOffset(t *testing.T) {
	r := R(10, 10, 100, 50).Inset(Only(5, 10, 15, 20))
	if !near(r.X, 30) || !near(r.Y, 15) || !near(r.W, 70) || !near(r.H, 30) {
		t.Fatalf("Inset 结果错误：%+v", r)
	}
	a := R(0, 0, 10, 10).InsetAll(2)
	if !near(a.X, 2) || !near(a.Y, 2) || !near(a.W, 6) || !near(a.H, 6) {
		t.Fatalf("InsetAll 结果错误：%+v", a)
	}
	o := R(0, 0, 10, 10).Offset(3, -2)
	if !near(o.X, 3) || !near(o.Y, -2) || !near(o.W, 10) {
		t.Fatalf("Offset 结果错误：%+v", o)
	}
}

func TestRectIntersect(t *testing.T) {
	a := R(0, 0, 10, 10)
	b := R(5, 5, 10, 10)
	i := a.Intersect(b)
	if !near(i.X, 5) || !near(i.Y, 5) || !near(i.W, 5) || !near(i.H, 5) {
		t.Fatalf("相交结果错误：%+v", i)
	}
	// 完全不相交、仅相切（边贴边）都返回空矩形
	if got := a.Intersect(R(20, 20, 5, 5)); got != (Rect{}) {
		t.Fatalf("不相交应返回空矩形，实际 %+v", got)
	}
	if got := a.Intersect(R(10, 0, 5, 5)); got != (Rect{}) {
		t.Fatalf("相切应返回空矩形，实际 %+v", got)
	}
}

func TestRectUnion(t *testing.T) {
	u := R(0, 0, 10, 10).Union(R(20, 5, 10, 10))
	if !near(u.X, 0) || !near(u.Y, 0) || !near(u.W, 30) || !near(u.H, 15) {
		t.Fatalf("Union 结果错误：%+v", u)
	}
}

func TestInsetsHelpers(t *testing.T) {
	if got := All(3); got != (Insets{3, 3, 3, 3}) {
		t.Fatalf("All 错误：%+v", got)
	}
	if got := Sym(4, 6); got != (Insets{6, 4, 6, 4}) {
		t.Fatalf("Sym 错误：%+v", got)
	}
	if got := Only(1, 2, 3, 4); !near(got.Horizontal(), 6) || !near(got.Vertical(), 4) {
		t.Fatalf("Horizontal/Vertical 错误：%+v", got)
	}
}
