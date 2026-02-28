package dsl

import (
	"context"
	"fmt"
	"strings"

	"github.com/sath/intent"
	"github.com/sath/metadata"
)

// MySQLGenerator 生成 MySQL 单表 SELECT/INSERT/UPDATE/DELETE，表名列名以 meta 校验并转义。
type MySQLGenerator struct{}

// Generate 实现 Generator。
func (g *MySQLGenerator) Generate(ctx context.Context, input *intent.ParsedInput, meta *metadata.Schema) (dsl string, params []any, err error) {
	if input == nil {
		return "", nil, fmt.Errorf("dsl: nil input")
	}
	table := strings.TrimSpace(input.Entities.Table)
	if table == "" {
		return "", nil, fmt.Errorf("dsl: missing table")
	}
	tbl := findTable(meta, table)
	if tbl == nil {
		return "", nil, fmt.Errorf("dsl: table %q not in metadata", table)
	}
	switch input.Intent {
	case intent.IntentQuery:
		return g.buildSelect(input, tbl, meta, &params)
	case intent.IntentInsert:
		return g.buildInsert(input, tbl, &params)
	case intent.IntentUpdate:
		return g.buildUpdate(input, tbl, &params)
	case intent.IntentDelete:
		return g.buildDelete(input, tbl, &params)
	case intent.IntentMetadata:
		return "", nil, fmt.Errorf("dsl: metadata intent does not produce SQL")
	default:
		return "", nil, fmt.Errorf("dsl: unsupported intent %q", input.Intent)
	}
}

func findTable(meta *metadata.Schema, name string) *metadata.Table {
	if meta == nil {
		return nil
	}
	for _, t := range meta.Tables {
		if t != nil && strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return nil
}

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func ensureColumns(tbl *metadata.Table, cols []string) ([]string, error) {
	if len(cols) == 0 {
		names := make([]string, 0, len(tbl.Columns))
		for _, c := range tbl.Columns {
			if c != nil {
				names = append(names, c.Name)
			}
		}
		return names, nil
	}
	allowed := make(map[string]bool)
	for _, c := range tbl.Columns {
		if c != nil {
			allowed[c.Name] = true
		}
	}
	for _, c := range cols {
		if !allowed[c] {
			return nil, fmt.Errorf("dsl: column %q not in table %q", c, tbl.Name)
		}
	}
	return cols, nil
}

func (g *MySQLGenerator) buildSelect(input *intent.ParsedInput, tbl *metadata.Table, meta *metadata.Schema, params *[]any) (string, []any, error) {
	cols, err := ensureColumns(tbl, input.Entities.Columns)
	if err != nil {
		return "", nil, err
	}
	selCols := "*"
	if len(cols) > 0 {
		parts := make([]string, len(cols))
		for i, c := range cols {
			parts[i] = quoteIdent(c)
		}
		selCols = strings.Join(parts, ", ")
	}
	if len(input.Entities.Aggregations) > 0 {
		parts := make([]string, 0, len(input.Entities.Aggregations))
		for _, a := range input.Entities.Aggregations {
			parts = append(parts, fmt.Sprintf("%s(%s)", strings.ToUpper(a.Func), quoteIdent(a.Field)))
		}
		selCols = strings.Join(parts, ", ")
	}
	sql := "SELECT " + selCols + " FROM " + quoteIdent(tbl.Name)
	var p []any
	if len(input.Entities.Conditions) > 0 {
		where, wp := buildWhere(input.Entities.Conditions)
		sql += " WHERE " + where
		p = append(p, wp...)
	}
	if len(input.Entities.OrderBy) > 0 {
		orderParts := make([]string, 0, len(input.Entities.OrderBy))
		for _, o := range input.Entities.OrderBy {
			dir := "ASC"
			if o.Desc {
				dir = "DESC"
			}
			orderParts = append(orderParts, quoteIdent(o.Field)+" "+dir)
		}
		sql += " ORDER BY " + strings.Join(orderParts, ", ")
	}
	if input.Entities.Limit > 0 {
		sql += " LIMIT ?"
		p = append(p, input.Entities.Limit)
	}
	if input.Entities.Offset > 0 {
		sql += " OFFSET ?"
		p = append(p, input.Entities.Offset)
	}
	*params = p
	return sql, p, nil
}

func buildWhere(conds []intent.Condition) (clause string, params []any) {
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		switch strings.ToLower(c.Op) {
		case "eq":
			parts = append(parts, "`"+c.Field+"` = ?")
			params = append(params, c.Value)
		case "ne":
			parts = append(parts, "`"+c.Field+"` != ?")
			params = append(params, c.Value)
		case "gt", "gte", "lt", "lte":
			parts = append(parts, "`"+c.Field+"` "+strings.ToUpper(c.Op)+" ?")
			params = append(params, c.Value)
		case "like":
			parts = append(parts, "`"+c.Field+"` LIKE ?")
			params = append(params, c.Value)
		default:
			parts = append(parts, "`"+c.Field+"` = ?")
			params = append(params, c.Value)
		}
	}
	return strings.Join(parts, " AND "), params
}

func (g *MySQLGenerator) buildInsert(input *intent.ParsedInput, tbl *metadata.Table, params *[]any) (string, []any, error) {
	if len(input.Entities.Values) == 0 {
		return "", nil, fmt.Errorf("dsl: insert requires values")
	}
	cols := make([]string, 0, len(input.Entities.Values))
	placeholders := make([]string, 0, len(input.Entities.Values))
	var p []any
	for k, v := range input.Entities.Values {
		cols = append(cols, quoteIdent(k))
		placeholders = append(placeholders, "?")
		p = append(p, v)
	}
	sql := "INSERT INTO " + quoteIdent(tbl.Name) + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	*params = p
	return sql, p, nil
}

func (g *MySQLGenerator) buildUpdate(input *intent.ParsedInput, tbl *metadata.Table, params *[]any) (string, []any, error) {
	if len(input.Entities.SetClause) == 0 {
		return "", nil, fmt.Errorf("dsl: update requires set_clause")
	}
	setParts := make([]string, 0, len(input.Entities.SetClause))
	var p []any
	for k, v := range input.Entities.SetClause {
		setParts = append(setParts, quoteIdent(k)+" = ?")
		p = append(p, v)
	}
	sql := "UPDATE " + quoteIdent(tbl.Name) + " SET " + strings.Join(setParts, ", ")
	if len(input.Entities.Conditions) > 0 {
		where, wp := buildWhere(input.Entities.Conditions)
		sql += " WHERE " + where
		p = append(p, wp...)
	}
	*params = p
	return sql, p, nil
}

func (g *MySQLGenerator) buildDelete(input *intent.ParsedInput, tbl *metadata.Table, params *[]any) (string, []any, error) {
	sql := "DELETE FROM " + quoteIdent(tbl.Name)
	var p []any
	if len(input.Entities.Conditions) > 0 {
		where, wp := buildWhere(input.Entities.Conditions)
		sql += " WHERE " + where
		p = wp
	}
	*params = p
	return sql, p, nil
}
