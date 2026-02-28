package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultPingTimeout = 2 * time.Second
	defaultPort        = 3306
	pingRetries        = 2
)

// MySQLDataSource 实现 DataSource，用于 MySQL；并暴露 DB() 供元数据拉取等使用。
type MySQLDataSource struct {
	id string
	db *sql.DB
}

// NewMySQLDataSourceFromDB 用于测试或已有 *sql.DB 时构造 MySQLDataSource，不管理 db 生命周期。
func NewMySQLDataSourceFromDB(id string, db *sql.DB) *MySQLDataSource {
	return &MySQLDataSource{id: id, db: db}
}

// ID 实现 DataSource。
func (m *MySQLDataSource) ID() string { return m.id }

// Ping 实现 Dataource，带超时与简单重试。
func (m *MySQLDataSource) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	var lastErr error
	for i := 0; i < pingRetries; i++ {
		lastErr = m.db.PingContext(ctx)
		if lastErr == nil {
			return nil
		}
		if i < pingRetries-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return lastErr
}

// Close 实现 DataSource。
func (m *MySQLDataSource) Close() error {
	return m.db.Close()
}

// DB 返回底层 *sql.DB，供元数据拉取等使用。
func (m *MySQLDataSource) DB() *sql.DB {
	return m.db
}

// buildDSN 从 Config 构建 DSN；若 Config.DSN 非空则直接使用。
func buildDSN(cfg Config) string {
	if cfg.DSN != "" {
		return cfg.DSN
	}
	port := cfg.Port
	if port <= 0 {
		port = defaultPort
	}
	// 简单拼接，敏感信息建议通过 DSN 环境变量注入
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.User, cfg.Password, cfg.Host, port, cfg.DBName)
}

// openMySQL 根据 cfg 打开 MySQL 连接并设置连接池。
func openMySQL(cfg Config) (*sql.DB, error) {
	dsn := buildDSN(cfg)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 10
	}
	db.SetMaxOpenConns(maxOpen)
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
	return db, nil
}

// RegisterMySQL 在 r 上注册 "mysql" 类型工厂。
func RegisterMySQL(r *Registry) {
	r.RegisterType("mysql", func(cfg Config) (DataSource, error) {
		db, err := openMySQL(cfg)
		if err != nil {
			return nil, err
		}
		return &MySQLDataSource{id: cfg.ID, db: db}, nil
	})
}
