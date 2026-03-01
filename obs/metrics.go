package obs

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_requests_total",
			Help: "Total number of agent requests.",
		},
		[]string{"agent", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_request_duration_seconds",
			Help:    "Agent request latency distributions.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"agent"},
	)
	tokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_tokens_total",
			Help: "Total token usage (input or output).",
		},
		[]string{"agent", "type"}, // type: "input" | "output"
	)

	// dataqueryToolCalls 统计数据查询相关工具被调用的次数及成功/失败情况。
	dataqueryToolCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dataquery_tool_calls_total",
			Help: "Total number of data query tool calls.",
		},
		[]string{"tool", "status"},
	)

	// dataqueryStepDuration 统计数据查询链路中关键步骤的耗时（如 react_run）。
	dataqueryStepDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dataquery_step_duration_seconds",
			Help:    "Duration of data query steps.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"step"},
	)
)

var reg = prometheus.NewRegistry()

func init() {
	reg.MustRegister(requests, requestDuration, tokensTotal, dataqueryToolCalls, dataqueryStepDuration)
}

// MetricsHandler 返回 Prometheus HTTP handler，可挂载到 /metrics。
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

// ObserveAgentRequest 记录一次 Agent 请求的指标。
// agentName 由调用方指定；status 建议为 "ok" 或 "error"。
func ObserveAgentRequest(agentName, status string, d time.Duration) {
	if agentName == "" {
		agentName = "default"
	}
	if status == "" {
		status = "ok"
	}
	requests.WithLabelValues(agentName, status).Inc()
	requestDuration.WithLabelValues(agentName).Observe(d.Seconds())
}

// ObserveTokenUsage 上报一次请求的 token 消耗（B.6.1）。agentName 为空时用 "default"。
func ObserveTokenUsage(agentName string, inputTokens, outputTokens int) {
	if agentName == "" {
		agentName = "default"
	}
	if inputTokens > 0 {
		tokensTotal.WithLabelValues(agentName, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		tokensTotal.WithLabelValues(agentName, "output").Add(float64(outputTokens))
	}
}

// ObserveDataQueryTool 记录一次数据查询工具调用。
// toolName 如 "list_tables"、"describe_table"、"execute_read"、"execute_write"；status 一般为 "ok" 或 "error"。
func ObserveDataQueryTool(toolName, status string, d time.Duration) {
	if toolName == "" {
		toolName = "unknown"
	}
	if status == "" {
		status = "ok"
	}
	dataqueryToolCalls.WithLabelValues(toolName, status).Inc()
	dataqueryStepDuration.WithLabelValues("tool_" + toolName).Observe(d.Seconds())
}

// ObserveDataQueryStep 记录数据查询链路中任意自定义步骤的耗时。
// 例如 step="react_run"、"confirm_write" 等。
func ObserveDataQueryStep(step string, d time.Duration) {
	if step == "" {
		step = "unknown"
	}
	dataqueryStepDuration.WithLabelValues(step).Observe(d.Seconds())
}
