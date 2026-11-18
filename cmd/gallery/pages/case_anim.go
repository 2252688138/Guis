// guis — gallery 页面拆分：组件动画
// 本文件属于 pages 包，与其它页面文件一起由 main.go 引入。
//
// 演示 ui/anim_widget.go 的组件动画：任意节点都能 .Anim(...) 绑定效果，
// 绘制时整棵子树（文字 / 图标 / 图片 / 底板）一起做位移、缩放与淡入。

package pages

import (
	"guis/ui"
	"strconv"
)

// 本页演示状态（切换页面后保留，无其它副作用）。
var (
	// animRound 出场动画播放轮次：拼进节点 Key，Key 一变播放状态就重建 → 从头再播。
	animRound int
	// animLoopOn 循环动画（呼吸 / 漂浮）开关，用来演示「绑定 / 不绑定」的切换。
	animLoopOn = true
	// animShakeReq 下一帧请求「抖一下」：置位后当帧调用 Node.Replay()。
	animShakeReq bool
	// animExitOn 出场演示开关：关掉后 Present 保留期内继续画，退场效果播完才消失。
	animExitOn = true
)

// animTile 一块演示瓦片：图标 + 名称 + 效果名。
func animTile(glyph, name, effect, color string) *ui.Node {
	return ui.Column(
		ui.Text(glyph).St(ui.St().Text(20, 600, color)),
		ui.Text(name).St(ui.St().Text(13, 600, ui.ColText)),
		ui.Text(effect).St(ui.St().Text(11, 400, ui.ColTextMuted)),
	).St(ui.St().W(132).H(96).Padding(12).R(12).GapOf(4).
		BgC(ui.ColHover).Stroke(1, ui.ColLine)).AlignTo(ui.AlignCenter)
}

// animShowcaseCard 出场动画：六种一次性效果 + 错峰延迟。
func animShowcaseCard(rk string) *ui.Node {
	return ui.Card(
		ui.Row(
			ui.Title("出场动画", "FadeIn / SlideIn / PopIn / ZoomIn / Bounce，DelayBy 做错峰"),
			ui.Spacer(),
			ui.Btn("重新播放", ui.BtnSecondary, func() {
				animRound++
				ui.RequestRedraw()
			}),
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Wrap(
			animTile("○", "淡入", "FadeIn(0.36)", ui.ColPrimary).
				Anim(ui.FadeIn(0.36)).K("anim-fade"+rk),
			animTile("↓", "上滑", "SlideIn(SideBottom)", ui.ColSuccess).
				Anim(ui.SlideIn(ui.SideBottom, 26, 0.4).DelayBy(0.05)).K("anim-up"+rk),
			animTile("→", "左滑", "SlideIn(SideLeft)", ui.ColWarn).
				Anim(ui.SlideIn(ui.SideLeft, 26, 0.4).DelayBy(0.1)).K("anim-left"+rk),
			animTile("◎", "弹出", "PopIn(0.42)", ui.ColText).
				Anim(ui.PopIn(0.42).DelayBy(0.15)).K("anim-pop"+rk),
			animTile("⤢", "收拢", "ZoomIn(0.4)", ui.ColPrimary).
				Anim(ui.ZoomIn(0.4).DelayBy(0.2)).K("anim-zoom"+rk),
			animTile("↑", "弹跳", "Bounce(0.6)", ui.ColSuccess).
				Anim(ui.Bounce(0.6).DelayBy(0.25)).K("anim-bounce"+rk),
		).St(ui.St().GapOf(12)),
		ui.Text("一次性效果以「节点首次出现」为起点；重新播放靠换 Key（也可以调 Node.Replay()）。").
			St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
	)
}

// animLoopCard 循环动画：呼吸与漂浮，可整体关掉。
func animLoopCard() *ui.Node {
	pulse := ui.Dot(ui.ColSuccess, 12)
	float := ui.Text("✈").St(ui.St().Text(20, 600, ui.ColPrimary))
	if animLoopOn {
		pulse.Anim(ui.Pulse(1.6).Amount(0.35))
		float.Anim(ui.FloatY(7, 2.2))
	}
	return ui.Card(
		ui.Row(
			ui.Title("循环动画", "Pulse 呼吸 / FloatY 漂浮：一直播放，用来做状态灯、加载提示"),
			ui.Spacer(),
			ui.Switch("anim-loop", animLoopOn, func(v bool) {
				animLoopOn = v
				ui.RequestRedraw()
			}),
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Column(
				pulse,
				ui.Text("Pulse(1.6).Amount(0.35)").St(ui.St().Text(11, 400, ui.ColTextMuted)),
			).St(ui.St().GapOf(8).W(160).Padding(12).R(12).BgC(ui.ColHover).Stroke(1, ui.ColLine)).
				AlignTo(ui.AlignCenter),
			ui.Column(
				float,
				ui.Text("FloatY(7, 2.2)").St(ui.St().Text(11, 400, ui.ColTextMuted)),
			).St(ui.St().GapOf(8).W(160).Padding(12).R(12).BgC(ui.ColHover).Stroke(1, ui.ColLine)).
				AlignTo(ui.AlignCenter),
			ui.Column(
				ui.Text("循环动画不会阻塞输入：命中、点击、滚动全部按原矩形计算。").
					St(ui.St().Text(12, 400, ui.ColTextMuted).WrapOn()),
			).St(ui.St().GrowTo(1)).AlignTo(ui.AlignCenter),
		).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
	)
}

// animInteractCard 交互反馈：抖动强调（Replay 触发）+ 叠加效果。
func animInteractCard() *ui.Node {
	shake := animTile("!", "校验失败", "Shake(0.45) 点按钮触发", ui.ColDanger).
		K("anim-shake").Anim(ui.Shake(0.45).Amount(9))
	if animShakeReq {
		// 同一帧内请求重播：置位只在当帧有效，之后由动画状态自己跑完
		shake.Replay()
		animShakeReq = false
	}
	return ui.Card(
		ui.Title("交互反馈", "Shake 抖一下 / 多个效果叠加：节点可以同时挂任意条"),
		ui.Row(
			ui.Btn("抖一下", ui.BtnDanger, func() {
				animShakeReq = true
				ui.RequestRedraw()
			}),
			shake,
			ui.Spacer(),
			animTile("★", "叠加", "PopIn + Pulse", ui.ColWarn).
				K("anim-stack").Anim(ui.PopIn(0.5), ui.Pulse(2.4).Amount(0.03)),
			animTile("✦", "绑定任意节点", "Card / Badge / Progress / 图标", ui.ColPrimary).
				K("anim-any").Anim(ui.SlideIn(ui.SideRight, 20, 0.5).DelayBy(0.1)),
		).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
		ui.Row(
			ui.Badge(3).Anim(ui.PopIn(0.5).DelayBy(0.2)).K("anim-badge"),
			ui.Tag("标签", ui.ColSuccess).Anim(ui.FadeIn(0.5).DelayBy(0.25)).K("anim-tag"),
			ui.Progress(0.62, ui.ColPrimary).Anim(ui.FadeIn(0.5).DelayBy(0.3)).K("anim-progress"),
			ui.Spacer(),
			ui.Text("透明度与位移都在顶点层完成，文字不会被「先画后擦」地闪一下。").
				St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
		).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
		ui.Text("开启系统「减弱动态效果」时所有效果直接落到终态（ui.ReducedMotion() 生效）。").
			St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
	)
}

// animMoreCard 更多特效：翻展开 / 旋转 / 自旋 / 摆动 / 果冻 / 心跳 / 强调 / 闪烁 / 掉落 / 橡皮筋。
func animMoreCard(rk string) *ui.Node {
	return ui.Card(
		ui.Row(
			ui.Title("更多特效", "FlipInX / FlipInY / RotateIn / Spin / Swing / Jello / Heartbeat / Tada / Flash / DropIn / RubberBand"),
			ui.Spacer(),
			ui.Btn("重新播放", ui.BtnSecondary, func() {
				animRound++
				ui.RequestRedraw()
			}),
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Wrap(
			animTile("◧", "横展开", "FlipInX(0.4)", ui.ColPrimary).
				Anim(ui.FlipInX(0.4).DelayBy(0.05)).K("anim-flipx"+rk),
			animTile("⬒", "纵展开", "FlipInY(0.4)", ui.ColSuccess).
				Anim(ui.FlipInY(0.4).DelayBy(0.1)).K("anim-flipy"+rk),
			animTile("↻", "旋转入场", "RotateIn(0.45)", ui.ColWarn).
				Anim(ui.RotateIn(0.45).DelayBy(0.15)).K("anim-rotin"+rk),
			animTile("⟳", "持续旋转", "Spin(1.4)", ui.ColText).
				K("anim-spin").Anim(ui.Spin(1.4)),
			animTile("⇆", "摆动", "Swing(0.7).LoopOn()", ui.ColPrimary).
				K("anim-swing").Anim(ui.Swing(0.7).LoopOn()),
			animTile("❉", "果冻", "Jello(0.6).LoopOn()", ui.ColSuccess).
				K("anim-jello").Anim(ui.Jello(0.6).LoopOn()),
			animTile("❤", "心跳", "Heartbeat(0.9).LoopOn()", ui.ColDanger).
				K("anim-heart").Anim(ui.Heartbeat(0.9).LoopOn()),
			animTile("★", "强调", "Tada(0.8).LoopOn()", ui.ColWarn).
				K("anim-tada").Anim(ui.Tada(0.8).LoopOn()),
			animTile("◉", "闪烁", "Flash(0.7).LoopOn()", ui.ColPrimary).
				K("anim-flash").Anim(ui.Flash(0.7).LoopOn()),
			animTile("⬇", "掉落", "DropIn(26, 0.5)", ui.ColSuccess).
				Anim(ui.DropIn(26, 0.5).DelayBy(0.2)).K("anim-drop"+rk),
			animTile("⇔", "橡皮筋", "RubberBand(0.7).LoopOn()", ui.ColText).
				K("anim-rubber").Anim(ui.RubberBand(0.7).LoopOn()),
		).St(ui.St().GapOf(12)),
		ui.Text("旋转类效果走绘制期的仿射变换（缩放 + 旋转 + 平移），文字与图标跟着一起转。").
			St(ui.St().Text(11.5, 400, ui.ColTextMuted)),
	)
}

// animExitCard 出场动画：Present 保留期 + *Out 效果，关掉后再播完退场才消失。
func animExitCard() *ui.Node {
	kids := make([]*ui.Node, 0, 2)
	if ui.Present("gallery-exit-demo", animExitOn, 0.24) {
		kids = append(kids, animTile("↩", "出场", "SlideOut + FadeOut", ui.ColWarn).
			K("anim-exit-demo").
			Anim(ui.PopIn(0.24)).
			AnimIf(!animExitOn, ui.SlideOut(ui.SideBottom, 18, 0.22), ui.FadeOut(0.22)))
	}
	kids = append(kids, ui.Text("Dialog / Drawer / Modal / Toast / Tooltip 已经内置这套进出场动画。").
		St(ui.St().Text(11.5, 400, ui.ColTextMuted)))
	return ui.Card(
		ui.Row(
			ui.Title("出场动画", "Present(key, shown, dur) 保留一小段，退场效果播完才真正消失"),
			ui.Spacer(),
			ui.Switch("anim-exit", animExitOn, func(v bool) {
				animExitOn = v
				ui.RequestRedraw()
			}),
		).St(ui.St().GapOf(10)).AlignTo(ui.AlignCenter),
		ui.Row(kids...).St(ui.St().GapOf(12)).AlignTo(ui.AlignCenter),
	)
}

// animCase 组件动画页。
func animCase() *ui.Node {
	rk := strconv.Itoa(animRound)
	return ui.Column(
		pageHead("组件动画", "给任意节点绑定动画效果：进出场 / 呼吸 / 抖动 / 旋转 / 强调，整棵子树一起动"),
		animShowcaseCard(rk),
		animMoreCard(rk),
		animLoopCard(),
		animInteractCard(),
		animExitCard(),
	).St(ui.St().Padding(26).GapOf(16))
}
