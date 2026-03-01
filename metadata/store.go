package metadata

import (
	"context"
	"errors"
)

// ErrUnsupportedDataSource 当数据源无法提供元数据所需能力时返回
var ErrUnsupportedDataSource = errors.New("metadata: unsupported datasource for schema fetch")

// Store 抽象元数据存取接口，供上层使用
type Store interface {
	// GetSchema 获取当前缓存的 Schema，如果不存在返回 nil, nil
	GetSchema(ctx context.Context) (*Schema, error)
	// Refresh 从外部数据源刷新 Schema
	Refresh(ctx context.Context) (*Schema, error)
	// GetTable 按名称查找单表
	GetTable(ctx context.Context, name string) (*Table, error)
}
