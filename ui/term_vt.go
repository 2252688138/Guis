// guis — 伪终端组件（一）：VT/ANSI 解析与屏幕网格
//
// 这是 xterm.js 的 Go 版核心：把 PTY 输出的字节流解析成「字符网格 + 属性」，
// 再由 term_paint.go 逐行绘制。刻意只保留终端实际用得到的子集：
//
//	光标移动 CUU/CUD/CUF/CUB/CNL/CPL/CHA/CUP/VPA/HVP
//	擦除     ED / EL / ECH
//	行编辑   IL / DL / ICH / DCH
//	滚动     SU / SD / DECSTBM（区域滚动）
//	SGR      粗体/暗淡/斜体/下划线/闪烁/反显/隐藏/删除线 + 16/256/真彩色
//	模式     DECAWM 自动换行、DECTCEM 光标显示、备用屏（1047/1049）、应用光标键
//	其它     OSC 0/2 标题、BEL、DSR/DA 应答、制表位、宽字符（CJK/Emoji 占两格）
//
// 解析是「字节驱动」的：一个 termScreen 可以被任意 goroutine 喂数据（调用方
// 负责加锁），因此 PTY 读协程可以边读边解析，UI 线程只负责绘制。
package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ---- 颜色 ----

// termColor 终端颜色（紧凑编码，避免每个单元格存两份 Paint）。
//
//	0                  = 默认前景/背景（跟随主题）
//	0x01_00_00 | n     = 256 色调色板下标 n
//	0x02_00_00 | 0xRRGGBB = 真彩色
type termColor uint32

const (
	termColorDefault     termColor = 0
	termColorIndexedBase termColor = 0x01000000
	termColorRGBBase     termColor = 0x02000000
)

// termIndexed 256 色调色板颜色。
func termIndexed(i int) termColor {
	if i < 0 || i > 255 {
		return termColorDefault
	}
	return termColorIndexedBase | termColor(i)
}

// termRGB 真彩色。
func termRGB(r, g, b uint8) termColor {
	return termColorRGBBase | termColor(r)<<16 | termColor(g)<<8 | termColor(b)
}

// ---- 单元格属性 ----

// termAttr 单元格样式位。
type termAttr uint16

const (
	termAttrBold termAttr = 1 << iota
	termAttrFaint
	termAttrItalic
	termAttrUnderline
	termAttrBlink
	termAttrReverse
	termAttrConceal
	termAttrStrike
	// termAttrWideTail 宽字符（CJK / Emoji）占位的第二格：不画、可被覆盖。
	termAttrWideTail
)

// termCell 一个字符单元格。
type termCell struct {
	ch   rune
	fg   termColor
	bg   termColor
	attr termAttr
}

// termBlank 空白单元格（ch 为 0 表示没有字形）。
func termBlank() termCell { return termCell{} }

// termLine 一行固定列数的单元格。
type termLine []termCell

// termScreen 终端屏幕模型（主屏 / 备用屏 / 回看缓冲）。
type termScreen struct {
	cols, rows int

	grid termLine

	curX, curY int
	wrap       bool // DECAWM 自动换行
	wrapDone   bool // 已到行尾，下一次输出先换行

	curFg   termColor
	curBg   termColor
	curAttr termAttr

	top, bot int // 滚动区域（0 基，含端点）

	cursorVisible bool
	cursorStyle   int // 0 块 1 竖线 2 下划线
	alt           bool
	altGrid       termLine
	altX, altY    int

	savedX, savedY   int
	savedFg, savedBg termColor
	savedAttr        termAttr

	scrollBack []termLine
	scrollMax  int

	tabs     []bool
	insert   bool
	appCur   bool // 应用光标键模式（DECCKM）
	appKey   bool // 应用小键盘（忽略，只记录）
	mouse    int  // 0 关；1000/1002/1003 记录
	mouseSGR bool
	brPaste  bool // 括号粘贴（2004）

	// 解析状态
	state    int
	params   []int
	curParam int
	hasParam bool
	private  byte
	inter    []byte
	osc      []byte
	utf8Tail []byte

	title    string
	bell     bool
	reply    []byte
	lastRune rune
	// dropped 回看缓冲被挤掉的行数（上层据此平移选区）。
	dropped int

	dirty bool
}

// 解析状态。
const (
	termStGround = iota
	termStEscape
	termStEscapeInter
	termStCSI
	termStOSC
	termStOSCEsc
	termStDCS
	termStDCSEsc
)

// newTermScreen 创建屏幕（cols × rows，回看上限 scrollMax 行）。
func newTermScreen(cols, rows, scrollMax int) *termScreen {
	if cols < 2 {
		cols = 80
	}
	if rows < 2 {
		rows = 24
	}
	if scrollMax < 0 {
		scrollMax = 0
	}
	s := &termScreen{
		cols: cols, rows: rows, scrollMax: scrollMax,
		wrap: true, cursorVisible: true,
		curFg: termColorDefault, curBg: termColorDefault,
	}
	s.grid = s.blankGrid()
	s.top, s.bot = 0, rows-1
	s.resetTabs()
	return s
}

// blankGrid 生成 rows × cols 的空白网格。
func (s *termScreen) blankGrid() termLine {
	lines := make(termLine, s.rows*s.cols)
	return lines
}

// lineAt 取第 row 行的切片（不检查越界）。
func (s *termScreen) lineAt(g termLine, row int) []termCell {
	return g[row*s.cols : (row+1)*s.cols]
}

// row 当前屏第 row 行。
func (s *termScreen) row(row int) []termCell {
	if row < 0 || row >= s.rows {
		return nil
	}
	return s.lineAt(s.grid, row)
}

// resetTabs 每 8 列一个制表位。
func (s *termScreen) resetTabs() {
	s.tabs = make([]bool, s.cols)
	for i := 8; i < s.cols; i += 8 {
		s.tabs[i] = true
	}
}

// clearCell 把单元格恢复成「当前背景色的空白」。
func (s *termScreen) clearCell(c *termCell) {
	*c = termCell{bg: s.curBg}
}

// clearRange 清空一行里的 [x0, x1)。
func (s *termScreen) clearRange(line []termCell, x0, x1 int) {
	if x0 < 0 {
		x0 = 0
	}
	if x1 > s.cols {
		x1 = s.cols
	}
	for x := x0; x < x1; x++ {
		s.clearCell(&line[x])
	}
}

// feed 解析一段字节流。
func (s *termScreen) feed(p []byte) {
	if len(p) == 0 {
		return
	}
	if len(s.utf8Tail) > 0 {
		buf := make([]byte, 0, len(s.utf8Tail)+len(p))
		buf = append(buf, s.utf8Tail...)
		buf = append(buf, p...)
		p = buf
		s.utf8Tail = nil
	}
	s.dirty = true
	i := 0
	for i < len(p) {
		b := p[i]
		switch s.state {
		case termStGround:
			if b == 0x1b {
				s.state = termStEscape
				i++
				continue
			}
			if b < 0x20 || b == 0x7f {
				s.control(b)
				i++
				continue
			}
			if b < 0x80 {
				s.putRune(rune(b))
				i++
				continue
			}
			if !utf8.FullRune(p[i:]) {
				s.utf8Tail = append(s.utf8Tail[:0], p[i:]...)
				return
			}
			r, size := utf8.DecodeRune(p[i:])
			if r == utf8.RuneError && size <= 1 {
				i++
				continue
			}
			s.putRune(r)
			i += size
		case termStEscape:
			s.escape(b)
			i++
		case termStEscapeInter:
			// ESC ( ) * + # % 等字符集选择：吃掉一个字节即可。
			if b == 0x1b {
				s.state = termStEscape
			} else {
				s.state = termStGround
			}
			i++
		case termStCSI:
			i = s.csi(p, i)
		case termStOSC:
			if b == 0x07 {
				s.finishOSC()
				i++
			} else if b == 0x1b {
				s.state = termStOSCEsc
				i++
			} else {
				if len(s.osc) < 4096 {
					s.osc = append(s.osc, b)
				}
				i++
			}
		case termStOSCEsc:
			if b == '\\' {
				s.finishOSC()
			} else {
				// 不是 ST：OSC 内容作废，按普通 ESC 序列继续。
				s.osc = s.osc[:0]
				s.state = termStEscape
				s.escape(b)
			}
			i++
		case termStDCS:
			if b == 0x1b {
				s.state = termStDCSEsc
			}
			i++
		case termStDCSEsc:
			if b == '\\' {
				s.state = termStGround
			} else {
				s.state = termStDCS
			}
			i++
		default:
			s.state = termStGround
			i++
		}
	}
}

// control 处理 C0 控制字符。
func (s *termScreen) control(b byte) {
	switch b {
	case 0x07: // BEL
		s.bell = true
	case 0x08: // BS
		if s.curX > 0 {
			s.curX--
		}
		s.wrapDone = false
	case 0x09: // HT
		s.tabForward(1)
	case 0x0a, 0x0b, 0x0c: // LF / VT / FF
		s.lineFeed()
		s.wrapDone = false
	case 0x0d: // CR
		s.curX = 0
		s.wrapDone = false
	case 0x0e, 0x0f: // SO / SI 字符集，忽略
	default:
	}
}

// escape 处理 ESC 之后的字节。
func (s *termScreen) escape(b byte) {
	s.state = termStGround
	switch b {
	case '[':
		s.state = termStCSI
		s.params = s.params[:0]
		s.curParam, s.hasParam, s.private = 0, false, 0
		s.inter = s.inter[:0]
	case ']':
		s.state = termStOSC
		s.osc = s.osc[:0]
	case 'P', '^', '_':
		s.state = termStDCS
	case '(' , ')', '*', '+', '-', '.', '/', '#', '%':
		s.state = termStEscapeInter
	case '7':
		s.saveCursor()
	case '8':
		s.restoreCursor()
	case 'D': // IND 索引
		s.lineFeed()
	case 'E': // NEL 下一行
		s.curX = 0
		s.lineFeed()
	case 'M': // RI 反向索引
		s.reverseIndex()
	case 'H': // HTS 设置制表位
		if s.curX >= 0 && s.curX < s.cols {
			s.tabs[s.curX] = true
		}
	case 'c': // RIS 完全重置
		s.reset()
	case '=': // DECKPAM
		s.appKey = true
	case '>': // DECKPNM
		s.appKey = false
	case '\\': // ST
	default:
	}
}

// csi 处理 CSI 序列，返回新的字节下标。
func (s *termScreen) csi(p []byte, i int) int {
	b := p[i]
	switch {
	case b >= 0x30 && b <= 0x39:
		s.curParam = s.curParam*10 + int(b-'0')
		if s.curParam > 65535 {
			s.curParam = 65535
		}
		s.hasParam = true
		return i + 1
	case b == ';' || b == ':':
		s.params = append(s.params, s.curParam)
		s.curParam, s.hasParam = 0, false
		return i + 1
	case b >= 0x3c && b <= 0x3f: // < = > ?
		if len(s.params) == 0 && !s.hasParam {
			s.private = b
		}
		return i + 1
	case b >= 0x20 && b <= 0x2f: // 中间字节
		s.inter = append(s.inter, b)
		return i + 1
	case b >= 0x40 && b <= 0x7e: // final
		if s.hasParam {
			s.params = append(s.params, s.curParam)
			s.hasParam = false
		}
		s.state = termStGround
		s.dispatchCSI(b)
		return i + 1
	case b == 0x1b:
		s.state = termStEscape
		return i + 1
	default:
		s.state = termStGround
		return i + 1
	}
}

// param 取第 i 个参数（缺省或 0 用 def）。
func (s *termScreen) param(i, def int) int {
	if i >= len(s.params) || s.params[i] <= 0 {
		return def
	}
	return s.params[i]
}

// rawParam 取第 i 个参数（缺省为 0）。
func (s *termScreen) rawParam(i int) int {
	if i >= len(s.params) {
		return 0
	}
	return s.params[i]
}

// dispatchCSI 执行 CSI 命令。
func (s *termScreen) dispatchCSI(final byte) {
	switch final {
	case '@': // ICH 插入字符
		n := s.param(0, 1)
		s.insertChars(n)
	case 'A': // CUU
		s.moveY(-s.param(0, 1))
	case 'B', 'e': // CUD / VPR
		s.moveY(s.param(0, 1))
	case 'C', 'a': // CUF / HPR
		s.moveX(s.param(0, 1))
	case 'D': // CUB
		s.moveX(-s.param(0, 1))
	case 'E': // CNL
		s.moveY(s.param(0, 1))
		s.curX = 0
		s.wrapDone = false
	case 'F': // CPL
		s.moveY(-s.param(0, 1))
		s.curX = 0
		s.wrapDone = false
	case 'G', '`': // CHA / HPA
		s.curX = s.clampCol(s.param(0, 1) - 1)
		s.wrapDone = false
	case 'H', 'f': // CUP / HVP
		row, col := s.param(0, 1)-1, s.param(1, 1)-1
		s.curY = s.clampRow(row)
		s.curX = s.clampCol(col)
		s.wrapDone = false
	case 'I': // CHT 制表位前进
		s.tabForward(s.param(0, 1))
	case 'J': // ED
		s.eraseDisplay(s.rawParam(0))
	case 'K': // EL
		s.eraseLine(s.rawParam(0))
	case 'L': // IL 插入行
		s.insertLines(s.param(0, 1))
	case 'M': // DL 删除行
		s.deleteLines(s.param(0, 1))
	case 'P': // DCH 删除字符
		s.deleteChars(s.param(0, 1))
	case 'S': // SU 上滚
		s.scrollUp(s.param(0, 1), true)
	case 'T': // SD 下滚
		s.scrollDown(s.param(0, 1), true)
	case 'X': // ECH 擦除字符
		n := s.param(0, 1)
		if ln := s.row(s.curY); ln != nil {
			s.clearRange(ln, s.curX, s.curX+n)
		}
	case 'Z': // CBT 制表位后退
		s.tabBack(s.param(0, 1))
	case 'b': // REP 重复上一字符
		s.repeatLast(s.param(0, 1))
	case 'c': // DA 设备属性
		if s.private == '>' {
			s.reply = append(s.reply, "\x1b[>0;276;0c"...)
		} else {
			s.reply = append(s.reply, "\x1b[?1;2c"...)
		}
	case 'd': // VPA 绝对行
		s.curY = s.clampRow(s.param(0, 1) - 1)
		s.wrapDone = false
	case 'g': // TBC 清除制表位
		switch s.rawParam(0) {
		case 0:
			if s.curX < s.cols {
				s.tabs[s.curX] = false
			}
		case 3:
			for i := range s.tabs {
				s.tabs[i] = false
			}
		}
	case 'h':
		s.setMode(true)
	case 'l':
		s.setMode(false)
	case 'm':
		s.sgr()
	case 'n': // DSR
		switch s.rawParam(0) {
		case 5:
			s.reply = append(s.reply, "\x1b[0n"...)
		case 6:
			s.reply = append(s.reply, []byte("\x1b["+strconv.Itoa(s.curY+1)+";"+strconv.Itoa(s.curX+1)+"R")...)
		}
	case 'q': // DECSCUSR 光标样式
		switch s.rawParam(0) {
		case 0, 1:
			s.cursorStyle = 0
		case 2:
			s.cursorStyle = 0
		case 3, 4:
			s.cursorStyle = 1
		case 5, 6:
			s.cursorStyle = 2
		}
	case 'r': // DECSTBM 滚动区域
		top := s.param(0, 1) - 1
		bot := s.param(1, s.rows) - 1
		if top < 0 {
			top = 0
		}
		if bot >= s.rows {
			bot = s.rows - 1
		}
		if top < bot {
			s.top, s.bot = top, bot
			s.curX, s.curY = 0, top
			s.wrapDone = false
		}
	case 's': // 保存光标（ANSI）
		s.saveCursor()
	case 'u': // 恢复光标（ANSI）
		s.restoreCursor()
	case 't': // 窗口操作：18 报告字符尺寸
		if s.rawParam(0) == 18 {
			s.reply = append(s.reply, []byte("\x1b[8;"+strconv.Itoa(s.rows)+";"+strconv.Itoa(s.cols)+"t")...)
		}
	case 'p': // CSI ! p 软重置
		if len(s.inter) > 0 && s.inter[0] == '!' {
			s.softReset()
		}
	default:
	}
}

// setMode 处理 SM/RM（含私有模式 ?）。
func (s *termScreen) setMode(on bool) {
	if s.private == '?' {
		for _, m := range s.params {
			switch m {
			case 1: // DECCKM 应用光标键
				s.appCur = on
			case 6: // DECOM 原点模式（忽略，按绝对定位）
			case 7: // DECAWM 自动换行
				s.wrap = on
			case 25: // DECTCEM 光标显示
				s.cursorVisible = on
			case 47, 1047:
				s.setAlt(on)
			case 1048:
				if on {
					s.saveCursor()
				} else {
					s.restoreCursor()
				}
			case 1049:
				if on {
					s.saveCursor()
					s.setAlt(true)
				} else {
					s.setAlt(false)
					s.restoreCursor()
				}
			case 1000, 1002, 1003:
				if on {
					s.mouse = m
				} else if s.mouse == m {
					s.mouse = 0
				}
			case 1006:
				s.mouseSGR = on
			case 2004:
				s.brPaste = on
			}
		}
		return
	}
	for _, m := range s.params {
		switch m {
		case 4: // IRM 插入模式
			s.insert = on
		}
	}
}

// sgr 处理 SGR（颜色 / 样式）。
func (s *termScreen) sgr() {
	if len(s.params) == 0 {
		s.curFg, s.curBg, s.curAttr = termColorDefault, termColorDefault, 0
		return
	}
	ps := s.params
	for i := 0; i < len(ps); i++ {
		p := ps[i]
		switch {
		case p == 0:
			s.curFg, s.curBg, s.curAttr = termColorDefault, termColorDefault, 0
		case p == 1:
			s.curAttr |= termAttrBold
		case p == 2:
			s.curAttr |= termAttrFaint
		case p == 3:
			s.curAttr |= termAttrItalic
		case p == 4:
			s.curAttr |= termAttrUnderline
		case p == 5 || p == 6:
			s.curAttr |= termAttrBlink
		case p == 7:
			s.curAttr |= termAttrReverse
		case p == 8:
			s.curAttr |= termAttrConceal
		case p == 9:
			s.curAttr |= termAttrStrike
		case p == 21:
			s.curAttr &^= termAttrBold
		case p == 22:
			s.curAttr &^= termAttrBold | termAttrFaint
		case p == 23:
			s.curAttr &^= termAttrItalic
		case p == 24:
			s.curAttr &^= termAttrUnderline
		case p == 25:
			s.curAttr &^= termAttrBlink
		case p == 27:
			s.curAttr &^= termAttrReverse
		case p == 28:
			s.curAttr &^= termAttrConceal
		case p == 29:
			s.curAttr &^= termAttrStrike
		case p >= 30 && p <= 37:
			s.curFg = termIndexed(p - 30)
		case p == 38:
			c, used := s.extColor(ps, i)
			s.curFg = c
			i += used
		case p == 39:
			s.curFg = termColorDefault
		case p >= 40 && p <= 47:
			s.curBg = termIndexed(p - 40)
		case p == 48:
			c, used := s.extColor(ps, i)
			s.curBg = c
			i += used
		case p == 49:
			s.curBg = termColorDefault
		case p == 58:
			// 下划线颜色：解析后丢弃（不做下划线分色）
			_, used := s.extColor(ps, i)
			i += used
		case p >= 90 && p <= 97:
			s.curFg = termIndexed(p - 90 + 8)
		case p >= 100 && p <= 107:
			s.curBg = termIndexed(p - 100 + 8)
		}
	}
}

// extColor 解析 38/48/58 的扩展颜色参数，返回颜色与消耗的参数个数。
func (s *termScreen) extColor(ps []int, i int) (termColor, int) {
	if i+1 >= len(ps) {
		return termColorDefault, 0
	}
	switch ps[i+1] {
	case 5: // 256 色
		if i+2 < len(ps) {
			return termIndexed(ps[i+2]), 2
		}
		return termColorDefault, 1
	case 2: // 真彩色（;2;r;g;b 或 :2::r:g:b，子参数被展平成连续值）
		j := i + 2
		// 某些终端会插入色彩空间 id（0），跳过
		if j < len(ps) && ps[j] == 0 && j+3 < len(ps) {
			j++
		}
		if j+2 < len(ps) {
			r, g, b := ps[j], ps[j+1], ps[j+2]
			return termRGB(uint8(r), uint8(g), uint8(b)), j + 3 - i - 1
		}
		return termColorDefault, len(ps) - i - 1
	}
	return termColorDefault, 1
}

// putRune 写入一个字符（处理宽字符与自动换行）。
func (s *termScreen) putRune(r rune) {
	w := termRuneWidth(r)
	if w <= 0 {
		return // 组合字符 / 零宽：忽略（不做叠加渲染）
	}
	if s.wrapDone {
		s.curX = 0
		s.lineFeed()
		s.wrapDone = false
	}
	if s.curX+w > s.cols {
		if s.wrap {
			s.curX = 0
			s.lineFeed()
		} else {
			s.curX = s.cols - w
			if s.curX < 0 {
				s.curX = 0
			}
		}
	}
	ln := s.row(s.curY)
	if ln == nil {
		return
	}
	if s.insert {
		copy(ln[s.curX+w:], ln[s.curX:s.cols-w])
	}
	attr := s.curAttr
	// 注意：首格不能打 WideTail 标记（绘制会跳过该单元格，中文会整段消失），
	// 只有尾格（ch==0 的占位格）才打标记。
	ln[s.curX] = termCell{ch: r, fg: s.curFg, bg: s.curBg, attr: attr}
	if w == 2 && s.curX+1 < s.cols {
		ln[s.curX+1] = termCell{ch: 0, fg: s.curFg, bg: s.curBg, attr: s.curAttr | termAttrWideTail}
	}
	s.lastRune = r
	s.curX += w
	if s.curX >= s.cols {
		s.curX = s.cols - 1
		s.wrapDone = true
	}
	s.dirty = true
}

// repeatLast 重复最后输出的字符（REP）。
func (s *termScreen) repeatLast(n int) {
	if s.lastRune == 0 {
		return
	}
	if n <= 0 {
		n = 1
	}
	if n > s.cols*s.rows {
		n = s.cols * s.rows
	}
	r := s.lastRune
	for i := 0; i < n; i++ {
		s.putRune(r)
	}
}

// lineFeed 换行（在滚动区内滚屏）。
func (s *termScreen) lineFeed() {
	if s.curY == s.bot {
		s.scrollUp(1, true)
	} else if s.curY < s.rows-1 {
		s.curY++
	}
	s.dirty = true
}

// reverseIndex 反向换行（RI）。
func (s *termScreen) reverseIndex() {
	if s.curY == s.top {
		s.scrollDown(1, true)
	} else if s.curY > 0 {
		s.curY--
	}
	s.dirty = true
}

// scrollUp 滚动区上滚 n 行（新行补空白）；keep=true 时推进回看缓冲。
func (s *termScreen) scrollUp(n int, keep bool) {
	if n <= 0 {
		return
	}
	h := s.bot - s.top + 1
	if n > h {
		n = h
	}
	if keep && s.top == 0 && s.bot == s.rows-1 && !s.alt && s.scrollMax > 0 {
		for i := 0; i < n; i++ {
			line := make(termLine, s.cols)
			copy(line, s.row(i))
			s.scrollBack = append(s.scrollBack, line)
		}
		// 超出上限：丢弃最旧的整行
		if over := len(s.scrollBack) - s.scrollMax; over > 0 {
			s.scrollBack = append(s.scrollBack[:0], s.scrollBack[over:]...)
			s.dropped += over
		}
	}
	if s.top > 0 || s.bot < s.rows-1 || s.alt {
		// 区域滚动：整体前移后再清尾部
		for y := s.top; y <= s.bot-n; y++ {
			copy(s.row(y), s.row(y+n))
		}
		for y := s.bot - n + 1; y <= s.bot; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
		return
	}
	// 全屏滚动：直接挪切片，再清尾部（避免整屏拷贝）
	if h == s.rows {
		copy(s.grid, s.grid[n*s.cols:])
		for y := s.rows - n; y < s.rows; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
		return
	}
	for y := s.top; y <= s.bot-n; y++ {
		copy(s.row(y), s.row(y+n))
	}
	for y := s.bot - n + 1; y <= s.bot; y++ {
		s.clearRange(s.row(y), 0, s.cols)
	}
}

// scrollDown 滚动区下滚 n 行。
func (s *termScreen) scrollDown(n int, keep bool) {
	if n <= 0 {
		return
	}
	h := s.bot - s.top + 1
	if n > h {
		n = h
	}
	for y := s.bot; y >= s.top+n; y-- {
		copy(s.row(y), s.row(y-n))
	}
	for y := s.top; y < s.top+n; y++ {
		s.clearRange(s.row(y), 0, s.cols)
	}
	s.dirty = true
}

// moveX 水平移动光标（夹在滚动……屏幕范围内）。
func (s *termScreen) moveX(d int) {
	s.curX = s.clampCol(s.curX + d)
	s.wrapDone = false
}

// moveY 垂直移动光标（不越过滚动区）。
func (s *termScreen) moveY(d int) {
	y := s.curY + d
	if s.curY >= s.top && s.curY <= s.bot {
		if y < s.top {
			y = s.top
		}
		if y > s.bot {
			y = s.bot
		}
	} else {
		y = s.clampRow(y)
	}
	s.curY = y
	s.wrapDone = false
}

// clampCol 列号夹取。
func (s *termScreen) clampCol(x int) int {
	if x < 0 {
		return 0
	}
	if x >= s.cols {
		return s.cols - 1
	}
	return x
}

// clampRow 行号夹取。
func (s *termScreen) clampRow(y int) int {
	if y < 0 {
		return 0
	}
	if y >= s.rows {
		return s.rows - 1
	}
	return y
}

// tabForward 前进 n 个制表位。
func (s *termScreen) tabForward(n int) {
	for ; n > 0; n-- {
		x := s.curX + 1
		for x < s.cols && !s.tabs[x] {
			x++
		}
		if x >= s.cols {
			x = s.cols - 1
		}
		s.curX = x
	}
	s.wrapDone = false
}

// tabBack 后退 n 个制表位。
func (s *termScreen) tabBack(n int) {
	for ; n > 0; n-- {
		x := s.curX - 1
		for x > 0 && !s.tabs[x] {
			x--
		}
		if x < 0 {
			x = 0
		}
		s.curX = x
	}
	s.wrapDone = false
}

// eraseLine 擦除行（0 光标到行尾 / 1 行首到光标 / 2 整行）。
func (s *termScreen) eraseLine(mode int) {
	ln := s.row(s.curY)
	if ln == nil {
		return
	}
	switch mode {
	case 0:
		s.clearRange(ln, s.curX, s.cols)
	case 1:
		s.clearRange(ln, 0, s.curX+1)
	case 2:
		s.clearRange(ln, 0, s.cols)
	}
	s.dirty = true
}

// eraseDisplay 擦除屏幕（0 到末尾 / 1 到开头 / 2 全部 / 3 全部 + 回看）。
func (s *termScreen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		s.eraseLine(0)
		for y := s.curY + 1; y < s.rows; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
	case 1:
		s.eraseLine(1)
		for y := 0; y < s.curY; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
	case 2:
		for y := 0; y < s.rows; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
	case 3:
		for y := 0; y < s.rows; y++ {
			s.clearRange(s.row(y), 0, s.cols)
		}
		s.scrollBack = nil
	}
	s.dirty = true
}

// insertLines 在光标行插入 n 行（仅滚动区内）。
func (s *termScreen) insertLines(n int) {
	if s.curY < s.top || s.curY > s.bot {
		return
	}
	if n <= 0 {
		n = 1
	}
	if s.curY+n > s.bot+1 {
		n = s.bot - s.curY + 1
	}
	for y := s.bot; y >= s.curY+n; y-- {
		copy(s.row(y), s.row(y-n))
	}
	for y := s.curY; y < s.curY+n; y++ {
		s.clearRange(s.row(y), 0, s.cols)
	}
	s.curX = 0
	s.dirty = true
}

// deleteLines 删除光标行开始的 n 行（仅滚动区内）。
func (s *termScreen) deleteLines(n int) {
	if s.curY < s.top || s.curY > s.bot {
		return
	}
	if n <= 0 {
		n = 1
	}
	if s.curY+n > s.bot+1 {
		n = s.bot - s.curY + 1
	}
	for y := s.curY; y <= s.bot-n; y++ {
		copy(s.row(y), s.row(y+n))
	}
	for y := s.bot - n + 1; y <= s.bot; y++ {
		s.clearRange(s.row(y), 0, s.cols)
	}
	s.curX = 0
	s.dirty = true
}

// insertChars 在光标处插入 n 个空白。
func (s *termScreen) insertChars(n int) {
	ln := s.row(s.curY)
	if ln == nil || n <= 0 {
		return
	}
	if n > s.cols-s.curX {
		n = s.cols - s.curX
	}
	copy(ln[s.curX+n:], ln[s.curX:s.cols-n])
	s.clearRange(ln, s.curX, s.curX+n)
	s.dirty = true
}

// deleteChars 删除光标处 n 个字符。
func (s *termScreen) deleteChars(n int) {
	ln := s.row(s.curY)
	if ln == nil || n <= 0 {
		return
	}
	if n > s.cols-s.curX {
		n = s.cols - s.curX
	}
	copy(ln[s.curX:], ln[s.curX+n:])
	s.clearRange(ln, s.cols-n, s.cols)
	s.dirty = true
}

// saveCursor 保存光标与当前属性（DECSC）。
func (s *termScreen) saveCursor() {
	s.savedX, s.savedY = s.curX, s.curY
	s.savedFg, s.savedBg, s.savedAttr = s.curFg, s.curBg, s.curAttr
}

// restoreCursor 恢复光标（DECRC）。
func (s *termScreen) restoreCursor() {
	s.curX, s.curY = s.clampCol(s.savedX), s.clampRow(s.savedY)
	s.curFg, s.curBg, s.curAttr = s.savedFg, s.savedBg, s.savedAttr
	s.wrapDone = false
}

// setAlt 切换备用屏。
func (s *termScreen) setAlt(on bool) {
	if on == s.alt {
		return
	}
	if on {
		s.altGrid = s.grid
		s.altX, s.altY = s.curX, s.curY
		s.grid = s.blankGrid()
		s.curX, s.curY = 0, 0
	} else {
		if s.altGrid != nil {
			s.grid = s.altGrid
			s.altGrid = nil
		} else {
			s.grid = s.blankGrid()
		}
		s.curX, s.curY = s.clampCol(s.altX), s.clampRow(s.altY)
	}
	s.alt = on
	s.top, s.bot = 0, s.rows-1
	s.wrapDone = false
	s.dirty = true
}

// softReset 软重置（DECSTR）。
func (s *termScreen) softReset() {
	s.cursorVisible = true
	s.insert, s.appCur, s.appKey = false, false, false
	s.mouse, s.mouseSGR, s.brPaste = 0, false, false
	s.top, s.bot = 0, s.rows-1
	s.curFg, s.curBg, s.curAttr = termColorDefault, termColorDefault, 0
	s.wrap = true
}

// reset 完全重置（RIS）。
func (s *termScreen) reset() {
	s.grid = s.blankGrid()
	s.altGrid = nil
	s.alt = false
	s.curX, s.curY = 0, 0
	s.savedX, s.savedY = 0, 0
	s.curFg, s.curBg, s.curAttr = termColorDefault, termColorDefault, 0
	s.savedFg, s.savedBg, s.savedAttr = termColorDefault, termColorDefault, 0
	s.top, s.bot = 0, s.rows-1
	s.wrap, s.wrapDone = true, false
	s.cursorVisible = true
	s.insert, s.appCur, s.appKey = false, false, false
	s.mouse, s.mouseSGR, s.brPaste = 0, false, false
	s.scrollBack = nil
	s.resetTabs()
	s.dirty = true
}

// finishOSC 结束 OSC 序列（标题 / 超链接等）。
func (s *termScreen) finishOSC() {
	s.state = termStGround
	if len(s.osc) == 0 {
		return
	}
	body := string(s.osc)
	s.osc = s.osc[:0]
	idx := strings.IndexByte(body, ';')
	if idx < 0 {
		return
	}
	cmd, arg := body[:idx], body[idx+1:]
	switch cmd {
	case "0", "2":
		s.title = arg
	}
}

// resize 调整网格尺寸（保留可见内容与回看）。
func (s *termScreen) resize(cols, rows int) {
	if cols < 2 || rows < 2 || (cols == s.cols && rows == s.rows) {
		return
	}
	s.doResize(cols, rows)
}

// doResize 实际执行尺寸变更。
func (s *termScreen) doResize(cols, rows int) {
	oldCols, oldRows := s.cols, s.rows
	// 裁掉超出的回看列
	for i, ln := range s.scrollBack {
		s.scrollBack[i] = resizeLine(ln, oldCols, cols)
	}
	newGrid := make(termLine, rows*cols)
	copyRows := rows
	if oldRows < copyRows {
		copyRows = oldRows
	}
	for y := 0; y < copyRows; y++ {
		src := s.lineAt(s.grid, y)
		line := resizeLine(src, oldCols, cols)
		copy(newGrid[y*cols:(y+1)*cols], line)
	}
	if s.altGrid != nil {
		newAlt := make(termLine, rows*cols)
		for y := 0; y < copyRows; y++ {
			src := s.lineAt(s.altGrid, y)
			line := resizeLine(src, oldCols, cols)
			copy(newAlt[y*cols:(y+1)*cols], line)
		}
		s.altGrid = newAlt
	}
	s.grid = newGrid
	s.cols, s.rows = cols, rows
	s.curX = s.clampCol(s.curX)
	s.curY = s.clampRow(s.curY)
	s.top, s.bot = 0, rows-1
	s.resetTabs()
	s.wrapDone = false
	s.dirty = true
}

// resizeLine 把一行按新宽度裁剪 / 补空白。
func resizeLine(src []termCell, oldCols, cols int) termLine {
	out := make(termLine, cols)
	n := oldCols
	if cols < n {
		n = cols
	}
	if n > len(src) {
		n = len(src)
	}
	if n > 0 {
		copy(out, src[:n])
	}
	return out
}

// takeReply 取走待回写的应答（DA/DSR），调用方负责写给 PTY。
func (s *termScreen) takeReply() []byte {
	if len(s.reply) == 0 {
		return nil
	}
	out := s.reply
	s.reply = nil
	return out
}

// takeTitle 取走标题（为空表示没有变化）。
func (s *termScreen) takeTitle() string {
	t := s.title
	s.title = ""
	return t
}

// takeBell 取走响铃标记。
func (s *termScreen) takeBell() bool {
	b := s.bell
	s.bell = false
	return b
}

// ---- 文本提取（选区 / 复制 / 测试）----

// cellAt 读屏幕单元格（越界返回空）。
func (s *termScreen) cellAt(x, y int) termCell {
	if y < 0 || y >= s.rows || x < 0 || x >= s.cols {
		return termCell{}
	}
	return s.row(y)[x]
}

// lineText 一行转文本（去掉宽字符尾格与行尾空白）。
func (s *termScreen) lineText(y int) string {
	ln := s.row(y)
	if ln == nil {
		return ""
	}
	var sb strings.Builder
	for x := 0; x < s.cols; x++ {
		c := ln[x]
		if c.attr&termAttrWideTail != 0 && c.ch == 0 {
			continue
		}
		if c.ch == 0 {
			sb.WriteByte(' ')
			continue
		}
		sb.WriteRune(c.ch)
	}
	return strings.TrimRight(sb.String(), " ")
}

// rangeText 提取 [x0,y0]~[x1,y1] 的文本（按行拼接，行尾去空白）。
func (s *termScreen) rangeText(x0, y0, x1, y1 int) string {
	if y0 > y1 || (y0 == y1 && x0 > x1) {
		x0, y0, x1, y1 = x1, y1, x0, y0
	}
	var sb strings.Builder
	for y := y0; y <= y1; y++ {
		ln := s.row(y)
		if ln == nil {
			continue
		}
		start, end := 0, s.cols
		if y == y0 {
			start = x0
		}
		if y == y1 {
			end = x1 + 1
		}
		var line strings.Builder
		for x := start; x < end && x < s.cols; x++ {
			c := ln[x]
			if c.attr&termAttrWideTail != 0 && c.ch == 0 {
				continue
			}
			if c.ch == 0 {
				line.WriteByte(' ')
				continue
			}
			line.WriteRune(c.ch)
		}
		sb.WriteString(strings.TrimRight(line.String(), " "))
		if y < y1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// ---- 宽字符宽度 ----

// termRuneWidth 字符占用的列数（0 / 1 / 2）。
func termRuneWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return 0
	case r < 0x7f:
		return 1
	case r >= 0x0300 && r <= 0x036f: // 组合附加符号
		return 0
	case r == 0x200b || r == 0x200c || r == 0x200d || r == 0xfeff:
		return 0
	case r >= 0xfe00 && r <= 0xfe0f: // 变体选择符
		return 0
	}
	if (r >= 0x1100 && r <= 0x115f) ||
		r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1f64f) ||
		(r >= 0x1f900 && r <= 0x1f9ff) ||
		(r >= 0x20000 && r <= 0x3fffd) {
		return 2
	}
	return 1
}
