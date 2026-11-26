// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package game

import "fmt"

// 脚本语法树与递归下降解析器（表达式用 Pratt 优先级爬升）。

// ---------------------------------------------------------------- 表达式

type scriptExpr interface{ scriptExprNode() }

type scriptNumLit struct{ v float64 }
type scriptStrLit struct{ v string }
type scriptBoolLit struct{ v bool }
type scriptNilLit struct{}
type scriptIdent struct {
	name string
	line int
}
type scriptArrayLit struct{ items []scriptExpr }
type scriptObjectLit struct {
	keys []string
	vals []scriptExpr
}
type scriptUnary struct {
	op string
	x  scriptExpr
}
type scriptBinary struct {
	op   string
	l, r scriptExpr
}
type scriptCall struct {
	callee scriptExpr
	args   []scriptExpr
	line   int
}
type scriptIndex struct {
	obj, idx scriptExpr
	line     int
}
type scriptMember struct {
	obj  scriptExpr
	name string
	line int
}

func (*scriptNumLit) scriptExprNode()    {}
func (*scriptStrLit) scriptExprNode()    {}
func (*scriptBoolLit) scriptExprNode()   {}
func (*scriptNilLit) scriptExprNode()    {}
func (*scriptIdent) scriptExprNode()     {}
func (*scriptArrayLit) scriptExprNode()  {}
func (*scriptObjectLit) scriptExprNode() {}
func (*scriptUnary) scriptExprNode()     {}
func (*scriptBinary) scriptExprNode()    {}
func (*scriptCall) scriptExprNode()      {}
func (*scriptIndex) scriptExprNode()     {}
func (*scriptMember) scriptExprNode()    {}

// ---------------------------------------------------------------- 语句

type scriptStmt interface{ scriptStmtNode() }

type scriptLet struct {
	name  string
	value scriptExpr
	line  int
}
type scriptAssign struct {
	target scriptExpr
	op     string
	value  scriptExpr
	line   int
}
type scriptExprStmt struct{ x scriptExpr }
type scriptIf struct {
	cond scriptExpr
	then []scriptStmt
	els  []scriptStmt
}
type scriptWhile struct {
	cond scriptExpr
	body []scriptStmt
}
type scriptFor struct {
	init scriptStmt
	cond scriptExpr
	post scriptStmt
	body []scriptStmt
}
type scriptForIn struct {
	name string
	iter scriptExpr
	body []scriptStmt
}
type scriptFnDecl struct {
	name   string
	params []string
	body   []scriptStmt
	line   int
}
type scriptReturn struct{ value scriptExpr }
type scriptBreak struct{}
type scriptContinue struct{}
type scriptBlock struct{ body []scriptStmt }

func (*scriptLet) scriptStmtNode()      {}
func (*scriptAssign) scriptStmtNode()   {}
func (*scriptExprStmt) scriptStmtNode() {}
func (*scriptIf) scriptStmtNode()       {}
func (*scriptWhile) scriptStmtNode()    {}
func (*scriptFor) scriptStmtNode()      {}
func (*scriptForIn) scriptStmtNode()    {}
func (*scriptFnDecl) scriptStmtNode()   {}
func (*scriptReturn) scriptStmtNode()   {}
func (*scriptBreak) scriptStmtNode()    {}
func (*scriptContinue) scriptStmtNode() {}
func (*scriptBlock) scriptStmtNode()    {}

// ---------------------------------------------------------------- 解析器

type scriptParser struct {
	toks []scriptToken
	pos  int
}

// scriptParse 解析源码为语句列表。
func scriptParse(src string) ([]scriptStmt, error) {
	toks, err := scriptLex(src)
	if err != nil {
		return nil, err
	}
	p := &scriptParser{toks: toks}
	stmts, err := p.parseBlockStmts("")
	if err != nil {
		return nil, err
	}
	if p.peek().kind != stEOF {
		return nil, p.errf("多余的 %s", p.peek())
	}
	return stmts, nil
}

func (p *scriptParser) peek() scriptToken { return p.toks[p.pos] }

func (p *scriptParser) peekAt(n int) scriptToken {
	i := p.pos + n
	if i >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[i]
}

func (p *scriptParser) next() scriptToken {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *scriptParser) errf(format string, args ...any) error {
	t := p.peek()
	return fmt.Errorf("脚本第 %d 行：%s", t.line, fmt.Sprintf(format, args...))
}

func (p *scriptParser) isPunct(s string) bool {
	t := p.peek()
	return t.kind == stPunct && t.text == s
}

func (p *scriptParser) isOp(s string) bool {
	t := p.peek()
	return t.kind == stOp && t.text == s
}

func (p *scriptParser) isKeyword(s string) bool {
	t := p.peek()
	return t.kind == stKeyword && t.text == s
}

func (p *scriptParser) acceptPunct(s string) bool {
	if p.isPunct(s) {
		p.next()
		return true
	}
	return false
}

func (p *scriptParser) expectPunct(s string) error {
	if !p.acceptPunct(s) {
		return p.errf("期望 %q，实际 %s", s, p.peek())
	}
	return nil
}

func (p *scriptParser) skipSemi() {
	for p.isPunct(";") {
		p.next()
	}
}

// parseBlockStmts 解析语句序列，end 为 "" 表示到文件末尾，否则到该标点。
func (p *scriptParser) parseBlockStmts(end string) ([]scriptStmt, error) {
	var out []scriptStmt
	for {
		p.skipSemi()
		t := p.peek()
		if t.kind == stEOF {
			if end != "" {
				return nil, p.errf("缺少 %q", end)
			}
			return out, nil
		}
		if end != "" && t.kind == stPunct && t.text == end {
			return out, nil
		}
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
}

// parseBracedBlock 解析 { ... } 块。
func (p *scriptParser) parseBracedBlock() ([]scriptStmt, error) {
	if err := p.expectPunct("{"); err != nil {
		return nil, err
	}
	body, err := p.parseBlockStmts("}")
	if err != nil {
		return nil, err
	}
	if err := p.expectPunct("}"); err != nil {
		return nil, err
	}
	return body, nil
}

func (p *scriptParser) parseStmt() (scriptStmt, error) {
	t := p.peek()
	if t.kind == stKeyword {
		switch t.text {
		case "let":
			return p.parseLet()
		case "if":
			return p.parseIf()
		case "while":
			return p.parseWhile()
		case "for":
			return p.parseFor()
		case "fn":
			return p.parseFnDecl()
		case "return":
			p.next()
			if p.isPunct(";") || p.isPunct("}") || p.peek().kind == stEOF {
				return &scriptReturn{}, nil
			}
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &scriptReturn{value: e}, nil
		case "break":
			p.next()
			return &scriptBreak{}, nil
		case "continue":
			p.next()
			return &scriptContinue{}, nil
		}
	}
	if t.kind == stPunct && t.text == "{" {
		body, err := p.parseBracedBlock()
		if err != nil {
			return nil, err
		}
		return &scriptBlock{body: body}, nil
	}
	// 表达式语句或赋值
	line := t.line
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind == stOp {
		op := p.peek().text
		switch op {
		case "=", "+=", "-=", "*=", "/=":
			p.next()
			if !isAssignable(e) {
				return nil, fmt.Errorf("脚本第 %d 行：左侧不可赋值", line)
			}
			v, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &scriptAssign{target: e, op: op, value: v, line: line}, nil
		}
	}
	return &scriptExprStmt{x: e}, nil
}

func isAssignable(e scriptExpr) bool {
	switch e.(type) {
	case *scriptIdent, *scriptIndex, *scriptMember:
		return true
	}
	return false
}

func (p *scriptParser) parseLet() (scriptStmt, error) {
	t := p.next() // let
	id := p.peek()
	if id.kind != stIdent {
		return nil, p.errf("let 后需要变量名，实际 %s", id)
	}
	p.next()
	if !p.isOp("=") {
		return nil, p.errf("let %s 缺少初始化表达式", id.text)
	}
	p.next()
	v, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &scriptLet{name: id.text, value: v, line: t.line}, nil
}

// parseParenCond 解析条件（括号可省略）。
func (p *scriptParser) parseParenCond() (scriptExpr, error) {
	if p.acceptPunct("(") {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}
		return e, nil
	}
	return p.parseExpr()
}

func (p *scriptParser) parseIf() (scriptStmt, error) {
	p.next() // if
	cond, err := p.parseParenCond()
	if err != nil {
		return nil, err
	}
	then, err := p.parseBracedBlock()
	if err != nil {
		return nil, err
	}
	st := &scriptIf{cond: cond, then: then}
	p.skipSemi()
	if p.isKeyword("else") {
		p.next()
		p.skipSemi()
		if p.isKeyword("if") {
			inner, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			st.els = []scriptStmt{inner}
		} else {
			els, err := p.parseBracedBlock()
			if err != nil {
				return nil, err
			}
			st.els = els
		}
	}
	return st, nil
}

func (p *scriptParser) parseWhile() (scriptStmt, error) {
	p.next() // while
	cond, err := p.parseParenCond()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBracedBlock()
	if err != nil {
		return nil, err
	}
	return &scriptWhile{cond: cond, body: body}, nil
}

func (p *scriptParser) parseFor() (scriptStmt, error) {
	p.next() // for
	// for x in expr { }
	if p.peek().kind == stIdent && p.peekAt(1).kind == stKeyword && p.peekAt(1).text == "in" {
		name := p.next().text
		p.next() // in
		iter, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		body, err := p.parseBracedBlock()
		if err != nil {
			return nil, err
		}
		return &scriptForIn{name: name, iter: iter, body: body}, nil
	}
	if p.acceptPunct("(") {
		var init scriptStmt
		if !p.isPunct(";") {
			s, err := p.parseStmt()
			if err != nil {
				return nil, err
			}
			init = s
		}
		if err := p.expectPunct(";"); err != nil {
			return nil, err
		}
		var cond scriptExpr
		if !p.isPunct(";") {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			cond = e
		}
		if err := p.expectPunct(";"); err != nil {
			return nil, err
		}
		var post scriptStmt
		if !p.isPunct(")") {
			s, err := p.parseStmt()
			if err != nil {
				return nil, err
			}
			post = s
		}
		if err := p.expectPunct(")"); err != nil {
			return nil, err
		}
		body, err := p.parseBracedBlock()
		if err != nil {
			return nil, err
		}
		return &scriptFor{init: init, cond: cond, post: post, body: body}, nil
	}
	return nil, p.errf("for 语法错误")
}

func (p *scriptParser) parseFnDecl() (scriptStmt, error) {
	t := p.next() // fn
	id := p.peek()
	if id.kind != stIdent {
		return nil, p.errf("fn 后需要函数名")
	}
	p.next()
	if err := p.expectPunct("("); err != nil {
		return nil, err
	}
	var params []string
	for !p.isPunct(")") {
		pt := p.peek()
		if pt.kind != stIdent {
			return nil, p.errf("参数名非法: %s", pt)
		}
		p.next()
		params = append(params, pt.text)
		if !p.acceptPunct(",") {
			break
		}
	}
	if err := p.expectPunct(")"); err != nil {
		return nil, err
	}
	body, err := p.parseBracedBlock()
	if err != nil {
		return nil, err
	}
	return &scriptFnDecl{name: id.text, params: params, body: body, line: t.line}, nil
}

// ---------------------------------------------------------------- 表达式

// parseExpr 解析完整表达式（含赋值右侧）。
func (p *scriptParser) parseExpr() (scriptExpr, error) { return p.parseBinary(1) }

// scriptPrec 二元运算符优先级（0 = 非二元）。
func scriptPrec(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=":
		return 3
	case "<", "<=", ">", ">=":
		return 4
	case "+", "-":
		return 5
	case "*", "/", "%":
		return 6
	}
	return 0
}

func (p *scriptParser) parseBinary(minPrec int) (scriptExpr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != stOp {
			return left, nil
		}
		prec := scriptPrec(t.text)
		if prec == 0 || prec < minPrec {
			return left, nil
		}
		p.next()
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &scriptBinary{op: t.text, l: left, r: right}
	}
}

func (p *scriptParser) parseUnary() (scriptExpr, error) {
	if p.peek().kind == stOp {
		op := p.peek().text
		if op == "!" || op == "-" || op == "+" {
			p.next()
			x, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			if op == "+" {
				return x, nil
			}
			return &scriptUnary{op: op, x: x}, nil
		}
		if op == "++" || op == "--" {
			return nil, p.errf("不支持前缀 %s，请用 += 1 / -= 1", op)
		}
	}
	return p.parsePostfix()
}

func (p *scriptParser) parsePostfix() (scriptExpr, error) {
	e, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.isPunct("("):
			line := p.peek().line
			p.next()
			var args []scriptExpr
			for !p.isPunct(")") {
				a, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if !p.acceptPunct(",") {
					break
				}
			}
			if err := p.expectPunct(")"); err != nil {
				return nil, err
			}
			e = &scriptCall{callee: e, args: args, line: line}
		case p.isPunct("["):
			line := p.peek().line
			p.next()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expectPunct("]"); err != nil {
				return nil, err
			}
			e = &scriptIndex{obj: e, idx: idx, line: line}
		case p.isPunct("."):
			line := p.peek().line
			p.next()
			id := p.peek()
			if id.kind != stIdent && id.kind != stKeyword {
				return nil, p.errf("成员名非法: %s", id)
			}
			p.next()
			e = &scriptMember{obj: e, name: id.text, line: line}
		default:
			return e, nil
		}
	}
}

func (p *scriptParser) parsePrimary() (scriptExpr, error) {
	t := p.peek()
	switch t.kind {
	case stNum:
		p.next()
		return &scriptNumLit{v: t.num}, nil
	case stStr:
		p.next()
		return &scriptStrLit{v: t.text}, nil
	case stKeyword:
		switch t.text {
		case "true":
			p.next()
			return &scriptBoolLit{v: true}, nil
		case "false":
			p.next()
			return &scriptBoolLit{v: false}, nil
		case "nil":
			p.next()
			return &scriptNilLit{}, nil
		}
		return nil, p.errf("意外的关键字 %q", t.text)
	case stIdent:
		p.next()
		return &scriptIdent{name: t.text, line: t.line}, nil
	case stPunct:
		switch t.text {
		case "(":
			p.next()
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expectPunct(")"); err != nil {
				return nil, err
			}
			return e, nil
		case "[":
			p.next()
			var items []scriptExpr
			for !p.isPunct("]") {
				e, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				items = append(items, e)
				if !p.acceptPunct(",") {
					break
				}
			}
			if err := p.expectPunct("]"); err != nil {
				return nil, err
			}
			return &scriptArrayLit{items: items}, nil
		case "{":
			p.next()
			var keys []string
			var vals []scriptExpr
			for !p.isPunct("}") {
				kt := p.peek()
				var key string
				switch kt.kind {
				case stIdent, stKeyword:
					key = kt.text
					p.next()
				case stStr:
					key = kt.text
					p.next()
				default:
					return nil, p.errf("对象键非法: %s", kt)
				}
				if !p.acceptPunct(":") {
					return nil, p.errf("对象键 %q 后需要 :", key)
				}
				v, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				keys = append(keys, key)
				vals = append(vals, v)
				if !p.acceptPunct(",") {
					break
				}
			}
			if err := p.expectPunct("}"); err != nil {
				return nil, err
			}
			return &scriptObjectLit{keys: keys, vals: vals}, nil
		}
	}
	return nil, p.errf("无法解析的 %s", t)
}
