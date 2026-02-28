package metadata

import (
	"context"
	"database/sql"

	"github.com/sath/datasource"
)

// FetchSchema 从 MySQL 的 information_schema 拉取当前库的表与列元数据。
// db 应为已连接并 USE 了目标库，或通过查询 information_schema 指定 TABLE_SCHEMA。
func FetchSchema(ctx context.Context, db *sql.DB) (*Schema, error) {
	// 使用当前连接默认库；若需多库可扩展参数 schemaName。
	var schemaName string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schemaName); err != nil {
		return nil, err
	}
	if schemaName == "" {
		schemaName = "unknown"
	}
	// 表列表
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`,
		schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []*Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, &Table{Name: name, Columns: nil})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 每表的列
	for _, t := range tables {
		cols, err := fetchColumns(ctx, db, schemaName, t.Name)
		if err != nil {
			return nil, err
		}
		t.Columns = cols
	}
	return &Schema{Name: schemaName, Tables: tables}, nil
}

func fetchColumns(ctx context.Context, db *sql.DB, schema, table string) ([]*Column, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`,
		schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []*Column
	for rows.Next() {
		var name, typ, nullable, key string
		if err := rows.Scan(&name, &typ, &nullable, &key); err != nil {
			return nil, err
		}
		cols = append(cols, &Column{
			Name:       name,
			Type:       typ,
			Nullable:   nullable == "YES",
			PrimaryKey: key == "PRI",
		})
	}
	return cols, rows.Err()
}

// RefreshFromRegistry 从 datasource.Registry 取得数据源并刷新 Store 中该数据源的元数据。
// 仅支持 *datasource.MySQLDataSource，其它类型返回 ErrUnsupportedDataSource。
func RefreshFromRegistry(ctx context.Context, registry *datasource.Registry, store Store, datasourceID string) error {
	ds, err := registry.Get(datasourceID)
	if err != nil {
		return err
	}
	mysqlDS, ok := ds.(*datasource.MySQLDataSource)
	if !ok {
		return ErrUnsupportedDataSource
	}
	db := mysqlDS.DB()
	return store.Refresh(ctx, datasourceID, func() (*Schema, error) {
		return FetchSchema(ctx, db)
	})
}
