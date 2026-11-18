// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

// Package httpc 是 guis 框架内置的 HTTP 客户端，用法对齐 Node/浏览器的 axios。
//
// 设计目标：一次配置、处处复用；声明式描述请求；拦截器串联；统一错误类型；
// 覆盖上传下载进度、重试、代理、Cookie、压缩、并发限流等真实工程需求。
// 本包只依赖标准库（brotli 解压用仓库已有依赖），可脱离 ui 单独使用。
//
// # 快速上手
//
//	// 全局默认实例（零配置即可用）
//	resp, err := httpc.Get("https://api.example.com/user/1")
//	if err != nil { /* 统一错误：httpc.IsTimeout / IsStatus / IsCancel */ }
//	var u User
//	if err := resp.Bind(&u); err != nil { ... }
//
//	// 独立实例：基地址 + 默认头 + 超时 + 重试
//	api := httpc.New(httpc.Config{
//		BaseURL: "https://api.example.com",
//		Header:  http.Header{"Authorization": {"Bearer xxx"}},
//		Timeout: 10 * time.Second,
//		Retry:   httpc.RetryConfig{Max: 2},
//	})
//	resp, err := api.Post("/login", httpc.Request{JSON: map[string]string{"user": "a"}})
//
// # 与 axios 的对应关系
//
//	axios.create(cfg)                  → httpc.New(httpc.Config{...})
//	axios.get(url, {params, headers})  → c.Get(url, httpc.WithParam(k, v), httpc.WithHeader(k, v))
//	axios.post(url, data)              → c.Post(url, httpc.Request{JSON: data})
//	axios.interceptors.request.use     → c.UseRequest(func(req *httpc.Request) error)
//	axios.interceptors.response.use    → c.UseResponse(func(resp *httpc.Response, req *httpc.Request) error)
//	axios.isCancel / isAxiosError      → httpc.IsCancel(err) / httpc.KindOf(err)
//	CancelToken                        → httpc.NewCancelToken()
//	onUploadProgress/onDownloadProgress → httpc.WithProgress(func(p httpc.Progress))
//	responseType                       → httpc.Request{ResponseType: httpc.ResponseStream}
//	validateStatus                     → httpc.WithValidateStatus(func(code int) bool)
//
// # 约定
//
//   - 响应体在返回前已读完（stream 模式除外），调用方无需手动 Close。
//   - 失败时返回 *httpc.Error，用 httpc.IsTimeout / IsCancel / IsStatus / IsNetwork 判别；
//     状态码错误同样携带 *Response，可读取业务错误体。
//   - 时间/字节类字段零值表示“使用默认值”；Timeout < 0 表示不限超时，MaxContentLength <= 0 表示不限长度。
package httpc
