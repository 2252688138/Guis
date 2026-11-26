package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// maxLineBytes 单行 JSON 上限，超限视为协议错误。
const maxLineBytes = 1 << 20

// errConnClosed 连接已关闭。
var errConnClosed = errors.New("插件连接已关闭")

// notifyHandler 处理插件 → 宿主的通知（progress / log / on_*）。
type notifyHandler func(method string, params json.RawMessage)

// rpcConn NDJSON 上的 JSON-RPC 连接（每行一条 JSON）。
type rpcConn struct {
	w        io.Writer
	onNotify notifyHandler

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcResponse
	closed  bool
	err     error
	done    chan struct{}
}

func newRPCConn(r io.Reader, w io.Writer, onNotify notifyHandler) *rpcConn {
	c := &rpcConn{
		w:        w,
		onNotify: onNotify,
		pending:  map[int64]chan rpcResponse{},
		done:     make(chan struct{}),
	}
	go c.readLoop(r)
	return c
}

func (c *rpcConn) readLoop(r io.Reader) {
	defer func() {
		c.close(errConnClosed)
	}()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			c.close(fmt.Errorf("插件输出不是合法 JSON: %w", err))
			return
		}
		if resp.ID == 0 {
			// 通知：交给宿主处理，不影响待响应请求
			if resp.Method != "" && c.onNotify != nil {
				c.onNotify(resp.Method, resp.Params)
			}
			continue
		}
		c.mu.Lock()
		ch := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- resp:
			default:
			}
		}
	}
	if err := sc.Err(); err != nil {
		c.close(fmt.Errorf("读取插件输出失败: %w", err))
		return
	}
	c.close(errConnClosed)
}

// call 发一条请求并等待响应。
func (c *rpcConn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	if c.closed {
		err := c.err
		c.mu.Unlock()
		if err == nil {
			err = errConnClosed
		}
		return nil, err
	}
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	rawParams, err := json.Marshal(params)
	if err != nil {
		c.drop(id)
		return nil, err
	}
	line, err := json.Marshal(rpcRequest{ID: id, Method: method, Params: rawParams})
	if err != nil {
		c.drop(id)
		return nil, err
	}
	if _, err := c.w.Write(append(line, '\n')); err != nil {
		c.drop(id)
		return nil, fmt.Errorf("写入插件请求失败: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("插件错误 %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.drop(id)
		return nil, ctx.Err()
	case <-c.done:
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err == nil {
			err = errConnClosed
		}
		return nil, err
	}
}

// notify 发一条通知（不等待响应）。
func (c *rpcConn) notify(method string, params any) error {
	c.mu.Lock()
	if c.closed {
		err := c.err
		c.mu.Unlock()
		if err == nil {
			err = errConnClosed
		}
		return err
	}
	c.mu.Unlock()
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	line, err := json.Marshal(rpcRequest{Method: method, Params: rawParams})
	if err != nil {
		return err
	}
	_, err = c.w.Write(append(line, '\n'))
	return err
}

func (c *rpcConn) drop(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *rpcConn) close(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if err != nil {
		c.err = err
	}
	close(c.done)
	c.mu.Unlock()
}

// Err 关闭原因（未关闭返回 nil）。
func (c *rpcConn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close 主动关闭连接（不等对端）。
func (c *rpcConn) Close() { c.close(errConnClosed) }
