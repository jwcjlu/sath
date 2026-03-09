package tool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sath/datasource"
	"github.com/sath/metadata"
	"github.com/sath/obs"
)

// DescribeTableConfig 用于构造 describe_table 工具的依赖。
type DescribeTableConfig struct {
	Store               *metadata.InMemoryStore
	Registry            *datasource.Registry
	DefaultDatasourceID string
}

// RegisterDescribeTableTool 向 r 注册 describe_table 工具。
// opts 可选：若 opts 中 Description 非空则覆盖默认描述（用于按数据源类型差异化表述）。
func RegisterDescribeTableTool(r *Registry, cfg *DescribeTableConfig, opts ...*RegisterToolOptions) error {
	desc := "Describe table structure in the current datasource. Returns columns with type and nullability."
	if len(opts) > 0 && opts[0] != nil && opts[0].Description != "" {
		desc = opts[0].Description
	}
	return r.Register(Tool{
		Name:        "describe_table",
		Description: desc,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_name": map[string]any{
					"type":        "string",
					"description": "Table name to describe",
				},
				"datasource_id": map[string]any{
					"type":        "string",
					"description": "Datasource ID; if omitted, the session default is used",
				},
			},
			"required": []string{"table_name"},
		},
		Execute: buildDescribeTableExecute(cfg),
	})
}

func buildDescribeTableExecute(cfg *DescribeTableConfig) ExecuteFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		start := time.Now()
		status := "ok"
		defer func() {
			obs.ObserveDataQueryTool("describe_table", status, time.Since(start))
		}()

		if cfg == nil || cfg.Store == nil {
			status = "error"
			return nil, errors.New("describe_table: not configured (missing store)")
		}

		datasourceID := cfg.DefaultDatasourceID
		if p := params["datasource_id"]; p != nil {
			if s, ok := p.(string); ok && s != "" {
				datasourceID = s
			}
		}
		if datasourceID == "" {
			status = "error"
			return nil, errors.New("describe_table: datasource_id is required (or set default)")
		}

		rawName, ok := params["table_name"]
		if !ok {
			status = "error"
			return nil, errors.New("describe_table: table_name is required")
		}
		tableName, ok := rawName.(string)
		if !ok || tableName == "" {
			status = "error"
			return nil, errors.New("describe_table: table_name must be a non-empty string")
		}

		tbl, err := cfg.Store.GetTable(ctx, tableName)
		if err != nil {
			status = "error"
			return nil, fmt.Errorf("describe_table: get table: %w", err)
		}
		if tbl == nil && cfg.Registry != nil {
			// 尝试从数据源刷新一次元数据
			if _, err := metadata.RefreshFromRegistry(ctx, cfg.Registry, cfg.Store, datasourceID); err != nil {
				status = "error"
				return nil, fmt.Errorf("describe_table: refresh schema: %w", err)
			}
			tbl, err = cfg.Store.GetTable(ctx, tableName)
			if err != nil {
				status = "error"
				return nil, fmt.Errorf("describe_table: get table after refresh: %w", err)
			}
		}
		if tbl == nil {
			status = "error"
			return nil, fmt.Errorf("describe_table: table not found: %s", tableName)
		}

		// 直接返回 metadata.Table，具备良好的 JSON 标签，便于模型消费。
		return *tbl, nil
	}
}
