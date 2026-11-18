package httpc

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// testClient 返回禁用代理的测试客户端，避免环境变量干扰。
func testClient(cfg ...Config) *Client {
	base := Config{Proxy: "none", Timeout: 5 * time.Second}
	if len(cfg) > 0 {
		base = mergeConfig(base, cfg[0])
	}
	return New(base)
}

// TestGetJSONBind 验证基础 GET 与 JSON 绑定。
func TestGetJSONBind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("方法 = %s，期望 GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":7,"name":"guis"}`)
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL + "/user")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if resp.Status != 200 || !resp.OK() {
		t.Fatalf("状态 = %d，期望 200", resp.Status)
	}
	var u struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := resp.Bind(&u); err != nil {
		t.Fatalf("Bind 失败: %v", err)
	}
	if u.ID != 7 || u.Name != "guis" {
		t.Fatalf("绑定结果 = %+v", u)
	}
	if resp.Text() != `{"id":7,"name":"guis"}` {
		t.Fatalf("Text() = %q", resp.Text())
	}
}

// TestPostJSON 验证 JSON 请求体与 Content-Type。
func TestPostJSON(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = io.WriteString(w, `ok`)
	}))
	defer srv.Close()

	_, err := testClient().Post(srv.URL+"/login", Request{JSON: map[string]string{"user": "a"}})
	if err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("Content-Type = %q", gotCT)
	}
	if gotBody != `{"user":"a"}` {
		t.Fatalf("body = %q", gotBody)
	}
}

// TestDataSerialization 验证 Data 的类型分派。
func TestDataSerialization(t *testing.T) {
	type capture struct {
		ct, body string
	}
	caps := make(chan capture, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		caps <- capture{ct: r.Header.Get("Content-Type"), body: string(b)}
	}))
	defer srv.Close()
	c := testClient()

	cases := []struct {
		name     string
		data     any
		wantCT   string
		wantBody string
	}{
		{"string", "hello", "text/plain", "hello"},
		{"bytes", []byte("raw"), "application/octet-stream", "raw"},
		{"form", url.Values{"a": {"1"}}, "application/x-www-form-urlencoded", "a=1"},
		{"struct", map[string]int{"n": 1}, "application/json", `{"n":1}`},
	}
	for _, tc := range cases {
		if _, err := c.Post(srv.URL, tc.data); err != nil {
			t.Fatalf("%s: Post 失败: %v", tc.name, err)
		}
		got := <-caps
		if !strings.HasPrefix(got.ct, tc.wantCT) {
			t.Errorf("%s: Content-Type = %q，期望 %q", tc.name, got.ct, tc.wantCT)
		}
		if got.body != tc.wantBody {
			t.Errorf("%s: body = %q，期望 %q", tc.name, got.body, tc.wantBody)
		}
	}
}

// TestFormBody 验证 Request.Form。
func TestFormBody(t *testing.T) {
	var body, ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, ct = string(b), r.Header.Get("Content-Type")
	}))
	defer srv.Close()

	_, err := testClient().Post(srv.URL, Request{Form: url.Values{"x": {"1"}, "y": {"2"}}})
	if err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if body != "x=1&y=2" {
		t.Fatalf("body = %q", body)
	}
	if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

// TestURLAndParams 验证 BaseURL 拼接与参数合并。
func TestURLAndParams(t *testing.T) {
	var gotRaw string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RequestURI()
	}))
	defer srv.Close()

	c := testClient(Config{BaseURL: srv.URL + "/api/", Params: Params{"v": 1, "keep": "a"}})
	if _, err := c.Get("users", WithParam("q", "搜 索"), WithParam("ids", []string{"1", "2"}), WithParam("keep", "b")); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	u, err := url.Parse(gotRaw)
	if err != nil {
		t.Fatalf("解析 URI 失败: %v", err)
	}
	if u.Path != "/api/users" {
		t.Fatalf("path = %q，期望 /api/users", u.Path)
	}
	q := u.Query()
	if q.Get("v") != "1" || q.Get("keep") != "b" || q.Get("q") != "搜 索" {
		t.Fatalf("query = %v", q)
	}
	if len(q["ids"]) != 2 {
		t.Fatalf("ids = %v", q["ids"])
	}
}

// TestHeaderMerge 验证默认头与请求头合并。
func TestHeaderMerge(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer srv.Close()

	c := testClient(Config{Header: http.Header{"X-App": {"guis"}, "Authorization": {"Bearer a"}}})
	if _, err := c.Get(srv.URL, WithHeader("X-App", "override"), WithBearer("b")); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got.Get("X-App") != "override" {
		t.Fatalf("X-App = %q", got.Get("X-App"))
	}
	if got.Get("Authorization") != "Bearer b" {
		t.Fatalf("Authorization = %q", got.Get("Authorization"))
	}
	if got.Get("User-Agent") == "" {
		t.Fatalf("缺少默认 User-Agent")
	}
}

// TestResponseTypes 验证四种响应类型。
func TestResponseTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "hello world")
	}))
	defer srv.Close()
	c := testClient()

	resp, err := c.Get(srv.URL, WithResponseType(ResponseText))
	if err != nil {
		t.Fatalf("text: %v", err)
	}
	if resp.Text() != "hello world" || resp.Type != ResponseText {
		t.Fatalf("text 响应 = %q type=%v", resp.Text(), resp.Type)
	}

	resp, err = c.Get(srv.URL, WithResponseType(ResponseBytes))
	if err != nil {
		t.Fatalf("bytes: %v", err)
	}
	if string(resp.Bytes()) != "hello world" {
		t.Fatalf("bytes 响应 = %q", resp.Bytes())
	}

	resp, err = c.Get(srv.URL, WithResponseType(ResponseStream))
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取 stream 失败: %v", err)
	}
	if string(body) != "hello world" {
		t.Fatalf("stream 响应 = %q", body)
	}
	if err := resp.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
}

// TestBindXML 验证 XML 响应绑定。
func TestBindXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<root><name>guis</name></root>`)
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	var out struct {
		Name string `xml:"name"`
	}
	if err := resp.Bind(&out); err != nil {
		t.Fatalf("Bind 失败: %v", err)
	}
	if out.Name != "guis" {
		t.Fatalf("name = %q", out.Name)
	}
	var m map[string]any
	if err := resp.Bind(&m); err == nil {
		t.Fatal("XML 无法绑定到 map 时应报错")
	}
}

// TestValidateStatus 验证状态码判定与错误携带响应。
func TestValidateStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Error-Code", "USER_NOT_FOUND")
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"err":"not found"}`)
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err == nil {
		t.Fatal("404 应返回错误")
	}
	if !IsStatus(err, 404) {
		t.Fatalf("IsStatus = false，err = %v", err)
	}
	e := ErrorOf(err)
	if e == nil || e.Kind != KindStatus || e.Code != "USER_NOT_FOUND" {
		t.Fatalf("错误详情 = %+v", e)
	}
	if resp == nil || resp.Status != 404 {
		t.Fatalf("应携带 404 响应，实际 = %+v", resp)
	}
	var body struct {
		Err string `json:"err"`
	}
	if err := resp.Bind(&body); err != nil || body.Err != "not found" {
		t.Fatalf("错误体读取失败: %v %+v", err, body)
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "USER_NOT_FOUND") {
		t.Fatalf("错误文本 = %q", err.Error())
	}

	// 自定义 validateStatus：404 视为成功
	resp2, err2 := testClient().Get(srv.URL, WithValidateStatus(func(code int) bool { return code < 500 }))
	if err2 != nil {
		t.Fatalf("自定义 validateStatus 应成功: %v", err2)
	}
	if resp2.Status != 404 {
		t.Fatalf("状态 = %d", resp2.Status)
	}
}

// TestTimeout 验证超时错误分类。
func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	_, err := testClient(Config{Timeout: 30 * time.Millisecond}).Get(srv.URL)
	if err == nil {
		t.Fatal("应超时")
	}
	if !IsTimeout(err) {
		t.Fatalf("IsTimeout = false，err = %v", err)
	}
	if KindOf(err) != KindTimeout {
		t.Fatalf("Kind = %q", KindOf(err))
	}
}

// TestContextCancel 验证 context 取消。
func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := testClient().Get(srv.URL, WithContext(ctx))
	if err == nil {
		t.Fatal("应取消")
	}
	if !IsCancel(err) || KindOf(err) != KindCanceled {
		t.Fatalf("取消错误识别失败: kind=%q err=%v", KindOf(err), err)
	}
}

// TestCancelToken 验证 CancelToken。
func TestCancelToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	tok := NewCancelToken()
	go func() {
		time.Sleep(20 * time.Millisecond)
		tok.Cancel()
	}()
	_, err := testClient().Get(srv.URL, WithCancelToken(tok))
	if !IsCancel(err) {
		t.Fatalf("CancelToken 未生效: %v", err)
	}
	if !tok.Canceled() || !errors.Is(tok.Err(), context.Canceled) {
		t.Fatalf("令牌状态异常: canceled=%v err=%v", tok.Canceled(), tok.Err())
	}
}

// TestInterceptorOrder 验证拦截器执行顺序与短路。
func TestInterceptorOrder(t *testing.T) {
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "server")
	}))
	defer srv.Close()

	c := testClient()
	c.UseRequest(func(req *Request) error {
		order = append(order, "req1")
		req.Header.Set("X-Step", "1")
		return nil
	}).UseRequest(func(req *Request) error {
		order = append(order, "req2")
		return nil
	}).UseResponse(func(resp *Response, req *Request) error {
		order = append(order, "resp1")
		return nil
	}).UseResponse(func(resp *Response, req *Request) error {
		order = append(order, "resp2")
		return nil
	})
	if _, err := c.Get(srv.URL); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	want := []string{"req1", "req2", "server", "resp2", "resp1"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("顺序 = %v，期望 %v", order, want)
	}
}

// TestInterceptorShortCircuit 验证请求拦截器错误可中止请求。
func TestInterceptorShortCircuit(t *testing.T) {
	var hit int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
	}))
	defer srv.Close()

	c := testClient()
	boom := errors.New("拦截器中止")
	c.UseRequest(func(req *Request) error { return boom })
	c.UseError(func(err error, req *Request) error {
		return fmt.Errorf("包装: %w", err)
	})
	_, err := c.Get(srv.URL)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("错误应包裹原错误: %v", err)
	}
	if hit != 0 {
		t.Fatalf("请求不应发出，服务端命中 %d 次", hit)
	}
}

// TestErrorRedaction 验证敏感头脱敏。
func TestErrorRedaction(t *testing.T) {
	e := &Error{Kind: KindStatus, Status: 401, URL: "http://x", Err: errors.New("Authorization: secret-token; Cookie: sid=abc")}
	msg := e.Error()
	if strings.Contains(msg, "secret-token") || strings.Contains(msg, "sid=abc") {
		t.Fatalf("错误文本泄露敏感信息: %s", msg)
	}
}

// TestKindHelpers 验证错误判定函数。
func TestKindHelpers(t *testing.T) {
	status := &Error{Kind: KindStatus, Status: 500}
	network := &Error{Kind: KindNetwork}
	timeout := &Error{Kind: KindTimeout}
	canceled := &Error{Kind: KindCanceled}
	if !IsStatus(status, 500) || IsStatus(status, 404) {
		t.Fatal("IsStatus 判定错误")
	}
	if !IsNetwork(network) || IsNetwork(status) {
		t.Fatal("IsNetwork 判定错误")
	}
	if !IsTimeout(timeout) || !timeout.Timeout() {
		t.Fatal("IsTimeout 判定错误")
	}
	if !IsCancel(canceled) {
		t.Fatal("IsCancel 判定错误")
	}
	if KindOf(fmt.Errorf("wrap: %w", status)) != KindStatus {
		t.Fatal("KindOf 应能穿透 wrapped 错误")
	}
	if !errors.Is(network, network) || network.Unwrap() != nil {
		t.Fatal("Unwrap 行为异常")
	}
}

// TestBasicAuth 验证 Basic 认证编码。
func TestBasicAuth(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	if _, err := testClient().Get(srv.URL, WithBasicAuth("u", "p")); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got != "Basic dTpw" {
		t.Fatalf("Authorization = %q", got)
	}
}

// TestConfigDefaults 验证零值与默认值。
func TestConfigDefaults(t *testing.T) {
	var zero Client
	cfg := zero.Config()
	if cfg.Timeout != defaultTimeout || cfg.MaxRedirects != defaultMaxRedirects || cfg.UserAgent != defaultUserAgent {
		t.Fatalf("默认配置异常: %+v", cfg)
	}
	c := New(Config{BaseURL: "http://a"}, Config{Timeout: time.Second})
	if c.Config().BaseURL != "http://a" || c.Config().Timeout != time.Second {
		t.Fatalf("多 Config 合并异常: %+v", c.Config())
	}
	clone := c.Clone()
	clone.Set(Config{BaseURL: "http://b"})
	if c.Config().BaseURL != "http://a" {
		t.Fatal("Clone 应相互独立")
	}
}

// TestJSONEncodeFailure 验证序列化失败被拦截。
func TestJSONEncodeFailure(t *testing.T) {
	_, err := testClient().Post("http://127.0.0.1:1/x", func() {})
	if err == nil || KindOf(err) != KindConfig {
		t.Fatalf("非法 JSON body 应返回 KindConfig，实际 %v", err)
	}
}

// TestRelativeURLWithoutBase 验证缺少 BaseURL 时报错。
func TestRelativeURLWithoutBase(t *testing.T) {
	_, err := testClient().Get("/rel")
	if err == nil || KindOf(err) != KindConfig {
		t.Fatalf("应返回 KindConfig 错误，实际 %v", err)
	}
}

// TestJSONFieldPriority 验证 JSON 优先于 Data。
func TestJSONFieldPriority(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()

	if _, err := testClient().Post(srv.URL, Request{Data: "data", JSON: map[string]string{"k": "v"}}); err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	if body != `{"k":"v"}` {
		t.Fatalf("body = %q，期望 JSON 优先", body)
	}
}

// TestResponseJSONDecodeError 验证 Bind 错误分类。
func TestResponseJSONDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "not-json")
	}))
	defer srv.Close()

	resp, err := testClient().Get(srv.URL)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	var v map[string]any
	if err := resp.Bind(&v); err == nil || KindOf(err) != KindDecode {
		t.Fatalf("应返回 KindDecode，实际 %v", err)
	}
	var raw []byte
	if err := json.Unmarshal([]byte("x"), &raw); err == nil {
		t.Fatal("unreachable")
	}
}

// 确保未使用的导入不会报错。
var _ = xml.Unmarshal
