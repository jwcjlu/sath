package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mysqlDataSource 实现 DataSource，并暴露底层 *sql.DB 供执行器与元数据使用。
type mysqlDataSource struct {
	id string
	db *sql.DB
}

func (m *mysqlDataSource) ID() string { return m.id }

func (m *mysqlDataSource) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

func (m *mysqlDataSource) Close() error {
	return m.db.Close()
}

// DB 返回底层 *sql.DB，供 executor 与 metadata 使用。
func (m *mysqlDataSource) DB() *sql.DB {
	return m.db
}

// NewMySQLDataSource 根据 Config 打开 MySQL 连接并配置连接池。
func NewMySQLDataSource(cfg Config) (*mysqlDataSource, error) {
	if cfg.ID == "" {
		return nil, fmt.Errorf("mysql datasource: missing id")
	}
	dsn := cfg.DSN
	if dsn == "" {
		if cfg.Host == "" || cfg.User == "" || cfg.DBName == "" {
			return nil, fmt.Errorf("mysql datasource: incomplete config for id=%s", cfg.ID)
		}
		port := cfg.Port
		if port == 0 {
			port = 3306
		}
		// 参考 go-sql-driver/mysql DSN 格式：user:password@tcp(host:port)/dbname?parseTime=true
		if cfg.Password != "" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4,utf8", cfg.User, cfg.Password, cfg.Host, port, cfg.DBName)
		} else {
			dsn = fmt.Sprintf("%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4,utf8", cfg.User, cfg.Host, port, cfg.DBName)
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql datasource: open: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	return &mysqlDataSource{
		id: cfg.ID,
		db: db,
	}, nil
}

// RegisterMySQL 在 Registry 上注册 "mysql" 类型的数据源工厂。
func RegisterMySQL(r *Registry) {
	if r == nil {
		return
	}
	r.RegisterType("mysql", func(cfg Config) (DataSource, error) {
		return NewMySQLDataSource(cfg)
	})
}
