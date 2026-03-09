package executor

import (
	"context"
	"database/sql"

	"github.com/sath/datasource"
)

// sqlDBProvider 用于类型断言：能提供 *sql.DB 的数据源走 MySQL 执行器。
type sqlDBProvider interface {
	DB() *sql.DB
}

// MultiExecutor 按数据源类型分发到 MySQL、Elasticsearch 或 MongoDB 执行器。
// 用于同时支持多种数据源时，由 DataQuery Handler 使用。
type MultiExecutor struct {
	Registry      *datasource.Registry
	MySQL         *MySQLExecutor
	Elasticsearch *ESExecutor
	Mongo         *MongoExecutor
}

// NewMultiExecutor 创建多数据源执行器。各具体执行器可为 nil，对应类型将返回 ErrUnsupportedDataSource。
func NewMultiExecutor(reg *datasource.Registry, mysql *MySQLExecutor, es *ESExecutor, mongo *MongoExecutor) *MultiExecutor {
	return &MultiExecutor{Registry: reg, MySQL: mysql, Elasticsearch: es, Mongo: mongo}
}

// Execute 实现 Executor。根据 datasourceID 对应数据源类型分发到 MySQL 或 ES。
func (e *MultiExecutor) Execute(ctx context.Context, datasourceID string, dsl string, opts ExecuteOptions) (*Result, error) {
	ds, err := e.Registry.Get(datasourceID)
	if err != nil {
		return nil, err
	}
	if e.MySQL != nil {
		if _, ok := ds.(sqlDBProvider); ok {
			return e.MySQL.Execute(ctx, datasourceID, dsl, opts)
		}
	}
	if e.Elasticsearch != nil {
		if _, ok := ds.(datasource.ESClientProvider); ok {
			return e.Elasticsearch.Execute(ctx, datasourceID, dsl, opts)
		}
	}
	if e.Mongo != nil {
		if _, ok := ds.(datasource.MongoDatabaseProvider); ok {
			return e.Mongo.Execute(ctx, datasourceID, dsl, opts)
		}
	}
	return nil, ErrUnsupportedDataSource
}
