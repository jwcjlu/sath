package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sath/agent"
	"github.com/sath/errs"
	"github.com/sath/model"
)

func dummyHandler(respText string, delay time.Duration) Handler {
	return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		return &agent.Response{Text: respText}, nil
	}
}

func TestCacheMiddleware_Basic(t *testing.T) {
	store := NewCacheStore(1 * time.Minute)
	calls := 0
	h := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		calls++
		return &agent.Response{Text: "ok"}, nil
	}
	handler := CacheMiddleware(store)(h)

	req := &agent.Request{}
	_, _ = handler(context.Background(), req)
	_, _ = handler(context.Background(), req)

	if calls != 1 {
		t.Fatalf("expected handler to be called once, got %d", calls)
	}
}

func TestRateLimitMiddleware_Global(t *testing.T) {
	limiter := NewRateLimiter(1, 0) // 只允许 1 次，且不自动恢复
	handler := RateLimitMiddleware(limiter, nil)(dummyHandler("ok", 0))

	req := &agent.Request{}
	_, err1 := handler(context.Background(), req)
	_, err2 := handler(context.Background(), req)

	if err1 != nil {
		t.Fatalf("first call should succeed, got %v", err1)
	}
	if !errors.Is(err2, errs.ErrRateLimited) {
		t.Fatalf("expected errs.ErrRateLimited on second call, got %v", err2)
	}
}

func TestContentSafetyMiddleware_BlockInput(t *testing.T) {
	filter := &SimpleBlocklistFilter{Blocked: []string{"badword"}}
	handler := ContentSafetyMiddleware(filter)(dummyHandler("ok", 0))

	req := &agent.Request{
		Messages: []model.Message{
			{Role: "user", Content: "this contains badword"},
		},
	}
	_, err := handler(context.Background(), req)
	if !errors.Is(err, errs.ErrContentBlocked) {
		t.Fatalf("expected errs.ErrContentBlocked, got %v", err)
	}
}
