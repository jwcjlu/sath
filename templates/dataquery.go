package templates

import (
	"context"

	"github.com/sath/agent"
	"github.com/sath/middleware"
)

// NewDataQueryHandler 构建数据对话 HTTP 处理器，调用 dataAgent.Run。
func NewDataQueryHandler(dataAgent agent.Agent, mws ...middleware.Middleware) middleware.Handler {
	final := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		return dataAgent.Run(ctx, req)
	}
	return middleware.Chain(final, mws...)
}
