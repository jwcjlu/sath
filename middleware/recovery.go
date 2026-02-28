package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/sath/agent"
	"github.com/sath/errs"
)

// RecoveryMiddleware 捕获 panic，避免进程崩溃。
func RecoveryMiddleware(next Handler) Handler {
	return func(ctx context.Context, req *agent.Request) (resp *agent.Response, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf(`{"level":"error","msg":"agent_panic","panic":"%v"}`, r)
				// 将 panic 映射为统一的内部错误，避免调用方拿到 nil err 导致误判。
				err = fmt.Errorf("%w: %v", errs.ErrInternal, r)
			}
		}()
		return next(ctx, req)
	}
}
