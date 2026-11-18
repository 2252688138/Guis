package httpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"time"
)

// SOCKS5 命令与认证方法。
const (
	socks5Version  = 0x05
	socks5NoAuth   = 0x00
	socks5UserPass = 0x02
	socks5NoAccept = 0xFF
	socks5CmdConn  = 0x01
	socks5AtypIPv4 = 0x01
	socks5AtypName = 0x03
	socks5AtypIPv6 = 0x04
)

// socks5DialContext 返回一个通过 SOCKS5 代理拨号的 DialContext。
func socks5DialContext(proxy *url.URL, d *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	host := proxy.Host
	if !hasPort(host) {
		host = net.JoinHostPort(host, "1080")
	}
	var user, pass string
	if proxy.User != nil {
		user = proxy.User.Username()
		pass, _ = proxy.User.Password()
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" && network != "tcp4" && network != "tcp6" {
			return nil, fmt.Errorf("socks5 代理仅支持 TCP，收到 %q", network)
		}
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return nil, fmt.Errorf("连接 socks5 代理失败: %w", err)
		}
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
		if err := socks5Handshake(conn, user, pass, address); err != nil {
			conn.Close()
			return nil, err
		}
		_ = conn.SetDeadline(time.Time{})
		return conn, nil
	}
}

// hasPort 判断 host 是否已带端口。
func hasPort(host string) bool {
	_, port, err := net.SplitHostPort(host)
	return err == nil && port != ""
}

// socks5Handshake 完成认证与 CONNECT 握手。
func socks5Handshake(conn net.Conn, user, pass, address string) error {
	methods := []byte{socks5NoAuth}
	if user != "" || pass != "" {
		methods = append(methods, socks5UserPass)
	}
	req := append([]byte{socks5Version, byte(len(methods))}, methods...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5 发送握手失败: %w", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("socks5 读取握手响应失败: %w", err)
	}
	if reply[0] != socks5Version {
		return fmt.Errorf("socks5 版本不匹配: %d", reply[0])
	}
	switch reply[1] {
	case socks5NoAuth:
	case socks5UserPass:
		if err := socks5UserPassAuth(conn, user, pass); err != nil {
			return err
		}
	case socks5NoAccept:
		return errors.New("socks5 代理不接受任何认证方式")
	default:
		return fmt.Errorf("socks5 代理要求不支持的认证方式: %d", reply[1])
	}

	dst, err := socks5DestAddr(address)
	if err != nil {
		return err
	}
	msg := []byte{socks5Version, socks5CmdConn, 0x00}
	msg = append(msg, dst...)
	if _, err := conn.Write(msg); err != nil {
		return fmt.Errorf("socks5 发送 CONNECT 失败: %w", err)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("socks5 读取 CONNECT 响应失败: %w", err)
	}
	if head[0] != socks5Version {
		return fmt.Errorf("socks5 CONNECT 版本异常: %d", head[0])
	}
	if head[1] != 0x00 {
		return fmt.Errorf("socks5 CONNECT 被拒绝: %s", socks5ReplyText(head[1]))
	}
	switch head[3] {
	case socks5AtypIPv4:
		if _, err := io.ReadFull(conn, make([]byte, 4+2)); err != nil {
			return err
		}
	case socks5AtypIPv6:
		if _, err := io.ReadFull(conn, make([]byte, 16+2)); err != nil {
			return err
		}
	case socks5AtypName:
		lb := make([]byte, 1)
		if _, err := io.ReadFull(conn, lb); err != nil {
			return err
		}
		if _, err := io.ReadFull(conn, make([]byte, int(lb[0])+2)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("socks5 CONNECT 响应地址类型异常: %d", head[3])
	}
	return nil
}

// socks5UserPassAuth 执行用户名/密码子协商。
func socks5UserPassAuth(conn net.Conn, user, pass string) error {
	if len(user) > 255 || len(pass) > 255 {
		return errors.New("socks5 用户名或密码过长（限 255 字节）")
	}
	msg := []byte{0x01, byte(len(user))}
	msg = append(msg, user...)
	msg = append(msg, byte(len(pass)))
	msg = append(msg, pass...)
	if _, err := conn.Write(msg); err != nil {
		return fmt.Errorf("socks5 发送认证信息失败: %w", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("socks5 读取认证响应失败: %w", err)
	}
	if reply[1] != 0x00 {
		return errors.New("socks5 用户名或密码错误")
	}
	return nil
}

// socks5DestAddr 编码目标地址（域名/IPv4/IPv6 + 端口）。
func socks5DestAddr(address string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("socks5 目标地址非法: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("socks5 目标端口非法: %w", err)
	}
	buf := make([]byte, 0, 1+1+len(host)+2)
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			buf = append(buf, socks5AtypIPv4)
			buf = append(buf, v4...)
		} else {
			buf = append(buf, socks5AtypIPv6)
			buf = append(buf, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("socks5 目标域名过长")
		}
		buf = append(buf, socks5AtypName, byte(len(host)))
		buf = append(buf, host...)
	}
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, uint16(port))
	return append(buf, portBuf...), nil
}

// socks5ReplyText 把 CONNECT 错误码转为文本。
func socks5ReplyText(code byte) string {
	switch code {
	case 0x01:
		return "代理服务器错误"
	case 0x02:
		return "连接不被允许"
	case 0x03:
		return "网络不可达"
	case 0x04:
		return "主机不可达"
	case 0x05:
		return "连接被拒绝"
	case 0x06:
		return "TTL 过期"
	case 0x07:
		return "不支持的命令"
	case 0x08:
		return "不支持的地址类型"
	default:
		return fmt.Sprintf("未知错误码 %d", code)
	}
}
