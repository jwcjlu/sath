package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/sath/agent"
)

// CacheEntry 表示缓存中的一条记录。
type CacheEntry struct {
	Response *agent.Response
	ExpireAt time.Time
}

// CacheStore 是一个简单的内存缓存实现，适合小规模/开发环境使用。
type CacheStore struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry
	ttl     time.Duration
}

func NewCacheStore(ttl time.Duration) *CacheStore {
	return &CacheStore{
		entries: make(map[string]CacheEntry),
		ttl:     ttl,
	}
}

func (c *CacheStore) Get(key string) (*agent.Response, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !e.ExpireAt.IsZero() && time.Now().After(e.ExpireAt) {
		return nil, false
	}
	return e.Response, true
}

func (c *CacheStore) Set(key string, resp *agent.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := CacheEntry{
		Response: resp,
	}
	if c.ttl > 0 {
		e.ExpireAt = time.Now().Add(c.ttl)
	}
	c.entries[key] = e
}

// CacheMiddleware 根据请求内容缓存 Agent 响应。
// 典型用法：在模型调用前面包一层，减少相同输入的重复调用。
func CacheMiddleware(store *CacheStore) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			if req == nil || store == nil {
				return next(ctx, req)
			}
			key := cacheKeyForRequest(req)
			if cached, ok := store.Get(key); ok {
				return cached, nil
			}
			resp, err := next(ctx, req)
			if err == nil && resp != nil {
				store.Set(key, resp)
			}
			return resp, err
		}
	}
}

func cacheKeyForRequest(req *agent.Request) string {
	h := sha256.New()
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte{':'})
		h.Write([]byte(m.Content))
		h.Write([]byte{';'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
