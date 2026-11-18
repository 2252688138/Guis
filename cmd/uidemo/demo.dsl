# 同一份界面的 DSL 写法：缩进 = 层级，属性 = ui.St() 的同名方法
# 改这里保存，窗口不动就能看到结果（Go 里跑：go run ./cmd/uidemo -f demo.dsl）

col gap=14
  text "改这个文件，界面立刻变" size=22 weight=700 color=#e6edf7
  text "这个界面完全由 demo.dsl 描述：一行一个节点，缩进决定层级，属性名就是 ui.St() 的方法名。按钮的行为仍然写在 Go 里。" size=13 color=#8b97ab

  row gap=10
    button "点我（Go 回调）" id=btn-hi onClick=say-hi
    button "换主题色" id=btn-accent onClick=accent
    button "手动重载" id=btn-reload onClick=reload

  box pad=16 r=12 bg=#151f33 gap=8
    text "卡片 = box + bg + r + pad" size=14 weight=600 color=#cfd8e6
    text "所有样式字段都对应 ui.St() 的同名方法（w/h/pad/gap/bg/r/stroke/shadow/size/color/align…）。" size=12 color=#8b97ab

  grid cols=3 gap=10
    box pad=14 bg=#141b2b r=10
      text "网格 1" size=13 color=#cfd8e6
    box pad=14 bg=#141b2b r=10
      text "网格 2" size=13 color=#cfd8e6
    box pad=14 bg=#141b2b r=10
      text "网格 3" size=13 color=#cfd8e6

  row gap=8
    icon star size=18 color=#f5c451
    text "图标也用名字引用内置图标集" size=13 color=#8b97ab

  scroll id=list grow=1 gap=6
    text "01 · 滚动区域里的第一项" size=13 color=#cfd8e6
    text "02 · 第二项" size=13 color=#cfd8e6
    text "03 · 第三项" size=13 color=#cfd8e6
    text "04 · 第四项" size=13 color=#cfd8e6
    text "05 · 第五项" size=13 color=#cfd8e6
    text "06 · 第六项" size=13 color=#cfd8e6
    text "07 · 第七项" size=13 color=#cfd8e6
    text "08 · 第八项" size=13 color=#cfd8e6
    text "09 · 第九项" size=13 color=#cfd8e6
    text "10 · 第十项" size=13 color=#cfd8e6
    text "11 · 第十一项" size=13 color=#cfd8e6
    text "12 · 第十二项" size=13 color=#cfd8e6
