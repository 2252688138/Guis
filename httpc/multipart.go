package httpc

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// FilePart 分段上传中的一个文件。Path 与 Reader 二选一（Path 优先）。
type FilePart struct {
	// Field 表单字段名。
	Field string
	// Path 文件路径（可重放，推荐）。
	Path string
	// Reader 文件内容（不可 Seek 时不能重试）。
	Reader io.Reader
	// FileName 上传后的文件名，空则取 Path 的文件名。
	FileName string
	// ContentType 文件 MIME，空则按扩展名推断。
	ContentType string
}

// Multipart 分段上传请求体。
type Multipart struct {
	// Fields 普通表单字段。
	Fields Params
	// Files 文件列表。
	Files []FilePart
	// Boundary 自定义分隔符，空则自动生成。
	Boundary string
}

// NewMultipart 创建分段上传请求体。
func NewMultipart() *Multipart { return &Multipart{} }

// Field 追加普通字段。
func (m *Multipart) Field(name string, value any) *Multipart {
	if m.Fields == nil {
		m.Fields = Params{}
	}
	m.Fields[name] = value
	return m
}

// AddFile 追加文件（本地路径）。
func (m *Multipart) AddFile(field, path string) *Multipart {
	m.Files = append(m.Files, FilePart{Field: field, Path: path})
	return m
}

// AddReader 追加文件（内容流）。
func (m *Multipart) AddReader(field, name string, r io.Reader) *Multipart {
	m.Files = append(m.Files, FilePart{Field: field, Reader: r, FileName: name})
	return m
}

// body 生成请求体。使用管道流式写入，避免大文件进内存。
func (m *Multipart) body() (*bodySource, error) {
	if m == nil {
		return nil, nil
	}
	boundary := m.Boundary
	if boundary == "" {
		rnd := make([]byte, 16)
		if _, err := rand.Read(rnd); err != nil {
			return nil, fmt.Errorf("生成 multipart 分隔符失败: %w", err)
		}
		boundary = hex.EncodeToString(rnd)
	}
	replayable := true
	for _, f := range m.Files {
		if f.Path == "" && !isReplayable(f.Reader) {
			replayable = false
			break
		}
	}
	open := func() (io.ReadCloser, int64, error) {
		pr, pw := io.Pipe()
		go func() {
			if err := m.write(pw, boundary); err != nil {
				pw.CloseWithError(err)
				return
			}
			pw.Close()
		}()
		return pr, -1, nil
	}
	return &bodySource{
		open:        open,
		replayable:  replayable,
		contentType: "multipart/form-data; boundary=" + boundary,
		size:        -1,
	}, nil
}

// write 把字段与文件写入管道。
func (m *Multipart) write(w io.Writer, boundary string) error {
	mw := multipart.NewWriter(w)
	if err := mw.SetBoundary(boundary); err != nil {
		return err
	}
	names := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		names = append(names, k)
	}
	sortStrings(names)
	for _, k := range names {
		for _, v := range valueStrings(m.Fields[k]) {
			if err := mw.WriteField(k, v); err != nil {
				return err
			}
		}
	}
	for _, f := range m.Files {
		field := f.Field
		if field == "" {
			field = "file"
		}
		if err := m.writeFile(mw, field, f); err != nil {
			return err
		}
	}
	return mw.Close()
}

// writeFile 写入单个文件部分。
func (m *Multipart) writeFile(mw *multipart.Writer, field string, f FilePart) error {
	name := f.FileName
	var src io.Reader
	if f.Path != "" {
		fh, err := os.Open(f.Path)
		if err != nil {
			return fmt.Errorf("打开上传文件失败: %w", err)
		}
		defer fh.Close()
		src = fh
		if name == "" {
			name = filepath.Base(f.Path)
		}
	} else if f.Reader != nil {
		src = f.Reader
	} else {
		return fmt.Errorf("文件字段 %q 既没有 Path 也没有 Reader", field)
	}
	if name == "" {
		name = "upload.bin"
	}
	h := make(map[string][]string, 1)
	ct := f.ContentType
	if ct == "" {
		ct = mimeByExt(name)
	}
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, name)}
	if ct != "" {
		h["Content-Type"] = []string{ct}
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, src)
	return err
}

// isReplayable 判断 reader 能否重读。
func isReplayable(r io.Reader) bool {
	if r == nil {
		return true
	}
	if _, ok := r.(io.Seeker); ok {
		return true
	}
	switch r.(type) {
	case *strings.Reader, *bytes.Reader, *bytes.Buffer:
		return true
	}
	return false
}

// sortStrings 极简排序（避免额外 import 依赖）。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// mimeByExt 按扩展名推断 MIME。
func mimeByExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return "application/json"
	case ".txt", ".md", ".log":
		return "text/plain; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".csv":
		return "text/csv"
	}
	return "application/octet-stream"
}
