package middleware

import (
	"context"

	"github.com/sath/agent"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware 在每次 Agent 调用时创建一个新的 Span，便于调用链追踪。
// 需要在应用启动时先调用 obs.InitTracer 配置全局 TracerProvider。
func TracingMiddleware(next Handler) Handler {
	tracer := otel.Tracer("github.com/sath/agent")
	return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		ctx, span := tracer.Start(ctx, "Agent.Run", trace.WithSpanKind(trace.SpanKindServer))
		if req != nil && req.Metadata != nil {
			if uid, ok := req.Metadata["user_id"].(string); ok && uid != "" {
				span.SetAttributes(attribute.String("user.id", uid))
			}
		}
		defer span.End()

		resp, err := next(ctx, req)
		if err != nil {
			span.RecordError(err)
		}
		return resp, err
	}
}
