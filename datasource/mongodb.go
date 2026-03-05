package datasource

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDatabaseProvider 由 MongoDB 数据源实现，供 metadata 与 executor 使用。
type MongoDatabaseProvider interface {
	MongoDatabase() *mongo.Database
}

// mongoDataSource 实现 DataSource 与 MongoDatabaseProvider。
type mongoDataSource struct {
	id string
	db *mongo.Database
}

func (m *mongoDataSource) ID() string { return m.id }

func (m *mongoDataSource) Ping(ctx context.Context) error {
	return m.db.Client().Ping(ctx, nil)
}

func (m *mongoDataSource) Close() error {
	return m.db.Client().Disconnect(context.Background())
}

// MongoDatabase 返回底层 *mongo.Database，供 executor 与 metadata 使用。
func (m *mongoDataSource) MongoDatabase() *mongo.Database {
	return m.db
}

// NewMongoDataSource 根据 Config 创建 MongoDB 数据源。
// DSN 形如 mongodb://user:pass@host:port/dbname；若 DSN 为空则使用 Host/Port/User/Password/DBName 组装。
func NewMongoDataSource(cfg Config) (*mongoDataSource, error) {
	if cfg.ID == "" {
		return nil, fmt.Errorf("mongodb datasource: missing id")
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("mongodb datasource: missing dbname for id=%s", cfg.ID)
	}

	uri := cfg.DSN
	if uri == "" {
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 27017
		}
		if cfg.User != "" {
			if cfg.Password != "" {
				uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", cfg.User, cfg.Password, host, port, cfg.DBName)
			} else {
				uri = fmt.Sprintf("mongodb://%s@%s:%d/%s", cfg.User, host, port, cfg.DBName)
			}
		} else {
			uri = fmt.Sprintf("mongodb://%s:%d/%s", host, port, cfg.DBName)
		}
	}

	clientOpts := options.Client().ApplyURI(uri)
	if cfg.MaxOpenConns > 0 {
		// 将 MaxOpenConns 粗略映射为连接池大小
		clientOpts.SetMaxPoolSize(uint64(cfg.MaxOpenConns))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongodb datasource: connect: %w", err)
	}

	db := client.Database(cfg.DBName)
	return &mongoDataSource{id: cfg.ID, db: db}, nil
}

// RegisterMongoDB 在 Registry 上注册 "mongodb" 类型的数据源工厂。
func RegisterMongoDB(r *Registry) {
	if r == nil {
		return
	}
	r.RegisterType("mongodb", func(cfg Config) (DataSource, error) {
		return NewMongoDataSource(cfg)
	})
}
