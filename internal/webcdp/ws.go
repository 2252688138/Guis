// guis
// Author  : Mr.Shu
// Website : www.cq6k.com
// QQ Group: 点击链接加入群聊【guis】：https://qm.qq.com/q/Be12Z5MVJ6

package webcdp

// 最小 WebSocket 客户端（RFC 6455）：只为 CDP 通道服务，零第三方依赖。
//
// 只实现客户端必需的部分：HTTP Upgrade 握手、文本帧收发、分片重组、
// ping/pong 应答、close 帧。不实现扩展与压缩（CDP 不用）。

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsConn 一条已握手的 WebSocket 连接（文本帧）。
type wsConn struct {
	conn net.Conn
	rd   *bufio.Reader
	mu   sync.Mutex // 串行化写入
	once sync.Once
}

// wsDial 建立 WebSocket 连接（地址形如 ws://127.0.0.1:9222/devtools/page/XXXX）。
func wsDial(rawURL string, timeout time.Duration) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// 握手
	var keyRaw [16]byte
	if _, err := rand.Read(keyRaw[:]); err != nil {
		_ = conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyRaw[:])
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	rd := bufio.NewReaderSize(conn, 64*1024)
	status, err := rd.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		_ = conn.Close()
		return nil, fmt.Errorf("webcdp: WebSocket 握手失败：%s", strings.TrimSpace(status))
	}
	var accept string
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(k), "Sec-WebSocket-Accept") {
			accept = strings.TrimSpace(v)
		}
	}
	sum := sha1.Sum([]byte(key + wsGUID))
	if accept != "" && accept != base64.StdEncoding.EncodeToString(sum[:]) {
		_ = conn.Close()
		return nil, errors.New("webcdp: WebSocket 校验值不匹配")
	}
	_ = conn.SetDeadline(time.Time{})
	return &wsConn{conn: conn, rd: rd}, nil
}

// Close 关闭连接。
func (c *wsConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.once.Do(func() {
		_ = c.writeFrame(0x8, nil)
		_ = c.conn.Close()
	})
	return nil
}

// WriteText 发送一个文本帧。
func (c *wsConn) WriteText(s string) error {
	return c.writeFrame(0x1, []byte(s))
}

// writeFrame 写一个客户端帧（必须加掩码）。
func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	if c == nil || c.conn == nil {
		return errors.New("webcdp: 连接已关闭")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	head := make([]byte, 0, 14)
	head = append(head, 0x80|opcode)
	n := len(payload)
	switch {
	case n < 126:
		head = append(head, 0x80|byte(n))
	case n < 65536:
		head = append(head, 0x80|126, 0, 0)
		binary.BigEndian.PutUint16(head[len(head)-2:], uint16(n))
	default:
		head = append(head, 0x80|127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(head[len(head)-8:], uint64(n))
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	head = append(head, mask[:]...)
	buf := make([]byte, len(head)+n)
	copy(buf, head)
	for i := 0; i < n; i++ {
		buf[len(head)+i] = payload[i] ^ mask[i&3]
	}
	_, err := c.conn.Write(buf)
	return err
}

// ReadText 读取下一条文本消息（自动应答 ping，分片自动重组）。
func (c *wsConn) ReadText() (string, error) {
	if c == nil || c.rd == nil {
		return "", errors.New("webcdp: 连接已关闭")
	}
	var payload []byte
	for {
		var hdr [2]byte
		if _, err := io.ReadFull(c.rd, hdr[:]); err != nil {
			return "", err
		}
		fin := hdr[0]&0x80 != 0
		opcode := hdr[0] & 0x0f
		masked := hdr[1]&0x80 != 0
		ln := int64(hdr[1] & 0x7f)
		switch ln {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(c.rd, ext[:]); err != nil {
				return "", err
			}
			ln = int64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(c.rd, ext[:]); err != nil {
				return "", err
			}
			ln = int64(binary.BigEndian.Uint64(ext[:]))
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(c.rd, mask[:]); err != nil {
				return "", err
			}
		}
		body := make([]byte, ln)
		if ln > 0 {
			if _, err := io.ReadFull(c.rd, body); err != nil {
				return "", err
			}
			if masked {
				for i := range body {
					body[i] ^= mask[i&3]
				}
			}
		}
		switch opcode {
		case 0x1, 0x0: // 文本 / 续帧
			payload = append(payload, body...)
			if fin {
				return string(payload), nil
			}
		case 0x8: // close
			return "", io.EOF
		case 0x9: // ping → pong
			_ = c.writeFrame(0xA, body)
		case 0xA: // pong：忽略
		default:
			if fin {
				return string(payload), nil
			}
		}
	}
}
