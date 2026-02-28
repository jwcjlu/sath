package middleware

import (
	"context"
	"time"

	"github.com/sath/agent"
	"github.com/sath/obs"
)

// MetricsMiddleware 使用 obs 中的 Prometheus 指标记录请求次数与耗时。
// agent 名称通过 Metadata["agent_name"] 传递，默认为 "default"。
func MetricsMiddleware(next Handler) Handler {
	return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		elapsed := time.Since(start)

		agentName := "default"
		if req != nil && req.Metadata != nil {
			if v, ok := req.Metadata["agent_name"].(string); ok && v != "" {
				agentName = v
			}
		}
		status := "ok"
		if err != nil {
			status = "error"
		}
		obs.ObserveAgentRequest(agentName, status, elapsed)
		if resp != nil && resp.Metadata != nil {
			if in, _ := resp.Metadata["token_input"].(int); in > 0 || resp.Metadata["token_output"] != nil {
				out, _ := resp.Metadata["token_output"].(int)
				obs.ObserveTokenUsage(agentName, in, out)
			}
		}
		return resp, err
	}
}
