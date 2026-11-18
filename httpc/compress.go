package httpc

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
)

// decompress 按 Content-Encoding 逐层解压，支持多层编码。
func decompress(r io.Reader, encoding string) (io.Reader, error) {
	encoding = strings.TrimSpace(encoding)
	if encoding == "" || strings.EqualFold(encoding, "identity") {
		return r, nil
	}
	parts := strings.Split(encoding, ",")
	out := r
	for i := len(parts) - 1; i >= 0; i-- {
		enc := strings.ToLower(strings.TrimSpace(parts[i]))
		switch enc {
		case "", "identity":
		case "gzip", "x-gzip":
			gr, err := gzip.NewReader(out)
			if err != nil {
				if err == io.EOF {
					return strings.NewReader(""), nil
				}
				return nil, fmt.Errorf("gzip 解压失败: %w", err)
			}
			out = gr
		case "br":
			out = brotli.NewReader(out)
		case "deflate":
			zr, err := zlib.NewReader(out)
			if err != nil {
				return nil, fmt.Errorf("deflate 解压失败: %w", err)
			}
			out = zr
		default:
			return nil, fmt.Errorf("不支持的 Content-Encoding: %q", enc)
		}
	}
	return out, nil
}
