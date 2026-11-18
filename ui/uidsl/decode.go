// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package uidsl

// JSON 描述格式。
//
// 顶层既可以直接是节点，也可以包一层 root（便于以后加 meta 字段）：
//
//	{
//	  "type": "col",
//	  "gap": 12, "pad": 16, "bg": "#0f1626",
//	  "children": [
//	    {"type": "text", "text": "界面标题", "size": 22, "weight": 700},
//	    {"type": "button", "text": "点我", "key": "btn1", "onClick": "hello"}
//	  ]
//	}

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Format 描述文件格式。
type Format string

const (
	// FormatJSON 标准 JSON。
	FormatJSON Format = "json"
	// FormatDSL 缩进嵌套的紧凑 DSL。
	FormatDSL Format = "dsl"
)

// FormatOf 按扩展名判断格式：.json / .jsonc 走 JSON，其余按 DSL。
func FormatOf(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".jsonc":
		return FormatJSON
	}
	return FormatDSL
}

// Parse 按指定格式把文本解析成声明树。
func Parse(src []byte, f Format) (Node, error) {
	switch f {
	case FormatJSON:
		return parseJSON(src)
	default:
		return parseDSL(string(src))
	}
}

// ParseFile 读文件并按扩展名选择格式。
func ParseFile(path string) (Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Node{}, err
	}
	n, err := Parse(b, FormatOf(path))
	if err != nil {
		return Node{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return n, nil
}

func parseJSON(src []byte) (Node, error) {
	// 匿名嵌入 Node：既支持 {"root":{...}}，也支持顶层直接就是节点。
	var wrap struct {
		Node
		Root *Node `json:"root"`
	}
	if err := json.Unmarshal(src, &wrap); err != nil {
		return Node{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if wrap.Root != nil {
		return *wrap.Root, nil
	}
	if wrap.Type == "" && len(wrap.Children) == 0 && wrap.Text == "" {
		return Node{}, fmt.Errorf("JSON 里没有可渲染的节点（需要 type / children，或 root）")
	}
	return wrap.Node, nil
}
