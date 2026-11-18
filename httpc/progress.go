package httpc

import (
	"io"
	"sync"
	"time"
)

// Progress 是一次传输的进度快照。
type Progress struct {
	// Phase "upload" 或 "download"。
	Phase string
	// Loaded 已传输字节数。
	Loaded int64
	// Total 总字节数，未知时为 -1。
	Total int64
	// Percent 完成百分比（0-100），Total 未知时为 0。
	Percent float64
	// Rate 瞬时速率（字节/秒）。
	Rate int64
	// Elapsed 从开始到当前的耗时。
	Elapsed time.Duration
	// Done 是否已完成（最后一条回调）。
	Done bool
}

// ProgressFunc 进度回调；会在传输线程调用，需尽快返回。
type ProgressFunc func(p Progress)

// 进度回调的触发阀值。
const (
	progressMinBytes = 64 << 10
	progressMinSteps = 1
)

// progressReader 在读取过程中上报进度（带节流）。
type progressReader struct {
	r        io.Reader
	fn       ProgressFunc
	phase    string
	total    int64
	loaded   int64
	start    time.Time
	last     time.Time
	lastPos  int64
	interval time.Duration
	closer   io.Closer
	mu       sync.Mutex
	done     bool
}

// newProgressReader 包装读取器。total <= 0 表示总长未知。
func newProgressReader(r io.Reader, total int64, fn ProgressFunc, phase string, interval time.Duration) io.Reader {
	if fn == nil {
		return r
	}
	if interval <= 0 {
		interval = defaultProgressInterval
	}
	pr := &progressReader{
		r:        r,
		fn:       fn,
		phase:    phase,
		total:    total,
		start:    time.Now(),
		last:     time.Now(),
		interval: interval,
	}
	if c, ok := r.(io.Closer); ok {
		pr.closer = c
	}
	return pr
}

// Read 实现 io.Reader。
func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.mu.Lock()
		p.loaded += int64(n)
		p.mu.Unlock()
		p.emit(false)
	}
	if err == io.EOF {
		p.emit(true)
	}
	return n, err
}

// Close 透传底层 Close。
func (p *progressReader) Close() error {
	if p.closer != nil {
		return p.closer.Close()
	}
	return nil
}

// emit 按节流阀值回调。
func (p *progressReader) emit(final bool) {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	now := time.Now()
	loaded := p.loaded
	since := now.Sub(p.last)
	stepDelta := loaded - p.lastPos
	sizeDelta := stepDelta >= progressMinBytes
	if !final && since < p.interval && !sizeDelta {
		p.mu.Unlock()
		return
	}
	if final {
		p.done = true
	}
	p.mu.Unlock()

	elapsed := now.Sub(p.start)
	pct := 0.0
	if p.total > 0 {
		pct = float64(loaded) / float64(p.total) * 100
		if pct > 100 {
			pct = 100
		}
	}
	var rate int64
	if step := now.Sub(p.last); step > 0 {
		rate = int64(float64(stepDelta) / step.Seconds())
	}
	p.fn(Progress{
		Phase:   p.phase,
		Loaded:  loaded,
		Total:   p.total,
		Percent: pct,
		Rate:    rate,
		Elapsed: elapsed,
		Done:    final,
	})

	p.mu.Lock()
	p.last = now
	p.lastPos = loaded
	p.mu.Unlock()
}
