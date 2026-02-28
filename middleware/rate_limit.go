package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/sath/agent"
	"github.com/sath/errs"
)

// 简单的基于内存的令牌桶限流实现。

type tokenBucket struct {
	capacity int
	tokens   float64
	refill   float64 // tokens per second
	last     time.Time
}

func newTokenBucket(capacity int, refillPerSecond float64) *tokenBucket {
	return &tokenBucket{
		capacity: capacity,
		tokens:   float64(capacity),
		refill:   refillPerSecond,
		last:     time.Now(),
	}
}

func (b *tokenBucket) allow(amount float64) bool {
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.refill
	if b.tokens > float64(b.capacity) {
		b.tokens = float64(b.capacity)
	}
	b.last = now
	if b.tokens >= amount {
		b.tokens -= amount
		return true
	}
	return false
}

// RateLimiter 支持按 key（如 userID/IP）限流。
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	// 每个 key 的桶容量与填充速率
	capacity        int
	refillPerSecond float64
}

func NewRateLimiter(capacity int, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		buckets:         make(map[string]*tokenBucket),
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
	}
}

func (r *RateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[key]
	if !ok {
		b = newTokenBucket(r.capacity, r.refillPerSecond)
		r.buckets[key] = b
	}
	return b.allow(1)
}

// KeyFunc 从请求中提取限流 key（例如 Metadata 中的 user_id 或 IP）。
type KeyFunc func(*agent.Request) string

// RateLimitMiddleware 创建一个限流中间件。
func RateLimitMiddleware(limiter *RateLimiter, keyFn KeyFunc) Middleware {
	if keyFn == nil {
		keyFn = func(_ *agent.Request) string { return "global" }
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			if limiter == nil {
				return next(ctx, req)
			}
			key := keyFn(req)
			if !limiter.allow(key) {
				return nil, errs.ErrRateLimited
			}
			return next(ctx, req)
		}
	}
}
