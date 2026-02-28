package executor

import (
	"context"
	"time"
)

// ExecuteOptions 执行选项：超时、行数限制、只读与可选参数。
type ExecuteOptions struct {
	Timeout  time.Duration
	MaxRows  int
	ReadOnly bool
	Params   []any // 参数化占位符参数，与 dsl 中的 ? 对应
}

// Result 执行结果。
type Result struct {
	Columns      []string   `json:"columns,omitempty"`
	Rows         [][]any   `json:"rows,omitempty"`
	AffectedRows int       `json:"affected_rows,omitempty"`
	LastInsertID int64     `json:"last_insert_id,omitempty"`
	Error        string    `json:"error,omitempty"` // 执行错误时的用户向描述
}

// Executor 执行某数据源上的 DSL（如 SQL）。
type Executor interface {
	Execute(ctx context.Context, datasourceID, dsl string, opts ExecuteOptions) (*Result, error)
}
