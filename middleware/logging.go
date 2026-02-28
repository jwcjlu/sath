package middleware

import (
	"context"
	"log"
	"time"

	"github.com/sath/agent"
)

// LoggingMiddleware 输出基础结构化日志到标准输出。
func LoggingMiddleware(next Handler) Handler {
	return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		elapsed := time.Since(start)

		log.Printf(`{"level":"info","msg":"agent_request","elapsed_ms":%d,"error":"%v"}`, elapsed.Milliseconds(), err)
		return resp, err
	}
}
