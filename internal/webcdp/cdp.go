// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package webcdp

// 页面级辅助命令：把 DevTools 协议里内嵌浏览器常用的部分包一层。
// 坐标一律是视图区 CSS 像素（视图区由 SetViewport 设定，与节点尺寸一致）。

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Page 页面操作。
type Page struct {
	S *Session
}

// Page 取页面操作句柄。
func (s *Session) Page() *Page { return &Page{S: s} }

// HistoryEntry 一条导航历史。
type HistoryEntry struct {
	ID    int64  `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Navigate 打开地址。
func (p *Page) Navigate(url string) error {
	if p == nil || p.S == nil {
		return nil
	}
	_, err := p.S.Call("Page.navigate", map[string]any{"url": url})
	return err
}

// Reload 刷新（忽略缓存开关由调用方决定）。
func (p *Page) Reload() error {
	_, err := p.S.Call("Page.reload", map[string]any{"ignoreCache": false})
	return err
}

// Stop 停止加载。
func (p *Page) Stop() error {
	_, err := p.S.Call("Page.stopLoading", nil)
	return err
}

// Evaluate 执行 JS 并取回文本结果（对象/数组按 JSON 返回）。
func (p *Page) Evaluate(js string) (string, error) {
	if p == nil || p.S == nil {
		return "", nil
	}
	raw, err := p.S.Call("Runtime.evaluate", map[string]any{
		"expression":    js,
		"returnByValue": true,
		"awaitPromise":  true,
		"userGesture":   true,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", nil
	}
	if out.ExceptionDetails != nil {
		return "", fmt.Errorf("webcdp: 脚本异常：%s", out.ExceptionDetails.Text)
	}
	if len(out.Result.Value) == 0 {
		return "", nil
	}
	if out.Result.Type == "string" {
		var s string
		if json.Unmarshal(out.Result.Value, &s) == nil {
			return s, nil
		}
	}
	return string(out.Result.Value), nil
}

// ScreenshotJPEG 抓一帧（quality 0~100，建议 60~80）。
func (p *Page) ScreenshotJPEG(quality int) ([]byte, error) {
	if quality <= 0 || quality > 100 {
		quality = 70
	}
	raw, err := p.S.CallTimeout("Page.captureScreenshot", map[string]any{
		"format":                "jpeg",
		"quality":               quality,
		"fromSurface":           true,
		"captureBeyondViewport": false,
	}, 15e9)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Data == "" {
		return nil, fmt.Errorf("webcdp: 截图结果为空")
	}
	return base64.StdEncoding.DecodeString(out.Data)
}

// SetViewport 设定视图区尺寸与像素比（尺寸变化时才需要调用）。
func (p *Page) SetViewport(w, h int, dpr float64) error {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if dpr <= 0 {
		dpr = 1
	}
	_, err := p.S.Call("Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             w,
		"height":            h,
		"deviceScaleFactor": dpr,
		"mobile":            false,
	})
	return err
}

// SetUserAgent 覆盖 UA（影响后续请求）。
func (p *Page) SetUserAgent(ua string) error {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return nil
	}
	_, err := p.S.Call("Network.setUserAgentOverride", map[string]any{"userAgent": ua})
	return err
}

// mouse 派发一次鼠标事件。
func (p *Page) mouse(kind string, x, y float64, button string, clicks int) error {
	if button == "" {
		button = "left"
	}
	params := map[string]any{
		"type":       kind,
		"x":          x,
		"y":          y,
		"button":     button,
		"clickCount": clicks,
	}
	if kind == "mouseMoved" {
		delete(params, "button")
		delete(params, "clickCount")
		params["buttons"] = 0
	}
	_, err := p.S.Call("Input.dispatchMouseEvent", params)
	return err
}

// MouseMove 移动鼠标。
func (p *Page) MouseMove(x, y float64) error { return p.mouse("mouseMoved", x, y, "none", 0) }

// MouseDown 按下。
func (p *Page) MouseDown(x, y float64, button string) error {
	return p.mouse("mousePressed", x, y, button, 1)
}

// MouseUp 松开。
func (p *Page) MouseUp(x, y float64, button string) error {
	return p.mouse("mouseReleased", x, y, button, 1)
}

// Click 点击一次。
func (p *Page) Click(x, y float64, button string) error {
	if err := p.MouseDown(x, y, button); err != nil {
		return err
	}
	return p.MouseUp(x, y, button)
}

// Wheel 滚轮（dy > 0 向下）。
func (p *Page) Wheel(x, y, dx, dy float64) error {
	_, err := p.S.Call("Input.dispatchMouseEvent", map[string]any{
		"type":   "mouseWheel",
		"x":      x,
		"y":      y,
		"deltaX": dx,
		"deltaY": dy,
	})
	return err
}

// keyInfo 常用按键的 CDP 描述。
type keyInfo struct {
	key     string
	code    string
	keyCode int
	text    string
}

var keyTable = map[string]keyInfo{
	"enter":      {"Enter", "Enter", 13, "\r"},
	"tab":        {"Tab", "Tab", 9, ""},
	"backspace":  {"Backspace", "Backspace", 8, ""},
	"escape":     {"Escape", "Escape", 27, ""},
	"esc":        {"Escape", "Escape", 27, ""},
	"delete":     {"Delete", "Delete", 46, ""},
	"arrowup":    {"ArrowUp", "ArrowUp", 38, ""},
	"up":         {"ArrowUp", "ArrowUp", 38, ""},
	"arrowdown":  {"ArrowDown", "ArrowDown", 40, ""},
	"down":       {"ArrowDown", "ArrowDown", 40, ""},
	"arrowleft":  {"ArrowLeft", "ArrowLeft", 37, ""},
	"left":       {"ArrowLeft", "ArrowLeft", 37, ""},
	"arrowright": {"ArrowRight", "ArrowRight", 39, ""},
	"right":      {"ArrowRight", "ArrowRight", 39, ""},
	"home":       {"Home", "Home", 36, ""},
	"end":        {"End", "End", 35, ""},
	"pageup":     {"PageUp", "PageUp", 33, ""},
	"pagedown":   {"PageDown", "PageDown", 34, ""},
	"space":      {" ", "Space", 32, " "},
}

// NamedKey 派发一个具名按键（enter / tab / arrowdown / space …）。
func (p *Page) NamedKey(name string) error {
	info, ok := keyTable[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return fmt.Errorf("webcdp: 不支持的按键 %q", name)
	}
	down := map[string]any{
		"type":                  "keyDown",
		"key":                   info.key,
		"code":                  info.code,
		"windowsVirtualKeyCode": info.keyCode,
		"nativeVirtualKeyCode":  info.keyCode,
		"text":                  info.text,
	}
	if _, err := p.S.Call("Input.dispatchKeyEvent", down); err != nil {
		return err
	}
	up := map[string]any{
		"type":                  "keyUp",
		"key":                   info.key,
		"code":                  info.code,
		"windowsVirtualKeyCode": info.keyCode,
		"nativeVirtualKeyCode":  info.keyCode,
	}
	_, err := p.S.Call("Input.dispatchKeyEvent", up)
	return err
}

// InsertText 输入一段文本（走输入法通道，适合自绘输入框转发）。
func (p *Page) InsertText(text string) error {
	if text == "" {
		return nil
	}
	_, err := p.S.Call("Input.insertText", map[string]any{"text": text})
	return err
}

// History 读取导航历史与当前下标。
func (p *Page) History() ([]HistoryEntry, int, error) {
	raw, err := p.S.Call("Page.getNavigationHistory", nil)
	if err != nil {
		return nil, -1, err
	}
	var out struct {
		CurrentIndex int            `json:"currentIndex"`
		Entries      []HistoryEntry `json:"entries"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, -1, err
	}
	return out.Entries, out.CurrentIndex, nil
}

// GoHistory 跳到历史的第 index 条。
func (p *Page) GoHistory(index int) error {
	entries, cur, err := p.History()
	if err != nil {
		return err
	}
	if index < 0 || index >= len(entries) || index == cur {
		return nil
	}
	_, err = p.S.Call("Page.navigateToHistoryEntry", map[string]any{"entryId": entries[index].ID})
	return err
}

// LoadHTML 直接替换文档内容（等价 iframe srcdoc / File → Open 的离线页面）。
func (p *Page) LoadHTML(html string) error {
	raw, err := p.S.Call("Page.getFrameTree", nil)
	if err != nil {
		return err
	}
	var tree struct {
		FrameTree struct {
			Frame struct {
				ID string `json:"id"`
			} `json:"frame"`
		} `json:"frameTree"`
	}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return err
	}
	id := tree.FrameTree.Frame.ID
	if id == "" {
		return fmt.Errorf("webcdp: 取不到主框架")
	}
	_, err = p.S.Call("Page.setDocumentContent", map[string]any{"frameId": id, "html": html})
	return err
}
