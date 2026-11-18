<!-- group: 系统能力 -->
# 网络请求 · guis/httpc（类 axios 客户端）

`guis/httpc` 是框架内置的 HTTP 客户端：实例化配置、拦截器、超时与取消、重试退避、代理、Cookie、压缩、multipart、上传下载进度、并发限流、统一错误类型一应俱全，用法对齐 Node / 浏览器的 axios。只依赖标准库（brotli 解压用仓库已有依赖），可以脱离 `ui` 单独使用。

```go
import "guis/httpc"
```

## 1. 快速上手

```go
// 1) 全局默认实例：零配置直接发请求
resp, err := httpc.Get("https://api.example.com/user/1")
if err != nil {
    switch {
    case httpc.IsTimeout(err): // 超时
    case httpc.IsCancel(err):  // 被取消
    case httpc.IsStatus(err, 404): // 状态码错误（resp 仍可读业务错误体）
    }
}
var u struct{ ID int `json:"id"`; Name string `json:"name"` }
if err := resp.Bind(&u); err != nil { /* KindDecode */ }

// 2) 独立实例：基地址 + 默认头 + 超时 + 重试
api := httpc.New(httpc.Config{
    BaseURL: "https://api.example.com",
    Header:  http.Header{"Authorization": {"Bearer xxx"}},
    Timeout: 10 * time.Second,
    Retry:   httpc.RetryConfig{Max: 2},
})
resp, err := api.Post("/login", httpc.Request{JSON: map[string]string{"user": "a"}})
```

## 2. 与 axios 的对照

| axios | httpc |
| --- | --- |
| `axios.create({baseURL, timeout, headers})` | `httpc.New(httpc.Config{BaseURL, Timeout, Header})` |
| `axios.get(url, {params, headers})` | `c.Get(url, httpc.WithParam(k, v), httpc.WithHeader(k, v))` |
| `axios.post(url, data)` | `c.Post(url, data)`（map/struct 自动 JSON） |
| `axios.request({method, url, data})` | `c.Do(&httpc.Request{Method: "PUT", URL: u, JSON: v})` |
| `instance.interceptors.request.use(fn)` | `c.UseRequest(func(req *httpc.Request) error)` |
| `instance.interceptors.response.use(fn)` | `c.UseResponse(func(resp *httpc.Response, req *httpc.Request) error)` |
| `axios.isCancel(err)` / `isAxiosError` | `httpc.IsCancel(err)` / `httpc.KindOf(err)` |
| `CancelToken` | `httpc.NewCancelToken()` + `httpc.WithCancelToken(tok)` |
| `signal: AbortController.signal` | `httpc.WithContext(ctx)` |
| `onUploadProgress` / `onDownloadProgress` | `httpc.WithProgress(func(p httpc.Progress))` |
| `responseType: 'json'\|'text'\|'blob'\|'stream'` | `httpc.ResponseJSON / ResponseText / ResponseBytes / ResponseStream` |
| `validateStatus` | `httpc.WithValidateStatus(func(code int) bool)` |
| `timeout` / `maxContentLength` / `maxRedirects` | `WithTimeout` / `WithMaxContentLength` / `WithMaxRedirects` |

## 3. 客户端配置（Config）

零值即默认配置；`New` 可传多个 `Config`，后者非零字段覆盖前者。运行期可用 `c.Set(cfg)` 热更新（会重建传输层）。

| 字段 | 说明 |
| --- | --- |
| `BaseURL` | 基础地址；请求 URL 为绝对地址时忽略 |
| `Header` / `Params` | 默认请求头 / 默认查询参数（请求级同名项覆盖） |
| `UserAgent` | 默认 UA，空则 `guis-httpc/1.0` |
| `Timeout` | 单请求总超时（含读完响应体）；0 取 30s，负数不限 |
| `ConnectTimeout` | 连接超时；0 取 min(Timeout, 15s) |
| `Proxy` | `http://` / `https://` / `socks5://user:pass@host:port` / `"none"`；空串尊重 `HTTP_PROXY` 等环境变量 |
| `TLS` | `*httpc.TLSConfig`：`Config` / `InsecureSkipVerify` / `RootCAs`(PEM) / `ClientCertFile`+`ClientKeyFile` |
| `Transport` | 自定义 `http.RoundTripper`（非空时忽略代理/TLS/连接池字段） |
| `Cookies` / `CookieJar` | 开启自动 Cookie（无 jar 时懒建内存 jar） |
| `MaxRedirects` | 最大重定向次数；0 取 10，负数禁止（3xx 直接返回，不视为错误） |
| `CheckRedirect` | 自定义重定向决策，与 `MaxRedirects` 同时生效 |
| `Retry` | `RetryConfig`（见第 6 节） |
| `MaxConcurrency` | 同时在途请求上限，<=0 不限 |
| `MaxIdleConns` | 连接池空闲连接上限，0 取 32 |
| `DisableCompression` | 关闭自动 `Accept-Encoding: gzip, br` 与解压 |
| `ValidateStatus` | 状态码成功判定，nil 时 200–299 |
| `ResponseType` | 默认响应类型 |
| `MaxContentLength` | 响应体上限字节数，<=0 不限 |
| `Progress` / `ProgressInterval` | 默认进度回调 / 节流间隔（0 取 50ms） |
| `Interceptors` | 三类拦截器集合 |
| `OnRequest` / `OnResponse` | 轻量钩子（日志用，不改变流程） |
| `Debug` | `func(format string, args ...any)`，输出重试/错误等内部事件 |

```go
c := httpc.New(httpc.Config{
    Proxy:   "socks5://127.0.0.1:1080",   // 也支持 http(s):// 与 "none"
    Cookies: true,
    TLS:     &httpc.TLSConfig{InsecureSkipVerify: true},
})
snap := c.Config()   // 返回配置快照（已规范化）
```

## 4. 请求描述（Request）

字段优先级：`Multipart > Form > JSON > Data > Body`。

| 字段 | 说明 |
| --- | --- |
| `Method` / `URL` / `BaseURL` | 方法（默认 GET）/ 地址 / 覆盖实例 BaseURL |
| `Params` / `ParamsEncoder` | 查询参数 / 自定义编码器（默认 key 排序 + 数组展开为重复 key） |
| `Header` | 与实例默认头合并 |
| `Data` | `[]byte`→`application/octet-stream`、`string`→`text/plain`、`url.Values`→表单、其他→JSON |
| `JSON` / `Form` / `Multipart` / `Body` | 明确指定请求体 |
| `ContentType` | 覆盖自动推导的 Content-Type |
| `Timeout` / `Context` / `Token` | 本请求超时 / 上游 context / 取消令牌 |
| `ResponseType` | 本请求响应类型 |
| `ValidateStatus` | 本请求状态码判定 |
| `Progress` / `ProgressInterval` | 本请求进度回调与节流 |
| `MaxContentLength` / `MaxRedirects` | 本请求覆盖值 |
| `Retry` / `NoRetry` | 覆盖重试策略 / 禁用重试 |
| `Cookies` | 额外携带的 Cookie |

方法别名与选项：

```go
c.Get(url, opts...)
c.Delete(url, opts...)
c.Head(url, opts...)
c.Options(url, opts...)
c.Post(url, body, opts...)
c.Put(url, body, opts...)
c.Patch(url, body, opts...)
c.Send("PATCH", url, body, opts...)
c.Do(&httpc.Request{...})

// 选项
httpc.WithHeader("X-Trace", "1")
httpc.WithParams(httpc.Params{"page": 2, "tags": []string{"a", "b"}})
httpc.WithBearer("token")
httpc.WithBasicAuth("user", "pass")
httpc.WithTimeout(5 * time.Second)
httpc.WithContext(ctx)
httpc.WithRetry(httpc.RetryConfig{Max: 3})
httpc.WithResponseType(httpc.ResponseStream)
httpc.WithMaxContentLength(8 << 20)
```

`Post`/`Put` 的 `body` 可以直接传 `httpc.Request{...}`（字段合并），也可以传任意值作为 `Data`。

## 5. 响应（Response）

| 成员 | 说明 |
| --- | --- |
| `Status` / `StatusText` / `Proto` / `Header` | 状态与头 |
| `Request` / `URL` | 底层 `*http.Request` 与最终地址（含重定向） |
| `Duration` / `Size` / `Attempts` | 耗时 / 响应体字节（解压后）/ 实际尝试次数 |
| `Data` / `Body` | 响应体字节 / stream 模式的 `io.ReadCloser` |
| `Cookies` | 响应下发的 Cookie |
| `Bytes()` / `Text()` | 字节 / 字符串 |
| `Bind(v any)` | 按 Content-Type 解码 JSON / XML，未知类型先试 JSON 再试 XML |
| `SaveTo(path)` | 落盘（临时文件 + rename，失败不留半成品） |
| `Close()` | 释放 stream 连接与超时 context |
| `OK()` | 200–299 |

```go
resp, err := c.Get(url, httpc.WithResponseType(httpc.ResponseStream))
if err != nil { return err }
defer resp.Close()                 // 释放连接
f, _ := os.Create("big.bin")
io.Copy(f, resp.Body)              // 自行流式读取
```

## 6. 重试（RetryConfig）

```go
c := httpc.New(httpc.Config{Retry: httpc.RetryConfig{
    Max:          3,                       // 重试次数（不含首次）
    Backoff:      200 * time.Millisecond,  // 首次退避，指数增长
    MaxBackoff:   5 * time.Second,
    Jitter:       0.2,                     // 抖动系数，负数禁用
    RetryStatus:  []int{408, 425},
    RetryOn:      nil,                     // nil = 默认策略
    DisableRetryAfter: false,              // 默认遵守 Retry-After
}})
```

- 默认重试：网络错误、超时、429、5xx；`KindCanceled/KindConfig/KindTooLarge/KindDecode` 不重试。
- `Retry-After`（秒或 HTTP 日期）优先于指数退避。
- 退避期间可被 context / CancelToken 中断（返回 `KindCanceled`）。
- 请求体需可重放：`[]byte` / `string` / JSON / Form / 文件路径都可；裸 `io.Reader`（未实现 `io.Seeker`）自动跳过重试。

## 7. 进度（Progress）

```go
resp, err := c.Get(url, httpc.WithProgress(func(p httpc.Progress) {
    // p.Phase: "upload" / "download"
    // p.Loaded / p.Total(-1 未知) / p.Percent / p.Rate(字节每秒) / p.Elapsed / p.Done
    bar.Set(p.Percent)  // 回调在传输线程，UI 里记得 ui.RequestRedraw()
}), httpc.WithProgressInterval(60*time.Millisecond))
```

回调默认按 50ms 或 64KiB 节流，避免拖慢传输与界面。

## 8. multipart 上传

```go
mp := httpc.NewMultipart().
    Field("name", "报告").                 // 普通字段（可重复调用）
    AddFile("file", "C:/tmp/a.pdf")        // 本地路径（可重放）
mp.Files = append(mp.Files, httpc.FilePart{
    Field: "inline", FileName: "note.txt",
    Reader: strings.NewReader("hello"),    // 内容流（不可 Seek 时不能重试）
})
resp, err := c.Post(url, httpc.Request{Multipart: mp},
    httpc.WithProgress(progress))          // 上传进度

// 便捷方法
c.Upload(url, []httpc.FilePart{{Field: "f", Path: "a.png"}})
c.UploadFields(url, httpc.Params{"tag": "v1"}, files)
```

文件以管道流式写入，不会把整个文件读进内存。

## 9. 下载

```go
resp, err := c.Download(url, "D:/downloads/a.zip", httpc.WithProgress(func(p httpc.Progress) {
    log.Printf("%.1f%% %d B/s", p.Percent, p.Rate)
}))
```

`Download` 用 `ResponseStream` + 临时文件 + rename 原子落盘，目录不存在会自动创建；返回的 `Response` 不含响应体，但可读取头与状态。

## 10. 拦截器

```go
c := httpc.New()
c.UseRequest(func(req *httpc.Request) error {        // 顺序执行
    if req.Header == nil { req.Header = http.Header{} }
    req.Header.Set("X-Trace", traceID())
    return nil                                        // 返回错误 = 中止请求
})
c.UseResponse(func(resp *httpc.Response, req *httpc.Request) error { // 逆序执行
    log.Printf("%s %s → %d in %s", req.Method, req.URL, resp.Status, resp.Duration)
    return nil
})
c.UseError(func(err error, req *httpc.Request) error {
    report(err)                                       // 可包装/吞掉错误
    return err
})
```

## 11. 错误处理

失败统一返回 `*httpc.Error`：

| 成员 | 说明 |
| --- | --- |
| `Kind` | `network` / `timeout` / `canceled` / `status` / `decode` / `too_large` / `config` / `unsupported` |
| `Op` / `Method` / `URL` | 请求方法与方法名（`Op` 为历史别名）/ 地址 |
| `Status` / `Code` | 状态码 / 业务错误码（响应头 `X-Error-Code`） |
| `Response` | 失败时的响应（可读取业务错误体） |
| `Attempts` / `RetryAfter` | 尝试次数 / 服务端建议等待 |
| `Err` | 底层错误，支持 `errors.Is/As` |

```go
if err != nil {
    switch httpc.KindOf(err) {
    case httpc.KindTimeout:  /* 可提示"网络超时" */
    case httpc.KindCanceled: /* 用户主动取消 */
    case httpc.KindStatus:
        if httpc.IsStatus(err, 401) { /* 需要登录 */ }
        if e := httpc.ErrorOf(err); e != nil && e.Response != nil {
            var biz struct{ Message string `json:"message"` }
            _ = e.Response.Bind(&biz)
        }
    }
}
```

错误文本会对 `Authorization` / `Cookie` / `Proxy-Authorization` 自动脱敏，可安全写日志。

## 12. 平台支持

| 平台 | 状态 |
| --- | --- |
| Windows / Linux / macOS | 完整支持（代理、TLS、Cookie、压缩、并发、进度） |
| Android | 与桌面共用 `net/http` 实现 |
| 浏览器（GOOS=js GOARCH=wasm） | 可编译并工作：请求走 Go 标准库的 fetch 传输；`Proxy` / 自定义 `Transport` / 客户端证书等系统级能力不可用 |

`go build ./httpc/` 在上述平台均通过（已随仓库验证）。

## 13. 组合示例：带重试与取消的上传

```go
c := httpc.New(httpc.Config{
    BaseURL:        "https://api.example.com",
    Timeout:        60 * time.Second,
    Retry:          httpc.RetryConfig{Max: 2},
    MaxConcurrency: 4,
    Cookies:        true,
})
c.UseRequest(func(req *httpc.Request) error {
    req.Header.Set("Authorization", "Bearer "+token())
    return nil
})

tok := httpc.NewCancelToken()
mp := httpc.NewMultipart().Field("kind", "avatar").AddFile("file", "avatar.png")
resp, err := c.Post("/upload", httpc.Request{Multipart: mp},
    httpc.WithCancelToken(tok),
    httpc.WithProgress(func(p httpc.Progress) { fmt.Printf("%.0f%%\r", p.Percent) }),
)
```

## 14. 在线演示（gallery）

`go run ./cmd/gallery --case net` 打开「网络请求」页，5 个页签对应上面全部能力：

| 页签 | 演示内容 |
| --- | --- |
| 基础请求 | 方法 / 路径 / 查询参数 / 请求头 / JSON 请求体，响应预览与耗时、大小、尝试次数 |
| 重试与超时 | 不稳定接口（前两次 500）观察退避与重试日志；慢接口 + 短超时观察 `KindTimeout`；中途取消观察 `KindCanceled` |
| 上传与下载 | 64KiB / 512KiB / 2MiB 下载（进度条 + 速度 + 原子落盘）、multipart 上传回显 |
| 拦截器与错误 | 拦截器注入 `X-Trace`、CookieJar 连续两次请求、404 / 500 / 超时 / 取消 / 坏 JSON / 网络不可达的 `Kind` 与 `Is*` 判定 |
| 示例代码 | 当前页签对应的完整可复制代码 |

演示页自带一个进程内 `httptest` 服务（`cmd/gallery/pages/net_demo_server.go`），完全离线：

| 端点 | 说明 |
| --- | --- |
| `GET /api/echo` | 回显方法 / 路径 / 查询 / 请求头 / 请求体 / Cookie |
| `GET /api/users` | 固定 JSON 数组（演示 `Bind`） |
| `GET /api/status?code=404` | 指定状态码 + `X-Error-Code: DEMO_404` |
| `GET /api/slow?ms=1500` | 延迟响应（演示超时 / 取消） |
| `GET /api/flaky?fail=2` | 前 2 次 500，之后 200（演示重试，`Retry-After: 0`） |
| `GET /api/download?kb=512` | 指定大小字节流（演示下载进度） |
| `POST /api/upload` | 解析 multipart，回显字段与文件名 / 大小 |
| `GET /api/cookie/echo` | 首次下发 `guis_sid`，之后回显（演示 CookieJar） |
| `GET /api/redirect?n=3` | 链式 302（演示重定向） |
| `GET /api/badjson` | 非法 JSON（演示 `KindDecode`） |
| `GET /api/gzip` | gzip 压缩响应（演示自动解压） |
| `GET /api/xml` | XML 响应（演示 `Bind` 自动识别） |