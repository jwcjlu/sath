package executor

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sath/datasource"
)

// dbProvider 由可执行 SQL 的数据源实现，用于暴露 *sql.DB。
type dbProvider interface {
	DB() *sql.DB
}

// MySQLExecutor 基于 datasource.Registry 的 MySQL 执行器，支持超时、MaxRows、只读拦截。
type MySQLExecutor struct {
	Registry *datasource.Registry
}

// NewMySQLExecutor 创建依赖给定 Registry 的 MySQL 执行器。
func NewMySQLExecutor(reg *datasource.Registry) *MySQLExecutor {
	return &MySQLExecutor{Registry: reg}
}

// isWriteDSL 根据 SQL 前缀判断是否为写操作（INSERT/UPDATE/DELETE/REPLACE/CREATE/DROP/ALTER/TRUNCATE 等）。
func isWriteDSL(dsl string) bool {
	s := strings.TrimSpace(dsl)
	if s == "" {
		return false
	}
	// 去除注释与多余空白后取首词
	upper := strings.ToUpper(s)
	for _, prefix := range []string{
		"INSERT", "UPDATE", "DELETE", "REPLACE",
		"CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME",
	} {
		if upper == prefix || strings.HasPrefix(upper, prefix+" ") || strings.HasPrefix(upper, prefix+"\t") || strings.HasPrefix(upper, prefix+"\n") {
			return true
		}
	}
	return false
}

// Execute 实现 Executor。只读模式下写操作返回 ErrReadOnlyViolation；查询支持超时与 MaxRows 截断。
func (e *MySQLExecutor) Execute(ctx context.Context, datasourceID string, dsl string, opts ExecuteOptions) (*Result, error) {
	log.Printf("exe sql: %s", dsl)
	ds, err := e.Registry.Get(datasourceID)
	if err != nil {
		return nil, err
	}
	provider, ok := ds.(dbProvider)
	if !ok {
		return nil, ErrUnsupportedDataSource
	}
	db := provider.DB()

	if opts.ReadOnly && isWriteDSL(dsl) {
		return nil, ErrReadOnlyViolation
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	if isWriteDSL(dsl) {
		return e.execWrite(ctx, db, dsl)
	}
	return e.execQuery(ctx, db, dsl, opts.MaxRows)
}

func (e *MySQLExecutor) execWrite(ctx context.Context, db *sql.DB, dsl string) (*Result, error) {
	res, err := db.ExecContext(ctx, dsl)
	if err != nil {
		return nil, fmt.Errorf("executor: exec write: %w", err)
	}
	affected, _ := res.RowsAffected()
	return &Result{AffectedRows: affected}, nil
}

// isMySQLSchemaRelated 判断 MySQL 驱动返回的错误是否与表/列结构相关（如未知列 1054）。
func isMySQLSchemaRelated(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Unknown column") || strings.Contains(s, "1054") || strings.Contains(s, "42S22")
}

// wrapMaybeSchemaRelated 若 err 为结构相关错误则包装为 SchemaRelatedError，否则按 format 包装。
func wrapMaybeSchemaRelated(err error, format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	if isMySQLSchemaRelated(err) {
		return &SchemaRelatedError{Err: wrapped}
	}
	return wrapped
}

func (e *MySQLExecutor) execQuery(ctx context.Context, db *sql.DB, dsl string, maxRows int) (*Result, error) {
	rows, err := db.QueryContext(ctx, dsl)
	if err != nil {
		return nil, wrapMaybeSchemaRelated(err, "executor: query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, wrapMaybeSchemaRelated(err, "executor: columns: %w", err)
	}
	out := &Result{Columns: cols}

	for rows.Next() {
		if maxRows > 0 && len(out.Rows) >= maxRows {
			break
		}
		dest := make([]any, len(cols))
		destPtr := make([]interface{}, len(cols))
		for i := range dest {
			destPtr[i] = &dest[i]
		}
		if err := rows.Scan(destPtr...); err != nil {
			return nil, wrapMaybeSchemaRelated(err, "executor: scan row: %w", err)
		}
		row := make([]any, len(dest))
		copy(row, dest)
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapMaybeSchemaRelated(err, "executor: iterate rows: %w", err)
	}
	return out, nil
}
