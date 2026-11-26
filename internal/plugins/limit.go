package plugins

import (
	"fmt"
	"sync"
	"time"
)

// limiter 每插件的并发与频率上限。
type limiter struct {
	mu          sync.Mutex
	maxRunning  int
	maxPerMin   int
	perPlugin   map[string]*pluginQuota
	now         func() time.Time
	windowLimit time.Duration
}

type pluginQuota struct {
	running     int
	windowStart time.Time
	count       int
}

func newLimiter(maxRunning, maxPerMin int) *limiter {
	if maxRunning <= 0 {
		maxRunning = DefaultMaxConcurrent
	}
	if maxPerMin <= 0 {
		maxPerMin = DefaultMaxPerMinute
	}
	return &limiter{
		maxRunning:  maxRunning,
		maxPerMin:   maxPerMin,
		perPlugin:   map[string]*pluginQuota{},
		now:         time.Now,
		windowLimit: time.Minute,
	}
}

// acquire 占用一个调用名额，返回释放函数；超限返回可读错误。
func (l *limiter) acquire(pluginID string) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	q := l.perPlugin[pluginID]
	if q == nil {
		q = &pluginQuota{}
		l.perPlugin[pluginID] = q
	}
	if q.running >= l.maxRunning {
		return nil, fmt.Errorf("插件 %s 并发调用已达上限（%d）", pluginID, l.maxRunning)
	}
	now := l.now()
	if q.windowStart.IsZero() || now.Sub(q.windowStart) >= l.windowLimit {
		q.windowStart = now
		q.count = 0
	}
	if q.count >= l.maxPerMin {
		return nil, fmt.Errorf("插件 %s 每分钟调用已达上限（%d 次）", pluginID, l.maxPerMin)
	}
	q.count++
	q.running++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			q.running--
			if q.running < 0 {
				q.running = 0
			}
			l.mu.Unlock()
		})
	}, nil
}
