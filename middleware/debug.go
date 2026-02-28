package middleware

import (
	"context"
	"log"
	runtimeDebug "runtime/debug"

	"github.com/sath/agent"
)

// DebugMiddleware 在 enabled 为 true 时输出脱敏的请求/响应摘要，并在发生错误时输出调用栈。
// 不记录完整消息内容，仅记录条数、长度等，避免敏感信息泄露。
func DebugMiddleware(enabled bool) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			if !enabled {
				return next(ctx, req)
			}
			msgCount := 0
			totalLen := 0
			if req != nil {
				msgCount = len(req.Messages)
				for _, m := range req.Messages {
					totalLen += len(m.Content)
					for _, p := range m.Parts {
						totalLen += len(p.Text) + len(p.URL)
					}
				}
			}
			reqID := ""
			if req != nil {
				reqID = req.RequestID
			}
			log.Printf(`{"level":"debug","msg":"request","request_id":%q,"message_count":%d,"content_length":%d}`,
				reqID, msgCount, totalLen)

			resp, err := next(ctx, req)
			if err != nil {
				log.Printf(`{"level":"debug","msg":"error","request_id":%q,"error":%q,"stack":%q}`,
					reqID, err.Error(), string(runtimeDebug.Stack()))
				return resp, err
			}
			replyLen := 0
			if resp != nil {
				replyLen = len(resp.Text)
			}
			log.Printf(`{"level":"debug","msg":"response","request_id":%q,"reply_length":%d}`,
				reqID, replyLen)
			return resp, err
		}
	}
}
