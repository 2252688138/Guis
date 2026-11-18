package httpc

import (
	"context"
	"errors"
	"sync"
)

// CancelToken 类 axios 的取消令牌，可跨并发安全使用。
type CancelToken struct {
	mu     sync.Mutex
	done   chan struct{}
	once   sync.Once
	reason error
}

// NewCancelToken 创建取消令牌。
func NewCancelToken() *CancelToken {
	return &CancelToken{done: make(chan struct{})}
}

// Cancel 取消请求；reason 可选（默认 context.Canceled）。
func (t *CancelToken) Cancel(reason ...error) {
	if t == nil {
		return
	}
	t.once.Do(func() {
		t.mu.Lock()
		if len(reason) > 0 && reason[0] != nil {
			t.reason = reason[0]
		} else {
			t.reason = context.Canceled
		}
		t.mu.Unlock()
		close(t.done)
	})
}

// Canceled 是否已取消。
func (t *CancelToken) Canceled() bool {
	if t == nil {
		return false
	}
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// Err 返回取消原因（未取消时为 nil）。
func (t *CancelToken) Err() error {
	if t == nil || !t.Canceled() {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reason == nil {
		return context.Canceled
	}
	return t.reason
}

// Done 返回取消信号通道。
func (t *CancelToken) Done() <-chan struct{} {
	if t == nil {
		ch := make(chan struct{})
		return ch
	}
	return t.done
}

// Context 基于 parent 派生一个在令牌取消时结束的 context。
// 调用方必须在请求结束后调用返回的 CancelFunc 释放 goroutine。
func (t *CancelToken) Context(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if t == nil {
		return context.WithCancel(parent)
	}
	ctx, cancel := context.WithCancel(parent)
	if t.Canceled() {
		cancel()
		return ctx, cancel
	}
	go func() {
		select {
		case <-t.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// IsCanceled 判断取消原因（兼容 errors.Is）。
func IsCanceled(err error) bool { return errors.Is(err, context.Canceled) }
