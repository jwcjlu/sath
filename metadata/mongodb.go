package metadata

import (
	"context"
	"fmt"

	"github.com/sath/datasource"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// FetchSchemaMongo 从 MongoDB 拉取当前库的元数据：集合列表及每个集合示例文档的顶层字段。
func FetchSchemaMongo(ctx context.Context, db *mongo.Database) (*Schema, error) {
	if db == nil {
		return nil, fmt.Errorf("metadata: mongodb database is nil")
	}

	schema := &Schema{
		Name: db.Name(),
	}

	collections, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("metadata: list collections: %w", err)
	}

	for _, name := range collections {
		tbl := Table{Name: name}

		coll := db.Collection(name)
		var doc bson.M
		err := coll.FindOne(ctx, bson.D{}).Decode(&doc)
		if err == nil && len(doc) > 0 {
			for field, val := range doc {
				tbl.Columns = append(tbl.Columns, Column{
					Name:       field,
					Type:       fmt.Sprintf("%T", val),
					IsNullable: true,
				})
			}
		}

		schema.Tables = append(schema.Tables, tbl)
	}

	return schema, nil
}

// RefreshFromRegistryMongo 扩展原有 RefreshFromRegistry，使其支持 MongoDB 数据源。
func RefreshFromRegistryMongo(ctx context.Context, reg *datasource.Registry, store *InMemoryStore, datasourceID string) (*Schema, error) {
	ds, err := reg.Get(datasourceID)
	if err != nil {
		return nil, err
	}

	if mp, ok := ds.(datasource.MongoDatabaseProvider); ok {
		store.fetch = func(ctx context.Context) (*Schema, error) {
			return FetchSchemaMongo(ctx, mp.MongoDatabase())
		}
		return store.Refresh(ctx)
	}
	return nil, ErrUnsupportedDataSource
}
