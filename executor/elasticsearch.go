package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/sath/datasource"
)

// ESExecutor 基于 datasource.Registry 的 Elasticsearch 执行器，支持只读 Search、超时与 MaxRows。
type ESExecutor struct {
	Registry *datasource.Registry
}

// NewESExecutor 创建依赖给定 Registry 的 ES 执行器。
func NewESExecutor(reg *datasource.Registry) *ESExecutor {
	return &ESExecutor{Registry: reg}
}

// Execute 实现 Executor。仅支持只读：dsl 为 Search 请求体 JSON；写操作（index/update/delete 等）在 ReadOnly 时拒绝。
func (e *ESExecutor) Execute(ctx context.Context, datasourceID string, dsl string, opts ExecuteOptions) (*Result, error) {
	ds, err := e.Registry.Get(datasourceID)
	if err != nil {
		return nil, err
	}
	ep, ok := ds.(datasource.ESClientProvider)
	if !ok {
		return nil, ErrUnsupportedDataSource
	}
	client := ep.ESClient()
	log.Printf("elasticsearch dsl %s", dsl)

	if opts.ReadOnly && isESWriteDSL(dsl) {
		return nil, ErrReadOnlyViolation
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	indexParam := ""
	if opts.Params != nil {
		if v, ok := opts.Params["index"].(string); ok && v != "" {
			indexParam = strings.TrimSpace(v)
		}
	}
	return e.execSearch(ctx, client, dsl, opts.MaxRows, indexParam)
}

// isESWriteDSL 根据请求体粗略判断是否为写操作（index/update/delete/bulk 等）。
func isESWriteDSL(dsl string) bool {
	s := strings.TrimSpace(strings.ToLower(dsl))
	if s == "" {
		return false
	}
	// 简单检查：包含 "index"/"update"/"delete"/"bulk" 等且非纯 query 结构
	if strings.Contains(s, `"index"`) || strings.Contains(s, `"update"`) ||
		strings.Contains(s, `"delete"`) || strings.Contains(s, `"bulk"`) {
		return true
	}
	return false
}

func (e *ESExecutor) execSearch(ctx context.Context, client *elasticsearch.Client, body string, maxRows int, index string) (*Result, error) {
	opts := []func(*esapi.SearchRequest){
		client.Search.WithContext(ctx),
		client.Search.WithBody(strings.NewReader(body)),
	}
	if index != "" {
		var indices []string
		for _, s := range strings.Split(index, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				indices = append(indices, s)
			}
		}
		if len(indices) > 0 {
			opts = append(opts, client.Search.WithIndex(indices...))
		}
	}
	res, err := client.Search(opts...)
	if err != nil {
		return nil, wrapESMaybeSchemaRelated(err, "executor: search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		errMsg := res.String()
		err := fmt.Errorf("executor: search: %s", errMsg)
		if isESSchemaRelated(errMsg) {
			return nil, &SchemaRelatedError{Err: err}
		}
		return nil, err
	}

	var out struct {
		Hits struct {
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("executor: decode search response: %w", err)
	}

	hits := out.Hits.Hits
	if maxRows > 0 && len(hits) > maxRows {
		hits = hits[:maxRows]
	}

	// 从首行收集列名（若为空则无列）
	var columns []string
	var rows [][]any
	if len(hits) > 0 {
		seen := make(map[string]struct{})
		for k := range hits[0].Source {
			seen[k] = struct{}{}
		}
		for k := range seen {
			columns = append(columns, k)
		}
		// 简单顺序：按 columns 顺序取每行
		for _, h := range hits {
			row := make([]any, len(columns))
			for i, col := range columns {
				row[i] = h.Source[col]
			}
			rows = append(rows, row)
		}
	}
	return &Result{Columns: columns, Rows: rows}, nil
}

func isESSchemaRelated(errMsg string) bool {
	return strings.Contains(errMsg, "No mapping found") ||
		strings.Contains(errMsg, "unknown field") ||
		strings.Contains(errMsg, "field_unknown") ||
		strings.Contains(errMsg, "strict_dynamic_mapping")
}

func wrapESMaybeSchemaRelated(err error, format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	if err != nil && isESSchemaRelated(err.Error()) {
		return &SchemaRelatedError{Err: wrapped}
	}
	return wrapped
}
