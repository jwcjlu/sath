package obs

import (
	"testing"
	"time"
)

func TestObserveDataQueryMetrics_NoPanic(t *testing.T) {
	// 调用数据查询相关指标函数，确保不会 panic，且能被 /metrics 暴露。
	ObserveDataQueryTool("list_tables", "ok", 10*time.Millisecond)
	ObserveDataQueryTool("execute_read", "error", 5*time.Millisecond)
	ObserveDataQueryStep("react_run", 20*time.Millisecond)
}
