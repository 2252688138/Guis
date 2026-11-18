package httpc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Params 查询参数表。值的合法形态：
//
//	string / []string / []byte / bool / 整数 / 浮点 / json.Number /
//	fmt.Stringer / time.Time / nil（跳过）/ 以及上述类型的切片（重复 key）。
type Params map[string]any

// ParamsEncoder 自定义参数编码器，输入已合并的参数表与原始 query，返回最终 query 串（不含 '?'）。
type ParamsEncoder func(p Params, rawQuery string) string

// valueStrings 把任意值展开成字符串列表；nil 返回 nil。
func valueStrings(v any) []string {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		return []string{x}
	case []string:
		return x
	case []byte:
		return []string{string(x)}
	case bool:
		return []string{strconv.FormatBool(x)}
	case int:
		return []string{strconv.Itoa(x)}
	case int8:
		return []string{strconv.FormatInt(int64(x), 10)}
	case int16:
		return []string{strconv.FormatInt(int64(x), 10)}
	case int32:
		return []string{strconv.FormatInt(int64(x), 10)}
	case int64:
		return []string{strconv.FormatInt(x, 10)}
	case uint:
		return []string{strconv.FormatUint(uint64(x), 10)}
	case uint8:
		return []string{strconv.FormatUint(uint64(x), 10)}
	case uint16:
		return []string{strconv.FormatUint(uint64(x), 10)}
	case uint32:
		return []string{strconv.FormatUint(uint64(x), 10)}
	case uint64:
		return []string{strconv.FormatUint(x, 10)}
	case float32:
		return []string{strconv.FormatFloat(float64(x), 'f', -1, 32)}
	case float64:
		return []string{strconv.FormatFloat(x, 'f', -1, 64)}
	case json.Number:
		return []string{x.String()}
	case time.Time:
		return []string{x.Format(time.RFC3339)}
	case fmt.Stringer:
		return []string{x.String()}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, valueStrings(e)...)
		}
		return out
	case url.Values:
		var out []string
		for _, e := range x {
			out = append(out, e...)
		}
		return out
	default:
		return []string{fmt.Sprint(x)}
	}
}

// encodeParams 默认编码器：key 排序保证稳定输出，数组展开为重复 key。
func encodeParams(p Params, rawQuery string) string {
	if len(p) == 0 {
		return rawQuery
	}
	keys := make([]string, 0, len(p))
	for k, v := range p {
		if len(valueStrings(v)) == 0 {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(rawQuery)
	for _, k := range keys {
		for _, val := range valueStrings(p[k]) {
			if b.Len() > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(val))
		}
	}
	return b.String()
}

// mergeParams 合并参数表，over 覆盖 base 同名项。
func mergeParams(base, over Params) Params {
	if len(base) == 0 && len(over) == 0 {
		return nil
	}
	out := make(Params, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}

// resolveURL 拼接 BaseURL 与 path，并注入查询参数。
func resolveURL(base, raw string, params Params, enc ParamsEncoder) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("请求地址为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("解析请求地址失败: %w", err)
	}
	if !u.IsAbs() {
		if base == "" {
			return "", fmt.Errorf("相对地址 %q 缺少 BaseURL", raw)
		}
		bu, err := url.Parse(base)
		if err != nil {
			return "", fmt.Errorf("解析 BaseURL 失败: %w", err)
		}
		bp := strings.TrimSuffix(bu.Path, "/")
		rp := u.Path
		if !strings.HasPrefix(rp, "/") && rp != "" {
			rp = "/" + rp
		}
		u.Scheme = bu.Scheme
		u.Host = bu.Host
		u.Path = bp + rp
		if u.RawQuery == "" {
			u.RawQuery = bu.RawQuery
		}
	}
	query := u.RawQuery
	if len(params) > 0 {
		if enc == nil {
			enc = encodeParams
		}
		query = enc(params, query)
	}
	u.RawQuery = query
	return u.String(), nil
}
