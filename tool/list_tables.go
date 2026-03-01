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

// ListTablesConfig 用于构造 list_tables 工具的依赖（Store、Registry、默认数据源 ID）。
type ListTablesConfig struct {
	Store               *metadata.InMemoryStore
	Registry            *datasource.Registry
	DefaultDatasourceID string
}

// RegisterListTablesTool 向 r 注册 list_tables 工具。cfg 可为 nil，此时 Execute 会返回“未配置”错误。
func RegisterListTablesTool(r *Registry, cfg *ListTablesConfig) error {
	return r.Register(Tool{
		Name:        "list_tables",
		Description: "List tables (or collections) in the current datasource. Returns table names and optional comments.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"datasource_id": map[string]any{
					"type":        "string",
					"description": "Datasource ID; if omitted, the session default is used",
				},
			},
			"required": []string{},
		},
		Execute: buildListTablesExecute(cfg),
	})
}

func buildListTablesExecute(cfg *ListTablesConfig) ExecuteFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		start := time.Now()
		status := "ok"
		defer func() {
			obs.ObserveDataQueryTool("list_tables", status, time.Since(start))
		}()

		if cfg == nil || cfg.Store == nil {
			status = "error"
			return nil, errors.New("list_tables: not configured (missing store)")
		}
		datasourceID := cfg.DefaultDatasourceID
		if p := params["datasource_id"]; p != nil {
			if s, ok := p.(string); ok && s != "" {
				datasourceID = s
			}
		}
		if datasourceID == "" {
			status = "error"
			return nil, errors.New("list_tables: datasource_id is required (or set default)")
		}

		schema, err := cfg.Store.GetSchema(ctx)
		if err != nil {
			status = "error"
			return nil, fmt.Errorf("list_tables: get schema: %w", err)
		}
		if schema == nil && cfg.Registry != nil {
			_, err = metadata.RefreshFromRegistry(ctx, cfg.Registry, cfg.Store, datasourceID)
			if err != nil {
				status = "error"
				return nil, fmt.Errorf("list_tables: refresh schema: %w", err)
			}
			schema, err = cfg.Store.GetSchema(ctx)
			if err != nil {
				status = "error"
				return nil, fmt.Errorf("list_tables: get schema after refresh: %w", err)
			}
		}
		if schema == nil {
			status = "error"
			return nil, errors.New("list_tables: no schema available")
		}

		out := make([]map[string]string, 0, len(schema.Tables))
		for _, t := range schema.Tables {
			row := map[string]string{"name": t.Name}
			if t.Comment != "" {
				row["comment"] = t.Comment
			}
			out = append(out, row)
		}
		return out, nil
	}
}

// ListTablesResult 供调用方做类型断言的返回结构（与 JSON 序列化一致）。
func ListTablesResult(raw any) ([]map[string]string, bool) {
	if raw == nil {
		return nil, false
	}
	// Execute 返回 []map[string]string
	slice, ok := raw.([]map[string]string)
	if ok {
		return slice, true
	}
	// 若从 JSON 反序列化得到 []interface{}
	if list, ok := raw.([]any); ok && len(list) > 0 {
		out := make([]map[string]string, 0, len(list))
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			row := make(map[string]string)
			for k, v := range m {
				if s, ok := v.(string); ok {
					row[k] = s
				}
			}
			out = append(out, row)
		}
		return out, true
	}
	return nil, false
}
