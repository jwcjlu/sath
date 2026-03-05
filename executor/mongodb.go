package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sath/datasource"
	"go.
	"go.mongodb.org/mongo-driver/bson"
	"github.com/sath/datasource"
)

// MongoExecutor 基于 datasource.Registry 的 MongoDB 执行器，目前仅支持只读 find 查询。
type MongoExecutor struct {
	Registry *datasource.Registry
}

// NewMongoExecutor 创建依赖给定 Registry 的 Mongo 执行器。
func NewMongoExecutor(reg *datasource.Registry) *MongoExecutor {
	return &MongoExecutor{Registry: reg}
}

// mongoQuery 描述 execute_read 传入的 Mongo 查询 DSL。
// 期望为 JSON 字符串，例如：
// {"collection":"users","filter":{"status":"active"},"limit":50}
	Collection string         `json:"collection"`
	Filter     map[string]any `json:"filter"`
	Limit      int64          `json:"limit"`
	Projection map[string]any `json:"projection"`
	Sort       map[string]int `json:"sort"`
	Sort       map[string]int         `json:"sort"`
}

// Execute 实现 Executor。仅支持只读 find 查询；写操作由上层 execute_write 工具与其他执行器处理。
func (e *MongoExecutor) Execute(ctx context.Context, datasourceID string, dsl string, opts ExecuteOptions) (*Result, error) {
	ds, err := e.Registry.Get(datasourceID)
	if err != nil {
		return nil, err
	}
	mp, ok := ds.(datasource.MongoDatabaseProvider)
	if !ok {
		return nil, ErrUnsupportedDataSource
	}
	db := mp.MongoDatabase()

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
		defer cancel()
	}

	var q mongoQuery
	if err := json.Unmarshal([]byte(dsl), &q); err != nil {
		return nil, fmt.Errorf("executor: parse mongo dsl as JSON: %w", err)
	}
	if q.Collection == "" {
		return nil, fmt.Errorf("executor: mongo dsl missing collection")
	}

	coll := db.Collection(q.Collection)

	filter := any(bson.D{})
	if q.Filter != nil {
		filter = q.Filter
	}

	findOpts := options.Find()
	if q.Limit > 0 {
		findOpts.SetLimit(q.Limit)
	}
	if opts.MaxRows > 0 && (q.Limit == 0 || q.Limit > int64(opts.MaxRows)) {
		findOpts.SetLimit(int64(opts.MaxRows))
	}
	if len(q.Projection) > 0 {
		findOpts.SetProjection(q.Projection)
	}
	if len(q.Sort) > 0 {
		findOpts.SetSort(q.Sort)
	}

	cursor, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("executor: mongo find: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("executor: mongo iterate: %w", err)
	}
	if len(docs) == 0 {
		return &Result{}, nil
	}

	// 简单从首行收集列名
	first := docs[0]
	columns := make([]string, 0, len(first))
	for k := range first {
		columns = append(columns, k)
	}

	rows := make([][]any, 0, len(docs))
	for _, doc := range docs {
		row := make([]any, len(columns))
		for i, col := range columns {
			row[i] = doc[col]
		}
		rows = append(rows, row)
	}

	return &Result{Columns: columns, Rows: rows}, nil
}
