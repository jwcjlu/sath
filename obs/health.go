package obs

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthCheckFunc 用于健康检查的单个检查函数；返回 nil 表示该组件正常。
type HealthCheckFunc func(ctx context.Context) error

// HealthResult 为 /health 端点的 JSON 响应结构。
type HealthResult struct {
	Status    string            `json:"status"`    // "ok" | "degraded" | "unhealthy"
	Checks    map[string]string `json:"checks"`    // 组件名 -> "ok" 或 "error: ..."
	Timestamp string            `json:"timestamp"` // RFC3339
}

// HealthHandler 返回一个 HTTP Handler，对每个已注册的检查执行一次（带超时），并返回聚合状态。
// checks 为 nil 或空时，返回 200 且 status "ok"、checks 为空 map。
func HealthHandler(checks map[string]HealthCheckFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result := HealthResult{
			Status:    "ok",
			Checks:    make(map[string]string),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}

		for name, fn := range checks {
			if fn == nil {
				result.Checks[name] = "ok"
				continue
			}
			if err := fn(ctx); err != nil {
				result.Checks[name] = "error: " + err.Error()
				result.Status = "unhealthy"
			} else {
				result.Checks[name] = "ok"
			}
		}

		if len(result.Checks) == 0 {
			result.Status = "ok"
		}

		w.Header().Set("Content-Type", "application/json")
		code := http.StatusOK
		if result.Status == "unhealthy" {
			code = http.StatusServiceUnavailable
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(result)
	})
}
