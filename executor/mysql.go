package executor

import (
	"context"
	"database/sql"
	"strings"

	"github.com/sath/datasource"
	"github.com/sath/errs"
)

// 写操作前缀（大小写不敏感）
var writePrefixes = []string{"insert ", "update ", "delete ", "replace ", "create ", "alter ", "drop "}

func isWriteDSL(dsl string) bool {
	upper := strings.TrimSpace(strings.ToLower(dsl))
	for _, p := range writePrefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

// MySQLExecutor 使用 datasource.Registry 获取 MySQL 数据源并执行 SQL。
type MySQLExecutor struct {
	Registry *datasource.Registry
}

// Execute 实现 Executor。
func (e *MySQLExecutor) Execute(ctx context.Context, datasourceID, dsl string, opts ExecuteOptions) (*Result, error) {
	if e.Registry == nil {
		return &Result{Error: "executor: no registry"}, errs.ErrInternal
	}
	ds, err := e.Registry.Get(datasourceID)
	if err != nil {
		return &Result{Error: err.Error()}, err
	}
	mysqlDS, ok := ds.(*datasource.MySQLDataSource)
	if !ok {
		return &Result{Error: "executor: datasource is not MySQL"}, errs.ErrBadRequest
	}
	db := mysqlDS.DB()
	if opts.ReadOnly && isWriteDSL(dsl) {
		return &Result{Error: "write not allowed in read-only mode"}, errs.ErrBadRequest
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	dsl = strings.TrimSpace(dsl)
	params := opts.Params
	if len(params) == 0 {
		params = nil
	}
	if isWriteDSL(dsl) {
		return e.execWrite(ctx, db, dsl, params)
	}
	return e.execQuery(ctx, db, dsl, params, opts.MaxRows)
}

func (e *MySQLExecutor) execQuery(ctx context.Context, db *sql.DB, dsl string, params []any, maxRows int) (*Result, error) {
	var rows *sql.Rows
	var err error
	if len(params) > 0 {
		rows, err = db.QueryContext(ctx, dsl, params...)
	} else {
		rows, err = db.QueryContext(ctx, dsl)
	}
	if err != nil {
		return &Result{Error: err.Error()}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return &Result{Error: err.Error()}, err
	}
	res := &Result{Columns: cols}
	for rows.Next() {
		if maxRows > 0 && len(res.Rows) >= maxRows {
			break
		}
		dest := make([]any, len(cols))
		destPtr := make([]any, len(cols))
		for i := range dest {
			destPtr[i] = &dest[i]
		}
		if err := rows.Scan(destPtr...); err != nil {
			return &Result{Error: err.Error()}, err
		}
		row := make([]any, len(cols))
		for i, v := range dest {
			row[i] = v
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return &Result{Error: err.Error()}, err
	}
	return res, nil
}

func (e *MySQLExecutor) execWrite(ctx context.Context, db *sql.DB, dsl string, params []any) (*Result, error) {
	var res sql.Result
	var err error
	if len(params) > 0 {
		res, err = db.ExecContext(ctx, dsl, params...)
	} else {
		res, err = db.ExecContext(ctx, dsl)
	}
	if err != nil {
		return &Result{Error: err.Error()}, err
	}
	affected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return &Result{AffectedRows: int(affected), LastInsertID: lastID}, nil
}
