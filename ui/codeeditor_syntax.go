// guis — 原生代码编辑器（二）：语法着色（内置解析器）
//
// 只对「可见行」着色，并把结果按行缓存：滚动回去不重算，编辑只让受影响的行
// 及其后的行失效（跨行注释 / 多行字符串会改变后续行的起始状态）。
// 支持：go / javascript / typescript / json / python / c 系 / java / csharp /
// rust / css / html / xml / sql / shell / yaml / markdown / plain。
package ui

import (
	"path/filepath"
	"strings"

	"guis/internal/gpu"
)

// codeTokKind 词法单元类别（决定颜色）。
type codeTokKind uint8

const (
	codePlain codeTokKind = iota
	codeKeyword
	codeType
	codeString
	codeComment
	codeNumber
	codeFunction
	codeOperator
	codePunct
	codeConstant
	codeAttribute
	codeTag
	codeInvalid
)

// codeSpan 一行内的着色片段（列按 rune 计，[Pos, End)）。
type codeSpan struct {
	Pos, End int
	Kind     codeTokKind
}

// codeLineInfo 单行的高亮与度量缓存。
type codeLineInfo struct {
	stamp uint64
	text  string
	spans []codeSpan
	next  uint8   // 行尾的跟行状态
	colX  []float32 // 行内累计列宽（rune 边界，懒计算）
	width float32 // 行宽（逻辑像素，绘制时填入）
	state uint8   // 行首的跨行状态

	// 软换行缓存：wrapW 是生成 wrap 时的文本区宽度，行内容变化时按 text 判定失效。
	wrapW float32
	wrap  []int32 // 段起始列（首段 0 不记录；nil 表示不换行）

	// 语义高亮合并缓存（semanticStamp 变化或行内容变化时失效）。
	merged      []codeSpan
	mergedStamp uint64
	mergedText  string

	// lexed 本行是否已经过词法分析。
	// 列宽 / 软换行会先建一个只有 text 的占位项，如果不区分，词法器
	// 会把占位项当成命中缓存返回空 spans，整篇就会失去语法着色。
	lexed bool
}

// codeCache 行级缓存（按行号索引；行号变化时整段失效）。
type codeCache struct {
	lines  map[int]*codeLineInfo
	stamp  uint64
	maxLen int
}

func newCodeCache() *codeCache {
	return &codeCache{lines: map[int]*codeLineInfo{}}
}

func (c *codeCache) reset() {
	c.lines = map[int]*codeLineInfo{}
	c.stamp++
}

// invalidateFrom 丢弃 startLine 及其之后的缓存（跨行状态可能整体改变）。
func (c *codeCache) invalidateFrom(startLine int) {
	if c == nil {
		return
	}
	if startLine < 0 {
		startLine = 0
	}
	for k := range c.lines {
		if k >= startLine {
			delete(c.lines, k)
		}
	}
	c.stamp++
}

// get 取一行缓存（未命中时用词法器计算），返回该行与行尾的跟行状态。
func (c *codeCache) get(e *CodeEditorState, line int, prevState uint8) (*codeLineInfo, uint8) {
	if info, ok := c.lines[line]; ok && info.lexed && info.text == e.Lines[line] && info.state == prevState {
		return info, info.next
	}
	spans, next := tokenizeCodeLine(e.Language, e.Lines[line], prevState)
	info := &codeLineInfo{stamp: c.stamp, text: e.Lines[line], spans: spans, state: prevState, next: next, width: -1, lexed: true}
	c.lines[line] = info
	return info, next
}

// ---- 语言关键字表 ----

var codeKeywords = map[string][]string{
	"go": {"break", "case", "chan", "const", "continue", "default", "defer", "else",
		"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
		"map", "package", "range", "return", "select", "struct", "switch", "type", "var"},
	"javascript": {"async", "await", "break", "case", "catch", "class", "const", "continue",
		"debugger", "default", "delete", "do", "else", "export", "extends", "finally",
		"for", "from", "function", "get", "if", "import", "in", "instanceof", "let",
		"new", "of", "return", "set", "static", "super", "switch", "this", "throw",
		"try", "typeof", "var", "void", "while", "yield"},
	"typescript": {"abstract", "as", "async", "await", "break", "case", "catch", "class",
		"const", "continue", "declare", "default", "delete", "do", "else", "enum",
		"export", "extends", "finally", "for", "from", "function", "get", "if",
		"implements", "import", "in", "instanceof", "interface", "keyof", "let",
		"namespace", "new", "of", "private", "protected", "public", "readonly",
		"return", "set", "static", "super", "switch", "this", "throw", "try",
		"type", "typeof", "var", "void", "while", "yield"},
	"java": {"abstract", "assert", "boolean", "break", "byte", "case", "catch", "char",
		"class", "const", "continue", "default", "do", "double", "else", "enum",
		"extends", "final", "finally", "float", "for", "if", "implements", "import",
		"instanceof", "int", "interface", "long", "native", "new", "package",
		"private", "protected", "public", "return", "short", "static", "strictfp",
		"super", "switch", "synchronized", "this", "throw", "throws", "transient",
		"try", "void", "volatile", "while"},
	"c": {"auto", "break", "case", "char", "const", "continue", "default", "do",
		"double", "else", "enum", "extern", "float", "for", "goto", "if", "inline",
		"int", "long", "register", "restrict", "return", "short", "signed",
		"sizeof", "static", "struct", "switch", "typedef", "union", "unsigned",
		"void", "volatile", "while"},
	"csharp": {"abstract", "as", "async", "await", "base", "bool", "break", "byte",
		"case", "catch", "char", "class", "const", "continue", "decimal", "default",
		"delegate", "do", "double", "else", "enum", "event", "explicit", "extern",
		"finally", "fixed", "float", "for", "foreach", "get", "goto", "if",
		"implicit", "in", "int", "interface", "internal", "is", "lock", "long",
		"namespace", "new", "object", "operator", "out", "override", "params",
		"private", "protected", "public", "readonly", "ref", "return", "set",
		"short", "sizeof", "stackalloc", "static", "string", "struct", "switch",
		"this", "throw", "try", "typeof", "uint", "ulong", "unsafe", "ushort",
		"using", "var", "virtual", "void", "volatile", "while", "yield"},
	"rust": {"as", "async", "await", "break", "const", "continue", "crate", "dyn",
		"else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let",
		"loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self",
		"Self", "static", "struct", "super", "trait", "true", "type", "unsafe",
		"use", "where", "while"},
	"python": {"and", "as", "assert", "async", "await", "break", "class", "continue",
		"def", "del", "elif", "else", "except", "finally", "for", "from", "global",
		"if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass",
		"raise", "return", "try", "while", "with", "yield"},
	"sql": {"add", "all", "alter", "and", "as", "asc", "begin", "between", "by",
		"case", "column", "commit", "create", "delete", "desc", "distinct", "drop",
		"else", "end", "exists", "from", "group", "having", "if", "in", "index",
		"inner", "insert", "into", "is", "join", "key", "left", "like", "limit",
		"not", "null", "on", "or", "order", "outer", "primary", "references",
		"right", "rollback", "select", "set", "table", "then", "union", "unique",
		"update", "values", "view", "when", "where"},
	"shell": {"if", "then", "else", "elif", "fi", "for", "while", "do", "done",
		"case", "esac", "function", "in", "return", "export", "local", "readonly",
		"echo", "cd", "set", "unset", "shift", "source"},
	"yaml": {"true", "false", "null", "yes", "no", "on", "off"},
}

var codeTypes = map[string][]string{
	"go":         {"bool", "byte", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "any"},
	"typescript": {"any", "boolean", "never", "number", "object", "string", "symbol", "unknown", "void"},
	"java":       {"boolean", "byte", "char", "double", "float", "int", "long", "short", "void"},
	"c":          {"bool", "char", "double", "float", "int", "long", "short", "size_t", "ssize_t", "uint8_t", "uint16_t", "uint32_t", "uint64_t", "int8_t", "int16_t", "int32_t", "int64_t", "void"},
	"csharp":     {"bool", "byte", "char", "decimal", "double", "float", "int", "long", "object", "sbyte", "short", "string", "uint", "ulong", "ushort", "void"},
	"rust":       {"bool", "char", "f32", "f64", "i8", "i16", "i32", "i64", "i128", "isize", "str", "u8", "u16", "u32", "u64", "u128", "usize"},
	"python":     {"bool", "bytes", "dict", "float", "int", "list", "object", "set", "str", "tuple"},
}

// codeLanguageByPath 按扩展名推断语言。
func codeLanguageByPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".json", ".jsonc":
		return "json"
	case ".py", ".pyw":
		return "python"
	case ".c", ".h", ".cc", ".cpp", ".hpp", ".cxx":
		return "c"
	case ".java", ".kt", ".kts":
		return "java"
	case ".cs":
		return "csharp"
	case ".rs":
		return "rust"
	case ".css", ".scss", ".less":
		return "css"
	case ".html", ".htm", ".vue", ".svelte":
		return "html"
	case ".xml", ".svg", ".xaml":
		return "xml"
	case ".sql":
		return "sql"
	case ".sh", ".bash", ".zsh", ".ps1":
		return "shell"
	case ".yml", ".yaml", ".toml", ".ini":
		return "yaml"
	case ".md", ".markdown":
		return "markdown"
	}
	return "plain"
}

// codeLineCommentToken 行注释标记（ToggleComment 用）。
func codeLineCommentToken(lang string) string {
	switch lang {
	case "go", "javascript", "typescript", "java", "c", "csharp", "rust", "css":
		return "//"
	case "python", "shell", "yaml":
		return "#"
	case "sql":
		return "--"
	}
	return ""
}

// codeLangFamily 词法族：c / python / json / html / css / sql / shell / md / plain。
func codeLangFamily(lang string) string {
	switch lang {
	case "go", "javascript", "typescript", "java", "c", "csharp", "rust":
		return "c"
	case "python":
		return "python"
	case "json":
		return "json"
	case "html", "xml":
		return lang
	case "css":
		return "css"
	case "sql":
		return "sql"
	case "shell":
		return "shell"
	case "yaml":
		return "yaml"
	case "markdown":
		return "markdown"
	}
	return "plain"
}

// 跨行状态（行首）：0 普通 / 1 块注释 / 2 Python 三引号 / 3 Go 原始字符串 / 4 HTML 注释
const (
	codeStateNormal uint8 = iota
	codeStateBlockComment
	codeStatePyTriple
	codeStateRawString
	codeStateHTMLComment
)

// tokenizeCodeLine 对一行着色，返回片段与行尾的跨行状态。
func tokenizeCodeLine(lang, line string, state uint8) ([]codeSpan, uint8) {
	if line == "" {
		return nil, state
	}
	switch codeLangFamily(lang) {
	case "c":
		kw := codeKeywords[lang]
		if kw == nil {
			kw = codeKeywords["c"]
		}
		return tokenizeCLike(line, state, kw, codeTypes[lang], lang)
	case "python":
		return tokenizePython(line, state)
	case "json":
		return tokenizeJSONLike(line, state, false)
	case "html", "xml":
		return tokenizeHTML(line, state)
	case "css":
		return tokenizeCSS(line, state)
	case "sql":
		return tokenizeCSimple(line, state, codeKeywords["sql"], nil, "--", "", true)
	case "shell":
		return tokenizeCSimple(line, state, codeKeywords["shell"], nil, "#", "", true)
	case "yaml":
		return tokenizeYAML(line, state)
	case "markdown":
		return tokenizeMarkdown(line, state)
	}
	return nil, codeStateNormal
}

// isKeyword 关键字命中。
func isKeyword(list []string, s string) bool {
	for _, k := range list {
		if k == s {
			return true
		}
	}
	return false
}

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 0x2E7F
}

func isIdentRune(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9')
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// tokenizeCLike C 系词法器（go/js/ts/java/c/csharp/rust 共用）。
func tokenizeCLike(line string, state uint8, kws, types []string, lang string) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0

	if state == codeStateBlockComment {
		if idx := strings.Index(line, "*/"); idx >= 0 {
			end := len([]rune(line[:idx])) + 2
			spans = append(spans, codeSpan{0, end, codeComment})
			i = end
			state = codeStateNormal
		} else {
			return []codeSpan{{0, n, codeComment}}, state
		}
	}
	if state == codeStateRawString {
		// Go 反引号原始字符串
		if idx := strings.Index(line, "`"); idx >= 0 {
			end := len([]rune(line[:idx])) + 1
			spans = append(spans, codeSpan{0, end, codeString})
			i = end
			state = codeStateNormal
		} else {
			return []codeSpan{{0, n, codeString}}, state
		}
	}

	for i < n {
		r := rs[i]
		// 空白
		if r == ' ' || r == '\t' {
			i++
			continue
		}
		// 行注释
		if r == '/' && i+1 < n && rs[i+1] == '/' {
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateNormal
		}
		// 块注释
		if r == '/' && i+1 < n && rs[i+1] == '*' {
			rest := string(rs[i:])
			if idx := strings.Index(rest, "*/"); idx >= 0 {
				end := i + len([]rune(rest[:idx])) + 2
				spans = append(spans, codeSpan{i, end, codeComment})
				i = end
				continue
			}
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateBlockComment
		}
		// 字符串
		if r == '"' || r == '\'' || (r == '`' && lang == "go") {
			start := i
			quote := r
			i++
			closed := false
			for i < n {
				if rs[i] == '\\' && quote != '`' {
					i += 2
					continue
				}
				if rs[i] == quote {
					i++
					closed = true
					break
				}
				i++
			}
			if i > n {
				i = n
			}
			spans = append(spans, codeSpan{start, i, codeString})
			if !closed && quote == '`' {
				return spans, codeStateRawString
			}
			continue
		}
		// 数字
		if isDigit(r) {
			start := i
			for i < n && (isDigit(rs[i]) || strings.ContainsRune("._xXbBoOaAbBcCdDeEfF+-i", rs[i])) {
				// 避免把 1+1 的 + 吃进数字
				if (rs[i] == '+' || rs[i] == '-') && i > start {
					prev := rs[i-1]
					if prev != 'e' && prev != 'E' && prev != 'p' && prev != 'P' {
						break
					}
				}
				i++
			}
			spans = append(spans, codeSpan{start, i, codeNumber})
			continue
		}
		// 标识符 / 关键字
		if isIdentStart(r) {
			start := i
			for i < n && isIdentRune(rs[i]) {
				i++
			}
			word := string(rs[start:i])
			kind := codePlain
			switch {
			case isKeyword(kws, word):
				kind = codeKeyword
			case isKeyword(types, word):
				kind = codeType
			case word == "true" || word == "false" || word == "nil" || word == "null" ||
				word == "None" || word == "True" || word == "False" || word == "undefined":
				kind = codeConstant
			case i < n && strings.ContainsRune("(", rs[i]):
				kind = codeFunction
			}
			if kind != codePlain {
				spans = append(spans, codeSpan{start, i, kind})
			}
			continue
		}
		// 运算符
		if strings.ContainsRune("+-*/%=<>!&|^~?:", r) {
			start := i
			for i < n && strings.ContainsRune("+-*/%=<>!&|^~?:", rs[i]) {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeOperator})
			continue
		}
		// 标点
		if strings.ContainsRune("()[]{},;.", r) {
			spans = append(spans, codeSpan{i, i + 1, codePunct})
			i++
			continue
		}
		i++
	}
	return spans, state
}

// tokenizeCSimple 只有行注释 / 字符串的简化词法器（SQL / Shell）。
func tokenizeCSimple(line string, state uint8, kws, types []string, lineComment, blockComment string, caseInsensitive bool) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0
	for i < n {
		r := rs[i]
		if r == ' ' || r == '\t' {
			i++
			continue
		}
		if lineComment != "" && r == rune(lineComment[0]) {
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateNormal
		}
		if r == '"' || r == '\'' {
			start := i
			quote := r
			i++
			for i < n {
				if rs[i] == '\\' {
					i += 2
					continue
				}
				if rs[i] == quote {
					i++
					break
				}
				i++
			}
			if i > n {
				i = n
			}
			spans = append(spans, codeSpan{start, i, codeString})
			continue
		}
		if isDigit(r) {
			start := i
			for i < n && (isDigit(rs[i]) || rs[i] == '.') {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeNumber})
			continue
		}
		if isIdentStart(r) {
			start := i
			for i < n && isIdentRune(rs[i]) {
				i++
			}
			word := string(rs[start:i])
			cmp := word
			if caseInsensitive {
				cmp = strings.ToLower(word)
			}
			if isKeyword(kws, cmp) {
				spans = append(spans, codeSpan{start, i, codeKeyword})
			}
			continue
		}
		if strings.ContainsRune("+-*/%=<>!&|^~", r) {
			spans = append(spans, codeSpan{i, i + 1, codeOperator})
		}
		i++
	}
	return spans, state
}

// tokenizePython Python 词法器（# 注释 / 三引号 / 关键字）。
func tokenizePython(line string, state uint8) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0
	if state == codeStatePyTriple {
		if idx := strings.Index(line, `"""`); idx >= 0 {
			end := len([]rune(line[:idx])) + 3
			spans = append(spans, codeSpan{0, end, codeString})
			i = end
			state = codeStateNormal
		} else {
			return []codeSpan{{0, n, codeString}}, state
		}
	}
	for i < n {
		r := rs[i]
		if r == ' ' || r == '\t' {
			i++
			continue
		}
		if r == '#' {
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateNormal
		}
		if r == '"' || r == '\'' {
			if i+2 < n && rs[i+1] == r && rs[i+2] == r {
				rest := string(rs[i+3:])
				if idx := strings.Index(rest, strings.Repeat(string(r), 3)); idx >= 0 {
					end := i + 3 + len([]rune(rest[:idx])) + 3
					spans = append(spans, codeSpan{i, end, codeString})
					i = end
					continue
				}
				spans = append(spans, codeSpan{i, n, codeString})
				return spans, codeStatePyTriple
			}
			start := i
			i++
			for i < n {
				if rs[i] == '\\' {
					i += 2
					continue
				}
				if rs[i] == r {
					i++
					break
				}
				i++
			}
			if i > n {
				i = n
			}
			spans = append(spans, codeSpan{start, i, codeString})
			continue
		}
		if isDigit(r) {
			start := i
			for i < n && (isDigit(rs[i]) || rs[i] == '.' || rs[i] == 'x' || rs[i] == 'e' || rs[i] == '_') {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeNumber})
			continue
		}
		if isIdentStart(r) {
			start := i
			for i < n && isIdentRune(rs[i]) {
				i++
			}
			word := string(rs[start:i])
			if isKeyword(codeKeywords["python"], word) {
				spans = append(spans, codeSpan{start, i, codeKeyword})
			} else if word == "self" || word == "cls" {
				spans = append(spans, codeSpan{start, i, codeConstant})
			} else if isKeyword(codeTypes["python"], word) {
				spans = append(spans, codeSpan{start, i, codeType})
			} else if i < n && rs[i] == '(' {
				spans = append(spans, codeSpan{start, i, codeFunction})
			}
			continue
		}
		if strings.ContainsRune("+-*/%=<>!&|^~", r) {
			spans = append(spans, codeSpan{i, i + 1, codeOperator})
		}
		i++
	}
	return spans, state
}

// tokenizeJSONLike JSON 词法（键名高亮为属性色）。
func tokenizeJSONLike(line string, state uint8, yamlMode bool) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0
	for i < n {
		r := rs[i]
		if r == ' ' || r == '\t' {
			i++
			continue
		}
		if yamlMode && r == '#' {
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateNormal
		}
		if r == '"' || (yamlMode && (r == '\'' || r == '`')) {
			start := i
			quote := r
			i++
			for i < n {
				if rs[i] == '\\' {
					i += 2
					continue
				}
				if rs[i] == quote {
					i++
					break
				}
				i++
			}
			if i > n {
				i = n
			}
			kind := codeString
			// 后面紧跟冒号 → 键名
			j := i
			for j < n && (rs[j] == ' ' || rs[j] == '\t') {
				j++
			}
			if j < n && (rs[j] == ':' || (yamlMode && rs[j] == ':')) {
				kind = codeAttribute
			}
			spans = append(spans, codeSpan{start, i, kind})
			continue
		}
		if isDigit(r) || (r == '-' && i+1 < n && isDigit(rs[i+1])) {
			start := i
			i++
			for i < n && (isDigit(rs[i]) || rs[i] == '.' || rs[i] == 'e' || rs[i] == 'E' || rs[i] == '+' || rs[i] == '-') {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeNumber})
			continue
		}
		if isIdentStart(r) {
			start := i
			for i < n && isIdentRune(rs[i]) {
				i++
			}
			word := rs[start:i]
			kind := codePlain
			switch string(word) {
			case "true", "false", "null", "True", "False", "Null":
				kind = codeConstant
			default:
				if yamlMode {
					j := i
					for j < n && (rs[j] == ' ' || rs[j] == '\t') {
						j++
					}
					if j < n && rs[j] == ':' {
						kind = codeAttribute
					}
				}
			}
			if kind != codePlain {
				spans = append(spans, codeSpan{start, i, kind})
			}
			continue
		}
		i++
	}
	return spans, state
}

func tokenizeYAML(line string, state uint8) ([]codeSpan, uint8) {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return nil, state
	}
	if strings.HasPrefix(trim, "#") {
		off := len([]rune(line)) - len([]rune(strings.TrimLeft(line, " \t")))
		return []codeSpan{{off, len([]rune(line)), codeComment}}, codeStateNormal
	}
	spans, st := tokenizeJSONLike(line, state, true)
	// 顶层 `key:` 的键名
	if strings.HasPrefix(trim, "- ") {
		off := len([]rune(line)) - len([]rune(strings.TrimLeft(line, " \t")))
		spans = append(spans, codeSpan{off, off + 1, codeOperator})
	}
	return spans, st
}

// tokenizeHTML 标签 / 属性 / 文本 / 注释。
func tokenizeHTML(line string, state uint8) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0
	if state == codeStateHTMLComment {
		if idx := strings.Index(string(rs), "-->"); idx >= 0 {
			end := len([]rune(string(rs[:idx]))) + 3
			spans = append(spans, codeSpan{0, end, codeComment})
			i = end
			state = codeStateNormal
		} else {
			return []codeSpan{{0, n, codeComment}}, state
		}
	}
	for i < n {
		if rs[i] == '<' {
			rest := string(rs[i:])
			if strings.HasPrefix(rest, "<!--") {
				if idx := strings.Index(rest, "-->"); idx >= 0 {
					end := i + len([]rune(rest[:idx])) + 3
					spans = append(spans, codeSpan{i, end, codeComment})
					i = end
					continue
				}
				spans = append(spans, codeSpan{i, n, codeComment})
				return spans, codeStateHTMLComment
			}
			end := i
			for end < n && rs[end] != '>' {
				end++
			}
			if end < n {
				end++
			}
			// 标签名
			j := i + 1
			if j < n && rs[j] == '/' {
				j++
			}
			ns := j
			for j < n && (isIdentRune(rs[j]) || rs[j] == '-' || rs[j] == ':') {
				j++
			}
			if j > ns {
				spans = append(spans, codeSpan{ns, j, codeTag})
			}
			// 属性名
			for j < end-1 {
				if rs[j] == ' ' || rs[j] == '\t' {
					j++
					continue
				}
				if rs[j] == '=' || rs[j] == '"' || rs[j] == '\'' || rs[j] == '/' {
					j++
					continue
				}
				k := j
				for k < end-1 && (isIdentRune(rs[k]) || rs[k] == '-' || rs[k] == ':') {
					k++
				}
				if k > j {
					spans = append(spans, codeSpan{j, k, codeAttribute})
					j = k
					continue
				}
				j++
			}
			i = end
			continue
		}
		i++
	}
	return spans, state
}

// tokenizeCSS 选择器 / 属性 / 值 / 注释。
func tokenizeCSS(line string, state uint8) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	var spans []codeSpan
	i := 0
	if state == codeStateBlockComment {
		if idx := strings.Index(line, "*/"); idx >= 0 {
			end := len([]rune(line[:idx])) + 2
			spans = append(spans, codeSpan{0, end, codeComment})
			i = end
			state = codeStateNormal
		} else {
			return []codeSpan{{0, n, codeComment}}, state
		}
	}
	for i < n {
		r := rs[i]
		if r == '/' && i+1 < n && rs[i+1] == '*' {
			rest := string(rs[i:])
			if idx := strings.Index(rest, "*/"); idx >= 0 {
				end := i + len([]rune(rest[:idx])) + 2
				spans = append(spans, codeSpan{i, end, codeComment})
				i = end
				continue
			}
			spans = append(spans, codeSpan{i, n, codeComment})
			return spans, codeStateBlockComment
		}
		if r == '"' || r == '\'' {
			start := i
			i++
			for i < n && rs[i] != r {
				if rs[i] == '\\' {
					i++
				}
				i++
			}
			if i < n {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeString})
			continue
		}
		if isDigit(r) {
			start := i
			for i < n && (isDigit(rs[i]) || rs[i] == '.' || rs[i] == '%' || isIdentStart(rs[i])) {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeNumber})
			continue
		}
		if r == '#' || r == '.' || r == ':' {
			start := i
			i++
			for i < n && (isIdentRune(rs[i]) || rs[i] == '-') {
				i++
			}
			spans = append(spans, codeSpan{start, i, codeTag})
			continue
		}
		if isIdentStart(r) {
			start := i
			for i < n && (isIdentRune(rs[i]) || rs[i] == '-') {
				i++
			}
			if i < n && rs[i] == ':' {
				spans = append(spans, codeSpan{start, i, codeAttribute})
			} else {
				spans = append(spans, codeSpan{start, i, codeKeyword})
			}
			continue
		}
		i++
	}
	return spans, state
}

// tokenizeMarkdown 标题 / 强调 / 行内代码 / 链接 / 引用。
func tokenizeMarkdown(line string, state uint8) ([]codeSpan, uint8) {
	rs := []rune(line)
	n := len(rs)
	if n == 0 {
		return nil, codeStateNormal
	}
	var spans []codeSpan
	trim := strings.TrimLeft(line, " ")
	lead := n - len([]rune(trim))
	switch {
	case strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~"):
		spans = append(spans, codeSpan{lead, n, codeKeyword})
		return spans, codeStateNormal
	case strings.HasPrefix(trim, "#"):
		k := 0
		for k < len(trim) && trim[k] == '#' {
			k++
		}
		spans = append(spans, codeSpan{lead, lead + k, codeKeyword})
		if n > lead+k {
			spans = append(spans, codeSpan{lead + k, n, codeFunction})
		}
		return spans, codeStateNormal
	case strings.HasPrefix(trim, ">"):
		spans = append(spans, codeSpan{lead, n, codeComment})
		return spans, codeStateNormal
	case strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") || strings.HasPrefix(trim, "+ "):
		spans = append(spans, codeSpan{lead, lead + 1, codeOperator})
	}
	// 行内代码 / 强调 / 链接（简单扫描）
	i := lead
	for i < n {
		if rs[i] == '`' {
			j := i + 1
			for j < n && rs[j] != '`' {
				j++
			}
			if j < n {
				j++
			}
			spans = append(spans, codeSpan{i, j, codeString})
			i = j
			continue
		}
		if rs[i] == '*' && i+1 < n && rs[i+1] == '*' {
			j := i + 2
			for j+1 < n && !(rs[j] == '*' && rs[j+1] == '*') {
				j++
			}
			if j+1 < n {
				j += 2
			} else {
				j = n
			}
			spans = append(spans, codeSpan{i, j, codeFunction})
			i = j
			continue
		}
		if rs[i] == '[' {
			j := i + 1
			for j < n && rs[j] != ']' {
				j++
			}
			if j < n {
				j++
			}
			spans = append(spans, codeSpan{i, j, codeAttribute})
			i = j
			continue
		}
		i++
	}
	return spans, codeStateNormal
}

// codePaintFor 词法类别 → 颜色。
func codePaintFor(t CodeTheme, k codeTokKind) gpu.Paint {
	switch k {
	case codeKeyword:
		return t.Keyword
	case codeType:
		return t.Type
	case codeString:
		return t.String
	case codeComment:
		return t.Comment
	case codeNumber:
		return t.Number
	case codeFunction:
		return t.Function
	case codeOperator:
		return t.Operator
	case codePunct:
		return t.Punct
	case codeConstant:
		return t.Constant
	case codeAttribute:
		return t.Attribute
	case codeTag:
		return t.Tag
	case codeInvalid:
		return t.Invalid
	}
	return t.Text
}
