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
	dataqueryIntentDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dataquery_intent_duration_seconds",
			Help:    "Data query intent recognition latency.",
			Buckets: prometheus.DefBuckets,
		},
	)
	dataqueryDSLDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dataquery_dsl_duration_seconds",
			Help:    "Data query DSL generation latency.",
			Buckets: prometheus.DefBuckets,
		},
	)
	dataqueryExecDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dataquery_exec_duration_seconds",
			Help:    "Data query execution latency.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

var reg = prometheus.NewRegistry()

func init() {
	reg.MustRegister(requests, requestDuration, tokensTotal, dataqueryIntentDuration, dataqueryDSLDuration, dataqueryExecDuration)
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

// ObserveDataQueryIntent 记录数据对话意图识别耗时。
func ObserveDataQueryIntent(d time.Duration) {
	dataqueryIntentDuration.Observe(d.Seconds())
}

// ObserveDataQueryDSL 记录数据对话 DSL 生成耗时。
func ObserveDataQueryDSL(d time.Duration) {
	dataqueryDSLDuration.Observe(d.Seconds())
}

// ObserveDataQueryExec 记录数据对话执行耗时；status 建议 "ok" 或 "error"。
func ObserveDataQueryExec(status string, d time.Duration) {
	if status == "" {
		status = "ok"
	}
	dataqueryExecDuration.WithLabelValues(status).Observe(d.Seconds())
}
