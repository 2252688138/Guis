package httpc

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// Upload 以 multipart/form-data 上传文件，等价于 Post(url, Request{Multipart: ...})。
func (c *Client) Upload(url string, files []FilePart, opts ...Option) (*Response, error) {
	mp := &Multipart{Files: files}
	return c.Do(buildRequest(http.MethodPost, url, Request{Multipart: mp}, opts))
}

// UploadFields 上传文件并携带普通表单字段。
func (c *Client) UploadFields(url string, fields Params, files []FilePart, opts ...Option) (*Response, error) {
	mp := &Multipart{Files: files, Fields: fields}
	return c.Do(buildRequest(http.MethodPost, url, Request{Multipart: mp}, opts))
}

// Download 流式下载到本地文件（原子写：先写临时文件再改名，失败不留半成品）。
// 返回的 Response 不含响应体（Data 为空），可用于读取头信息。
func (c *Client) Download(url, path string, opts ...Option) (*Response, error) {
	if path == "" {
		return nil, &Error{Kind: KindConfig, Op: http.MethodGet, URL: url, Err: fmt.Errorf("下载路径为空")}
	}
	if err := ensureDir(path); err != nil {
		return nil, &Error{Kind: KindConfig, Op: http.MethodGet, URL: url, Err: err}
	}
	all := append([]Option{WithResponseType(ResponseStream)}, opts...)
	resp, err := c.Do(buildRequest(http.MethodGet, url, nil, all))
	if err != nil {
		if resp != nil {
			resp.Close()
		}
		return resp, err
	}
	defer resp.Close()
	if resp.Body == nil {
		return resp, &Error{Kind: KindNetwork, Op: http.MethodGet, URL: url, Err: fmt.Errorf("响应体为空")}
	}
	if err := copyToFileAtomic(path, resp.Body); err != nil {
		return resp, &Error{Kind: KindNetwork, Op: http.MethodGet, URL: url, Err: err}
	}
	return resp, nil
}

// ensureDir 确保目标目录存在。
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// Upload 使用默认客户端上传文件。
func Upload(url string, files []FilePart, opts ...Option) (*Response, error) {
	return Default().Upload(url, files, opts...)
}

// Download 使用默认客户端下载文件。
func Download(url, path string, opts ...Option) (*Response, error) {
	return Default().Download(url, path, opts...)
}
