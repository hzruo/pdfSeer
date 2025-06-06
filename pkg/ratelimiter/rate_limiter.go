package ratelimiter

import (
	"context"
	"sync"
	"time"
)

// RateLimiter 频率限制器
type RateLimiter struct {
	interval time.Duration
	burst    int
	tokens   chan struct{}
	ticker   *time.Ticker
	done     chan struct{}
	mu       sync.Mutex
}

// NewRateLimiter 创建频率限制器
func NewRateLimiter(intervalSec float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		interval: time.Duration(intervalSec * float64(time.Second)),
		burst:    burst,
		tokens:   make(chan struct{}, burst),
		done:     make(chan struct{}),
	}

	// 初始化令牌桶
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}

	// 启动令牌补充协程
	rl.start()
	
	return rl
}

// start 启动令牌补充
func (rl *RateLimiter) start() {
	rl.ticker = time.NewTicker(rl.interval)
	
	go func() {
		defer rl.ticker.Stop()
		
		for {
			select {
			case <-rl.ticker.C:
				// 尝试添加令牌
				select {
				case rl.tokens <- struct{}{}:
					// 成功添加令牌
				default:
					// 令牌桶已满
				}
			case <-rl.done:
				return
			}
		}
	}()
}

// Wait 等待获取令牌
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.done:
		return context.Canceled
	}
}

// TryAcquire 尝试获取令牌（非阻塞）
func (rl *RateLimiter) TryAcquire() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// UpdateRate 更新频率限制
func (rl *RateLimiter) UpdateRate(intervalSec float64, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 停止当前ticker
	if rl.ticker != nil {
		rl.ticker.Stop()
	}

	// 更新参数
	rl.interval = time.Duration(intervalSec * float64(time.Second))
	rl.burst = burst

	// 重新创建令牌桶
	rl.tokens = make(chan struct{}, burst)
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}

	// 重新启动
	rl.start()
}

// Close 关闭频率限制器
func (rl *RateLimiter) Close() {
	close(rl.done)
	if rl.ticker != nil {
		rl.ticker.Stop()
	}
}

// GetStats 获取统计信息
func (rl *RateLimiter) GetStats() (available int, capacity int) {
	return len(rl.tokens), rl.burst
}
