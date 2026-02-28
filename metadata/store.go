package metadata

import (
	"context"
	"errors"
)

var ErrUnsupportedDataSource = errors.New("metadata: unsupported datasource type for schema fetch")

// FetchFunc 由调用方提供，用于在 Refresh 时拉取某数据源的 Schema。
type FetchFunc func() (*Schema, error)

// Store 定义元数据存储与刷新接口。
type Store interface {
	GetSchema(ctx context.Context, datasourceID string) (*Schema, error)
	// Refresh 使用 fetch 拉取 Schema 并写入存储；fetch 由调用方根据数据源类型构造（见 T-04）。
	Refresh(ctx context.Context, datasourceID string, fetch FetchFunc) error
	// GetTable 可选：按表名返回单表元数据。
	GetTable(ctx context.Context, datasourceID, tableName string) (*Table, error)
}
