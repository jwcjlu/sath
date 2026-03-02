package tool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sath/executor"
	"github.com/sath/obs"
)

// ExecuteReadConfig 构造 execute_read 工具所需依赖。
type ExecuteReadConfig struct {
	Exec                executor.Executor
	DefaultDatasourceID string
	// DefaultTimeoutSec 默认超时时间（秒），0 表示无限制。
	DefaultTimeoutSec int
	// DefaultMaxRows 默认最大行数，0 表示无限制。
	DefaultMaxRows int
}

// RegisterExecuteReadTool 向注册表中注册 execute_read 工具。
// opts 可选：若 opts 中 Description 非空则覆盖默认描述（用于按数据源类型差异化表述）。
func RegisterExecuteReadTool(r *Registry, cfg *ExecuteReadConfig, opts ...*RegisterToolOptions) error {
	desc := "Execute a read-only DSL (e.g. SQL SELECT) on the current datasource and return rows."
	if len(opts) > 0 && opts[0] != nil && opts[0].Description != "" {
		desc = opts[0].Description
	}
	return r.Register(Tool{
		Name:        "execute_read",
		Description: desc,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dsl": map[string]any{
					"type":        "string",
					"description": "Read-only DSL to execute (e.g. SQL SELECT). If omitted, `query` will be used.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Alias of `dsl`.",
				},
				"datasource_id": map[string]any{
					"type":        "string",
					"description": "Datasource ID; if omitted, the session default is used.",
				},
				"timeout_sec": map[string]any{
					"type":        "integer",
					"description": "Execution timeout in seconds; non-negative.",
				},
				"max_rows": map[string]any{
					"type":        "integer",
					"description": "Maximum number of rows to return; non-negative.",
				},
				"index": map[string]any{
					"type":        "string",
					"description": "Optional. Elasticsearch: target index, comma-separated indices, or index pattern (e.g. vm-manager-*). Omit to search all indices.",
				},
			},
			"required": []string{},
		},
		Execute: buildExecuteReadExecute(cfg),
	})
}

func buildExecuteReadExecute(cfg *ExecuteReadConfig) ExecuteFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		start := time.Now()
		status := "ok"
		defer func() {
			obs.ObserveDataQueryTool("execute_read", status, time.Since(start))
		}()

		if cfg == nil || cfg.Exec == nil {
			status = "error"
			return nil, errors.New("execute_read: not configured (missing executor)")
		}

		datasourceID := cfg.DefaultDatasourceID
		if v := params["datasource_id"]; v != nil {
			if s, ok := v.(string); ok && s != "" {
				datasourceID = s
			}
		}
		if datasourceID == "" {
			status = "error"
			return nil, errors.New("execute_read: datasource_id is required (or set default)")
		}

		// dsl 或 query 至少需要一个
		var dsl string
		if v, ok := params["dsl"]; ok {
			if s, ok := v.(string); ok {
				dsl = s
			}
		}
		if dsl == "" {
			if v, ok := params["query"]; ok {
				if s, ok := v.(string); ok {
					dsl = s
				}
			}
		}
		if dsl == "" {
			status = "error"
			return nil, errors.New("execute_read: dsl (or query) is required and must be a string")
		}

		timeout := cfg.DefaultTimeoutSec
		if v, ok := params["timeout_sec"]; ok {
			if n, ok := toIntNonNegative(v); ok {
				timeout = n
			} else {
				status = "error"
				return nil, errors.New("execute_read: timeout_sec must be a non-negative number")
			}
		}

		maxRows := cfg.DefaultMaxRows
		if v, ok := params["max_rows"]; ok {
			if n, ok := toIntNonNegative(v); ok {
				maxRows = n
			} else {
				status = "error"
				return nil, errors.New("execute_read: max_rows must be a non-negative number")
			}
		}

		opts := executor.ExecuteOptions{
			Timeout:  timeout,
			MaxRows:  maxRows,
			ReadOnly: true,
			Params:   params,
		}

		res, err := cfg.Exec.Execute(ctx, datasourceID, dsl, opts)
		if err != nil {
			status = "error"
			if executor.IsSchemaRelated(err) {
				return nil, fmt.Errorf("execute_read: %w; 请先对该表/索引调用 describe_table 获取正确结构后再重试 execute_read。", err)
			}
			return nil, fmt.Errorf("execute_read: %w", err)
		}
		return res, nil
	}
}

// toIntNonNegative 尝试从多种数字类型解析出非负 int。
func toIntNonNegative(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		if x < 0 {
			return 0, false
		}
		return x, true
	case int32:
		if x < 0 {
			return 0, false
		}
		return int(x), true
	case int64:
		if x < 0 {
			return 0, false
		}
		return int(x), true
	case float32:
		if x < 0 {
			return 0, false
		}
		return int(x), true
	case float64:
		if x < 0 {
			return 0, false
		}
		return int(x), true
	default:
		return 0, false
	}
}
